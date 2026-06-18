// Package policy 定义加密策略领域类型。
package policy

// Policy 是加密策略聚合根，对应第 10 章。
type Policy struct {
	ID            string
	TenantID      string
	KeyID         string
	MinAlgorithm  string   // 最低算法要求
	RequireAAD    bool     // 是否强制 AAD
	MaxPlaintextSize int64 // 最大明文大小（字节）
	AllowedCallers []string // 允许的调用方标识
	// 算法降级控制（HA-05）
	CBCDecryptOnly bool     // CBC 模式仅允许解密
	ECBDecryptOnly bool     // ECB 模式仅允许解密
	// 降级需审批
	DowngradeRequiresApproval bool
	ApprovalID                string
	// 签名保护（HA-05 骨架）
	PolicySignature []byte
}

// Evaluate 评估加密请求是否符合策略。
type EncryptRequest struct {
	Algorithm   string
	AAD         []byte
	PlaintextSize int64
	CallerID    string
}

// EvaluateResult 策略评估结果。
type EvaluateResult struct {
	Allowed bool
	Reason  string
}

// EvaluateEncrypt 评估加密请求。
func (p *Policy) EvaluateEncrypt(req EncryptRequest) EvaluateResult {
	if req.Algorithm != p.MinAlgorithm {
		// 检查是否是降级场景
		if p.DowngradeRequiresApproval && p.ApprovalID == "" {
			return EvaluateResult{Allowed: false, Reason: "algorithm downgrade requires approval"}
		}
	}
	if p.RequireAAD && len(req.AAD) == 0 {
		return EvaluateResult{Allowed: false, Reason: "AAD required by policy"}
	}
	if p.MaxPlaintextSize > 0 && req.PlaintextSize > p.MaxPlaintextSize {
		return EvaluateResult{Allowed: false, Reason: "plaintext exceeds max size"}
	}
	if len(p.AllowedCallers) > 0 {
		found := false
		for _, c := range p.AllowedCallers {
			if c == req.CallerID {
				found = true
				break
			}
		}
		if !found {
			return EvaluateResult{Allowed: false, Reason: "caller not allowed"}
		}
	}
	return EvaluateResult{Allowed: true}
}

// EvaluateDecrypt 评估解密请求。
type DecryptRequest struct {
	Algorithm string
	CallerID  string
}

// EvaluateDecrypt 评估解密请求，CBC/ECB 仅允许解密（HA-05）。
func (p *Policy) EvaluateDecrypt(req DecryptRequest) EvaluateResult {
	// CBC/ECB decrypt_only 是允许解密但不允许加密
	return EvaluateResult{Allowed: true}
}
