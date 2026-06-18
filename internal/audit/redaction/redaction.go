// Package redaction 实现审计事件脱敏，确保敏感字段不会泄露到审计日志中。
//
// 对审计事件的 Details 字段做递归脱敏处理，
// 将敏感字段（如 key_material、dek、wrapped_dek、crk、plaintext、secret、token、password）
// 的值替换为 [REDACTED]。
package redaction

import (
	"strings"

	domain "github.com/HangzeGao/trae/key-vault/internal/domain/audit"
)

// redactedValue 是脱敏后的占位值。
const redactedValue = "[REDACTED]"

// Redactor 对审计事件的 Details 字段做脱敏。
type Redactor struct {
	sensitiveKeys map[string]bool
}

// NewRedactor 创建一个脱敏器，内置默认敏感字段。
//
// 默认敏感字段: key_material, dek, wrapped_dek, crk, plaintext, secret, token, password
func NewRedactor() *Redactor {
	return &Redactor{
		sensitiveKeys: map[string]bool{
			"key_material": true,
			"dek":          true,
			"wrapped_dek":  true,
			"crk":          true,
			"plaintext":    true,
			"secret":       true,
			"token":        true,
			"password":     true,
		},
	}
}

// IsSensitive 判断给定键名是否为敏感字段（大小写不敏感）。
func (r *Redactor) IsSensitive(key string) bool {
	return r.sensitiveKeys[strings.ToLower(key)]
}

// AddSensitiveKey 添加自定义敏感字段名（大小写不敏感）。
func (r *Redactor) AddSensitiveKey(key string) {
	r.sensitiveKeys[strings.ToLower(key)] = true
}

// Redact 对事件的 Details 做脱敏处理。
//
// 直接修改传入的事件，将 Details 中所有敏感字段的值替换为 [REDACTED]。
// 若 Details 为 nil 则不做任何操作。
func (r *Redactor) Redact(event *domain.Event) {
	if event == nil || event.Details == nil {
		return
	}
	event.Details = r.RedactMap(event.Details)
}

// RedactMap 递归脱敏 map 中的敏感字段。
//
// 返回一个新的 map，其中敏感字段的值被替换为 [REDACTED]，
// 嵌套的 map 和切片也会递归处理。
// 键名匹配大小写不敏感。
func (r *Redactor) RedactMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}

	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		if r.sensitiveKeys[strings.ToLower(k)] {
			result[k] = redactedValue
			continue
		}
		result[k] = r.redactValue(v)
	}
	return result
}

// redactValue 递归脱敏值中的嵌套 map 和切片。
func (r *Redactor) redactValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return r.RedactMap(val)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = r.redactValue(item)
		}
		return result
	default:
		return v
	}
}
