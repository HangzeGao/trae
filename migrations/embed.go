// Package migrations 内嵌数据库迁移 SQL 文件。
//
// 该包通过 go:embed 将 migrations 目录下的所有 .sql 文件嵌入二进制，
// 供 internal/repository/postgres.Migrator 在运行时执行。
package migrations

import "embed"

// SQLFiles 嵌入 migrations 目录下的所有文件。
//
// 使用 //go:embed 指令将当前包目录（即项目根的 migrations/）下的
// 所有文件嵌入到编译产物中，便于迁移执行器读取。
//
//go:embed *.sql
var SQLFiles embed.FS
