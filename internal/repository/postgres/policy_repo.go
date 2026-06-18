package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	domain "github.com/HangzeGao/trae/key-vault/internal/domain/policy"
)

// PolicyRepository 是加密策略的 PostgreSQL 仓储实现。
//
// 所有读操作接受 *sql.DB，写操作接受 *sql.Tx 以支持事务编排。
// 查询均带 tenant_id 过滤实现租户隔离。
type PolicyRepository struct{}

// NewPolicyRepository 创建一个 PolicyRepository 实例。
func NewPolicyRepository() *PolicyRepository {
	return &PolicyRepository{}
}

// Create 在事务中创建策略记录。
//
// created_at / updated_at 由数据库默认值（NOW()）填充。
func (r *PolicyRepository) Create(ctx context.Context, tx *sql.Tx, p *domain.Policy) error {
	const q = `INSERT INTO policies
		(id, tenant_id, key_id, min_algorithm, require_aad, max_plaintext_size,
		 allowed_callers, cbc_decrypt_only, ecb_decrypt_only,
		 downgrade_requires_approval, approval_id, policy_signature)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	// 处理空切片：PostgreSQL TEXT[] 期望 '{}' 而非 NULL
	callers := p.AllowedCallers
	if callers == nil {
		callers = []string{}
	}

	// 处理可空字段
	var approvalID interface{}
	if p.ApprovalID != "" {
		approvalID = p.ApprovalID
	}
	var policySignature interface{}
	if len(p.PolicySignature) > 0 {
		policySignature = p.PolicySignature
	}

	_, err := tx.ExecContext(ctx, q,
		p.ID,
		p.TenantID,
		p.KeyID,
		p.MinAlgorithm,
		p.RequireAAD,
		p.MaxPlaintextSize,
		callers,
		p.CBCDecryptOnly,
		p.ECBDecryptOnly,
		p.DowngradeRequiresApproval,
		approvalID,
		policySignature,
	)
	if err != nil {
		return fmt.Errorf("insert policy: %w", err)
	}
	return nil
}

// GetByKeyID 按租户和密钥 ID 查询策略（租户隔离）。
//
// 每个密钥最多关联一条策略。若策略不存在返回 sql.ErrNoRows。
func (r *PolicyRepository) GetByKeyID(ctx context.Context, db *sql.DB, tenantID, keyID string) (*domain.Policy, error) {
	const q = `SELECT id, tenant_id, key_id, min_algorithm, require_aad, max_plaintext_size,
		allowed_callers, cbc_decrypt_only, ecb_decrypt_only,
		downgrade_requires_approval, approval_id, policy_signature
		FROM policies WHERE tenant_id = $1 AND key_id = $2`

	p := &domain.Policy{}
	var (
		approvalID      sql.NullString
		policySignature []byte
	)
	err := db.QueryRowContext(ctx, q, tenantID, keyID).Scan(
		&p.ID,
		&p.TenantID,
		&p.KeyID,
		&p.MinAlgorithm,
		&p.RequireAAD,
		&p.MaxPlaintextSize,
		&p.AllowedCallers,
		&p.CBCDecryptOnly,
		&p.ECBDecryptOnly,
		&p.DowngradeRequiresApproval,
		&approvalID,
		&policySignature,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("policy for key %s not found in tenant %s: %w", keyID, tenantID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get policy: %w", err)
	}
	if approvalID.Valid {
		p.ApprovalID = approvalID.String
	}
	p.PolicySignature = policySignature
	return p, nil
}

// Update 在事务中更新策略记录。
//
// 以 policy.ID 为主键更新所有字段，并刷新 updated_at 时间戳。
func (r *PolicyRepository) Update(ctx context.Context, tx *sql.Tx, p *domain.Policy) error {
	const q = `UPDATE policies SET
		min_algorithm = $1,
		require_aad = $2,
		max_plaintext_size = $3,
		allowed_callers = $4,
		cbc_decrypt_only = $5,
		ecb_decrypt_only = $6,
		downgrade_requires_approval = $7,
		approval_id = $8,
		policy_signature = $9,
		updated_at = $10
		WHERE id = $11 AND tenant_id = $12`

	callers := p.AllowedCallers
	if callers == nil {
		callers = []string{}
	}

	var approvalID interface{}
	if p.ApprovalID != "" {
		approvalID = p.ApprovalID
	}
	var policySignature interface{}
	if len(p.PolicySignature) > 0 {
		policySignature = p.PolicySignature
	}

	result, err := tx.ExecContext(ctx, q,
		p.MinAlgorithm,
		p.RequireAAD,
		p.MaxPlaintextSize,
		callers,
		p.CBCDecryptOnly,
		p.ECBDecryptOnly,
		p.DowngradeRequiresApproval,
		approvalID,
		policySignature,
		time.Now().UTC(),
		p.ID,
		p.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update policy: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update policy: policy %s not found in tenant %s", p.ID, p.TenantID)
	}
	return nil
}
