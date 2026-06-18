package envelope

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	stderrors "errors"
	"testing"

	"github.com/HangzeGao/trae/key-vault/internal/crypto/aead"
	kverr "github.com/HangzeGao/trae/key-vault/internal/errors"
)

// mustRandBytes 生成 n 字节随机数据，失败时调用 t.Fatal 终止测试。
func mustRandBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("生成随机字节失败: %v", err)
	}
	return b
}

// TestEnvelopeBuildParse 验证构建后解析应还原所有字段。
func TestEnvelopeBuildParse(t *testing.T) {
	suiteID := SuiteAES256GCM
	dekKID := "kid-1234-5678"
	nonce := mustRandBytes(t, 12)
	ciphertext := mustRandBytes(t, 48)
	aad := []byte("canonical-aad")

	env, err := Build(suiteID, dekKID, nonce, ciphertext, aad)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	parsed, err := Parse(env)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}

	if parsed.SuiteID != suiteID {
		t.Errorf("SuiteID 不匹配: got 0x%02x, want 0x%02x", parsed.SuiteID, suiteID)
	}
	if parsed.DEKKID != dekKID {
		t.Errorf("DEKKID 不匹配: got %q, want %q", parsed.DEKKID, dekKID)
	}
	if !bytes.Equal(parsed.Nonce, nonce) {
		t.Errorf("Nonce 不匹配: got %x, want %x", parsed.Nonce, nonce)
	}
	if !bytes.Equal(parsed.Ciphertext, ciphertext) {
		t.Errorf("Ciphertext 不匹配: got %x, want %x", parsed.Ciphertext, ciphertext)
	}
	if !bytes.Equal(parsed.AAD, aad) {
		t.Errorf("AAD 不匹配: got %x, want %x", parsed.AAD, aad)
	}
}

// TestEnvelopeParseInvalidMagic 验证错误 magic 应失败。
func TestEnvelopeParseInvalidMagic(t *testing.T) {
	suiteID := SuiteAES256GCM
	dekKID := "kid"
	nonce := mustRandBytes(t, 12)
	ciphertext := mustRandBytes(t, 32)
	aad := []byte("aad")

	env, err := Build(suiteID, dekKID, nonce, ciphertext, aad)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 篡改 magic 为 "XXXX"
	tampered := make([]byte, len(env))
	copy(tampered, env)
	copy(tampered[0:4], []byte("XXXX"))

	if _, err := Parse(tampered); err == nil {
		t.Fatal("错误 magic 应解析失败，但成功了")
	}
}

// TestEnvelopeParseTruncated 验证截断数据应失败。
func TestEnvelopeParseTruncated(t *testing.T) {
	suiteID := SuiteAES256GCM
	dekKID := "kid"
	nonce := mustRandBytes(t, 12)
	ciphertext := mustRandBytes(t, 32)
	aad := []byte("aad")

	env, err := Build(suiteID, dekKID, nonce, ciphertext, aad)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 截断到只剩最小固定字段长度的一半，必然失败
	truncated := env[:len(env)/2]
	if _, err := Parse(truncated); err == nil {
		t.Fatal("截断数据应解析失败，但成功了")
	}

	// 截断到 0 字节也应失败
	if _, err := Parse(nil); err == nil {
		t.Fatal("空数据应解析失败，但成功了")
	}

	// 截断到刚好小于最小长度（minLen - 1）应失败
	minLen := magicLen + versionLen + suiteIDLen +
		dekKIDLenField + nonceLenField + ctLenField + aadLenField
	short := env[:minLen-1]
	if _, err := Parse(short); err == nil {
		t.Fatal("小于最小长度的数据应解析失败，但成功了")
	}
}

// TestSuiteIDConversion 验证算法名与 suite_id 互转。
func TestSuiteIDConversion(t *testing.T) {
	// AES_256_GCM <-> SuiteAES256GCM
	got, err := SuiteIDFor(AlgAES256GCM)
	if err != nil {
		t.Fatalf("SuiteIDFor(AES_256_GCM) 失败: %v", err)
	}
	if got != SuiteAES256GCM {
		t.Errorf("SuiteIDFor(AES_256_GCM) = 0x%02x, want 0x%02x", got, SuiteAES256GCM)
	}
	alg, err := AlgorithmFor(SuiteAES256GCM)
	if err != nil {
		t.Fatalf("AlgorithmFor(SuiteAES256GCM) 失败: %v", err)
	}
	if alg != AlgAES256GCM {
		t.Errorf("AlgorithmFor(SuiteAES256GCM) = %q, want %q", alg, AlgAES256GCM)
	}

	// SM4_128_GCM <-> SuiteSM4GCM
	got, err = SuiteIDFor(AlgSM4GCM)
	if err != nil {
		t.Fatalf("SuiteIDFor(SM4_128_GCM) 失败: %v", err)
	}
	if got != SuiteSM4GCM {
		t.Errorf("SuiteIDFor(SM4_128_GCM) = 0x%02x, want 0x%02x", got, SuiteSM4GCM)
	}
	alg, err = AlgorithmFor(SuiteSM4GCM)
	if err != nil {
		t.Fatalf("AlgorithmFor(SuiteSM4GCM) 失败: %v", err)
	}
	if alg != AlgSM4GCM {
		t.Errorf("AlgorithmFor(SuiteSM4GCM) = %q, want %q", alg, AlgSM4GCM)
	}

	// 不支持的算法名应失败
	if _, err := SuiteIDFor("AES_128_GCM"); err == nil {
		t.Fatal("不支持的算法名应返回错误")
	}
	// 不支持的 suite_id 应失败
	if _, err := AlgorithmFor(0xFF); err == nil {
		t.Fatal("不支持的 suite_id 应返回错误")
	}
}

// TestEnvelopeRoundTripWithAEAD 验证完整往返：AEAD 加密 -> 构建 Envelope -> 解析 -> AEAD 解密。
func TestEnvelopeRoundTripWithAEAD(t *testing.T) {
	// 1. 准备 DEK 与 AEAD provider
	dek := mustRandBytes(t, 32)
	provider, err := aead.New(aead.AlgAES256GCM, dek)
	if err != nil {
		t.Fatalf("创建 AEAD provider 失败: %v", err)
	}

	// 2. 准备明文（base64 编码）、AAD、nonce
	plaintext, err := base64.StdEncoding.DecodeString("ZW52ZWxvcGUgcm91bmQgdHJpcCB0ZXN0")
	if err != nil {
		t.Fatalf("解码测试明文失败: %v", err)
	}
	aadParts := [][]byte{[]byte("kid-abc"), []byte("v1")}
	canonicalAAD := Canonical(aadParts...)
	nonce := mustRandBytes(t, 12)

	// 3. AEAD 加密
	ciphertext, err := provider.Encrypt(plaintext, canonicalAAD, nonce)
	if err != nil {
		t.Fatalf("AEAD 加密失败: %v", err)
	}

	// 4. 构建 Envelope
	suiteID, err := SuiteIDFor(aead.AlgAES256GCM)
	if err != nil {
		t.Fatalf("获取 suite_id 失败: %v", err)
	}
	dekKID := "dek-kid-roundtrip"
	env, err := Build(suiteID, dekKID, nonce, ciphertext, canonicalAAD)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 5. 解析 Envelope
	parsed, err := Parse(env)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}

	// 6. 验证解析出的字段
	if parsed.SuiteID != suiteID {
		t.Fatalf("SuiteID 不匹配: got 0x%02x, want 0x%02x", parsed.SuiteID, suiteID)
	}
	if parsed.DEKKID != dekKID {
		t.Fatalf("DEKKID 不匹配: got %q, want %q", parsed.DEKKID, dekKID)
	}
	if !bytes.Equal(parsed.Nonce, nonce) {
		t.Fatalf("Nonce 不匹配")
	}
	if !bytes.Equal(parsed.Ciphertext, ciphertext) {
		t.Fatalf("Ciphertext 不匹配")
	}
	if !bytes.Equal(parsed.AAD, canonicalAAD) {
		t.Fatalf("AAD 不匹配")
	}

	// 7. 验证 AAD 规范化一致性
	if !Verify(parsed.AAD, aadParts...) {
		t.Fatal("解析出的 AAD 与原始 parts 验证失败")
	}

	// 8. AEAD 解密还原明文
	got, err := provider.Decrypt(parsed.Ciphertext, parsed.AAD, parsed.Nonce)
	if err != nil {
		t.Fatalf("AEAD 解密失败: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("解密结果与原文不符: got %q, want %q", got, plaintext)
	}
}

// TestEnvelopeParseInvalidVersion 验证不支持的版本应失败。
func TestEnvelopeParseInvalidVersion(t *testing.T) {
	suiteID := SuiteAES256GCM
	dekKID := "kid"
	nonce := mustRandBytes(t, 12)
	ciphertext := mustRandBytes(t, 32)
	aad := []byte("aad")

	env, err := Build(suiteID, dekKID, nonce, ciphertext, aad)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 篡改 version 字段（magic 之后第 1 个字节）
	tampered := make([]byte, len(env))
	copy(tampered, env)
	tampered[magicLen] = 0xFF

	if _, err := Parse(tampered); err == nil {
		t.Fatal("不支持的版本应解析失败，但成功了")
	}
}

// TestEnvelopeParseTruncatedAtCiphertext 验证在 ciphertext 字段截断应失败。
func TestEnvelopeParseTruncatedAtCiphertext(t *testing.T) {
	suiteID := SuiteAES256GCM
	dekKID := "kid"
	nonce := mustRandBytes(t, 12)
	ciphertext := mustRandBytes(t, 32)
	aad := []byte("aad")

	env, err := Build(suiteID, dekKID, nonce, ciphertext, aad)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 截断到 ciphertext 字段中间（去掉末尾 10 字节）
	truncated := env[:len(env)-10]
	_, err = Parse(truncated)
	if err == nil {
		t.Fatal("在 ciphertext 字段截断应解析失败，但成功了")
	}
	// 验证返回的是 EnvelopeInvalid 错误码
	var ee *kverr.Error
	if !stderrors.As(err, &ee) {
		t.Fatalf("应返回 *errors.Error 类型, got %T", err)
	}
	if ee.Code != kverr.CodeEnvelopeInvalid {
		t.Fatalf("错误码应为 ENVELOPE_INVALID, got %s", ee.Code)
	}
}

// TestEnvelopeBuildEmptyFields 验证空变长字段也能正确构建与解析。
func TestEnvelopeBuildEmptyFields(t *testing.T) {
	suiteID := SuiteSM4GCM
	dekKID := ""
	nonce := []byte{}
	ciphertext := []byte{}
	aad := []byte{}

	env, err := Build(suiteID, dekKID, nonce, ciphertext, aad)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	parsed, err := Parse(env)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}

	if parsed.SuiteID != suiteID {
		t.Errorf("SuiteID 不匹配: got 0x%02x, want 0x%02x", parsed.SuiteID, suiteID)
	}
	if parsed.DEKKID != "" {
		t.Errorf("DEKKID 应为空, got %q", parsed.DEKKID)
	}
	if len(parsed.Nonce) != 0 {
		t.Errorf("Nonce 应为空, got %x", parsed.Nonce)
	}
	if len(parsed.Ciphertext) != 0 {
		t.Errorf("Ciphertext 应为空, got %x", parsed.Ciphertext)
	}
	if len(parsed.AAD) != 0 {
		t.Errorf("AAD 应为空, got %x", parsed.AAD)
	}
}
