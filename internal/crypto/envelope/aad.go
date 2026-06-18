// Package envelope 定义数据信封（Envelope v1）格式与 AAD 规范序列化。
//
// 信封格式（第 7.2 节）：
//
//	magic(4) || version(1) || suite_id(1) || dek_kid_len(2) || dek_kid ||
//	nonce_len(1) || nonce || ct_len(4) || ciphertext || aad_len(4) || aad
//
// AAD 规范序列化（第 7.3 节）使用长度前缀避免歧义拼接。
package envelope

import (
	"bytes"
	"encoding/binary"
)

// Canonical 将多个 AAD 部分按长度前缀+内容方式序列化为规范字节串。
//
// 格式: varint(len(part1)) || part1 || varint(len(part2)) || part2 || ...
//
// 长度前缀使用 binary.PutUvarint 编码，确保不同 parts 序列产生不同输出，
// 避免歧义拼接攻击（例如 ("ab","c") 与 ("a","bc") 产生不同的 canonical AAD）。
func Canonical(parts ...[]byte) []byte {
	// 预估容量：每部分内容长度 + varint 最大 10 字节
	total := 0
	for _, p := range parts {
		total += len(p) + binary.MaxVarintLen64
	}
	buf := make([]byte, 0, total)
	var lenBuf [binary.MaxVarintLen64]byte
	for _, p := range parts {
		n := binary.PutUvarint(lenBuf[:], uint64(len(p)))
		buf = append(buf, lenBuf[:n]...)
		buf = append(buf, p...)
	}
	return buf
}

// Verify 验证给定 parts 序列化后是否与 canonicalAAD 一致。
// 返回 true 表示 AAD 匹配，false 表示不匹配。
func Verify(canonicalAAD []byte, parts ...[]byte) bool {
	return bytes.Equal(canonicalAAD, Canonical(parts...))
}
