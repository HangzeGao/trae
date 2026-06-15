package tpm

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenSimulator(t *testing.T) {
	d, err := Open(Options{Mode: ModeSimulator})
	require.NoError(t, err)
	require.NotNil(t, d)
	require.Equal(t, "simulator", d.Mode())
	require.NoError(t, d.Close())
}

func TestOpenAutoFallsBackToSimulator(t *testing.T) {
	d, err := Open(Options{Mode: ModeAuto, DevicePath: "/nonexistent/tpm0"})
	require.NoError(t, err)
	require.Equal(t, "simulator", d.Mode())
	require.NoError(t, d.Close())
}

func TestOpenPhysicalFailsWithoutDevice(t *testing.T) {
	_, err := Open(Options{Mode: ModePhysical, DevicePath: "/nonexistent/tpm0"})
	require.Error(t, err)
}

func TestSRKSealUnseal(t *testing.T) {
	dir := t.TempDir()
	srk, err := EnsureSRK("simulator", dir+"/srk.state")
	require.NoError(t, err)
	ark := make([]byte, 32)
	_, _ = rand.Read(ark)
	sealed, err := srk.SealARK(ark, []int{0, 1, 2, 3, 4, 5, 6, 7})
	require.NoError(t, err)
	require.Equal(t, sealedARKVersion, sealed.Version)
	require.Equal(t, 256, len(sealed.Cipher))

	got, err := srk.UnsealARK(sealed)
	require.NoError(t, err)
	require.Equal(t, ark, got)
}

func TestSRKPersist(t *testing.T) {
	dir := t.TempDir()
	statePath := dir + "/srk.state"
	arkPath := dir + "/sealed.gob"

	// 第一次:创建并封存
	srk1, err := EnsureSRK("simulator", statePath)
	require.NoError(t, err)
	ark := make([]byte, 32)
	_, _ = rand.Read(ark)
	sealed, err := srk1.SealARK(ark, []int{0, 7})
	require.NoError(t, err)
	require.NoError(t, SaveSealedARK(arkPath, sealed))

	// 第二次:重新加载 SRK 并解封
	srk2, err := EnsureSRK("simulator", statePath)
	require.NoError(t, err)
	loaded, err := LoadSealedARK(arkPath)
	require.NoError(t, err)
	got, err := srk2.UnsealARK(loaded)
	require.NoError(t, err)
	require.Equal(t, ark, got)
}
