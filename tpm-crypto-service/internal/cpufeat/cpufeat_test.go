package cpufeat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetect(t *testing.T) {
	f := Detect()
	m := f.Map()
	require.Len(t, m, 8)
	// 在 x86-64 平台上 SSE4.1 几乎总为 true
	require.True(t, m["sse4_1"], "SSE4.1 should be present on amd64")
}

func TestMapRoundtrip(t *testing.T) {
	f := Features{AESNI: true, GFNI: true}
	m := f.Map()
	require.Equal(t, true, m["aesni"])
	require.Equal(t, true, m["gfni"])
	require.Equal(t, false, m["avx512f"])
	require.Len(t, m, 8)
}

func TestHasFastPaths(t *testing.T) {
	f := Detect()
	_ = f.HasAESGCMFastPath()
	_ = f.HasSM4FastPath()
}
