// Package key 定义密钥领域的核心类型和状态机。
package key

import (
	"time"
)

// KeyStatus 密钥状态，对应第 14.1 节状态机。
type KeyStatus string

const (
	StatusActive    KeyStatus = "ACTIVE"
	StatusDisabled  KeyStatus = "DISABLED"
	StatusDestroyed KeyStatus = "DESTROYED"
)

// KeyAlgorithm 密钥算法套件。
type KeyAlgorithm string

const (
	AlgAES256GCM KeyAlgorithm = "AES_256_GCM"
	AlgSM4GCM    KeyAlgorithm = "SM4_128_GCM"
)

// KeyPurpose 密钥用途。
type KeyPurpose string

const (
	PurposeDataEncryption KeyPurpose = "data_encryption"
	PurposeKeyWrapping    KeyPurpose = "key_wrapping"
)

// Key 是密钥聚合根。
type Key struct {
	ID            string
	TenantID      string
	Name          string
	Algorithm     KeyAlgorithm
	Purpose       KeyPurpose
	Status        KeyStatus
	CurrentVersion int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DestroyedAt   *time.Time
	// ready_reason 区分准入依据（HA-08），P0 为 "static_registration"
	ReadyReason   string
}

// KeyVersion 是密钥版本，包含 wrapped DEK。
type KeyVersion struct {
	ID            string
	KeyID         string
	Version       int
	WrappedDEK    []byte    // Envelope v1 格式
	DEKKeyID      string    // 用于 cache key
	KID           string    // 对外暴露的 key_id/version 标识
	CreatedAt     time.Time
	CreatedBy     string
	RotationReason string   // "initial" | "scheduled" | "manual" | "compromise"
}

// CanEncrypt 判断当前状态是否允许加密。
func (k *Key) CanEncrypt() bool {
	return k.Status == StatusActive
}

// CanDecrypt 判断当前状态是否允许解密。
func (k *Key) CanDecrypt() bool {
	return k.Status == StatusActive || k.Status == StatusDisabled
}

// CanRotate 判断当前状态是否允许轮转。
func (k *Key) CanRotate() bool {
	return k.Status == StatusActive
}

// CanDisable 判断当前状态是否允许禁用。
func (k *Key) CanDisable() bool {
	return k.Status == StatusActive
}

// CanDestroy 判断当前状态是否允许销毁。
func (k *Key) CanDestroy() bool {
	return k.Status == StatusActive || k.Status == StatusDisabled
}

// Transition 执行状态迁移，返回新状态或错误。
// 状态机规则（第 14.1 节）：
//   ACTIVE -> DISABLED (disable)
//   ACTIVE -> DESTROYED (destroy)
//   DISABLED -> ACTIVE (enable)
//   DISABLED -> DESTROYED (destroy)
//   DESTROYED -> (terminal)
func (k *Key) Transition(action string) error {
	switch action {
	case "disable":
		if k.Status != StatusActive {
			return ErrInvalidTransition("disable", k.Status)
		}
		k.Status = StatusDisabled
	case "enable":
		if k.Status != StatusDisabled {
			return ErrInvalidTransition("enable", k.Status)
		}
		k.Status = StatusActive
	case "destroy":
		if k.Status != StatusActive && k.Status != StatusDisabled {
			return ErrInvalidTransition("destroy", k.Status)
		}
		k.Status = StatusDestroyed
		now := time.Now()
		k.DestroyedAt = &now
	default:
		return ErrUnknownAction(action)
	}
	k.UpdatedAt = time.Now()
	return nil
}

// TransitionError 状态迁移错误。
type TransitionError struct {
	Action  string
	Current KeyStatus
}

func (e TransitionError) Error() string {
	return "invalid state transition: action=" + e.Action + " current=" + string(e.Current)
}

func ErrInvalidTransition(action string, current KeyStatus) TransitionError {
	return TransitionError{Action: action, Current: current}
}

type UnknownActionError struct{ Action string }

func (e UnknownActionError) Error() string { return "unknown action: " + e.Action }

func ErrUnknownAction(action string) UnknownActionError {
	return UnknownActionError{Action: action}
}
