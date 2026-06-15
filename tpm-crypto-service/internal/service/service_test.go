package service_test

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/tpm-crypto/tpm-crypto-service/internal/cpufeat"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
	"github.com/tpm-crypto/tpm-crypto-service/internal/keystore"
	"github.com/tpm-crypto/tpm-crypto-service/internal/obs"
	"github.com/tpm-crypto/tpm-crypto-service/internal/service"
	"github.com/tpm-crypto/tpm-crypto-service/internal/tpm"
)

func newTestService(t *testing.T) (*service.Service, func()) {
	t.Helper()
	dir := t.TempDir()
	srkPath := filepath.Join(dir, "srk.state")
	sealedPath := filepath.Join(dir, "ark.blob")

	srk, err := tpm.EnsureSRK("simulator", srkPath)
	if err != nil {
		t.Fatalf("ensure SRK: %v", err)
	}
	ark := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, ark); err != nil {
		t.Fatalf("rand: %v", err)
	}
	s, err := srk.SealARK(ark, []int{0, 1, 2, 3, 4, 5, 6, 7})
	if err != nil {
		t.Fatalf("seal ARK: %v", err)
	}
	if err := tpm.SaveSealedARK(sealedPath, s); err != nil {
		t.Fatalf("save sealed ARK: %v", err)
	}

	factory, err := crypto.NewFactory("auto", cpufeat.Detect())
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	metrics := obs.NewMetricsWith(prometheus.NewRegistry())
	ks := keystore.NewMemory()

	svc := &service.Service{
		KS:      ks,
		SRK:     srk,
		Factory: factory,
		ARK:     ark,
		Log:     zap.NewNop(),
		Metrics: metrics,
		TPMMode: "simulator",
		Caller:  "test",
	}
	cleanup := func() {
		_ = os.RemoveAll(dir)
	}
	return svc, cleanup
}

func TestService_CreateDescribeRotateRevoke(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()

	meta, err := svc.CreateDataKey(ctx, "k1", common.AES, 256)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if meta.Version != 1 || meta.Status != common.Active {
		t.Fatalf("unexpected meta: %+v", meta)
	}
	dm, err := svc.DescribeDataKey(ctx, "k1", 0)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if dm.KeyID != "k1" {
		t.Fatalf("describe wrong key id: %+v", dm)
	}
	rmeta, err := svc.RotateDataKey(ctx, "k1")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rmeta.Version != 2 {
		t.Fatalf("rotate version: %d", rmeta.Version)
	}
	// 旧版本应可继续解密
	old, err := svc.DescribeDataKey(ctx, "k1", 1)
	if err != nil {
		t.Fatalf("describe v1: %v", err)
	}
	if old.Status != common.Retired {
		t.Fatalf("v1 status: %s", old.Status)
	}
	// revoke
	if _, err := svc.RevokeDataKey(ctx, "k1", 0); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.DescribeDataKey(ctx, "k1", 0); err == nil {
		t.Fatalf("describe after revoke should fail")
	}
}

func TestService_EncryptDecrypt_AESGCM(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()
	if _, err := svc.CreateDataKey(ctx, "k-aes", common.AES, 256); err != nil {
		t.Fatalf("create: %v", err)
	}
	plain := []byte("hello world — AES-GCM 测试明文")
	aad := []byte("user=42")
	ct, iv, tag, usedV, err := svc.Encrypt(ctx, "k-aes", 0, common.AES, common.GCM, plain, aad)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if usedV != 1 {
		t.Fatalf("used version: %d", usedV)
	}
	pt, err := svc.Decrypt(ctx, "k-aes", usedV, common.AES, common.GCM, ct, iv, tag, aad)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(pt) != string(plain) {
		t.Fatalf("plain mismatch: %q vs %q", pt, plain)
	}
}

func TestService_EncryptDecrypt_SM4CBC(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()
	if _, err := svc.CreateDataKey(ctx, "k-sm4", common.SM4, 128); err != nil {
		t.Fatalf("create sm4: %v", err)
	}
	// CBC + HMAC,需要 ≥32 字节
	plain := make([]byte, 64)
	for i := range plain {
		plain[i] = byte(i)
	}
	ct, iv, tag, usedV, err := svc.Encrypt(ctx, "k-sm4", 0, common.SM4, common.CBC, plain, nil)
	if err != nil {
		t.Fatalf("sm4-cbc encrypt: %v", err)
	}
	pt, err := svc.Decrypt(ctx, "k-sm4", usedV, common.SM4, common.CBC, ct, iv, tag, nil)
	if err != nil {
		t.Fatalf("sm4-cbc decrypt: %v", err)
	}
	if string(pt) != string(plain) {
		t.Fatalf("sm4 cbc plain mismatch")
	}
}

func TestService_DuplicateKeyID(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()
	if _, err := svc.CreateDataKey(ctx, "dup", common.AES, 128); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.CreateDataKey(ctx, "dup", common.AES, 128)
	if err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestService_RevokedKeyRefusesDecrypt(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()
	ctx := context.Background()
	if _, err := svc.CreateDataKey(ctx, "revk", common.AES, 128); err != nil {
		t.Fatalf("create: %v", err)
	}
	// 加密成功
	ct, iv, tag, usedV, err := svc.Encrypt(ctx, "revk", 0, common.AES, common.GCM, []byte("p"), nil)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// revoke 该版本
	if _, err := svc.RevokeDataKey(ctx, "revk", usedV); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.Decrypt(ctx, "revk", usedV, common.AES, common.GCM, ct, iv, tag, nil); err == nil {
		t.Fatalf("decrypt should fail on revoked key")
	}
}
