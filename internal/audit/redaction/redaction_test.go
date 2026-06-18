package redaction

import (
	"testing"

	domain "github.com/HangzeGao/trae/key-vault/internal/domain/audit"
)

// TestRedactSensitiveFields 验证敏感字段被脱敏。
func TestRedactSensitiveFields(t *testing.T) {
	r := NewRedactor()
	sensitiveFields := []string{
		"key_material",
		"dek",
		"wrapped_dek",
		"crk",
		"plaintext",
		"secret",
		"token",
		"password",
	}

	for _, field := range sensitiveFields {
		if !r.IsSensitive(field) {
			t.Errorf("字段 %q 应被识别为敏感字段", field)
		}
	}

	// 构造包含所有敏感字段的事件
	details := map[string]interface{}{
		"key_material": "sensitive-key-material-value",
		"dek":          "data-encryption-key",
		"wrapped_dek":  "wrapped-dek-blob",
		"crk":          "customer-root-key",
		"plaintext":    "secret plaintext",
		"secret":       "my-secret",
		"token":        "bearer-token",
		"password":     "user-password",
		"normal_field": "normal-value",
	}
	event := &domain.Event{
		ID:        "evt-1",
		EventType: "key.created",
		Details:   details,
	}

	r.Redact(event)

	for _, field := range sensitiveFields {
		v, ok := event.Details[field]
		if !ok {
			t.Errorf("脱敏后字段 %q 应仍存在", field)
			continue
		}
		if v != redactedValue {
			t.Errorf("字段 %q 应被脱敏为 %q, got %v", field, redactedValue, v)
		}
	}

	// 非敏感字段应保留
	if event.Details["normal_field"] != "normal-value" {
		t.Errorf("非敏感字段 normal_field 应保留原值, got %v", event.Details["normal_field"])
	}
}

// TestRedactNestedMap 验证嵌套 map 中的敏感字段被脱敏。
func TestRedactNestedMap(t *testing.T) {
	r := NewRedactor()

	details := map[string]interface{}{
		"outer_secret": "outer-secret-value",
		"nested": map[string]interface{}{
			"inner_dek":   "inner-dek-value",
			"inner_normal": "inner-normal",
			"deep_nested": map[string]interface{}{
				"deep_password": "deep-password",
				"deep_normal":   "deep-normal",
			},
		},
		"list_with_maps": []interface{}{
			map[string]interface{}{
				"item_token": "item-token",
				"item_name":  "item-name",
			},
			"plain-string-item",
		},
	}
	event := &domain.Event{
		ID:      "evt-2",
		Details: details,
	}

	r.Redact(event)

	// 外层敏感字段应脱敏
	if event.Details["outer_secret"] != redactedValue {
		t.Errorf("outer_secret 应被脱敏, got %v", event.Details["outer_secret"])
	}

	// 嵌套 map 中的敏感字段应脱敏
	nested, ok := event.Details["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested 应仍为 map[string]interface{}")
	}
	if nested["inner_dek"] != redactedValue {
		t.Errorf("inner_dek 应被脱敏, got %v", nested["inner_dek"])
	}
	if nested["inner_normal"] != "inner-normal" {
		t.Errorf("inner_normal 应保留原值, got %v", nested["inner_normal"])
	}

	// 深层嵌套
	deepNested, ok := nested["deep_nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("deep_nested 应仍为 map[string]interface{}")
	}
	if deepNested["deep_password"] != redactedValue {
		t.Errorf("deep_password 应被脱敏, got %v", deepNested["deep_password"])
	}
	if deepNested["deep_normal"] != "deep-normal" {
		t.Errorf("deep_normal 应保留原值, got %v", deepNested["deep_normal"])
	}

	// 切片中的 map 敏感字段应脱敏
	listWithMaps, ok := event.Details["list_with_maps"].([]interface{})
	if !ok {
		t.Fatalf("list_with_maps 应仍为 []interface{}")
	}
	if len(listWithMaps) != 2 {
		t.Fatalf("list_with_maps 长度应为 2, got %d", len(listWithMaps))
	}
	item0, ok := listWithMaps[0].(map[string]interface{})
	if !ok {
		t.Fatalf("list_with_maps[0] 应仍为 map[string]interface{}")
	}
	if item0["item_token"] != redactedValue {
		t.Errorf("item_token 应被脱敏, got %v", item0["item_token"])
	}
	if item0["item_name"] != "item-name" {
		t.Errorf("item_name 应保留原值, got %v", item0["item_name"])
	}
	if listWithMaps[1] != "plain-string-item" {
		t.Errorf("plain-string-item 应保留原值, got %v", listWithMaps[1])
	}
}

// TestRedactNonSensitiveFields 验证非敏感字段保留。
func TestRedactNonSensitiveFields(t *testing.T) {
	r := NewRedactor()

	nonSensitiveFields := map[string]interface{}{
		"key_id":       "kid-1234",
		"algorithm":    "AES_256_GCM",
		"status":       "ACTIVE",
		"tenant_id":    "tenant-1",
		"version":      1,
		"created_at":   "2026-01-01T00:00:00Z",
		"request_id":   "req-abc",
		"actor":        "user-1",
		"resource_id":  "res-1",
		"bool_field":   true,
		"int_field":    42,
	}

	original := make(map[string]interface{}, len(nonSensitiveFields))
	for k, v := range nonSensitiveFields {
		original[k] = v
	}

	event := &domain.Event{
		ID:      "evt-3",
		Details: nonSensitiveFields,
	}

	r.Redact(event)

	for k, want := range original {
		got, ok := event.Details[k]
		if !ok {
			t.Errorf("非敏感字段 %q 应仍存在", k)
			continue
		}
		if got != want {
			t.Errorf("非敏感字段 %q 应保留原值 %v, got %v", k, want, got)
		}
	}
}

// TestRedactEmptyDetails 验证空 Details 不 panic。
func TestRedactEmptyDetails(t *testing.T) {
	r := NewRedactor()

	// nil Details
	event := &domain.Event{
		ID:      "evt-4",
		Details: nil,
	}
	// 不应 panic
	r.Redact(event)
	if event.Details != nil {
		t.Errorf("nil Details 应保持 nil, got %v", event.Details)
	}

	// 空 map Details
	event2 := &domain.Event{
		ID:      "evt-5",
		Details: map[string]interface{}{},
	}
	r.Redact(event2)
	if len(event2.Details) != 0 {
		t.Errorf("空 Details 应保持空, got %v", event2.Details)
	}

	// nil event 不应 panic
	r.Redact(nil)
}

// TestRedactCaseInsensitive 验证键名匹配大小写不敏感。
func TestRedactCaseInsensitive(t *testing.T) {
	r := NewRedactor()

	cases := []string{
		"DEK",
		"Dek",
		"PASSWORD",
		"Password",
		"TOKEN",
		"Token",
		"SECRET",
		"CRK",
		"PLAINTEXT",
		"KEY_MATERIAL",
		"WRAPPED_DEK",
	}
	for _, c := range cases {
		if !r.IsSensitive(c) {
			t.Errorf("字段 %q 应被识别为敏感字段（大小写不敏感）", c)
		}
	}

	// 混合大小写也应脱敏
	details := map[string]interface{}{
		"DEK":      "upper-dek",
		"Password": "mixed-password",
		"TOKEN":    "upper-token",
	}
	event := &domain.Event{
		ID:      "evt-6",
		Details: details,
	}
	r.Redact(event)

	if event.Details["DEK"] != redactedValue {
		t.Errorf("DEK 应被脱敏, got %v", event.Details["DEK"])
	}
	if event.Details["Password"] != redactedValue {
		t.Errorf("Password 应被脱敏, got %v", event.Details["Password"])
	}
	if event.Details["TOKEN"] != redactedValue {
		t.Errorf("TOKEN 应被脱敏, got %v", event.Details["TOKEN"])
	}
}

// TestAddSensitiveKey 验证添加自定义敏感字段。
func TestAddSensitiveKey(t *testing.T) {
	r := NewRedactor()

	// 初始时 "custom_secret" 不在敏感字段中
	if r.IsSensitive("custom_secret") {
		t.Fatal("custom_secret 初始不应为敏感字段")
	}

	// 添加自定义敏感字段
	r.AddSensitiveKey("custom_secret")
	if !r.IsSensitive("custom_secret") {
		t.Fatal("添加后 custom_secret 应为敏感字段")
	}

	// 大小写不敏感
	if !r.IsSensitive("CUSTOM_SECRET") {
		t.Fatal("CUSTOM_SECRET 应被识别为敏感字段（大小写不敏感）")
	}

	// 验证脱敏生效
	details := map[string]interface{}{
		"custom_secret": "custom-value",
		"normal":        "normal-value",
	}
	event := &domain.Event{
		ID:      "evt-7",
		Details: details,
	}
	r.Redact(event)

	if event.Details["custom_secret"] != redactedValue {
		t.Errorf("custom_secret 应被脱敏, got %v", event.Details["custom_secret"])
	}
	if event.Details["normal"] != "normal-value" {
		t.Errorf("normal 应保留原值, got %v", event.Details["normal"])
	}
}

// TestRedactMapDirectly 验证直接调用 RedactMap。
func TestRedactMapDirectly(t *testing.T) {
	r := NewRedactor()

	m := map[string]interface{}{
		"dek":    "dek-value",
		"normal": "normal-value",
		"nested": map[string]interface{}{
			"password": "nested-password",
		},
	}

	result := r.RedactMap(m)

	if result["dek"] != redactedValue {
		t.Errorf("dek 应被脱敏, got %v", result["dek"])
	}
	if result["normal"] != "normal-value" {
		t.Errorf("normal 应保留原值, got %v", result["normal"])
	}
	nested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested 应仍为 map[string]interface{}")
	}
	if nested["password"] != redactedValue {
		t.Errorf("nested.password 应被脱敏, got %v", nested["password"])
	}

	// 原始 map 不应被修改（RedactMap 返回新 map）
	if m["dek"] != "dek-value" {
		t.Errorf("原始 map 不应被修改, got %v", m["dek"])
	}
}

// TestRedactMapNil 验证 RedactMap 处理 nil。
func TestRedactMapNil(t *testing.T) {
	r := NewRedactor()
	result := r.RedactMap(nil)
	if result != nil {
		t.Errorf("RedactMap(nil) 应返回 nil, got %v", result)
	}
}

// TestRedactPreservesEventMetadata 验证脱敏只影响 Details，不影响其他字段。
func TestRedactPreservesEventMetadata(t *testing.T) {
	r := NewRedactor()

	event := &domain.Event{
		ID:           "evt-meta",
		TenantID:     "tenant-meta",
		EventType:    "key.created",
		Severity:     domain.SeverityInfo,
		Actor:        "actor-meta",
		Action:       "create",
		ResourceType: "key",
		ResourceID:   "res-meta",
		Result:       "success",
		RequestID:    "req-meta",
		Details: map[string]interface{}{
			"dek": "dek-value",
		},
	}

	r.Redact(event)

	// 其他字段应保持不变
	if event.ID != "evt-meta" {
		t.Errorf("ID 应保持不变, got %q", event.ID)
	}
	if event.TenantID != "tenant-meta" {
		t.Errorf("TenantID 应保持不变, got %q", event.TenantID)
	}
	if event.EventType != "key.created" {
		t.Errorf("EventType 应保持不变, got %q", event.EventType)
	}
	if event.Actor != "actor-meta" {
		t.Errorf("Actor 应保持不变, got %q", event.Actor)
	}
	if event.ResourceID != "res-meta" {
		t.Errorf("ResourceID 应保持不变, got %q", event.ResourceID)
	}
	if event.RequestID != "req-meta" {
		t.Errorf("RequestID 应保持不变, got %q", event.RequestID)
	}
	// Details 中的敏感字段应被脱敏
	if event.Details["dek"] != redactedValue {
		t.Errorf("dek 应被脱敏, got %v", event.Details["dek"])
	}
}
