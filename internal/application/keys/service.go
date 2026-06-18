// Package keys 实现密钥生命周期管理服务（第 9.3 节）。
//
// Service 是密钥聚合根的应用服务，封装创建、查询、禁用、启用、销毁、轮转等
// 生命周期操作。所有写操作通过 WithTx 保证事务原子性；高风险操作（destroy、
// rotate）通过 Emitter.EmitWithTx 先写 WAL 审计（fail-closed）；状态迁移
// 统一调用 domain.Key.Transition() 方法（HA-06 状态机集中）；所有查询带
// tenant_id 过滤实现租户隔离，跨租户访问统一返回 PERMISSION_DENIED（HA-11）。
package keys

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/audit"
	domainkey "github.com/HangzeGao/trae/key-vault/internal/domain/key"
	domainpolicy "github.com/HangzeGao/trae/key-vault/internal/domain/policy"
	apperrors "github.com/HangzeGao/trae/key-vault/internal/errors"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
	"github.com/HangzeGao/trae/key-vault/internal/repository/postgres"
	"github.com/HangzeGao/trae/key-vault/internal/tpm/provider"
)

// 默认分页参数。
const (
	defaultLimit  = 50
	defaultOffset = 0
	maxLimit      = 200
)

// Service 是密钥生命周期管理服务（第 9.3 节）。
//
// 封装密钥的创建、查询、禁用、启用、销毁、轮转等操作，协调 TPM Provider、
// 仓储、审计发射器等组件。所有写操作通过 WithTx 保证事务原子性。
type Service struct {
	db         *sql.DB
	keyRepo    *postgres.KeyRepository
	policyRepo *postgres.PolicyRepository
	tpm        provider.Provider
	audit      *audit.Emitter
	log        *observability.Logger
}

// NewService 创建一个密钥生命周期管理服务实例。
//
// db 用于事务编排与直接查询；keyRepo / policyRepo 提供聚合根持久化；
// tpm 提供 DEK 生成与密封；auditEmitter 提供审计事件发射；
// log 用于结构化日志输出。
func NewService(
	db *sql.DB,
	keyRepo *postgres.KeyRepository,
	policyRepo *postgres.PolicyRepository,
	tpm provider.Provider,
	auditEmitter *audit.Emitter,
	log *observability.Logger,
) *Service {
	return &Service{
		db:         db,
		keyRepo:    keyRepo,
		policyRepo: policyRepo,
		tpm:        tpm,
		audit:      auditEmitter,
		log:        log,
	}
}

// CreateKeyRequest 创建密钥请求。
type CreateKeyRequest struct {
	TenantID       string
	Name           string
	Algorithm      string // "AES_256_GCM" | "SM4_128_GCM"
	Purpose        string // "data_encryption" | "key_wrapping"
	CreatedBy      string
	RequestID      string
	IdempotencyKey string
	// 策略字段
	RequireAAD       bool
	MaxPlaintextSize int64
	AllowedCallers   []string
}

// CreateKeyResult 创建密钥结果。
type CreateKeyResult struct {
	KeyID          string
	KID            string // 对外标识
	Algorithm      string
	Status         string
	CurrentVersion int
	CreatedAt      time.Time
}

// KeyInfo 密钥信息视图。
type KeyInfo struct {
	ID             string
	Name           string
	Algorithm      string
	Purpose        string
	Status         string
	CurrentVersion int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RotateKeyResult 轮转密钥结果。
type RotateKeyResult struct {
	NewVersion int
	KID        string
	RotatedAt  time.Time
}

// CreateKey 创建密钥（第 9.3 节流程）。
//
// 流程：
//  1. 幂等检查：若同名密钥已存在则返回 DBConflict
//  2. 生成并密封 DEK（调用 TPM GenerateAndSealDEK）
//  3. 事务内：写 keys、key_versions、policies、审计事件
//  4. 返回密钥信息
//
// 算法与用途取值由 domain 层常量约束；非法取值返回 InvalidRequest。
// TPM 失败返回 TPMUnavailable；DB 冲突返回 DBConflict。
func (s *Service) CreateKey(ctx context.Context, req CreateKeyRequest) (*CreateKeyResult, error) {
	// 参数校验
	if req.TenantID == "" || req.Name == "" {
		return nil, apperrors.InvalidRequest("tenant_id and name are required")
	}

	algorithm := domainkey.KeyAlgorithm(req.Algorithm)
	purpose := domainkey.KeyPurpose(req.Purpose)
	if err := validateAlgorithm(algorithm); err != nil {
		return nil, err
	}
	if err := validatePurpose(purpose); err != nil {
		return nil, err
	}

	// 幂等检查：同名密钥已存在则返回冲突
	existing, err := s.keyRepo.GetKeyByName(ctx, s.db, req.TenantID, req.Name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.log.Error("idempotency check failed",
			"error", err,
			"tenant_id", req.TenantID,
			"name", req.Name,
		)
		return nil, apperrors.Internal(err)
	}
	if existing != nil {
		return nil, apperrors.DBConflict(fmt.Errorf("key name %q already exists in tenant %s", req.Name, req.TenantID))
	}

	// 生成并密封 DEK（调用 TPM）
	sealedDEK, dekKID, err := s.tpm.GenerateAndSealDEK(ctx, req.Algorithm)
	if err != nil {
		s.log.Error("tpm generate and seal dek failed",
			"error", err,
			"algorithm", req.Algorithm,
		)
		return nil, apperrors.TPMUnavailable(err)
	}

	// 生成 ID
	keyID := generateUUID()
	versionID := generateUUID()
	policyID := generateUUID()
	now := time.Now().UTC()
	const initialVersion = 1
	kid := formatKID(keyID, initialVersion)

	// 构造领域对象
	k := &domainkey.Key{
		ID:             keyID,
		TenantID:       req.TenantID,
		Name:           req.Name,
		Algorithm:      algorithm,
		Purpose:        purpose,
		Status:         domainkey.StatusActive,
		CurrentVersion: initialVersion,
		CreatedAt:      now,
		UpdatedAt:      now,
		ReadyReason:    "static_registration",
	}

	kv := &domainkey.KeyVersion{
		ID:             versionID,
		KeyID:          keyID,
		Version:        initialVersion,
		WrappedDEK:     sealedDEK,
		DEKKeyID:       dekKID,
		KID:            kid,
		CreatedAt:      now,
		CreatedBy:      req.CreatedBy,
		RotationReason: "initial",
	}

	p := &domainpolicy.Policy{
		ID:             policyID,
		TenantID:       req.TenantID,
		KeyID:          keyID,
		MinAlgorithm:   req.Algorithm,
		RequireAAD:     req.RequireAAD,
		MaxPlaintextSize: req.MaxPlaintextSize,
		AllowedCallers: req.AllowedCallers,
	}

	// 事务内写入 keys、key_versions、policies、审计事件
	err = postgres.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := s.keyRepo.CreateKey(ctx, tx, k); err != nil {
			return fmt.Errorf("create key: %w", err)
		}
		if err := s.keyRepo.CreateKeyVersion(ctx, tx, kv); err != nil {
			return fmt.Errorf("create key version: %w", err)
		}
		if err := s.policyRepo.Create(ctx, tx, p); err != nil {
			return fmt.Errorf("create policy: %w", err)
		}

		// 审计事件（key.create 非高风险，fail-open）
		event := audit.BuildEvent(
			req.TenantID,
			"key.create",
			req.CreatedBy,
			"create",
			"key",
			keyID,
			"success",
			req.RequestID,
			map[string]interface{}{
				"name":      req.Name,
				"algorithm": req.Algorithm,
				"purpose":   req.Purpose,
				"version":   initialVersion,
			},
		)
		if err := s.audit.EmitWithTx(ctx, tx, event); err != nil {
			return fmt.Errorf("emit audit: %w", err)
		}
		return nil
	})
	if err != nil {
		s.log.Error("create key transaction failed",
			"error", err,
			"tenant_id", req.TenantID,
			"name", req.Name,
		)
		return nil, apperrors.Internal(err)
	}

	s.log.Info("key created",
		"key_id", keyID,
		"tenant_id", req.TenantID,
		"name", req.Name,
		"algorithm", req.Algorithm,
	)

	return &CreateKeyResult{
		KeyID:          keyID,
		KID:            kid,
		Algorithm:      req.Algorithm,
		Status:         string(domainkey.StatusActive),
		CurrentVersion: initialVersion,
		CreatedAt:      now,
	}, nil
}

// GetKey 查询密钥信息。
//
// 查询带 tenant_id 过滤实现租户隔离；若密钥不存在或跨租户访问，
// 统一返回 CrossTenantDenied（HA-11，不区分不存在与无权限）。
func (s *Service) GetKey(ctx context.Context, tenantID, keyID string) (*KeyInfo, error) {
	k, err := s.keyRepo.GetKey(ctx, s.db, tenantID, keyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 跨租户访问统一返回 PERMISSION_DENIED（HA-11）
			return nil, apperrors.CrossTenantDenied()
		}
		s.log.Error("get key failed",
			"error", err,
			"tenant_id", tenantID,
			"key_id", keyID,
		)
		return nil, apperrors.Internal(err)
	}
	return toKeyInfo(k), nil
}

// DisableKey 禁用密钥。
//
// 仅 ACTIVE 状态可禁用；状态迁移通过 domain.Key.Transition("disable") 完成（HA-06）。
// 操作在事务内完成，审计事件（key.disable）随业务事务提交。
func (s *Service) DisableKey(ctx context.Context, tenantID, keyID, actor, requestID string) error {
	k, err := s.loadKey(ctx, tenantID, keyID)
	if err != nil {
		return err
	}

	if !k.CanDisable() {
		if k.Status == domainkey.StatusDestroyed {
			return apperrors.KeyDestroyed()
		}
		return apperrors.InvalidRequest("key is not in a state that can be disabled")
	}

	if err := k.Transition("disable"); err != nil {
		return apperrors.InvalidRequest(err.Error())
	}

	return postgres.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := s.keyRepo.UpdateKeyStatus(ctx, tx, keyID, k.Status); err != nil {
			return fmt.Errorf("update key status: %w", err)
		}

		event := audit.BuildEvent(
			tenantID,
			"key.disable",
			actor,
			"disable",
			"key",
			keyID,
			"success",
			requestID,
			map[string]interface{}{
				"previous_status": string(domainkey.StatusActive),
				"new_status":      string(domainkey.StatusDisabled),
			},
		)
		if err := s.audit.EmitWithTx(ctx, tx, event); err != nil {
			return fmt.Errorf("emit audit: %w", err)
		}
		return nil
	})
}

// EnableKey 启用密钥。
//
// 仅 DISABLED 状态可启用；状态迁移通过 domain.Key.Transition("enable") 完成（HA-06）。
// 操作在事务内完成，审计事件（key.enable）随业务事务提交。
func (s *Service) EnableKey(ctx context.Context, tenantID, keyID, actor, requestID string) error {
	k, err := s.loadKey(ctx, tenantID, keyID)
	if err != nil {
		return err
	}

	if k.Status == domainkey.StatusDestroyed {
		return apperrors.KeyDestroyed()
	}

	if err := k.Transition("enable"); err != nil {
		return apperrors.InvalidRequest(err.Error())
	}

	return postgres.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := s.keyRepo.UpdateKeyStatus(ctx, tx, keyID, k.Status); err != nil {
			return fmt.Errorf("update key status: %w", err)
		}

		event := audit.BuildEvent(
			tenantID,
			"key.enable",
			actor,
			"enable",
			"key",
			keyID,
			"success",
			requestID,
			map[string]interface{}{
				"previous_status": string(domainkey.StatusDisabled),
				"new_status":      string(domainkey.StatusActive),
			},
		)
		if err := s.audit.EmitWithTx(ctx, tx, event); err != nil {
			return fmt.Errorf("emit audit: %w", err)
		}
		return nil
	})
}

// DestroyKey 销毁密钥（高风险操作，需 WAL 审计）。
//
// 销毁后 WrappedDEK 不可用，但保留元数据。仅 ACTIVE 或 DISABLED 状态可销毁；
// 状态迁移通过 domain.Key.Transition("destroy") 完成（HA-06）。
// 高风险操作通过 EmitWithTx 先写 WAL 审计（fail-closed），确保审计持久化
// 在业务事务提交前完成；WAL 写入失败则回滚业务事务。
func (s *Service) DestroyKey(ctx context.Context, tenantID, keyID, actor, requestID string) error {
	k, err := s.loadKey(ctx, tenantID, keyID)
	if err != nil {
		return err
	}

	if !k.CanDestroy() {
		if k.Status == domainkey.StatusDestroyed {
			return apperrors.KeyDestroyed()
		}
		return apperrors.InvalidRequest("key is not in a state that can be destroyed")
	}

	previousStatus := k.Status
	if err := k.Transition("destroy"); err != nil {
		return apperrors.InvalidRequest(err.Error())
	}

	destroyedAt := time.Now().UTC()
	if k.DestroyedAt != nil {
		destroyedAt = *k.DestroyedAt
	}

	return postgres.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		// 更新状态、updated_at、destroyed_at（UpdateKeyStatus 不处理 destroyed_at，直接 SQL）
		if err := destroyKeyInTx(ctx, tx, keyID, tenantID, k.Status, destroyedAt); err != nil {
			return fmt.Errorf("destroy key: %w", err)
		}

		// 高风险审计事件（key.destroy）：EmitWithTx 先写 WAL（fail-closed），再写 DB
		event := audit.BuildEvent(
			tenantID,
			"key.destroy",
			actor,
			"destroy",
			"key",
			keyID,
			"success",
			requestID,
			map[string]interface{}{
				"previous_status": string(previousStatus),
				"new_status":      string(domainkey.StatusDestroyed),
			},
		)
		if err := s.audit.EmitWithTx(ctx, tx, event); err != nil {
			return fmt.Errorf("emit audit (fail-closed): %w", err)
		}
		return nil
	})
}

// RotateKey 轮转密钥（高风险操作，需 WAL 审计）。
//
// 生成新版本 DEK，旧版本保留可解密。仅 ACTIVE 状态可轮转；
// 新版本号 = current_version + 1；通过 TPM 生成并密封新 DEK。
// 高风险操作通过 EmitWithTx 先写 WAL 审计（fail-closed），确保审计持久化
// 在业务事务提交前完成；WAL 写入失败则回滚业务事务。
func (s *Service) RotateKey(ctx context.Context, tenantID, keyID, actor, reason, requestID string) (*RotateKeyResult, error) {
	k, err := s.loadKey(ctx, tenantID, keyID)
	if err != nil {
		return nil, err
	}

	if !k.CanRotate() {
		if k.Status == domainkey.StatusDestroyed {
			return nil, apperrors.KeyDestroyed()
		}
		return nil, apperrors.InvalidRequest("key is not in a state that can be rotated")
	}

	// 轮转原因校验
	if reason == "" {
		reason = "manual"
	}
	if err := validateRotationReason(reason); err != nil {
		return nil, err
	}

	// 生成并密封新 DEK
	sealedDEK, dekKID, err := s.tpm.GenerateAndSealDEK(ctx, string(k.Algorithm))
	if err != nil {
		s.log.Error("tpm generate and seal dek failed during rotation",
			"error", err,
			"key_id", keyID,
			"algorithm", k.Algorithm,
		)
		return nil, apperrors.TPMUnavailable(err)
	}

	newVersion := k.CurrentVersion + 1
	versionID := generateUUID()
	now := time.Now().UTC()
	kid := formatKID(keyID, newVersion)

	kv := &domainkey.KeyVersion{
		ID:             versionID,
		KeyID:          keyID,
		Version:        newVersion,
		WrappedDEK:     sealedDEK,
		DEKKeyID:       dekKID,
		KID:            kid,
		CreatedAt:      now,
		CreatedBy:      actor,
		RotationReason: reason,
	}

	err = postgres.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		// 创建新版本
		if err := s.keyRepo.CreateKeyVersion(ctx, tx, kv); err != nil {
			return fmt.Errorf("create key version: %w", err)
		}
		// 更新 current_version（repository 无此方法，直接 SQL）
		if err := updateCurrentVersionInTx(ctx, tx, keyID, tenantID, newVersion); err != nil {
			return fmt.Errorf("update current version: %w", err)
		}

		// 高风险审计事件（key.rotate）：EmitWithTx 先写 WAL（fail-closed），再写 DB
		event := audit.BuildEvent(
			tenantID,
			"key.rotate",
			actor,
			"rotate",
			"key",
			keyID,
			"success",
			requestID,
			map[string]interface{}{
				"previous_version": k.CurrentVersion,
				"new_version":      newVersion,
				"reason":           reason,
			},
		)
		if err := s.audit.EmitWithTx(ctx, tx, event); err != nil {
			return fmt.Errorf("emit audit (fail-closed): %w", err)
		}
		return nil
	})
	if err != nil {
		s.log.Error("rotate key transaction failed",
			"error", err,
			"key_id", keyID,
			"tenant_id", tenantID,
		)
		return nil, apperrors.Internal(err)
	}

	s.log.Info("key rotated",
		"key_id", keyID,
		"tenant_id", tenantID,
		"new_version", newVersion,
		"reason", reason,
	)

	return &RotateKeyResult{
		NewVersion: newVersion,
		KID:        kid,
		RotatedAt:  now,
	}, nil
}

// ListKeys 列出租户下的密钥。
//
// 按 created_at 降序分页返回；limit/offset 为 0 或负数时使用默认值。
// 查询带 tenant_id 过滤实现租户隔离。
func (s *Service) ListKeys(ctx context.Context, tenantID string, limit, offset int) ([]*KeyInfo, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = defaultOffset
	}

	rows, err := s.db.QueryContext(ctx, listKeysSQL, tenantID, limit, offset)
	if err != nil {
		s.log.Error("list keys failed",
			"error", err,
			"tenant_id", tenantID,
		)
		return nil, apperrors.Internal(err)
	}
	defer rows.Close()

	var list []*KeyInfo
	for rows.Next() {
		k := &domainkey.Key{}
		var destroyedAt sql.NullTime
		if err := rows.Scan(
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
		); err != nil {
			s.log.Error("scan key failed",
				"error", err,
				"tenant_id", tenantID,
			)
			return nil, apperrors.Internal(err)
		}
		if destroyedAt.Valid {
			t := destroyedAt.Time
			k.DestroyedAt = &t
		}
		list = append(list, toKeyInfo(k))
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal(err)
	}
	return list, nil
}

// loadKey 加载密钥并处理错误映射。
//
// 不存在或跨租户访问统一返回 CrossTenantDenied（HA-11）。
func (s *Service) loadKey(ctx context.Context, tenantID, keyID string) (*domainkey.Key, error) {
	k, err := s.keyRepo.GetKey(ctx, s.db, tenantID, keyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.CrossTenantDenied()
		}
		s.log.Error("load key failed",
			"error", err,
			"tenant_id", tenantID,
			"key_id", keyID,
		)
		return nil, apperrors.Internal(err)
	}
	return k, nil
}

// toKeyInfo 将 domain.Key 转换为 KeyInfo 视图。
func toKeyInfo(k *domainkey.Key) *KeyInfo {
	return &KeyInfo{
		ID:             k.ID,
		Name:           k.Name,
		Algorithm:      string(k.Algorithm),
		Purpose:        string(k.Purpose),
		Status:         string(k.Status),
		CurrentVersion: k.CurrentVersion,
		CreatedAt:      k.CreatedAt,
		UpdatedAt:      k.UpdatedAt,
	}
}

// listKeysSQL 列出租户密钥的 SQL（带 tenant_id 过滤，按 created_at 降序）。
const listKeysSQL = `SELECT id, tenant_id, name, algorithm, purpose, status, current_version, ready_reason,
	created_at, updated_at, destroyed_at
	FROM keys WHERE tenant_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

// destroyKeyInTx 在事务中更新密钥状态为 DESTROYED 并设置 destroyed_at。
//
// repository.UpdateKeyStatus 不处理 destroyed_at 字段，故直接执行 SQL。
// WHERE 子句带 tenant_id 过滤实现租户隔离（防御性）。
func destroyKeyInTx(ctx context.Context, tx *sql.Tx, keyID, tenantID string, status domainkey.KeyStatus, destroyedAt time.Time) error {
	const q = `UPDATE keys SET status = $1, updated_at = $2, destroyed_at = $3
		WHERE id = $4 AND tenant_id = $5`
	result, err := tx.ExecContext(ctx, q, string(status), time.Now().UTC(), destroyedAt, keyID, tenantID)
	if err != nil {
		return fmt.Errorf("update key for destroy: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("destroy key: key %s not found in tenant %s", keyID, tenantID)
	}
	return nil
}

// updateCurrentVersionInTx 在事务中更新密钥的 current_version。
//
// repository 无此方法，故直接执行 SQL。
// WHERE 子句带 tenant_id 过滤实现租户隔离（防御性）。
func updateCurrentVersionInTx(ctx context.Context, tx *sql.Tx, keyID, tenantID string, version int) error {
	const q = `UPDATE keys SET current_version = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4`
	result, err := tx.ExecContext(ctx, q, version, time.Now().UTC(), keyID, tenantID)
	if err != nil {
		return fmt.Errorf("update current version: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("update current version: key %s not found in tenant %s", keyID, tenantID)
	}
	return nil
}

// formatKID 构造对外暴露的 KID 标识（key_id/v{version}）。
func formatKID(keyID string, version int) string {
	return fmt.Sprintf("%s/v%d", keyID, version)
}

// generateUUID 使用 crypto/rand 生成 UUID v4 字符串。
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// 设置版本位 (version 4)
	b[6] = (b[6] & 0x0f) | 0x40
	// 设置变体位 (variant 10xx)
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// validateAlgorithm 校验算法取值。
func validateAlgorithm(alg domainkey.KeyAlgorithm) error {
	switch alg {
	case domainkey.AlgAES256GCM, domainkey.AlgSM4GCM:
		return nil
	default:
		return apperrors.InvalidRequest(fmt.Sprintf("unsupported algorithm: %s", alg))
	}
}

// validatePurpose 校验用途取值。
func validatePurpose(p domainkey.KeyPurpose) error {
	switch p {
	case domainkey.PurposeDataEncryption, domainkey.PurposeKeyWrapping:
		return nil
	default:
		return apperrors.InvalidRequest(fmt.Sprintf("unsupported purpose: %s", p))
	}
}

// validateRotationReason 校验轮转原因取值。
func validateRotationReason(reason string) error {
	switch reason {
	case "scheduled", "manual", "compromise":
		return nil
	default:
		return apperrors.InvalidRequest(fmt.Sprintf("unsupported rotation reason: %s", reason))
	}
}
