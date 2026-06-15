package sm4

import (
	"crypto/rand"
	"testing"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
)

func benchKey(b *testing.B) []byte {
	k := make([]byte, 16)
	_, _ = rand.Read(k)
	return k
}

func benchPlain(b *testing.B, n int) []byte {
	p := make([]byte, n)
	_, _ = rand.Read(p)
	return p
}

func BenchmarkSM4_GCM_1KB(b *testing.B) {
	c, _ := New(common.GCM)
	key := benchKey(b)
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

func BenchmarkSM4_GCM_64KB(b *testing.B) {
	c, _ := New(common.GCM)
	key := benchKey(b)
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

func BenchmarkSM4_CBC_HMAC_1KB(b *testing.B) {
	c, _ := New(common.CBC)
	key := benchKey(b)
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
