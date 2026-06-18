package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	domain "github.com/HangzeGao/trae/key-vault/internal/domain/node"
)

// NodeRepository 是节点注册信息的 PostgreSQL 仓储实现。
//
// 所有读操作接受 *sql.DB，写操作接受 *sql.Tx 以支持事务编排。
// 查询均带 tenant_id 过滤实现租户隔离。
type NodeRepository struct{}

// NewNodeRepository 创建一个 NodeRepository 实例。
func NewNodeRepository() *NodeRepository {
	return &NodeRepository{}
}

// Create 在事务中创建节点记录。
//
// registered_at / last_seen_at 若为零值则由数据库默认值（NOW()）填充。
func (r *NodeRepository) Create(ctx context.Context, tx *sql.Tx, n *domain.Node) error {
	const q = `INSERT INTO nodes
		(id, tenant_id, hostname, status, cluster_epoch, attestation_epoch,
		 ready_reason, service_token_hash, registered_at, last_seen_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	now := time.Now().UTC()
	if n.RegisteredAt.IsZero() {
		n.RegisteredAt = now
	}
	if n.LastSeenAt.IsZero() {
		n.LastSeenAt = now
	}

	var revokedAt interface{}
	if n.RevokedAt != nil {
		revokedAt = *n.RevokedAt
	}

	var serviceTokenHash interface{}
	if n.ServiceTokenHash != "" {
		serviceTokenHash = n.ServiceTokenHash
	}

	var readyReason interface{}
	if n.ReadyReason != "" {
		readyReason = n.ReadyReason
	}

	_, err := tx.ExecContext(ctx, q,
		n.ID,
		n.TenantID,
		n.Hostname,
		string(n.Status),
		n.ClusterEpoch,
		n.AttestationEpoch,
		readyReason,
		serviceTokenHash,
		n.RegisteredAt,
		n.LastSeenAt,
		revokedAt,
	)
	if err != nil {
		return fmt.Errorf("insert node: %w", err)
	}
	return nil
}

// Get 按租户和节点 ID 查询节点（租户隔离）。
//
// 若节点不存在返回 sql.ErrNoRows。
func (r *NodeRepository) Get(ctx context.Context, db *sql.DB, tenantID, nodeID string) (*domain.Node, error) {
	const q = `SELECT id, tenant_id, hostname, status, cluster_epoch, attestation_epoch,
		ready_reason, service_token_hash, registered_at, last_seen_at, revoked_at
		FROM nodes WHERE tenant_id = $1 AND id = $2`

	n := &domain.Node{}
	var (
		readyReason      sql.NullString
		serviceTokenHash sql.NullString
		revokedAt        sql.NullTime
	)
	err := db.QueryRowContext(ctx, q, tenantID, nodeID).Scan(
		&n.ID,
		&n.TenantID,
		&n.Hostname,
		&n.Status,
		&n.ClusterEpoch,
		&n.AttestationEpoch,
		&readyReason,
		&serviceTokenHash,
		&n.RegisteredAt,
		&n.LastSeenAt,
		&revokedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("node %s not found in tenant %s: %w", nodeID, tenantID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get node: %w", err)
	}
	if readyReason.Valid {
		n.ReadyReason = readyReason.String
	}
	if serviceTokenHash.Valid {
		n.ServiceTokenHash = serviceTokenHash.String
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		n.RevokedAt = &t
	}
	return n, nil
}

// UpdateStatus 在事务中更新节点状态。
//
// node_id 为全局唯一主键。状态变更时同步刷新 last_seen_at。
func (r *NodeRepository) UpdateStatus(ctx context.Context, tx *sql.Tx, nodeID string, status domain.NodeStatus) error {
	const q = `UPDATE nodes SET status = $1, last_seen_at = $2 WHERE id = $3`

	result, err := tx.ExecContext(ctx, q, string(status), time.Now().UTC(), nodeID)
	if err != nil {
		return fmt.Errorf("update node status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update node status: node %s not found", nodeID)
	}
	return nil
}

// UpdateLastSeen 更新节点的最后心跳时间。
//
// 注意：该方法接受 *sql.DB 而非 *sql.Tx，因为心跳更新通常是独立操作，
// 不需要与其它写入组成事务。node_id 为全局唯一主键。
func (r *NodeRepository) UpdateLastSeen(ctx context.Context, db *sql.DB, nodeID string) error {
	const q = `UPDATE nodes SET last_seen_at = $1 WHERE id = $2`

	result, err := db.ExecContext(ctx, q, time.Now().UTC(), nodeID)
	if err != nil {
		return fmt.Errorf("update node last_seen: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update node last_seen: node %s not found", nodeID)
	}
	return nil
}
