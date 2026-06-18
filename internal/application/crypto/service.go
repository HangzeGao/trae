// Package crypto 实现数据面加解密服务（第 9.4 节）。
//
// Service 是数据面加解密的应用服务，封装加密、解密、生成数据密钥等操作。
// 加密流程严格按第 9.4 节：查密钥 -> 策略评估 -> DEK lease -> nonce 分配 ->
// AEAD 加密 -> 构建 Envelope v1 -> 审计/指标 -> 清零 DEK。
// 解密流程：解析信封 -> 提取 KID -> 查密钥 -> 策略评估 -> AAD 验证 ->
// DEK lease -> AEAD 解密 -> 审计/指标 -> 清零 DEK。
// 所有 DEK 明文使用后必须清零，避免内存残留。
package crypto

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/audit"
	"github.com/HangzeGao/trae/key-vault/internal/crypto/aead"
	"github.com/HangzeGao/trae/key-vault/internal/crypto/datakey"
	"github.com/HangzeGao/trae/key-vault/internal/crypto/envelope"
	"github.com/HangzeGao/trae/key-vault/internal/crypto/nonce"
	domainkey "github.com/HangzeGao/trae/key-vault/internal/domain/key"
	domainpolicy "github.com/HangzeGao/trae/key-vault/internal/domain/policy"
	apperrors "github.com/HangzeGao/trae/key-vault/internal/errors"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
	"github.com/HangzeGao/trae/key-vault/internal/repository/postgres"
	"github.com/HangzeGao/trae/key-vault/internal/resolver/keyresolver"
)

// Service 是加解密服务（第 9.4 节）。
//
// 封装加密、解密、生成数据密钥等数据面操作，协调密钥仓储、策略仓储、
// 密钥解析器、审计发射器等组件。所有 DEK 明文使用后必须清零。
type Service struct {
	db         *sql.DB
	keyRepo    *postgres.KeyRepository
	policyRepo *postgres.PolicyRepository
	resolver   *keyresolver.Resolver
	audit      *audit.Emitter
	log        *observability.Logger
	metrics    *observability.Metrics
}

// NewService 创建一个加解密服务实例。
//
// db 用于密钥与策略查询；keyRepo / policyRepo 提供聚合根持久化；
// resolver 提供 DEK lease 签发与 nonce 分配；auditEmitter 提供审计事件发射；
// log 用于结构化日志输出。指标使用全局 MetricsInstance。
func NewService(
	db *sql.DB,
	keyRepo *postgres.KeyRepository,
	policyRepo *postgres.PolicyRepository,
	resolver *keyresolver.Resolver,
	auditEmitter *audit.Emitter,
	log *observability.Logger,
) *Service {
	return &Service{
		db:         db,
		keyRepo:    keyRepo,
		policyRepo: policyRepo,
		resolver:   resolver,
		audit:      auditEmitter,
		log:        log,
		metrics:    observability.MetricsInstance(),
	}
}

// EncryptRequest 加密请求。
type EncryptRequest struct {
	TenantID  string
	KeyID     string
	Version   int // 0 表示使用当前版本
	Plaintext []byte
	AAD       []byte
	CallerID  string
	RequestID string
}

// EncryptResponse 加密响应。
type EncryptResponse struct {
	Envelope  []byte // Envelope v1 格式的密文信封
	KID       string // 使用的密钥版本标识
	Algorithm string
}

// Encrypt 加密数据（第 9.4 节流程）。
//
// 流程：
//  1. 查询密钥，检查状态（CanEncrypt）
//  2. 策略评估（EvaluateEncrypt）
//  3. 通过 resolver 获取 DEK lease
//  4. 通过 resolver 分配 nonce
//  5. 使用 AEAD 加密
//  6. 构建 Envelope v1
//  7. 审计、指标
//  8. 清零 DEK 明文
//
// 密钥状态不允许加密时返回 KeyDisabled/KeyDestroyed；
// 策略评估失败返回 PolicyDenied；跨租户访问统一返回 PERMISSION_DENIED（HA-11）。
func (s *Service) Encrypt(ctx context.Context, req EncryptRequest) (*EncryptResponse, error) {
	start := time.Now()
	s.metrics.IncEncryptOp()
	defer func() {
		s.metrics.RecordEncryptLatency(time.Since(start))
	}()

	// 1. 查询密钥，检查状态（CanEncrypt）
	k, err := s.loadKey(ctx, req.TenantID, req.KeyID)
	if err != nil {
		s.metrics.IncEncryptError()
		return nil, err
	}

	if !k.CanEncrypt() {
		s.metrics.IncEncryptError()
		return nil, statusError(k.Status)
	}

	// 解析版本：0 表示使用当前版本
	version := req.Version
	if version == 0 {
		version = k.CurrentVersion
	}

	algorithm := string(k.Algorithm)

	// 2. 策略评估（EvaluateEncrypt）
	if err := s.evaluateEncryptPolicy(ctx, req.TenantID, req.KeyID, algorithm, req.AAD, req.Plaintext, req.CallerID); err != nil {
		s.metrics.IncEncryptError()
		return nil, err
	}

	// 3. 通过 resolver 获取 DEK lease
	lease, err := s.resolver.IssueDEKLease(ctx, req.TenantID, req.KeyID, version)
	if err != nil {
		s.metrics.IncEncryptError()
		return nil, err
	}
	defer zeroBytes(lease.DEK)

	// 4. 通过 resolver 分配 nonce
	nonceNum, err := s.resolver.AllocateNonce(ctx, lease.KID)
	if err != nil {
		s.metrics.IncEncryptError()
		return nil, err
	}
	nonceBytes := nonce.NonceToBytes(nonceNum)

	// 5. 使用 AEAD 加密
	provider, err := aead.New(algorithm, lease.DEK)
	if err != nil {
		s.metrics.IncEncryptError()
		return nil, apperrors.Internal(err)
	}

	// 计算 canonical AAD，用于 AEAD 加密与信封存储
	canonicalAAD := envelope.Canonical(req.AAD)
	ciphertext, err := provider.Encrypt(req.Plaintext, canonicalAAD, nonceBytes[:])
	if err != nil {
		s.metrics.IncEncryptError()
		return nil, apperrors.Internal(err)
	}

	// 6. 构建 Envelope v1
	suiteID, err := envelope.SuiteIDFor(algorithm)
	if err != nil {
		s.metrics.IncEncryptError()
		return nil, apperrors.Internal(err)
	}

	envBytes, err := envelope.Build(suiteID, lease.KID, nonceBytes[:], ciphertext, canonicalAAD)
	if err != nil {
		s.metrics.IncEncryptError()
		return nil, err
	}

	// 7. 审计、指标
	s.emitAudit(ctx, req.TenantID, "crypto.encrypt", req.CallerID, "encrypt", "key", req.KeyID, "success", req.RequestID, map[string]interface{}{
		"kid":       lease.KID,
		"algorithm": algorithm,
		"version":   version,
	})

	return &EncryptResponse{
		Envelope:  envBytes,
		KID:       lease.KID,
		Algorithm: algorithm,
	}, nil
}

// DecryptRequest 解密请求。
type DecryptRequest struct {
	TenantID  string
	Envelope  []byte // Envelope v1 格式
	AAD       []byte // 额外的 AAD（与加密时一致）
	CallerID  string
	RequestID string
}

// DecryptResponse 解密响应。
type DecryptResponse struct {
	Plaintext []byte
	KID       string
	Algorithm string
}

// Decrypt 解密数据（第 9.4 节流程）。
//
// 流程：
//  1. 解析 Envelope v1
//  2. 从 envelope 中提取 KID，定位密钥版本
//  3. 查询密钥，检查状态（CanDecrypt）
//  4. 策略评估（EvaluateDecrypt）
//  5. AAD 验证（AAD_MISMATCH 错误）
//  6. 通过 resolver 获取 DEK lease
//  7. 使用 AEAD 解密
//  8. 审计、指标
//  9. 清零 DEK 明文
//
// 信封格式不合法返回 EnvelopeInvalid；AAD 验证失败返回 AADMismatch；
// 密钥状态不允许解密返回 KeyDestroyed；跨租户访问统一返回 PERMISSION_DENIED（HA-11）。
func (s *Service) Decrypt(ctx context.Context, req DecryptRequest) (*DecryptResponse, error) {
	start := time.Now()
	s.metrics.IncDecryptOp()
	defer func() {
		s.metrics.RecordDecryptLatency(time.Since(start))
	}()

	// 1. 解析 Envelope v1
	parsed, err := envelope.Parse(req.Envelope)
	if err != nil {
		s.metrics.IncDecryptError()
		return nil, err
	}

	// 2. 从 envelope 中提取 KID，定位密钥版本
	keyID, version, err := parseKID(parsed.DEKKID)
	if err != nil {
		s.metrics.IncDecryptError()
		return nil, apperrors.EnvelopeInvalid(err)
	}

	// 3. 查询密钥，检查状态（CanDecrypt）
	k, err := s.loadKey(ctx, req.TenantID, keyID)
	if err != nil {
		s.metrics.IncDecryptError()
		return nil, err
	}

	if !k.CanDecrypt() {
		s.metrics.IncDecryptError()
		return nil, statusError(k.Status)
	}

	algorithm, err := envelope.AlgorithmFor(parsed.SuiteID)
	if err != nil {
		s.metrics.IncDecryptError()
		return nil, apperrors.EnvelopeInvalid(err)
	}

	// 校验信封算法与密钥算法一致（防御性检查）
	if string(k.Algorithm) != algorithm {
		s.metrics.IncDecryptError()
		return nil, apperrors.EnvelopeInvalid(fmt.Errorf("algorithm mismatch: key=%s, envelope=%s", k.Algorithm, algorithm))
	}

	// 4. 策略评估（EvaluateDecrypt）
	if err := s.evaluateDecryptPolicy(ctx, req.TenantID, keyID, algorithm, req.CallerID); err != nil {
		s.metrics.IncDecryptError()
		return nil, err
	}

	// 5. AAD 验证（AAD_MISMATCH 错误）
	// 比较信封中存储的 canonical AAD 与请求 AAD 的 canonical 形式
	if !envelope.Verify(parsed.AAD, req.AAD) {
		s.metrics.IncDecryptError()
		return nil, apperrors.AADMismatch()
	}

	// 6. 通过 resolver 获取 DEK lease
	lease, err := s.resolver.IssueDEKLease(ctx, req.TenantID, keyID, version)
	if err != nil {
		s.metrics.IncDecryptError()
		return nil, err
	}
	defer zeroBytes(lease.DEK)

	// 7. 使用 AEAD 解密
	provider, err := aead.New(algorithm, lease.DEK)
	if err != nil {
		s.metrics.IncDecryptError()
		return nil, apperrors.Internal(err)
	}

	// 使用信封中存储的 canonical AAD 进行解密
	plaintext, err := provider.Decrypt(parsed.Ciphertext, parsed.AAD, parsed.Nonce)
	if err != nil {
		s.metrics.IncDecryptError()
		return nil, apperrors.Internal(err)
	}

	// 8. 审计、指标
	s.emitAudit(ctx, req.TenantID, "crypto.decrypt", req.CallerID, "decrypt", "key", keyID, "success", req.RequestID, map[string]interface{}{
		"kid":       parsed.DEKKID,
		"algorithm": algorithm,
	})

	return &DecryptResponse{
		Plaintext: plaintext,
		KID:       parsed.DEKKID,
		Algorithm: algorithm,
	}, nil
}

// GenerateDataKeyRequest 生成数据密钥请求。
type GenerateDataKeyRequest struct {
	TenantID  string
	KeyID     string
	Algorithm string // "AES_256_GCM" | "SM4_128_GCM"
	CallerID  string
	RequestID string
}

// GenerateDataKeyResponse 生成数据密钥响应。
type GenerateDataKeyResponse struct {
	PlaintextDEK []byte // 明文 DEK（调用方自行保管）
	WrappedDEK   []byte // 用 KEK 包装的 DEK
	KID          string
}

// GenerateDataKey 生成数据密钥（第 9.5 节）。
//
// 用于信封加密场景，调用方获得明文 DEK 和包装后的 DEK。
// 流程：
//  1. 查询密钥，检查状态（CanEncrypt）
//  2. 通过 resolver 获取 KEK（密钥的 DEK）lease
//  3. 生成随机 DEK
//  4. 用 KEK 包装 DEK
//  5. 审计
//  6. 清零 KEK 明文
//
// KEK 必须为 AES_256_GCM 算法（datakey.Wrap 使用 AES-256-GCM 包装）。
// 调用方负责在使用完毕后清零明文 DEK。
func (s *Service) GenerateDataKey(ctx context.Context, req GenerateDataKeyRequest) (*GenerateDataKeyResponse, error) {
	// 1. 查询密钥，检查状态（CanEncrypt）
	k, err := s.loadKey(ctx, req.TenantID, req.KeyID)
	if err != nil {
		return nil, err
	}

	if !k.CanEncrypt() {
		return nil, statusError(k.Status)
	}

	// KEK 必须为 AES_256_GCM（datakey.Wrap 使用 AES-256-GCM 包装，要求 32 字节密钥）
	if k.Algorithm != domainkey.AlgAES256GCM {
		return nil, apperrors.InvalidRequest("GenerateDataKey requires an AES_256_GCM key for wrapping")
	}

	// 确定 DEK 算法：请求指定则用请求的，否则用密钥的算法
	algorithm := req.Algorithm
	if algorithm == "" {
		algorithm = string(k.Algorithm)
	}
	if err := validateAlgorithm(algorithm); err != nil {
		return nil, err
	}

	version := k.CurrentVersion

	// 2. 通过 resolver 获取 KEK（密钥的 DEK）lease
	lease, err := s.resolver.IssueDEKLease(ctx, req.TenantID, req.KeyID, version)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(lease.DEK)

	// 3. 生成随机 DEK
	plaintextDEK, err := datakey.Generate(algorithm)
	if err != nil {
		return nil, apperrors.Internal(err)
	}

	// 4. 用 KEK 包装 DEK
	wrappedDEK, err := datakey.Wrap(plaintextDEK, lease.DEK)
	if err != nil {
		zeroBytes(plaintextDEK)
		return nil, apperrors.Internal(err)
	}

	// 5. 审计
	s.emitAudit(ctx, req.TenantID, "datakey.generate", req.CallerID, "generate", "key", req.KeyID, "success", req.RequestID, map[string]interface{}{
		"kid":       lease.KID,
		"algorithm": algorithm,
	})

	return &GenerateDataKeyResponse{
		PlaintextDEK: plaintextDEK,
		WrappedDEK:   wrappedDEK,
		KID:          lease.KID,
	}, nil
}

// loadKey 加载密钥并处理错误映射。
//
// 不存在或跨租户访问统一返回 CrossTenantDenied（HA-11，不区分不存在与无权限）。
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

// evaluateEncryptPolicy 评估加密策略。
//
// 策略不存在时允许操作（无策略 = 无限制）；策略评估失败返回 PolicyDenied。
func (s *Service) evaluateEncryptPolicy(ctx context.Context, tenantID, keyID, algorithm string, aad, plaintext []byte, callerID string) error {
	p, err := s.policyRepo.GetByKeyID(ctx, s.db, tenantID, keyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		s.log.Error("get policy failed",
			"error", err,
			"tenant_id", tenantID,
			"key_id", keyID,
		)
		return apperrors.Internal(err)
	}

	result := p.EvaluateEncrypt(domainpolicy.EncryptRequest{
		Algorithm:     algorithm,
		AAD:           aad,
		PlaintextSize: int64(len(plaintext)),
		CallerID:      callerID,
	})
	if !result.Allowed {
		return apperrors.PolicyDenied(result.Reason)
	}
	return nil
}

// evaluateDecryptPolicy 评估解密策略。
//
// 策略不存在时允许操作（无策略 = 无限制）；策略评估失败返回 PolicyDenied。
func (s *Service) evaluateDecryptPolicy(ctx context.Context, tenantID, keyID, algorithm, callerID string) error {
	p, err := s.policyRepo.GetByKeyID(ctx, s.db, tenantID, keyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		s.log.Error("get policy failed",
			"error", err,
			"tenant_id", tenantID,
			"key_id", keyID,
		)
		return apperrors.Internal(err)
	}

	result := p.EvaluateDecrypt(domainpolicy.DecryptRequest{
		Algorithm: algorithm,
		CallerID:  callerID,
	})
	if !result.Allowed {
		return apperrors.PolicyDenied(result.Reason)
	}
	return nil
}

// emitAudit 发射审计事件（fail-open，错误仅记录日志）。
//
// crypto.encrypt / crypto.decrypt / datakey.generate 均非高风险事件，
// 审计写入失败时仅记录日志，不影响业务流程。
func (s *Service) emitAudit(ctx context.Context, tenantID, eventType, actor, action, resourceType, resourceID, result, requestID string, details map[string]interface{}) {
	event := audit.BuildEvent(tenantID, eventType, actor, action, resourceType, resourceID, result, requestID, details)
	if err := s.audit.Emit(ctx, event); err != nil {
		s.log.Error("audit emit failed",
			"error", err,
			"event_type", eventType,
			"tenant_id", tenantID,
		)
	}
}

// statusError 根据密钥状态返回相应的错误。
func statusError(status domainkey.KeyStatus) error {
	switch status {
	case domainkey.StatusDisabled:
		return apperrors.KeyDisabled()
	case domainkey.StatusDestroyed:
		return apperrors.KeyDestroyed()
	default:
		return apperrors.InvalidRequest("key status does not allow this operation")
	}
}

// parseKID 从 KID（key_id/v{version}）中解析出 key_id 和 version。
//
// KID 格式为 key_id/v{version}，使用 LastIndex 定位最后的 /v 段，
// 兼容 key_id 中包含 / 字符的情况。
func parseKID(kid string) (keyID string, version int, err error) {
	idx := strings.LastIndex(kid, "/v")
	if idx < 0 {
		return "", 0, fmt.Errorf("invalid KID format: %s", kid)
	}
	keyID = kid[:idx]
	versionStr := kid[idx+2:]
	v, err := strconv.Atoi(versionStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid version in KID: %s", kid)
	}
	if v <= 0 {
		return "", 0, fmt.Errorf("invalid version in KID: %s", kid)
	}
	return keyID, v, nil
}

// zeroBytes 将 b 清零，使用 runtime.KeepAlive 防止编译器优化掉写入。
//
// 用于在 DEK lease 使用完毕后清除内存中的明文密钥材料。
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}

// validateAlgorithm 校验算法取值。
func validateAlgorithm(alg string) error {
	switch alg {
	case string(domainkey.AlgAES256GCM), string(domainkey.AlgSM4GCM):
		return nil
	default:
		return apperrors.InvalidRequest(fmt.Sprintf("unsupported algorithm: %s", alg))
	}
}
