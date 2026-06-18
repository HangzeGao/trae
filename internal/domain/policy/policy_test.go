package policy

import (
	"testing"
)

// TestEvaluateEncryptAllowed 验证符合策略允许加密。
func TestEvaluateEncryptAllowed(t *testing.T) {
	p := &Policy{
		ID:               "policy-1",
		TenantID:         "tenant-1",
		KeyID:            "key-1",
		MinAlgorithm:     "AES_256_GCM",
		RequireAAD:       true,
		MaxPlaintextSize: 1024,
		AllowedCallers:   []string{"caller-1", "caller-2"},
	}

	req := EncryptRequest{
		Algorithm:     "AES_256_GCM",
		AAD:           []byte("valid-aad"),
		PlaintextSize: 100,
		CallerID:      "caller-1",
	}

	result := p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("符合策略应允许加密, got denied: %s", result.Reason)
	}
}

// TestEvaluateEncryptAADRequired 验证 RequireAAD=true 且无 AAD 应拒绝。
func TestEvaluateEncryptAADRequired(t *testing.T) {
	p := &Policy{
		ID:               "policy-2",
		TenantID:         "tenant-1",
		KeyID:            "key-2",
		MinAlgorithm:     "AES_256_GCM",
		RequireAAD:       true,
		MaxPlaintextSize: 1024,
		AllowedCallers:   []string{"caller-1"},
	}

	// 无 AAD 应拒绝
	req := EncryptRequest{
		Algorithm:     "AES_256_GCM",
		AAD:           nil,
		PlaintextSize: 100,
		CallerID:      "caller-1",
	}
	result := p.EvaluateEncrypt(req)
	if result.Allowed {
		t.Fatal("RequireAAD=true 且无 AAD 应拒绝")
	}
	if result.Reason == "" {
		t.Fatal("拒绝时应提供原因")
	}

	// 空 AAD 也应拒绝
	req.AAD = []byte{}
	result = p.EvaluateEncrypt(req)
	if result.Allowed {
		t.Fatal("RequireAAD=true 且空 AAD 应拒绝")
	}

	// 有 AAD 应允许
	req.AAD = []byte("valid-aad")
	result = p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("有 AAD 应允许, got denied: %s", result.Reason)
	}

	// RequireAAD=false 时无 AAD 应允许
	p.RequireAAD = false
	req.AAD = nil
	result = p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("RequireAAD=false 时无 AAD 应允许, got denied: %s", result.Reason)
	}
}

// TestEvaluateEncryptMaxSize 验证超过 MaxPlaintextSize 应拒绝。
func TestEvaluateEncryptMaxSize(t *testing.T) {
	p := &Policy{
		ID:               "policy-3",
		TenantID:         "tenant-1",
		KeyID:            "key-3",
		MinAlgorithm:     "AES_256_GCM",
		RequireAAD:       false,
		MaxPlaintextSize: 1024,
		AllowedCallers:   []string{"caller-1"},
	}

	// 刚好等于上限应允许
	req := EncryptRequest{
		Algorithm:     "AES_256_GCM",
		AAD:           nil,
		PlaintextSize: 1024,
		CallerID:      "caller-1",
	}
	result := p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("等于上限应允许, got denied: %s", result.Reason)
	}

	// 超过上限应拒绝
	req.PlaintextSize = 1025
	result = p.EvaluateEncrypt(req)
	if result.Allowed {
		t.Fatal("超过 MaxPlaintextSize 应拒绝")
	}
	if result.Reason == "" {
		t.Fatal("拒绝时应提供原因")
	}

	// MaxPlaintextSize=0 表示不限制
	p.MaxPlaintextSize = 0
	req.PlaintextSize = 1 << 30 // 1GB
	result = p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("MaxPlaintextSize=0 应不限制大小, got denied: %s", result.Reason)
	}
}

// TestEvaluateEncryptCallerNotAllowed 验证不在 AllowedCallers 中应拒绝。
func TestEvaluateEncryptCallerNotAllowed(t *testing.T) {
	p := &Policy{
		ID:               "policy-4",
		TenantID:         "tenant-1",
		KeyID:            "key-4",
		MinAlgorithm:     "AES_256_GCM",
		RequireAAD:       false,
		MaxPlaintextSize: 1024,
		AllowedCallers:   []string{"caller-1", "caller-2"},
	}

	// 在列表中应允许
	req := EncryptRequest{
		Algorithm:     "AES_256_GCM",
		AAD:           nil,
		PlaintextSize: 100,
		CallerID:      "caller-1",
	}
	result := p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("在 AllowedCallers 中应允许, got denied: %s", result.Reason)
	}

	// 不在列表中应拒绝
	req.CallerID = "caller-3"
	result = p.EvaluateEncrypt(req)
	if result.Allowed {
		t.Fatal("不在 AllowedCallers 中应拒绝")
	}
	if result.Reason == "" {
		t.Fatal("拒绝时应提供原因")
	}

	// 空 CallerID 应拒绝
	req.CallerID = ""
	result = p.EvaluateEncrypt(req)
	if result.Allowed {
		t.Fatal("空 CallerID 应拒绝（不在列表中）")
	}

	// AllowedCallers 为空表示不限制
	p.AllowedCallers = nil
	req.CallerID = "any-caller"
	result = p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("AllowedCallers 为空应不限制调用方, got denied: %s", result.Reason)
	}
}

// TestEvaluateDecryptAllowed 验证解密默认允许。
func TestEvaluateDecryptAllowed(t *testing.T) {
	p := &Policy{
		ID:               "policy-5",
		TenantID:         "tenant-1",
		KeyID:            "key-5",
		MinAlgorithm:     "AES_256_GCM",
		RequireAAD:       true,
		MaxPlaintextSize: 1024,
		AllowedCallers:   []string{"caller-1"},
	}

	// 即使加密策略严格，解密也应默认允许
	req := DecryptRequest{
		Algorithm: "AES_256_GCM",
		CallerID:  "any-caller",
	}
	result := p.EvaluateDecrypt(req)
	if !result.Allowed {
		t.Fatalf("解密应默认允许, got denied: %s", result.Reason)
	}

	// 不同调用方也应允许
	req.CallerID = "another-caller"
	result = p.EvaluateDecrypt(req)
	if !result.Allowed {
		t.Fatalf("解密应默认允许所有调用方, got denied: %s", result.Reason)
	}
}

// TestEvaluateEncryptAlgorithmMismatch 验证算法不匹配场景。
func TestEvaluateEncryptAlgorithmMismatch(t *testing.T) {
	// 不需要审批的降级场景
	p := &Policy{
		ID:               "policy-6",
		TenantID:         "tenant-1",
		KeyID:            "key-6",
		MinAlgorithm:     "AES_256_GCM",
		RequireAAD:       false,
		MaxPlaintextSize: 1024,
		AllowedCallers:   []string{"caller-1"},
	}

	// 算法不匹配但不需要审批，应允许（其他条件满足时）
	req := EncryptRequest{
		Algorithm:     "SM4_128_GCM",
		AAD:           nil,
		PlaintextSize: 100,
		CallerID:      "caller-1",
	}
	result := p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("算法不匹配但无需审批应允许, got denied: %s", result.Reason)
	}

	// 需要审批但无 ApprovalID 应拒绝
	p.DowngradeRequiresApproval = true
	p.ApprovalID = ""
	result = p.EvaluateEncrypt(req)
	if result.Allowed {
		t.Fatal("需要审批但无 ApprovalID 应拒绝")
	}
	if result.Reason == "" {
		t.Fatal("拒绝时应提供原因")
	}

	// 需要审批且有 ApprovalID 应允许
	p.ApprovalID = "approval-123"
	result = p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("需要审批且有 ApprovalID 应允许, got denied: %s", result.Reason)
	}
}

// TestEvaluateEncryptAllConditionsMet 验证所有条件满足时允许。
func TestEvaluateEncryptAllConditionsMet(t *testing.T) {
	p := &Policy{
		ID:               "policy-7",
		TenantID:         "tenant-1",
		KeyID:            "key-7",
		MinAlgorithm:     "AES_256_GCM",
		RequireAAD:       true,
		MaxPlaintextSize: 4096,
		AllowedCallers:   []string{"svc-a", "svc-b", "svc-c"},
	}

	req := EncryptRequest{
		Algorithm:     "AES_256_GCM",
		AAD:           []byte("canonical-aad"),
		PlaintextSize: 2048,
		CallerID:      "svc-b",
	}

	result := p.EvaluateEncrypt(req)
	if !result.Allowed {
		t.Fatalf("所有条件满足应允许, got denied: %s", result.Reason)
	}
}

// TestEvaluateEncryptMultipleFailures 验证多个条件不满足时返回第一个失败原因。
func TestEvaluateEncryptMultipleFailures(t *testing.T) {
	p := &Policy{
		ID:               "policy-8",
		TenantID:         "tenant-1",
		KeyID:            "key-8",
		MinAlgorithm:     "AES_256_GCM",
		RequireAAD:       true,
		MaxPlaintextSize: 100,
		AllowedCallers:   []string{"caller-1"},
	}

	// 同时违反多个条件：无 AAD、超过大小、调用方不在列表
	req := EncryptRequest{
		Algorithm:     "AES_256_GCM",
		AAD:           nil,
		PlaintextSize: 200,
		CallerID:      "caller-wrong",
	}
	result := p.EvaluateEncrypt(req)
	if result.Allowed {
		t.Fatal("违反多个条件应拒绝")
	}
	if result.Reason == "" {
		t.Fatal("拒绝时应提供原因")
	}
}
