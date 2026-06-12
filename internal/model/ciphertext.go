package model

import "encoding/json"

// CiphertextFormat 表示序列化的密文结构
// 包含解密所需的全部元数据
type CiphertextFormat struct {
	Algorithm Algorithm `json:"algorithm"`
	Mode      Mode      `json:"mode"`
	IV        []byte    `json:"iv"`
	Tag       []byte    `json:"tag,omitempty"` // 仅用于 GCM 模式
	Data      []byte    `json:"data"`
}

// Marshal 将密文格式序列化为 JSON 字节
func (c *CiphertextFormat) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalCiphertext 将 JSON 字节反序列化为 CiphertextFormat
func UnmarshalCiphertext(data []byte) (*CiphertextFormat, error) {
	var ct CiphertextFormat
	if err := json.Unmarshal(data, &ct); err != nil {
		return nil, ErrInvalidCiphertext
	}
	return &ct, nil
}
