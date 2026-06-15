package aes

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
)

func TestAES_GCM_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	plain := bytes.Repeat([]byte("A"), 1024)
	c, err := New(common.GCM, 32)
	require.NoError(t, err)
	ct, iv, tag, err := c.Encrypt(key, plain, nil, []byte("aad"))
	require.NoError(t, err)
	require.Equal(t, len(plain), len(ct))
	require.Equal(t, 12, len(iv))
	require.Equal(t, 16, len(tag))
	pt, err := c.Decrypt(key, ct, iv, tag, []byte("aad"))
	require.NoError(t, err)
	require.True(t, bytes.Equal(plain, pt))
}

func TestAES_GCM_TamperRejected(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	plain := []byte("hello, world")
	c, _ := New(common.GCM, 32)
	ct, iv, tag, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	tag[0] ^= 0x01
	_, err = c.Decrypt(key, ct, iv, tag, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrAuth)
}

func TestAES_CBC_HMAC_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 16)
	plain := []byte("the quick brown fox jumps over the lazy dog")
	c, _ := New(common.CBC, 16)
	ct, iv, tag, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	pt, err := c.Decrypt(key, ct, iv, tag, nil)
	require.NoError(t, err)
	require.Equal(t, plain, pt)
}

func TestAES_CBC_HMAC_TamperRejected(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 16)
	plain := []byte("hello-cbc")
	c, _ := New(common.CBC, 16)
	ct, iv, tag, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	tag[5] ^= 0xFF
	_, err = c.Decrypt(key, ct, iv, tag, nil)
	require.Error(t, err)
}

func TestAES_CTR_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x22}, 32)
	plain := []byte("stream-mode-ctr-test-data")
	c, _ := New(common.CTR, 32)
	ct, iv, _, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 16, len(iv))
	pt, err := c.Decrypt(key, ct, iv, nil, nil)
	require.NoError(t, err)
	require.Equal(t, plain, pt)
}

func TestAES_CFB_OFB_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x33}, 16)
	plain := []byte("stream-mode-test-data-1234")
	for _, mode := range []common.BlockMode{common.CFB, common.OFB} {
		c, _ := New(mode, 16)
		ct, iv, _, err := c.Encrypt(key, plain, nil, nil)
		require.NoError(t, err)
		pt, err := c.Decrypt(key, ct, iv, nil, nil)
		require.NoError(t, err)
		require.Equal(t, plain, pt, "mode=%s", mode)
	}
}

func TestAES_ECB_SingleBlock(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 16)
	plain := []byte("1234567890ab") // 12 字节 < 16
	c, _ := New(common.ECB, 16)
	ct, _, _, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	pt, err := c.Decrypt(key, ct, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, plain, pt)
}

func TestAES_ECB_RefusesMultiBlock(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 16)
	plain := bytes.Repeat([]byte{0xAA}, 32)
	c, _ := New(common.ECB, 16)
	_, _, _, err := c.Encrypt(key, plain, nil, nil)
	require.ErrorIs(t, err, common.ErrInvalidArgument)
}

func TestAES_UnsupportedKeyLength(t *testing.T) {
	_, err := New(common.GCM, 20) // 160 bit 无效
	require.Error(t, err)
}

func TestAES_GCM_Stable(t *testing.T) {
	// 加密两次,密文必须不同(因为 nonce 随机)
	key := bytes.Repeat([]byte{0x11}, 16)
	c, _ := New(common.GCM, 16)
	plain := []byte("repeat")
	ct1, iv1, tag1, _ := c.Encrypt(key, plain, nil, nil)
	ct2, iv2, tag2, _ := c.Encrypt(key, plain, nil, nil)
	require.NotEqual(t, iv1, iv2)
	require.NotEqual(t, ct1, ct2)
	require.NotEqual(t, tag1, tag2)
}
