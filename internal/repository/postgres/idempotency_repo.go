package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// IdempotencyRecord 是幂等键记录，对应 idempotency_keys 表。
//
// 用于实现请求幂等性：客户端携带幂等键发起请求时，服务端先查幂等表，
// 若命中且未过期则直接返回缓存的响应；否则执行请求并写入记录。
type IdempotencyRecord struct {
	Key            string    // 幂等键（客户端提供）
	TenantID       string    // 租户 ID
	RequestHash    string    // 请求体哈希，用于校验请求一致性
	ResponseStatus *int      // 缓存的响应状态码（首次执行后填充）
	ResponseBody   []byte     // 缓存的响应体（首次执行后填充）
	CreatedAt      time.Time  // 创建时间
	ExpiresAt      time.Time  // 过期时间
}

// IdempotencyRepository 是幂等键的 PostgreSQL 仓储实现。
//
// Get 使用 *sql.DB 读取（无需事务），Save 使用 *sql.Tx 写入
// 以便与业务操作组成事务。
type IdempotencyRepository struct{}

// NewIdempotencyRepository 创建一个 IdempotencyRepository 实例。
func NewIdempotencyRepository() *IdempotencyRepository {
	return &IdempotencyRepository{}
}

// Get 按幂等键查询记录。
//
// 仅返回未过期的记录。若记录不存在或已过期返回 sql.ErrNoRows。
func (r *IdempotencyRepository) Get(ctx context.Context, db *sql.DB, key string) (*IdempotencyRecord, error) {
	const q = `SELECT key, tenant_id, request_hash, response_status, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE key = $1 AND expires_at > $2`

	rec := &IdempotencyRecord{}
	var (
		responseStatus sql.NullInt64
		responseBody   []byte
	)
	err := db.QueryRowContext(ctx, q, key, time.Now().UTC()).Scan(
		&rec.Key,
		&rec.TenantID,
		&rec.RequestHash,
		&responseStatus,
		&responseBody,
		&rec.CreatedAt,
		&rec.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("idempotency key %s not found or expired: %w", key, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get idempotency record: %w", err)
	}
	if responseStatus.Valid {
		s := int(responseStatus.Int64)
		rec.ResponseStatus = &s
	}
	rec.ResponseBody = responseBody
	return rec, nil
}

// Save 在事务中保存幂等键记录。
//
// 若键已存在（主键冲突）返回错误，调用方应通过 Get 先检查。
// created_at 若为零值则由数据库默认值（NOW()）填充。
func (r *IdempotencyRepository) Save(ctx context.Context, tx *sql.Tx, record *IdempotencyRecord) error {
	const q = `INSERT INTO idempotency_keys
		(key, tenant_id, request_hash, response_status, response_body, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var responseStatus interface{}
	if record.ResponseStatus != nil {
		responseStatus = *record.ResponseStatus
	}
	var responseBody interface{}
	if len(record.ResponseBody) > 0 {
		responseBody = record.ResponseBody
	}
	var createdAt interface{}
	if !record.CreatedAt.IsZero() {
		createdAt = record.CreatedAt
	}

	_, err := tx.ExecContext(ctx, q,
		record.Key,
		record.TenantID,
		record.RequestHash,
		responseStatus,
		responseBody,
		createdAt,
		record.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("save idempotency record: %w", err)
	}
	return nil
}
