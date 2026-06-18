// Package node 定义节点领域类型和状态机。
package node

import "time"

// NodeStatus 节点状态，对应第 14.3 节。
type NodeStatus string

const (
	StatusRegistered NodeStatus = "REGISTERED"
	StatusReady      NodeStatus = "READY"
	StatusFrozen     NodeStatus = "FROZEN"
	StatusRevoked    NodeStatus = "REVOKED"
)

// Node 是节点聚合根。
type Node struct {
	ID               string
	TenantID         string
	Hostname         string
	Status           NodeStatus
	ClusterEpoch     int64     // 集群 epoch（HA-08 骨架）
	AttestationEpoch int64     // 证明 epoch（HA-08）
	ReadyReason      string    // "static_registration" | "attestation_epoch:E"
	RegisteredAt     time.Time
	LastSeenAt       time.Time
	RevokedAt        *time.Time
	// P0: service token hash; P1: attestation document
	ServiceTokenHash string
}

// Transition 执行节点状态迁移（第 14.3 节）。
//   REGISTERED -> READY (auth_or_attestation_pass)
//   READY -> FROZEN (freeze)
//   FROZEN -> READY (unfreeze)
//   READY/FROZEN -> REVOKED (revoke)
//   REVOKED -> (terminal)
func (n *Node) Transition(action string) error {
	switch action {
	case "activate":
		if n.Status != StatusRegistered {
			return ErrInvalidTransition("activate", n.Status)
		}
		n.Status = StatusReady
	case "freeze":
		if n.Status != StatusReady {
			return ErrInvalidTransition("freeze", n.Status)
		}
		n.Status = StatusFrozen
	case "unfreeze":
		if n.Status != StatusFrozen {
			return ErrInvalidTransition("unfreeze", n.Status)
		}
		n.Status = StatusReady
	case "revoke":
		if n.Status != StatusReady && n.Status != StatusFrozen {
			return ErrInvalidTransition("revoke", n.Status)
		}
		n.Status = StatusRevoked
		now := time.Now()
		n.RevokedAt = &now
	default:
		return ErrUnknownAction(action)
	}
	return nil
}

// CanServeCrypto 判断节点是否可以服务加密请求。
func (n *Node) CanServeCrypto() bool {
	return n.Status == StatusReady
}

type TransitionError struct {
	Action  string
	Current NodeStatus
}

func (e TransitionError) Error() string {
	return "invalid node transition: action=" + e.Action + " current=" + string(e.Current)
}

func ErrInvalidTransition(action string, current NodeStatus) TransitionError {
	return TransitionError{Action: action, Current: current}
}

type UnknownActionError struct{ Action string }

func (e UnknownActionError) Error() string { return "unknown action: " + e.Action }

func ErrUnknownAction(action string) UnknownActionError {
	return UnknownActionError{Action: action}
}
