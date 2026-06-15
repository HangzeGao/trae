package aes

import (
	"crypto/rand"
	"testing"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
)

// benchmarks 仅用于 go test -bench,生成 bench/results/... 与 baseline 比对。
// 不在单元测试覆盖率统计中。

func benchKey(b *testing.B, n int) []byte {
	k := make([]byte, n)
	_, _ = rand.Read(k)
	return k
}

func benchPlain(b *testing.B, n int) []byte {
	p := make([]byte, n)
	_, _ = rand.Read(p)
	return p
}

func BenchmarkAES128_GCM_1KB(b *testing.B) {
	c, _ := New(common.GCM, 16)
	key := benchKey(b, 16)
	plain := benchPlain(b, 1024)
	iv := make([]byte, 12)
	_, _ = rand.Read(iv)
	b.ReportAllocs()
	b.SetBytes(int64(len(plain)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, _, tag, err := c.Encrypt(key, plain, iv, nil)
		if err != nil {
			b.Fatal(err)
		}
		_, err = c.Decrypt(key, ct, iv, tag, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAES256_GCM_1KB(b *testing.B) {
	c, _ := New(common.GCM, 32)
	key := benchKey(b, 32)
	plain := benchPlain(b, 1024)
	iv := make([]byte, 12)
	_, _ = rand.Read(iv)
	b.ReportAllocs()
	b.SetBytes(int64(len(plain)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, _, tag, err := c.Encrypt(key, plain, iv, nil)
		if err != nil {
			b.Fatal(err)
		}
		_, err = c.Decrypt(key, ct, iv, tag, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAES128_GCM_64KB(b *testing.B) {
	c, _ := New(common.GCM, 16)
	key := benchKey(b, 16)
	plain := benchPlain(b, 64*1024)
	iv := make([]byte, 12)
	_, _ = rand.Read(iv)
	b.ReportAllocs()
	b.SetBytes(int64(len(plain)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, _, tag, err := c.Encrypt(key, plain, iv, nil)
		if err != nil {
			b.Fatal(err)
		}
		_, err = c.Decrypt(key, ct, iv, tag, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAES128_CBC_HMAC_1KB(b *testing.B) {
	c, _ := New(common.CBC, 16)
	key := benchKey(b, 16)
	plain := benchPlain(b, 1024)
	iv := make([]byte, 16)
	_, _ = rand.Read(iv)
	b.ReportAllocs()
	b.SetBytes(int64(len(plain)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, _, tag, err := c.Encrypt(key, plain, iv, nil)
		if err != nil {
			b.Fatal(err)
		}
		_, err = c.Decrypt(key, ct, iv, tag, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAES128_CTR_64KB(b *testing.B) {
	c, _ := New(common.CTR, 16)
	key := benchKey(b, 16)
	plain := benchPlain(b, 64*1024)
	iv := make([]byte, 16)
	_, _ = rand.Read(iv)
	b.ReportAllocs()
	b.SetBytes(int64(len(plain)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, _, _, err := c.Encrypt(key, plain, iv, nil)
		if err != nil {
			b.Fatal(err)
		}
		_, err = c.Decrypt(key, ct, iv, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
