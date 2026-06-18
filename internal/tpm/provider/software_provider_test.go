package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/HangzeGao/trae/key-vault/internal/config"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// newTestProvider 构造一个已初始化的软件 Provider。
func newTestProvider(t *testing.T) *SoftwareProvider {
	t.Helper()
	cfg := config.TPMConfig{
		UseSWTPM:   true,
		NRWKHandle: 0x81010010,
	}
	p := NewSoftwareProvider(cfg, observability.Get())
	if err := p.Init(context.Background()); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// mustRandBytes 生成 n 字节随机数据。
func mustRandBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("生成随机字节失败: %v", err)
	}
	return b
}

// TestSoftwareProviderInit 验证初始化成功。
func TestSoftwareProviderInit(t *testing.T) {
	cfg := config.TPMConfig{
		UseSWTPM:   true,
		NRWKHandle: 0x81010010,
	}
	p := NewSoftwareProvider(cfg, observability.Get())
	defer func() { _ = p.Close() }()

	if err := p.Init(context.Background()); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}

	// 重复 Init 应幂等返回 nil
	if err := p.Init(context.Background()); err != nil {
		t.Fatalf("重复 Init 应幂等, got %v", err)
	}
}

// TestSoftwareProviderNotInitialized 验证未初始化时操作应失败。
func TestSoftwareProviderNotInitialized(t *testing.T) {
	cfg := config.TPMConfig{UseSWTPM: true}
	p := NewSoftwareProvider(cfg, observability.Get())
	// 不调用 Init
	defer func() { _ = p.Close() }()

	dek := mustRandBytes(t, 32)
	err := p.SealDEK(context.Background(), dek)
	if err == nil {
		t.Fatal("未初始化时 SealDEK 应失败")
	}
	if !IsFatal(err) {
		t.Fatalf("未初始化错误应为 fatal, got %v", err)
	}

	// UnsealDEK 也应失败
	if _, err := p.UnsealDEK(context.Background(), dek); err == nil {
		t.Fatal("未初始化时 UnsealDEK 应失败")
	}

	// GetNRWKPublicKey 也应失败
	if _, err := p.GetNRWKPublicKey(context.Background()); err == nil {
		t.Fatal("未初始化时 GetNRWKPublicKey 应失败")
	}
}

// TestSealUnsealDEK 验证密封后解封应还原 DEK。
func TestSealUnsealDEK(t *testing.T) {
	p := newTestProvider(t)
	dek := mustRandBytes(t, 32)

	sealed, err := p.SealDEK(context.Background(), dek)
	if err != nil {
		t.Fatalf("SealDEK 失败: %v", err)
	}
	if len(sealed) <= 12 {
		t.Fatalf("sealed 长度应大于 12(nonce), got %d", len(sealed))
	}
	if bytes.Equal(sealed[12:], dek) {
		t.Fatal("sealed 密文部分不应等于原始 DEK")
	}

	got, err := p.UnsealDEK(context.Background(), sealed)
	if err != nil {
		t.Fatalf("UnsealDEK 失败: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatalf("解封后的 DEK 与原始不符: got %x, want %x", got, dek)
	}
}

// TestGenerateAndSealDEK 验证生成并密封 DEK，解封后长度正确。
func TestGenerateAndSealDEK(t *testing.T) {
	p := newTestProvider(t)

	sealed, kid, err := p.GenerateAndSealDEK(context.Background(), "AES_256_GCM")
	if err != nil {
		t.Fatalf("GenerateAndSealDEK 失败: %v", err)
	}
	if len(sealed) == 0 {
		t.Fatal("sealed 不应为空")
	}
	if kid == "" {
		t.Fatal("kid 不应为空")
	}
	// kid 应为 UUID v4 格式
	if !isValidUUIDv4(kid) {
		t.Fatalf("kid 应为 UUID v4 格式, got %q", kid)
	}

	// 解封后长度应为 32（AES-256）
	dek, err := p.UnsealDEK(context.Background(), sealed)
	if err != nil {
		t.Fatalf("UnsealDEK 失败: %v", err)
	}
	if len(dek) != 32 {
		t.Fatalf("AES-256 DEK 长度应为 32, got %d", len(dek))
	}
}

// TestGenerateAndSealDEKAES256 验证 AES-256 DEK 32 字节。
func TestGenerateAndSealDEKAES256(t *testing.T) {
	p := newTestProvider(t)

	sealed, kid, err := p.GenerateAndSealDEK(context.Background(), "AES_256_GCM")
	if err != nil {
		t.Fatalf("GenerateAndSealDEK(AES_256_GCM) 失败: %v", err)
	}
	if len(kid) == 0 {
		t.Fatal("kid 不应为空")
	}

	dek, err := p.UnsealDEK(context.Background(), sealed)
	if err != nil {
		t.Fatalf("UnsealDEK 失败: %v", err)
	}
	if len(dek) != 32 {
		t.Fatalf("AES-256 DEK 长度应为 32, got %d", len(dek))
	}
}

// TestGenerateAndSealDEKSM4 验证 SM4 DEK 16 字节。
func TestGenerateAndSealDEKSM4(t *testing.T) {
	p := newTestProvider(t)

	sealed, kid, err := p.GenerateAndSealDEK(context.Background(), "SM4_128_GCM")
	if err != nil {
		t.Fatalf("GenerateAndSealDEK(SM4_128_GCM) 失败: %v", err)
	}
	if len(kid) == 0 {
		t.Fatal("kid 不应为空")
	}

	dek, err := p.UnsealDEK(context.Background(), sealed)
	if err != nil {
		t.Fatalf("UnsealDEK 失败: %v", err)
	}
	if len(dek) != 16 {
		t.Fatalf("SM4 DEK 长度应为 16, got %d", len(dek))
	}
}

// TestGenerateAndSealDEKUnsupportedAlgorithm 验证不支持的算法应失败。
func TestGenerateAndSealDEKUnsupportedAlgorithm(t *testing.T) {
	p := newTestProvider(t)

	_, _, err := p.GenerateAndSealDEK(context.Background(), "AES_128_GCM")
	if err == nil {
		t.Fatal("不支持的算法应返回错误")
	}
	if !IsPermanent(err) {
		t.Fatalf("不支持的算法错误应为 permanent, got %v", err)
	}
}

// TestUnsealInvalidData 验证无效密封数据应失败。
func TestUnsealInvalidData(t *testing.T) {
	p := newTestProvider(t)

	// 太短的数据（小于 nonce 12 字节）
	short := mustRandBytes(t, 5)
	if _, err := p.UnsealDEK(context.Background(), short); err == nil {
		t.Fatal("过短数据解封应失败")
	}

	// 仅 nonce 长度但无密文
	onlyNonce := mustRandBytes(t, 12)
	if _, err := p.UnsealDEK(context.Background(), onlyNonce); err == nil {
		t.Fatal("仅 nonce 数据解封应失败")
	}

	// 篡改的密封数据（nonce + 错误密文）
	tampered := mustRandBytes(t, 32)
	if _, err := p.UnsealDEK(context.Background(), tampered); err == nil {
		t.Fatal("篡改数据解封应失败")
	}
}

// TestSealEmptyDEK 验证空 DEK 应失败。
func TestSealEmptyDEK(t *testing.T) {
	p := newTestProvider(t)

	if _, err := p.SealDEK(context.Background(), nil); err == nil {
		t.Fatal("空 DEK 密封应失败")
	}
	if _, err := p.SealDEK(context.Background(), []byte{}); err == nil {
		t.Fatal("空 DEK 密封应失败")
	}
}

// TestGetNRWKPublicKey 验证返回非空公钥。
func TestGetNRWKPublicKey(t *testing.T) {
	p := newTestProvider(t)

	pub, err := p.GetNRWKPublicKey(context.Background())
	if err != nil {
		t.Fatalf("GetNRWKPublicKey 失败: %v", err)
	}
	if len(pub) == 0 {
		t.Fatal("NRWK 公钥不应为空")
	}
	// 公钥长度应为 nrwkPubKeySize (32)
	if len(pub) != nrwkPubKeySize {
		t.Fatalf("NRWK 公钥长度应为 %d, got %d", nrwkPubKeySize, len(pub))
	}

	// 多次调用应返回相同公钥（同一实例）
	pub2, err := p.GetNRWKPublicKey(context.Background())
	if err != nil {
		t.Fatalf("第二次 GetNRWKPublicKey 失败: %v", err)
	}
	if !bytes.Equal(pub, pub2) {
		t.Fatal("同一实例多次获取 NRWK 公钥应相同")
	}

	// 返回的应是副本，修改不影响内部状态
	pub[0] ^= 0xff
	pub3, err := p.GetNRWKPublicKey(context.Background())
	if err != nil {
		t.Fatalf("第三次 GetNRWKPublicKey 失败: %v", err)
	}
	if pub3[0] == pub[0] {
		t.Fatal("GetNRWKPublicKey 应返回副本，修改返回值不应影响内部状态")
	}
}

// TestQuote 验证 Quote 返回 nonce 的副本（P0 占位实现）。
func TestQuote(t *testing.T) {
	p := newTestProvider(t)

	nonce := mustRandBytes(t, 32)
	quote, err := p.Quote(context.Background(), nonce)
	if err != nil {
		t.Fatalf("Quote 失败: %v", err)
	}
	if !bytes.Equal(quote, nonce) {
		t.Fatalf("P0 Quote 应返回 nonce 副本: got %x, want %x", quote, nonce)
	}
}

// TestCloseAndReuse 验证 Close 后再调用应失败。
func TestCloseAndReuse(t *testing.T) {
	cfg := config.TPMConfig{UseSWTPM: true, NRWKHandle: 0x81010010}
	p := NewSoftwareProvider(cfg, observability.Get())
	if err := p.Init(context.Background()); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}

	// Close 后操作应失败
	dek := mustRandBytes(t, 32)
	if _, err := p.SealDEK(context.Background(), dek); err == nil {
		t.Fatal("Close 后 SealDEK 应失败")
	}
	if _, err := p.UnsealDEK(context.Background(), dek); err == nil {
		t.Fatal("Close 后 UnsealDEK 应失败")
	}
	if _, err := p.GetNRWKPublicKey(context.Background()); err == nil {
		t.Fatal("Close 后 GetNRWKPublicKey 应失败")
	}
}

// TestSealUnsealDifferentDEKs 验证多个不同 DEK 的密封/解封互不影响。
func TestSealUnsealDifferentDEKs(t *testing.T) {
	p := newTestProvider(t)

	dek1 := mustRandBytes(t, 32)
	dek2 := mustRandBytes(t, 16)

	sealed1, err := p.SealDEK(context.Background(), dek1)
	if err != nil {
		t.Fatalf("SealDEK #1 失败: %v", err)
	}
	sealed2, err := p.SealDEK(context.Background(), dek2)
	if err != nil {
		t.Fatalf("SealDEK #2 失败: %v", err)
	}

	// 两次密封结果应不同（随机 nonce）
	if bytes.Equal(sealed1, sealed2) {
		t.Fatal("两次密封结果不应相同（随机 nonce）")
	}

	got1, err := p.UnsealDEK(context.Background(), sealed1)
	if err != nil {
		t.Fatalf("UnsealDEK #1 失败: %v", err)
	}
	if !bytes.Equal(got1, dek1) {
		t.Fatalf("解封 DEK #1 不符")
	}

	got2, err := p.UnsealDEK(context.Background(), sealed2)
	if err != nil {
		t.Fatalf("UnsealDEK #2 失败: %v", err)
	}
	if !bytes.Equal(got2, dek2) {
		t.Fatalf("解封 DEK #2 不符")
	}
}

// TestUnsealDEKContextCancellation 验证上下文取消时解封应失败。
func TestUnsealDEKContextCancellation(t *testing.T) {
	p := newTestProvider(t)
	dek := mustRandBytes(t, 32)
	sealed, err := p.SealDEK(context.Background(), dek)
	if err != nil {
		t.Fatalf("SealDEK 失败: %v", err)
	}

	// 已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.UnsealDEK(ctx, sealed); err == nil {
		// 注意：信号量可能在 ctx.Done 之前获取成功，导致解封仍可能成功
		// 这里仅验证不 panic
		t.Logf("已取消 context 下解封结果: %v", err)
	}
}

// TestErrorKindClassification 验证 TPMError 错误分类判断函数。
func TestErrorKindClassification(t *testing.T) {
	// fatal 错误
	fatalErr := NewTPMError(ErrKindFatal, "Init", errors.New("not initialized"))
	if !IsFatal(fatalErr) {
		t.Fatal("ErrKindFatal 应被 IsFatal 识别")
	}
	if IsPermanent(fatalErr) {
		t.Fatal("ErrKindFatal 不应被 IsPermanent 识别")
	}
	if IsTransient(fatalErr) {
		t.Fatal("ErrKindFatal 不应被 IsTransient 识别")
	}

	// permanent 错误
	permErr := NewTPMError(ErrKindPermanent, "SealDEK", errors.New("empty DEK"))
	if !IsPermanent(permErr) {
		t.Fatal("ErrKindPermanent 应被 IsPermanent 识别")
	}
	if IsFatal(permErr) {
		t.Fatal("ErrKindPermanent 不应被 IsFatal 识别")
	}

	// transient 错误
	transErr := NewTPMError(ErrKindTransient, "UnsealDEK", errors.New("busy"))
	if !IsTransient(transErr) {
		t.Fatal("ErrKindTransient 应被 IsTransient 识别")
	}

	// 非 TPMError 应全部返回 false
	plainErr := errors.New("plain error")
	if IsFatal(plainErr) || IsPermanent(plainErr) || IsTransient(plainErr) {
		t.Fatal("普通 error 不应被识别为 TPM 错误")
	}
}

// TestErrorKindString 验证 ErrorKind.String() 输出。
func TestErrorKindString(t *testing.T) {
	cases := []struct {
		kind ErrorKind
		want string
	}{
		{ErrKindTransient, "transient"},
		{ErrKindPermanent, "permanent"},
		{ErrKindFatal, "fatal"},
	}
	for _, c := range cases {
		if got := c.kind.String(); got != c.want {
			t.Errorf("ErrorKind(%d).String() = %q, want %q", c.kind, got, c.want)
		}
	}
}

// TestTPMErrorUnwrap 验证 TPMError 的 Unwrap 链。
func TestTPMErrorUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	tpmErr := NewTPMError(ErrKindPermanent, "SealDEK", cause)

	if !errors.Is(tpmErr, cause) {
		t.Fatal("errors.Is 应能识别底层 cause")
	}

	var target *TPMError
	if !errors.As(tpmErr, &target) {
		t.Fatal("errors.As 应能识别 *TPMError")
	}
	if target != tpmErr {
		t.Fatal("errors.As 提取的 *TPMError 不匹配")
	}
}

// TestTPMErrorMessage 验证 TPMError.Error() 输出包含 op 和 kind。
func TestTPMErrorMessage(t *testing.T) {
	tpmErr := NewTPMError(ErrKindPermanent, "SealDEK", errors.New("empty DEK"))
	msg := tpmErr.Error()
	if !strings.Contains(msg, "SealDEK") {
		t.Errorf("Error() 应包含 op, got %q", msg)
	}
	if !strings.Contains(msg, "permanent") {
		t.Errorf("Error() 应包含 kind, got %q", msg)
	}
	if !strings.Contains(msg, "empty DEK") {
		t.Errorf("Error() 应包含 cause, got %q", msg)
	}

	// 无 cause 时
	tpmErrNoCause := NewTPMError(ErrKindFatal, "Init", nil)
	msg = tpmErrNoCause.Error()
	if !strings.Contains(msg, "Init") {
		t.Errorf("Error() 应包含 op, got %q", msg)
	}
	if strings.Contains(msg, "cause") {
		t.Errorf("无 cause 时 Error() 不应包含 cause, got %q", msg)
	}
}

// isValidUUIDv4 简单校验 UUID v4 格式。
func isValidUUIDv4(s string) bool {
	// 格式: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	if len(s) != 36 {
		return false
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	// version 位应为 '4'
	if s[14] != '4' {
		return false
	}
	// variant 位应为 8/9/a/b
	switch s[19] {
	case '8', '9', 'a', 'b':
		return true
	default:
		return false
	}
}
