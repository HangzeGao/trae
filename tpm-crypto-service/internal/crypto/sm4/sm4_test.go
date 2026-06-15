package sm4

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
)

func sm4Key(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 16)
	_, _ = rand.Read(k)
	return k
}

func TestSM4_GCM_RoundTrip(t *testing.T) {
	key := sm4Key(t)
	plain := bytes.Repeat([]byte("X"), 512)
	c, err := New(common.GCM)
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

func TestSM4_GCM_TamperRejected(t *testing.T) {
	key := sm4Key(t)
	plain := bytes.Repeat([]byte("a"), 64) // gmsm Sm4GCM 对 < 16 字节输入有缺陷
	c, _ := New(common.GCM)
	ct, iv, tag, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	tag[0] ^= 0xFF
	_, err = c.Decrypt(key, ct, iv, tag, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, common.ErrAuth)
}

func TestSM4_CBC_HMAC_RoundTrip(t *testing.T) {
	key := sm4Key(t)
	plain := []byte("the sm4 cbc round trip test")
	c, _ := New(common.CBC)
	ct, iv, tag, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	pt, err := c.Decrypt(key, ct, iv, tag, nil)
	require.NoError(t, err)
	require.Equal(t, plain, pt)
}

func TestSM4_CTR_RoundTrip(t *testing.T) {
	key := sm4Key(t)
	plain := []byte("sm4-ctr-stream-test")
	c, _ := New(common.CTR)
	ct, iv, _, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	pt, err := c.Decrypt(key, ct, iv, nil, nil)
	require.NoError(t, err)
	require.Equal(t, plain, pt)
}

func TestSM4_CFB_OFB_RoundTrip(t *testing.T) {
	key := sm4Key(t)
	plain := []byte("sm4-stream-modes-data")
	for _, mode := range []common.BlockMode{common.CFB, common.OFB} {
		c, _ := New(mode)
		ct, iv, _, err := c.Encrypt(key, plain, nil, nil)
		require.NoError(t, err)
		pt, err := c.Decrypt(key, ct, iv, nil, nil)
		require.NoError(t, err)
		require.Equal(t, plain, pt, "mode=%s", mode)
	}
}

func TestSM4_ECB_SingleBlock(t *testing.T) {
	key := sm4Key(t)
	plain := []byte("123456789012") // 12 字节
	c, _ := New(common.ECB)
	ct, _, _, err := c.Encrypt(key, plain, nil, nil)
	require.NoError(t, err)
	pt, err := c.Decrypt(key, ct, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, plain, pt)
}

func TestSM4_ECB_RefusesMultiBlock(t *testing.T) {
	key := sm4Key(t)
	plain := bytes.Repeat([]byte{0xAA}, 32)
	c, _ := New(common.ECB)
	_, _, _, err := c.Encrypt(key, plain, nil, nil)
	require.ErrorIs(t, err, common.ErrInvalidArgument)
}

func TestSM4_KeyLengthFixed(t *testing.T) {
	c, err := New(common.GCM)
	require.NoError(t, err)
	require.Equal(t, 16, c.KeyLength())
}
