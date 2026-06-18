package key

import (
	"testing"
	"time"
)

// newTestKey 构造一个 ACTIVE 状态的测试 Key。
func newTestKey(status KeyStatus) *Key {
	return &Key{
		ID:         "key-1",
		TenantID:   "tenant-1",
		Name:       "test-key",
		Algorithm:  AlgAES256GCM,
		Purpose:    PurposeDataEncryption,
		Status:     status,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// TestKeyCanEncrypt 验证 ACTIVE 可加密，DISABLED/DESTROYED 不可。
func TestKeyCanEncrypt(t *testing.T) {
	cases := []struct {
		status KeyStatus
		want   bool
	}{
		{StatusActive, true},
		{StatusDisabled, false},
		{StatusDestroyed, false},
	}
	for _, c := range cases {
		k := newTestKey(c.status)
		if got := k.CanEncrypt(); got != c.want {
			t.Errorf("状态 %s CanEncrypt = %v, want %v", c.status, got, c.want)
		}
	}
}

// TestKeyCanDecrypt 验证 ACTIVE/DISABLED 可解密，DESTROYED 不可。
func TestKeyCanDecrypt(t *testing.T) {
	cases := []struct {
		status KeyStatus
		want   bool
	}{
		{StatusActive, true},
		{StatusDisabled, true},
		{StatusDestroyed, false},
	}
	for _, c := range cases {
		k := newTestKey(c.status)
		if got := k.CanDecrypt(); got != c.want {
			t.Errorf("状态 %s CanDecrypt = %v, want %v", c.status, got, c.want)
		}
	}
}

// TestKeyCanRotate 验证仅 ACTIVE 可轮转。
func TestKeyCanRotate(t *testing.T) {
	cases := []struct {
		status KeyStatus
		want   bool
	}{
		{StatusActive, true},
		{StatusDisabled, false},
		{StatusDestroyed, false},
	}
	for _, c := range cases {
		k := newTestKey(c.status)
		if got := k.CanRotate(); got != c.want {
			t.Errorf("状态 %s CanRotate = %v, want %v", c.status, got, c.want)
		}
	}
}

// TestKeyCanDisable 验证仅 ACTIVE 可禁用。
func TestKeyCanDisable(t *testing.T) {
	cases := []struct {
		status KeyStatus
		want   bool
	}{
		{StatusActive, true},
		{StatusDisabled, false},
		{StatusDestroyed, false},
	}
	for _, c := range cases {
		k := newTestKey(c.status)
		if got := k.CanDisable(); got != c.want {
			t.Errorf("状态 %s CanDisable = %v, want %v", c.status, got, c.want)
		}
	}
}

// TestKeyCanDestroy 验证 ACTIVE/DISABLED 可销毁，DESTROYED 不可。
func TestKeyCanDestroy(t *testing.T) {
	cases := []struct {
		status KeyStatus
		want   bool
	}{
		{StatusActive, true},
		{StatusDisabled, true},
		{StatusDestroyed, false},
	}
	for _, c := range cases {
		k := newTestKey(c.status)
		if got := k.CanDestroy(); got != c.want {
			t.Errorf("状态 %s CanDestroy = %v, want %v", c.status, got, c.want)
		}
	}
}

// TestTransitionActiveToDisabled 验证 ACTIVE -> DISABLED。
func TestTransitionActiveToDisabled(t *testing.T) {
	k := newTestKey(StatusActive)
	if err := k.Transition("disable"); err != nil {
		t.Fatalf("disable 失败: %v", err)
	}
	if k.Status != StatusDisabled {
		t.Fatalf("状态应为 DISABLED, got %s", k.Status)
	}
}

// TestTransitionDisabledToActive 验证 DISABLED -> ACTIVE。
func TestTransitionDisabledToActive(t *testing.T) {
	k := newTestKey(StatusDisabled)
	if err := k.Transition("enable"); err != nil {
		t.Fatalf("enable 失败: %v", err)
	}
	if k.Status != StatusActive {
		t.Fatalf("状态应为 ACTIVE, got %s", k.Status)
	}
}

// TestTransitionActiveToDestroyed 验证 ACTIVE -> DESTROYED。
func TestTransitionActiveToDestroyed(t *testing.T) {
	k := newTestKey(StatusActive)
	if err := k.Transition("destroy"); err != nil {
		t.Fatalf("destroy 失败: %v", err)
	}
	if k.Status != StatusDestroyed {
		t.Fatalf("状态应为 DESTROYED, got %s", k.Status)
	}
	if k.DestroyedAt == nil {
		t.Fatal("DestroyedAt 应被设置")
	}
}

// TestTransitionDisabledToDestroyed 验证 DISABLED -> DESTROYED。
func TestTransitionDisabledToDestroyed(t *testing.T) {
	k := newTestKey(StatusDisabled)
	if err := k.Transition("destroy"); err != nil {
		t.Fatalf("destroy 失败: %v", err)
	}
	if k.Status != StatusDestroyed {
		t.Fatalf("状态应为 DESTROYED, got %s", k.Status)
	}
	if k.DestroyedAt == nil {
		t.Fatal("DestroyedAt 应被设置")
	}
}

// TestTransitionDestroyedTerminal 验证 DESTROYED 不可迁移。
func TestTransitionDestroyedTerminal(t *testing.T) {
	k := newTestKey(StatusDestroyed)

	// 任何动作都应失败
	for _, action := range []string{"disable", "enable", "destroy"} {
		kCopy := newTestKey(StatusDestroyed)
		if err := kCopy.Transition(action); err == nil {
			t.Errorf("DESTROYED 状态下执行 %q 应失败", action)
		}
	}

	// 直接对 k 测试一次 disable
	if err := k.Transition("disable"); err == nil {
		t.Fatal("DESTROYED -> disable 应失败")
	}
}

// TestTransitionInvalidAction 验证无效动作应返回错误。
func TestTransitionInvalidAction(t *testing.T) {
	k := newTestKey(StatusActive)
	err := k.Transition("invalid_action")
	if err == nil {
		t.Fatal("无效动作应返回错误")
	}
	if _, ok := err.(UnknownActionError); !ok {
		t.Fatalf("应返回 UnknownActionError, got %T", err)
	}
}

// TestTransitionInvalidSourceState 验证从非法源状态迁移应返回错误。
func TestTransitionInvalidSourceState(t *testing.T) {
	// DISABLED 不能 disable
	k := newTestKey(StatusDisabled)
	err := k.Transition("disable")
	if err == nil {
		t.Fatal("DISABLED -> disable 应失败")
	}
	if _, ok := err.(TransitionError); !ok {
		t.Fatalf("应返回 TransitionError, got %T", err)
	}

	// ACTIVE 不能 enable
	k = newTestKey(StatusActive)
	if err := k.Transition("enable"); err == nil {
		t.Fatal("ACTIVE -> enable 应失败")
	}

	// DESTROYED 不能 destroy（已 terminal）
	k = newTestKey(StatusDestroyed)
	if err := k.Transition("destroy"); err == nil {
		t.Fatal("DESTROYED -> destroy 应失败")
	}
}

// TestTransitionUpdatesTimestamp 验证迁移会更新 UpdatedAt。
func TestTransitionUpdatesTimestamp(t *testing.T) {
	k := newTestKey(StatusActive)
	originalUpdatedAt := k.UpdatedAt

	// 等待一小段时间确保时间戳不同
	time.Sleep(time.Millisecond)
	if err := k.Transition("disable"); err != nil {
		t.Fatalf("disable 失败: %v", err)
	}
	if !k.UpdatedAt.After(originalUpdatedAt) {
		t.Fatal("UpdatedAt 应在迁移后被更新")
	}
}

// TestTransitionFullLifecycle 验证完整生命周期：ACTIVE -> DISABLED -> ACTIVE -> DESTROYED。
func TestTransitionFullLifecycle(t *testing.T) {
	k := newTestKey(StatusActive)

	// ACTIVE -> DISABLED
	if err := k.Transition("disable"); err != nil {
		t.Fatalf("disable 失败: %v", err)
	}
	if k.Status != StatusDisabled {
		t.Fatalf("状态应为 DISABLED, got %s", k.Status)
	}

	// DISABLED -> ACTIVE
	if err := k.Transition("enable"); err != nil {
		t.Fatalf("enable 失败: %v", err)
	}
	if k.Status != StatusActive {
		t.Fatalf("状态应为 ACTIVE, got %s", k.Status)
	}

	// ACTIVE -> DESTROYED
	if err := k.Transition("destroy"); err != nil {
		t.Fatalf("destroy 失败: %v", err)
	}
	if k.Status != StatusDestroyed {
		t.Fatalf("状态应为 DESTROYED, got %s", k.Status)
	}
	if k.DestroyedAt == nil {
		t.Fatal("DestroyedAt 应被设置")
	}
}
