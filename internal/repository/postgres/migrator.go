package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/HangzeGao/trae/key-vault/migrations"
)

// Migrator 是数据库迁移执行器。
//
// 它负责按文件名顺序执行 migrations 目录下嵌入的 SQL 文件，
// 并通过 schema_migrations 表记录已执行的迁移，支持幂等执行。
type Migrator struct {
	db *sql.DB
}

// New 创建一个 Migrator 实例。
func New(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// schemaMigrationsTable 用于记录已执行迁移的表名。
const schemaMigrationsTable = "schema_migrations"

// ensureMigrationsTable 创建 schema_migrations 表（若不存在）。
//
// 表结构：
//
//	CREATE TABLE schema_migrations (
//	    version VARCHAR(256) PRIMARY KEY,  -- 迁移文件名（不含扩展名）
//	    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
//	);
func (m *Migrator) ensureMigrationsTable(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS ` + schemaMigrationsTable + ` (
		version VARCHAR(256) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`
	_, err := m.db.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("create %s table: %w", schemaMigrationsTable, err)
	}
	return nil
}

// listAppliedMigrations 返回已执行的迁移版本列表。
func (m *Migrator) listAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT version FROM `+schemaMigrationsTable)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan migration version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

// listEmbeddedMigrations 返回嵌入的迁移文件名列表（按名称排序）。
//
// 返回的文件名包含 .sql 扩展名，version 为去掉扩展名后的部分。
func listEmbeddedMigrations() ([]string, error) {
	entries, err := fs.ReadDir(migrations.SQLFiles, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)
	return files, nil
}

// Run 执行所有未应用的迁移。
//
// 该方法是幂等的：已记录在 schema_migrations 表中的迁移不会重复执行。
// 每个迁移文件在独立事务中执行，失败时回滚该迁移但不影响已应用的迁移。
func (m *Migrator) Run(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return err
	}

	applied, err := m.listAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	files, err := listEmbeddedMigrations()
	if err != nil {
		return err
	}

	for _, file := range files {
		version := strings.TrimSuffix(file, ".sql")
		if applied[version] {
			continue
		}

		if err := m.applyMigration(ctx, version, file); err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
	}
	return nil
}

// applyMigration 在独立事务中执行单个迁移文件并记录到 schema_migrations。
func (m *Migrator) applyMigration(ctx context.Context, version, file string) error {
	content, err := migrations.SQLFiles.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read migration file %s: %w", file, err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for migration %s: %w", version, err)
	}
	// 失败路径回滚；成功路径由 Commit 提交
	defer func() { _ = tx.Rollback() }()

	// 执行迁移 SQL
	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("exec migration sql: %w", err)
	}

	// 记录迁移版本
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO `+schemaMigrationsTable+` (version) VALUES ($1)`,
		version); err != nil {
		return fmt.Errorf("record migration version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}
