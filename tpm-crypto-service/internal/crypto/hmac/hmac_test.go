package hmac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveAndVerify(t *testing.T) {
	base := []byte("0123456789abcdef0123456789abcdef") // 32B
	kEnc, kMac := DeriveSubKeys(base)
	require.Len(t, kEnc, 32)
	require.Len(t, kMac, 32)

	// 派生稳定
	kEnc2, kMac2 := DeriveSubKeys(base)
	require.Equal(t, kEnc, kEnc2)
	require.Equal(t, kMac, kMac2)

	// Verify 正确路径
	msg := []byte("hello, world")
	mac := Compute(kMac, msg)
	require.NoError(t, Verify(kMac, msg, mac))

	// Verify 错误路径
	require.Error(t, Verify(kMac, msg, []byte("not-a-real-mac-aaaaaaaaaaaaaaaa")))
}
