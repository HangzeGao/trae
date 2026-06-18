// Package provider 定义 TPM 抽象接口及其软件模拟实现。
//
// 该包对应设计文档第 6 章「TPM Provider 层」，向上层（envelope/datakey
// resolver）提供统一的 SealDEK/UnsealDEK/Quote 等语义，屏蔽底层 TPM 硬件
// 与软件模拟之间的差异。P0 阶段仅提供 SoftwareProvider，P1 阶段可替换为
// 基于 google/go-tpm 的真实硬件实现。
package provider

import (
	"context"
	"errors"
	"fmt"
)

// Provider 是 TPM 抽象接口（第 6 章）。
//
// 实现方需保证：
//   - SealDEK/UnsealDEK 互为逆操作，密封产物可持久化到数据库
//   - UnsealDEK 受并发信号量限制（HA-12），避免 TPM 资源争用
//   - 所有可恢复错误返回 *TPMError 并标注 ErrorKind，便于上层重试决策
type Provider interface {
	// Init 初始化 TPM，创建/加载 NRWK 和 AK。
	Init(ctx context.Context) error

	// SealDEK 用 TPM 父密钥（NRWK）密封 DEK。
	// 返回密封后的 blob（可持久化到数据库）。
	SealDEK(ctx context.Context, dek []byte) (sealed []byte, err error)

	// UnsealDEK 解封 DEK。
	// 从密封 blob 还原 DEK 明文。
	UnsealDEK(ctx context.Context, sealed []byte) (dek []byte, err error)

	// GenerateAndSealDEK 生成随机 DEK 并密封。
	// algorithm: "AES_256_GCM" (32字节) 或 "SM4_128_GCM" (16字节)
	GenerateAndSealDEK(ctx context.Context, algorithm string) (sealed []byte, dekKID string, err error)

	// GetNRWKPublicKey 获取 NRWK 公钥（用于证明和验证）。
	GetNRWKPublicKey(ctx context.Context) ([]byte, error)

	// Quote 生成 TPM Quote（P1 证明用，P0 返回占位）。
	Quote(ctx context.Context, nonce []byte) ([]byte, error)

	// Close 关闭 TPM 会话。
	Close() error
}

// ErrorKind 标识 TPM 错误的分类（第 6.4 节），用于上层重试决策。
type ErrorKind int

const (
	// ErrKindTransient 可重试错误：资源忙、超时等瞬时故障。
	ErrKindTransient ErrorKind = iota
	// ErrKindPermanent 不可重试错误：句柄不存在、策略不满足等。
	ErrKindPermanent
	// ErrKindFatal 致命错误：TPM 不可用、硬件故障等。
	ErrKindFatal
)

// String 返回 ErrorKind 的可读表示，便于日志输出。
func (k ErrorKind) String() string {
	switch k {
	case ErrKindTransient:
		return "transient"
	case ErrKindPermanent:
		return "permanent"
	case ErrKindFatal:
		return "fatal"
	default:
		return fmt.Sprintf("unknown(%d)", int(k))
	}
}

// TPMError 是 TPM Provider 层的统一错误类型（第 6.4 节）。
//
// 通过 ErrorKind 区分可重试/不可重试/致命错误，调用方使用
// IsTransient/IsPermanent/IsFatal 进行分类判断。
type TPMError struct {
	// Kind 错误分类。
	Kind ErrorKind
	// Op 触发错误的操作名（如 "SealDEK"、"UnsealDEK"）。
	Op string
	// Cause 原始错误。
	Cause error
}

// Error 实现 error 接口。
func (e *TPMError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("tpm %s: %s: %v", e.Op, e.Kind, e.Cause)
	}
	return fmt.Sprintf("tpm %s: %s", e.Op, e.Kind)
}

// Unwrap 暴露底层错误，支持 errors.Is/As 链式判断。
func (e *TPMError) Unwrap() error {
	return e.Cause
}

// NewTPMError 构造一个 TPMError。
func NewTPMError(kind ErrorKind, op string, cause error) *TPMError {
	return &TPMError{Kind: kind, Op: op, Cause: cause}
}

// IsTransient 判断错误是否为可重试的瞬时错误。
func IsTransient(err error) bool {
	var e *TPMError
	if errors.As(err, &e) {
		return e.Kind == ErrKindTransient
	}
	return false
}

// IsPermanent 判断错误是否为不可重试的永久错误。
func IsPermanent(err error) bool {
	var e *TPMError
	if errors.As(err, &e) {
		return e.Kind == ErrKindPermanent
	}
	return false
}

// IsFatal 判断错误是否为致命错误（TPM 不可用、硬件故障等）。
func IsFatal(err error) bool {
	var e *TPMError
	if errors.As(err, &e) {
		return e.Kind == ErrKindFatal
	}
	return false
}
