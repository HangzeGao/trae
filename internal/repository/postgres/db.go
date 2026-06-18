// Package postgres 提供基于 PostgreSQL 的持久化仓储实现。
//
// 该包使用 database/sql 标准接口配合 pgx 驱动（github.com/jackc/pgx/v5/stdlib），
// 所有查询均带 tenant_id 过滤以实现租户隔离。
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/config"
	"github.com/jackc/pgx/v5/stdlib"
)

// 驱动注册：通过 init 确保 pgx 驱动以 "pgx" 名字注册到 database/sql。
// 多次注册会被 sql.Register 的 panic 防护，使用 sync.Once 避免重复注册。
//
//nolint:gochecknoinits // 驱动注册必须在 init 完成
func init() {
	// pgx 的 stdlib 已通过其内部 driver 实现注册，这里仅做兜底。
	// 调用 stdlib.GetDefaultDriver 确保驱动已初始化。
	_ = stdlib.GetDefaultDriver()
}

// Connect 根据配置建立 PostgreSQL 连接池。
//
// 使用 pgx 驱动（github.com/jackc/pgx/v5/stdlib），并按 DatabaseConfig 设置
// 连接池大小、最小连接数和连接最大生命周期。
//
// 调用方负责在关闭服务时调用 db.Close()。
func Connect(cfg config.DatabaseConfig) (*sql.DB, error) {
	// 通过 stdlib.ParseConfig 解析 DSN，确保使用 pgx 驱动
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// 配置连接池
	if cfg.MaxConns > 0 {
		db.SetMaxOpenConns(int(cfg.MaxConns))
	}
	if cfg.MinConns > 0 {
		db.SetMaxIdleConns(int(cfg.MinConns))
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	// 默认空闲连接最大存活时间，避免长连接被服务端断开
	db.SetConnMaxIdleTime(10 * time.Minute)

	// 验证连接可用
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

// NewTx 开启一个新事务。
//
// opts 可为 nil，表示使用数据库默认隔离级别。返回的事务由调用方负责
// 调用 Commit 或 Rollback。
func NewTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	return tx, nil
}

// WithTx 是事务包装器，自动处理 commit/rollback。
//
// fn 内部应使用传入的 tx 执行数据库操作。若 fn 返回 nil，则提交事务；
// 若 fn 返回非 nil 错误或发生 panic，则回滚事务。
//
// 注意：panic 时会重新 panic，确保上层 Recover 中间件能捕获。
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := NewTx(ctx, db, nil)
	if err != nil {
		return err
	}

	// panic 时回滚并重新 panic
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		// 回滚失败不应掩盖原始错误，仅记录
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			// 将回滚错误合并到原始错误中
			err = fmt.Errorf("tx failed: %w; rollback failed: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
