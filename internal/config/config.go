package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the runtime configuration for the key-vault service.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	TPM       TPMConfig       `yaml:"tpm"`
	Auth      AuthConfig      `yaml:"auth"`
	Policy    PolicyConfig    `yaml:"policy"`
	Audit     AuditConfig     `yaml:"audit"`
	Nonce     NonceConfig     `yaml:"nonce"`
	DataKey   DataKeyConfig   `yaml:"datakey"`
	Baseline  BaselineConfig  `yaml:"baseline"`
	LogLevel  string          `yaml:"log_level"`
}

type ServerConfig struct {
	HTTPListenAddr     string        `yaml:"http_listen_addr"`
	HTTPSListenAddr    string        `yaml:"https_listen_addr"`
	TLSCertFile        string        `yaml:"tls_cert_file"`
	TLSKeyFile         string        `yaml:"tls_key_file"`
	ReadTimeout        time.Duration `yaml:"read_timeout"`
	WriteTimeout       time.Duration `yaml:"write_timeout"`
	MaxRequestBody     int           `yaml:"max_request_body"` // bytes; default 64 KiB
	PlaneIsolationMode string        `yaml:"plane_isolation_mode"` // "logical" (P0) or "physical"
	IPAllowlist        []string      `yaml:"ip_allowlist"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"` // "postgres" or "memory"
	DSN    string `yaml:"dsn"`
}

type TPMConfig struct {
	Provider string `yaml:"provider"` // "swtpm" (software-backed for P0) or "tpm2"
	StateDir string `yaml:"state_dir"`
}

type AuthConfig struct {
	// JWT validation
	JWTIssuer    string   `yaml:"jwt_issuer"`
	JWTAudience  string   `yaml:"jwt_audience"`
	JWTAlgWhite  []string `yaml:"jwt_alg_whitelist"` // e.g. ["RS256","ES256"]
	JWKSetURL    string   `yaml:"jwk_set_url"`
	DefaultTokenTTL time.Duration `yaml:"default_token_ttl"`

	// HMAC request signing
	HMACEnabled    bool   `yaml:"hmac_enabled"`
	HMACSecretB64  string `yaml:"hmac_secret_b64"`
	HMACMaxSkew    time.Duration `yaml:"hmac_max_skew"`

	// Static service tokens (P0 bootstrap): token -> principal mapping.
	// Loaded from a file path or env var; never logged.
	StaticTokensFile string `yaml:"static_tokens_file"`
}

type PolicyConfig struct {
	ConfigPath string `yaml:"config_path"`
}

type AuditConfig struct {
	WALDir          string        `yaml:"wal_dir"`
	WALMaxSizeBytes int64         `yaml:"wal_max_size_bytes"`
	WALEnabled      bool          `yaml:"wal_enabled"`
	BufferSize      int           `yaml:"buffer_size"`
}

type NonceConfig struct {
	LeaseSize          uint64        `yaml:"lease_size"`           // counters per lease
	PrefetchWatermark  float64       `yaml:"prefetch_watermark"`   // 0.70
	ThrottleWatermark  float64       `yaml:"throttle_watermark"`   // 0.90
	LeaseTTL           time.Duration `yaml:"lease_ttl"`
	RateWindow         time.Duration `yaml:"rate_window"`
	RateSigmaThreshold float64       `yaml:"rate_sigma_threshold"`
	UnusedRatioAlert   float64       `yaml:"unused_ratio_alert"`
}

type DataKeyConfig struct {
	DefaultTTL  time.Duration `yaml:"default_ttl"`
	MaxTTL      time.Duration `yaml:"max_ttl"`
	QuotaPerMin int           `yaml:"quota_per_min"`
}

type BaselineConfig struct {
	// Whitelists for host security baseline (design §6.6).
	SELinuxRequired     bool     `yaml:"selinux_required"`
	AllowedKernelVers   []string `yaml:"allowed_kernel_versions"`
	AllowedTPM2TSSVers  []string `yaml:"allowed_tpm2_tss_versions"`
	AllowedVirtPlatform []string `yaml:"allowed_virtualization_platforms"`
}

// Default returns a config with safe P0 defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPListenAddr:     ":8080",
			ReadTimeout:        15 * time.Second,
			WriteTimeout:       15 * time.Second,
			MaxRequestBody:     64 * 1024,
			PlaneIsolationMode: "logical",
		},
		Database: DatabaseConfig{Driver: "memory"},
		TPM:      TPMConfig{Provider: "swtpm", StateDir: "/tmp/kvlt-tpm"},
		Auth: AuthConfig{
			JWTAlgWhite:      []string{"RS256", "ES256"},
			DefaultTokenTTL:  15 * time.Minute,
			HMACEnabled:      true,
			HMACMaxSkew:      5 * time.Minute,
		},
		Nonce: NonceConfig{
			LeaseSize:          1024,
			PrefetchWatermark:  0.70,
			ThrottleWatermark:  0.90,
			LeaseTTL:           10 * time.Minute,
			RateWindow:         time.Minute,
			RateSigmaThreshold: 3.0,
			UnusedRatioAlert:   0.50,
		},
		DataKey: DataKeyConfig{
			DefaultTTL:  5 * time.Minute,
			MaxTTL:      15 * time.Minute,
			QuotaPerMin: 100,
		},
		Audit: AuditConfig{
			WALDir:          "/tmp/kvlt-wal",
			WALMaxSizeBytes: 64 * 1024 * 1024,
			WALEnabled:      true,
			BufferSize:      1024,
		},
		Baseline: BaselineConfig{
			SELinuxRequired: true,
		},
		LogLevel: "INFO",
	}
}

// Load reads a YAML config file and applies env overrides.
func Load(path string) (*Config, error) {
	c := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(b, c); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}
	// Env overrides (KVLT_ prefix).
	applyEnv(c)
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func applyEnv(c *Config) {
	if v := os.Getenv("KVLT_HTTP_ADDR"); v != "" {
		c.Server.HTTPListenAddr = v
	}
	if v := os.Getenv("KVLT_DB_DRIVER"); v != "" {
		c.Database.Driver = v
	}
	if v := os.Getenv("KVLT_DB_DSN"); v != "" {
		c.Database.DSN = v
	}
	if v := os.Getenv("KVLT_TPM_PROVIDER"); v != "" {
		c.TPM.Provider = v
	}
	if v := os.Getenv("KVLT_JWT_ISSUER"); v != "" {
		c.Auth.JWTIssuer = v
	}
	if v := os.Getenv("KVLT_JWT_AUDIENCE"); v != "" {
		c.Auth.JWTAudience = v
	}
	if v := os.Getenv("KVLT_LOG_LEVEL"); v != "" {
		c.LogLevel = strings.ToUpper(v)
	}
	if v := os.Getenv("KVLT_WAL_DIR"); v != "" {
		c.Audit.WALDir = v
	}
}

// Validate enforces safe P0 defaults.
func (c *Config) Validate() error {
	if c.Server.MaxRequestBody <= 0 || c.Server.MaxRequestBody > 64*1024 {
		// Hard cap per design §12.1: 64 KiB; can be lowered by policy.
		c.Server.MaxRequestBody = 64 * 1024
	}
	if c.Nonce.PrefetchWatermark <= 0 || c.Nonce.PrefetchWatermark >= 1 {
		c.Nonce.PrefetchWatermark = 0.70
	}
	if c.Nonce.ThrottleWatermark <= c.Nonce.PrefetchWatermark || c.Nonce.ThrottleWatermark > 1 {
		c.Nonce.ThrottleWatermark = 0.90
	}
	if c.DataKey.MaxTTL > 15*time.Minute {
		return fmt.Errorf("datakey max_ttl must not exceed 15 minutes per HA-10")
	}
	if c.DataKey.DefaultTTL <= 0 || c.DataKey.DefaultTTL > c.DataKey.MaxTTL {
		c.DataKey.DefaultTTL = 5 * time.Minute
	}
	if c.Auth.HMACEnabled && c.Auth.HMACSecretB64 == "" && c.Auth.StaticTokensFile == "" {
		// Allow no static tokens if JWT is configured.
	}
	if c.Server.PlaneIsolationMode != "logical" && c.Server.PlaneIsolationMode != "physical" {
		c.Server.PlaneIsolationMode = "logical"
	}
	return nil
}
