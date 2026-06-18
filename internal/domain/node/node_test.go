package node

import (
	"testing"
	"time"
)

// newTestNode 构造一个指定状态的测试 Node。
func newTestNode(status NodeStatus) *Node {
	return &Node{
		ID:           "node-1",
		TenantID:     "tenant-1",
		Hostname:     "host-1",
		Status:       status,
		ClusterEpoch: 1,
		RegisteredAt: time.Now(),
		LastSeenAt:   time.Now(),
	}
}

// TestNodeTransitionRegisteredToReady 验证 REGISTERED -> READY。
func TestNodeTransitionRegisteredToReady(t *testing.T) {
	n := newTestNode(StatusRegistered)
	if err := n.Transition("activate"); err != nil {
		t.Fatalf("activate 失败: %v", err)
	}
	if n.Status != StatusReady {
		t.Fatalf("状态应为 READY, got %s", n.Status)
	}
}

// TestNodeTransitionReadyToFrozen 验证 READY -> FROZEN。
func TestNodeTransitionReadyToFrozen(t *testing.T) {
	n := newTestNode(StatusReady)
	if err := n.Transition("freeze"); err != nil {
		t.Fatalf("freeze 失败: %v", err)
	}
	if n.Status != StatusFrozen {
		t.Fatalf("状态应为 FROZEN, got %s", n.Status)
	}
}

// TestNodeTransitionFrozenToReady 验证 FROZEN -> READY。
func TestNodeTransitionFrozenToReady(t *testing.T) {
	n := newTestNode(StatusFrozen)
	if err := n.Transition("unfreeze"); err != nil {
		t.Fatalf("unfreeze 失败: %v", err)
	}
	if n.Status != StatusReady {
		t.Fatalf("状态应为 READY, got %s", n.Status)
	}
}

// TestNodeTransitionToRevoked 验证 READY/FROZEN -> REVOKED。
func TestNodeTransitionToRevoked(t *testing.T) {
	// READY -> REVOKED
	n := newTestNode(StatusReady)
	if err := n.Transition("revoke"); err != nil {
		t.Fatalf("READY -> revoke 失败: %v", err)
	}
	if n.Status != StatusRevoked {
		t.Fatalf("状态应为 REVOKED, got %s", n.Status)
	}
	if n.RevokedAt == nil {
		t.Fatal("READY -> revoke 后 RevokedAt 应被设置")
	}

	// FROZEN -> REVOKED
	n = newTestNode(StatusFrozen)
	if err := n.Transition("revoke"); err != nil {
		t.Fatalf("FROZEN -> revoke 失败: %v", err)
	}
	if n.Status != StatusRevoked {
		t.Fatalf("状态应为 REVOKED, got %s", n.Status)
	}
	if n.RevokedAt == nil {
		t.Fatal("FROZEN -> revoke 后 RevokedAt 应被设置")
	}
}

// TestNodeTransitionRevokedTerminal 验证 REVOKED 不可迁移。
func TestNodeTransitionRevokedTerminal(t *testing.T) {
	for _, action := range []string{"activate", "freeze", "unfreeze", "revoke"} {
		n := newTestNode(StatusRevoked)
		if err := n.Transition(action); err == nil {
			t.Errorf("REVOKED 状态下执行 %q 应失败", action)
		}
	}
}

// TestNodeCanServeCrypto 验证仅 READY 可服务。
func TestNodeCanServeCrypto(t *testing.T) {
	cases := []struct {
		status NodeStatus
		want   bool
	}{
		{StatusRegistered, false},
		{StatusReady, true},
		{StatusFrozen, false},
		{StatusRevoked, false},
	}
	for _, c := range cases {
		n := newTestNode(c.status)
		if got := n.CanServeCrypto(); got != c.want {
			t.Errorf("状态 %s CanServeCrypto = %v, want %v", c.status, got, c.want)
		}
	}
}

// TestNodeTransitionInvalidAction 验证无效动作应返回错误。
func TestNodeTransitionInvalidAction(t *testing.T) {
	n := newTestNode(StatusReady)
	err := n.Transition("invalid_action")
	if err == nil {
		t.Fatal("无效动作应返回错误")
	}
	if _, ok := err.(UnknownActionError); !ok {
		t.Fatalf("应返回 UnknownActionError, got %T", err)
	}
}

// TestNodeTransitionInvalidSourceState 验证从非法源状态迁移应返回错误。
func TestNodeTransitionInvalidSourceState(t *testing.T) {
	// REGISTERED 不能 freeze
	n := newTestNode(StatusRegistered)
	err := n.Transition("freeze")
	if err == nil {
		t.Fatal("REGISTERED -> freeze 应失败")
	}
	if _, ok := err.(TransitionError); !ok {
		t.Fatalf("应返回 TransitionError, got %T", err)
	}

	// READY 不能 activate
	n = newTestNode(StatusReady)
	if err := n.Transition("activate"); err == nil {
		t.Fatal("READY -> activate 应失败")
	}

	// FROZEN 不能 freeze
	n = newTestNode(StatusFrozen)
	if err := n.Transition("freeze"); err == nil {
		t.Fatal("FROZEN -> freeze 应失败")
	}

	// REGISTERED 不能 revoke（仅 READY/FROZEN 可 revoke）
	n = newTestNode(StatusRegistered)
	if err := n.Transition("revoke"); err == nil {
		t.Fatal("REGISTERED -> revoke 应失败")
	}
}

// TestNodeTransitionFullLifecycle 验证完整生命周期：
// REGISTERED -> READY -> FROZEN -> READY -> REVOKED。
func TestNodeTransitionFullLifecycle(t *testing.T) {
	n := newTestNode(StatusRegistered)

	// REGISTERED -> READY
	if err := n.Transition("activate"); err != nil {
		t.Fatalf("activate 失败: %v", err)
	}
	if n.Status != StatusReady {
		t.Fatalf("状态应为 READY, got %s", n.Status)
	}
	if !n.CanServeCrypto() {
		t.Fatal("READY 状态应可服务")
	}

	// READY -> FROZEN
	if err := n.Transition("freeze"); err != nil {
		t.Fatalf("freeze 失败: %v", err)
	}
	if n.Status != StatusFrozen {
		t.Fatalf("状态应为 FROZEN, got %s", n.Status)
	}
	if n.CanServeCrypto() {
		t.Fatal("FROZEN 状态不应可服务")
	}

	// FROZEN -> READY
	if err := n.Transition("unfreeze"); err != nil {
		t.Fatalf("unfreeze 失败: %v", err)
	}
	if n.Status != StatusReady {
		t.Fatalf("状态应为 READY, got %s", n.Status)
	}

	// READY -> REVOKED
	if err := n.Transition("revoke"); err != nil {
		t.Fatalf("revoke 失败: %v", err)
	}
	if n.Status != StatusRevoked {
		t.Fatalf("状态应为 REVOKED, got %s", n.Status)
	}
	if n.RevokedAt == nil {
		t.Fatal("RevokedAt 应被设置")
	}
	if n.CanServeCrypto() {
		t.Fatal("REVOKED 状态不应可服务")
	}
}
