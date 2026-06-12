package model

// GenerateKeyRequest 是 POST /api/v1/keys/generate 的请求
type GenerateKeyRequest struct {
	Algorithm Algorithm `json:"algorithm"`
}

// GenerateKeyResponse 是 POST /api/v1/keys/generate 的响应
type GenerateKeyResponse struct {
	EncryptedDataKey []byte    `json:"encrypted_data_key"` // JSON 中为 base64 编码
	PlaintextDataKey []byte    `json:"plaintext_data_key"` // JSON 中为 base64 编码
	Algorithm        Algorithm `json:"algorithm"`
}

// EncryptRequest 是 POST /api/v1/encrypt 的请求
type EncryptRequest struct {
	Plaintext        []byte    `json:"plaintext"`          // JSON 中为 base64 编码
	EncryptedDataKey []byte    `json:"encrypted_data_key"` // JSON 中为 base64 编码
	Algorithm        Algorithm `json:"algorithm"`
	Mode             Mode      `json:"mode"`
}

// EncryptResponse 是 POST /api/v1/encrypt 的响应
type EncryptResponse struct {
	Ciphertext []byte `json:"ciphertext"` // JSON 中为 base64 编码（CiphertextFormat 序列化后）
}

// DecryptRequest 是 POST /api/v1/decrypt 的请求
type DecryptRequest struct {
	Ciphertext       []byte    `json:"ciphertext"`         // JSON 中为 base64 编码（CiphertextFormat 序列化后）
	EncryptedDataKey []byte    `json:"encrypted_data_key"` // JSON 中为 base64 编码
	Algorithm        Algorithm `json:"algorithm"`
	Mode             Mode      `json:"mode"`
}

// DecryptResponse 是 POST /api/v1/decrypt 的响应
type DecryptResponse struct {
	Plaintext []byte `json:"plaintext"` // JSON 中为 base64 编码
}

// RotateKeyRequest 是 POST /api/v1/keys/rotate 的请求
type RotateKeyRequest struct {
	OldEncryptedDataKey []byte    `json:"old_encrypted_data_key"` // JSON 中为 base64 编码
	Algorithm           Algorithm `json:"algorithm"`
}

// RotateKeyResponse 是 POST /api/v1/keys/rotate 的响应
type RotateKeyResponse struct {
	NewEncryptedDataKey []byte    `json:"new_encrypted_data_key"` // JSON 中为 base64 编码
	NewPlaintextDataKey []byte    `json:"new_plaintext_data_key"` // JSON 中为 base64 编码
	Algorithm           Algorithm `json:"algorithm"`
}

// DecryptKeyRequest 是 POST /api/v1/keys/decrypt 的请求
type DecryptKeyRequest struct {
	EncryptedDataKey []byte `json:"encrypted_data_key"` // JSON 中为 base64 编码
}

// DecryptKeyResponse 是 POST /api/v1/keys/decrypt 的响应
type DecryptKeyResponse struct {
	PlaintextDataKey []byte `json:"plaintext_data_key"` // JSON 中为 base64 编码
}

// HealthResponse 是 GET /health 的响应
type HealthResponse struct {
	Status       string `json:"status"`
	TPMAvailable bool   `json:"tpm_available"`
}

// AlgorithmInfo 描述一个受支持的算法及其模式
type AlgorithmInfo struct {
	Algorithm Algorithm `json:"algorithm"`
	KeyBits   int       `json:"key_bits"`
	Modes     []Mode    `json:"modes"`
}

// AlgorithmsResponse 是 GET /api/v1/algorithms 的响应
type AlgorithmsResponse struct {
	Algorithms []AlgorithmInfo `json:"algorithms"`
}

// ErrorResponse 是标准错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
