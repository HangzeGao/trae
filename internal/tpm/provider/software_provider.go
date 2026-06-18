package provider

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	"github.com/HangzeGao/trae/key-vault/internal/config"
	"github.com/HangzeGao/trae/key-vault/internal/crypto/datakey"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// 默认 TPM 解封并发上限（HA-12）。
const defaultMaxConcurrency = 2

// sealKeySize 是模拟 NRWK 的密封密钥长度（AES-256-GCM 要求 32 字节）。
const sealKeySize = 32

// nrwkPubKeySize 是模拟 NRWK 公钥的长度。
const nrwkPubKeySize = 32

// SoftwareProvider 是不依赖真实 TPM 的软件模拟实现。
//
// 使用 AES-256-GCM 作为密封机制，模拟 TPM 的密封/解封语义。
// P0 用于集成测试和开发，P1 替换为真实 TPM 实现。
//
// 该实现通过 datakey.Wrap/Unwrap 复用已有的 AEAD 加解密逻辑：
//   - SealDEK 等价于 datakey.Wrap(dek, sealKey)
//   - UnsealDEK 等价于 datakey.Unwrap(sealed, sealKey)
//
// UnsealDEK 受信号量限制并发（HA-12），避免在真实 TPM 场景下出现资源争用。
type SoftwareProvider struct {
	mu             sync.Mutex
	sealKey        []byte // 模拟 NRWK 的密封密钥
	nrwkPubKey     []byte // 模拟 NRWK 公钥
	handle         uint32
	initialized    bool
	log            *observability.Logger
	semaphore      chan struct{} // TPM 解封并发上限（HA-12）
	maxConcurrency int
}

// NewSoftwareProvider 构造一个软件模拟 Provider。
//
// cfg.NRWKHandle 用于标识模拟的 NRWK 持久句柄；maxConcurrency 默认 2
// （HA-12），通过内置信号量限制 UnsealDEK 并发。
func NewSoftwareProvider(cfg config.TPMConfig, log *observability.Logger) *SoftwareProvider {
	maxConcurrency := defaultMaxConcurrency
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	return &SoftwareProvider{
		handle:         cfg.NRWKHandle,
		maxConcurrency: maxConcurrency,
		semaphore:      make(chan struct{}, maxConcurrency),
		log:            log,
	}
}

// Init 生成或加载密封密钥。
//
// P0 阶段每次启动都重新生成随机的 sealKey 与 nrwkPubKey（进程内有效），
// 这意味着重启后旧的密封 blob 将无法解封；P1 阶段将从 TPM 持久句柄加载。
func (p *SoftwareProvider) Init(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	sealKey := make([]byte, sealKeySize)
	if _, err := rand.Read(sealKey); err != nil {
		return NewTPMError(ErrKindFatal, "Init", fmt.Errorf("generate seal key: %w", err))
	}

	nrwkPubKey := make([]byte, nrwkPubKeySize)
	if _, err := rand.Read(nrwkPubKey); err != nil {
		return NewTPMError(ErrKindFatal, "Init", fmt.Errorf("generate NRWK public key: %w", err))
	}

	p.sealKey = sealKey
	p.nrwkPubKey = nrwkPubKey
	p.initialized = true

	if p.log != nil {
		p.log.Info("software TPM provider initialized",
			"nrwk_handle", fmt.Sprintf("0x%08x", p.handle),
			"max_concurrency", p.maxConcurrency,
		)
	}
	return nil
}

// SealDEK 使用 AES-256-GCM 密封 DEK。
//
// 密封格式: nonce(12) || ciphertext（含 GCM tag）。
// 使用 sealKey 作为 KEK，等价于 datakey.Wrap(dek, sealKey)。
func (p *SoftwareProvider) SealDEK(ctx context.Context, dek []byte) ([]byte, error) {
	if err := p.requireInit(); err != nil {
		return nil, err
	}
	if len(dek) == 0 {
		return nil, NewTPMError(ErrKindPermanent, "SealDEK", errors.New("empty DEK"))
	}

	p.mu.Lock()
	sealKey := p.sealKey
	p.mu.Unlock()

	sealed, err := datakey.Wrap(dek, sealKey)
	if err != nil {
		return nil, NewTPMError(ErrKindPermanent, "SealDEK", err)
	}
	return sealed, nil
}

// UnsealDEK 解封 DEK。
//
// 受并发信号量限制（HA-12: TPMUnsealConcurrency 默认 2），
// 避免在真实 TPM 场景下出现资源争用。解封失败时记录 metrics。
func (p *SoftwareProvider) UnsealDEK(ctx context.Context, sealed []byte) ([]byte, error) {
	if err := p.requireInit(); err != nil {
		return nil, err
	}

	// 获取信号量，限制并发解封数量（HA-12）。
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, NewTPMError(ErrKindTransient, "UnsealDEK", ctx.Err())
	}

	metrics := observability.MetricsInstance()
	metrics.IncTPMUnseal()

	p.mu.Lock()
	sealKey := p.sealKey
	p.mu.Unlock()

	dek, err := datakey.Unwrap(sealed, sealKey)
	if err != nil {
		metrics.IncTPMUnsealError()
		return nil, NewTPMError(ErrKindPermanent, "UnsealDEK", err)
	}
	return dek, nil
}

// GenerateAndSealDEK 生成随机 DEK 并密封。
//
// algorithm 取值：
//   - "AES_256_GCM"：生成 32 字节 DEK
//   - "SM4_128_GCM"：生成 16 字节 DEK
//
// dekKID 为 UUID v4 格式，使用 crypto/rand 生成（项目未引入 google/uuid）。
func (p *SoftwareProvider) GenerateAndSealDEK(ctx context.Context, algorithm string) ([]byte, string, error) {
	if err := p.requireInit(); err != nil {
		return nil, "", err
	}

	dek, err := datakey.Generate(algorithm)
	if err != nil {
		return nil, "", NewTPMError(ErrKindPermanent, "GenerateAndSealDEK", err)
	}

	sealed, err := p.SealDEK(ctx, dek)
	if err != nil {
		return nil, "", err
	}

	kid, err := newUUIDv4()
	if err != nil {
		return nil, "", NewTPMError(ErrKindFatal, "GenerateAndSealDEK", err)
	}
	return sealed, kid, nil
}

// GetNRWKPublicKey 获取 NRWK 公钥（用于证明和验证）。
//
// P0 返回进程启动时生成的占位公钥；P1 返回真实 NRWK 公钥。
func (p *SoftwareProvider) GetNRWKPublicKey(ctx context.Context) ([]byte, error) {
	if err := p.requireInit(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]byte, len(p.nrwkPubKey))
	copy(out, p.nrwkPubKey)
	return out, nil
}

// Quote 生成 TPM Quote（P1 证明用，P0 返回占位）。
//
// P0 阶段不依赖真实 TPM，返回 nonce 的副本作为占位 Quote，
// 仅供集成测试调用路径使用，不具备远程证明语义。
func (p *SoftwareProvider) Quote(ctx context.Context, nonce []byte) ([]byte, error) {
	if err := p.requireInit(); err != nil {
		return nil, err
	}
	out := make([]byte, len(nonce))
	copy(out, nonce)
	return out, nil
}

// Close 关闭 TPM 会话。
//
// 软件模拟实现释放内存中的密封密钥并标记为未初始化。
func (p *SoftwareProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.sealKey {
		p.sealKey[i] = 0
	}
	p.sealKey = nil
	p.nrwkPubKey = nil
	p.initialized = false
	return nil
}

// requireInit 检查 Provider 是否已初始化。
func (p *SoftwareProvider) requireInit() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.initialized {
		return NewTPMError(ErrKindFatal, "Init", errors.New("provider not initialized"))
	}
	return nil
}

// newUUIDv4 使用 crypto/rand 生成 UUID v4 字符串。
//
// 格式: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx，其中 y ∈ {8,9,a,b}。
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// 设置 version 4 与 variant 位。
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
