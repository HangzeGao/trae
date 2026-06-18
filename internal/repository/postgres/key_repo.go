package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	domain "github.com/HangzeGao/trae/key-vault/internal/domain/key"
)

// KeyRepository 是密钥聚合根的 PostgreSQL 仓储实现。
//
// 所有读操作接受 *sql.DB（连接池或事务外的查询），
// 写操作接受 *sql.Tx 以支持事务编排。
type KeyRepository struct{}

// NewKeyRepository 创建一个 KeyRepository 实例。
func NewKeyRepository() *KeyRepository {
	return &KeyRepository{}
}

// CreateKey 在事务中创建密钥记录。
func (r *KeyRepository) CreateKey(ctx context.Context, tx *sql.Tx, key *domain.Key) error {
	const q = `INSERT INTO keys
		(id, tenant_id, name, algorithm, purpose, status, current_version, ready_reason, created_at, updated_at, destroyed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	var destroyedAt interface{}
	if key.DestroyedAt != nil {
		destroyedAt = *key.DestroyedAt
	}

	now := time.Now().UTC()
	if key.CreatedAt.IsZero() {
		key.CreatedAt = now
	}
	if key.UpdatedAt.IsZero() {
		key.UpdatedAt = now
	}

	_, err := tx.ExecContext(ctx, q,
		key.ID,
		key.TenantID,
		key.Name,
		string(key.Algorithm),
		string(key.Purpose),
		string(key.Status),
		key.CurrentVersion,
		key.ReadyReason,
		key.CreatedAt,
		key.UpdatedAt,
		destroyedAt,
	)
	if err != nil {
		return fmt.Errorf("insert key: %w", err)
	}
	return nil
}

// GetKey 按租户和密钥 ID 查询密钥（租户隔离）。
//
// 若密钥不存在返回 sql.ErrNoRows。
func (r *KeyRepository) GetKey(ctx context.Context, db *sql.DB, tenantID, keyID string) (*domain.Key, error) {
	const q = `SELECT id, tenant_id, name, algorithm, purpose, status, current_version, ready_reason,
		created_at, updated_at, destroyed_at
		FROM keys WHERE tenant_id = $1 AND id = $2`

	k := &domain.Key{}
	var destroyedAt sql.NullTime
	err := db.QueryRowContext(ctx, q, tenantID, keyID).Scan(
		&k.ID,
		&k.TenantID,
		&k.Name,
		&k.Algorithm,
		&k.Purpose,
		&k.Status,
		&k.CurrentVersion,
		&k.ReadyReason,
		&k.CreatedAt,
		&k.UpdatedAt,
		&destroyedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("key %s not found in tenant %s: %w", keyID, tenantID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get key: %w", err)
	}
	if destroyedAt.Valid {
		t := destroyedAt.Time
		k.DestroyedAt = &t
	}
	return k, nil
}

// GetKeyByName 按租户和密钥名称查询密钥（租户隔离）。
//
// 若密钥不存在返回 sql.ErrNoRows。
func (r *KeyRepository) GetKeyByName(ctx context.Context, db *sql.DB, tenantID, name string) (*domain.Key, error) {
	const q = `SELECT id, tenant_id, name, algorithm, purpose, status, current_version, ready_reason,
		created_at, updated_at, destroyed_at
		FROM keys WHERE tenant_id = $1 AND name = $2`

	k := &domain.Key{}
	var destroyedAt sql.NullTime
	err := db.QueryRowContext(ctx, q, tenantID, name).Scan(
		&k.ID,
		&k.TenantID,
		&k.Name,
		&k.Algorithm,
		&k.Purpose,
		&k.Status,
		&k.CurrentVersion,
		&k.ReadyReason,
		&k.CreatedAt,
		&k.UpdatedAt,
		&destroyedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("key with name %s not found in tenant %s: %w", name, tenantID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get key by name: %w", err)
	}
	if destroyedAt.Valid {
		t := destroyedAt.Time
		k.DestroyedAt = &t
	}
	return k, nil
}

// UpdateKeyStatus 在事务中更新密钥状态及 updated_at 时间戳。
func (r *KeyRepository) UpdateKeyStatus(ctx context.Context, tx *sql.Tx, keyID string, status domain.KeyStatus) error {
	const q = `UPDATE keys SET status = $1, updated_at = $2 WHERE id = $3`

	result, err := tx.ExecContext(ctx, q, string(status), time.Now().UTC(), keyID)
	if err != nil {
		return fmt.Errorf("update key status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update key status: key %s not found", keyID)
	}
	return nil
}

// CreateKeyVersion 在事务中创建密钥版本记录。
func (r *KeyRepository) CreateKeyVersion(ctx context.Context, tx *sql.Tx, kv *domain.KeyVersion) error {
	const q = `INSERT INTO key_versions
		(id, key_id, version, wrapped_dek, dek_kid, kid, created_at, created_by, rotation_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	if kv.CreatedAt.IsZero() {
		kv.CreatedAt = time.Now().UTC()
	}

	_, err := tx.ExecContext(ctx, q,
		kv.ID,
		kv.KeyID,
		kv.Version,
		kv.WrappedDEK,
		kv.DEKKeyID,
		kv.KID,
		kv.CreatedAt,
		kv.CreatedBy,
		kv.RotationReason,
	)
	if err != nil {
		return fmt.Errorf("insert key version: %w", err)
	}
	return nil
}

// GetKeyVersion 按密钥 ID 和版本号查询密钥版本。
//
// key_id 为全局唯一主键，通过它查询不会跨租户泄露数据。
// 若版本不存在返回 sql.ErrNoRows。
func (r *KeyRepository) GetKeyVersion(ctx context.Context, db *sql.DB, keyID string, version int) (*domain.KeyVersion, error) {
	const q = `SELECT id, key_id, version, wrapped_dek, dek_kid, kid, created_at, created_by, rotation_reason
		FROM key_versions WHERE key_id = $1 AND version = $2`

	kv := &domain.KeyVersion{}
	err := db.QueryRowContext(ctx, q, keyID, version).Scan(
		&kv.ID,
		&kv.KeyID,
		&kv.Version,
		&kv.WrappedDEK,
		&kv.DEKKeyID,
		&kv.KID,
		&kv.CreatedAt,
		&kv.CreatedBy,
		&kv.RotationReason,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("key version %d for key %s not found: %w", version, keyID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get key version: %w", err)
	}
	return kv, nil
}

// ListKeyVersions 列出指定密钥的所有版本（按版本号升序）。
//
// key_id 为全局唯一主键，通过它查询不会跨租户泄露数据。
func (r *KeyRepository) ListKeyVersions(ctx context.Context, db *sql.DB, keyID string) ([]*domain.KeyVersion, error) {
	const q = `SELECT id, key_id, version, wrapped_dek, dek_kid, kid, created_at, created_by, rotation_reason
		FROM key_versions WHERE key_id = $1 ORDER BY version ASC`

	rows, err := db.QueryContext(ctx, q, keyID)
	if err != nil {
		return nil, fmt.Errorf("list key versions: %w", err)
	}
	defer rows.Close()

	var list []*domain.KeyVersion
	for rows.Next() {
		kv := &domain.KeyVersion{}
		if err := rows.Scan(
			&kv.ID,
			&kv.KeyID,
			&kv.Version,
			&kv.WrappedDEK,
			&kv.DEKKeyID,
			&kv.KID,
			&kv.CreatedAt,
			&kv.CreatedBy,
			&kv.RotationReason,
		); err != nil {
			return nil, fmt.Errorf("scan key version: %w", err)
		}
		list = append(list, kv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate key versions: %w", err)
	}
	return list, nil
}
