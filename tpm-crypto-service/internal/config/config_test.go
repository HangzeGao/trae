package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	c := Default()
	require.Equal(t, ":9090", c.Server.GRPCAddr)
	require.Equal(t, ":8080", c.Server.HTTPAddr)
	require.Equal(t, 30*time.Second, c.Server.ShutdownTimeout)
	require.Equal(t, "isolated", c.Deploy.Mode)
	require.Equal(t, "auto", c.Crypto.Backend)
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	yaml := `server:
  grpc_addr: ":19090"
  http_addr: ":18080"
deploy:
  mode: "cluster"
crypto:
  backend: "openssl"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs)
	require.NoError(t, fs.Parse([]string{"--config=" + path}))

	c, err := Load(fs)
	require.NoError(t, err)
	require.Equal(t, ":19090", c.Server.GRPCAddr)
	require.Equal(t, ":18080", c.Server.HTTPAddr)
	require.Equal(t, "cluster", c.Deploy.Mode)
	require.Equal(t, "openssl", c.Crypto.Backend)
}

func TestFlagOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	yaml := `server:
  grpc_addr: ":19090"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs)
	require.NoError(t, fs.Parse([]string{"--config=" + path, "--server.grpc_addr=:29999"}))

	c, err := Load(fs)
	require.NoError(t, err)
	require.Equal(t, ":29999", c.Server.GRPCAddr)
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	yaml := `log:
  level: "info"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	BindFlags(fs)
	require.NoError(t, fs.Parse([]string{"--config=" + path}))

	t.Setenv("TCS_LOG_LEVEL", "debug")
	c, err := Load(fs)
	require.NoError(t, err)
	require.Equal(t, "debug", c.Log.Level)
}
