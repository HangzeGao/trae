package config

import (
	"fmt"
	"os"
	"time"
)

// Config 是服务总配置。
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	TPM       TPMConfig       `yaml:"tpm"`
	Auth      AuthConfig      `yaml:"auth"`
	Crypto    CryptoConfig    `yaml:"crypto"`
	Audit     AuditConfig     `yaml:"audit"`
	Logging   LoggingConfig   `yaml:"logging"`
}

type ServerConfig struct {
	HTTPAddr        string        `yaml:"http_addr"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
}

type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Name            string        `yaml:"name"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	SSLMode         string        `yaml:"ssl_mode"`
	MaxConns        int32         `yaml:"max_conns"`
	MinConns        int32         `yaml:"min_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode)
}

type TPMConfig struct {
	DevicePath     string `yaml:"device_path"`     // /dev/tpm0 或 swtpm socket
	UseSWTPM       bool   `yaml:"use_swtpm"`       // P0 集成测试用 swtpm
	SWTPMSocket    string `yaml:"swtpm_socket"`
	NRWKHandle     uint32 `yaml:"nrwk_handle"`     // 持久句柄 0x81010010
	AKHandle       uint32 `yaml:"ak_handle"`       // 0x81010100
}

type AuthConfig struct {
	JWTIssuer        string        `yaml:"jwt_issuer"`
	JWTAudience      string        `yaml:"jwt_audience"`
	JWKURL           string        `yaml:"jwk_url"`
	JWKCacheTTL      time.Duration `yaml:"jwk_cache_ttl"`
	TokenMaxTTL      time.Duration `yaml:"token_max_ttl"`      // 高权限 token 默认 ≤15min
	HMACSecretRef    string        `yaml:"hmac_secret_ref"`    // 引用，不存明文
	TimestampSkew    time.Duration `yaml:"timestamp_skew"`     // 默认 300s
	NonceWindow      time.Duration `yaml:"nonce_window"`       // 至少 2 倍 skew
}

type CryptoConfig struct {
	DefaultSuite    string `yaml:"default_suite"`    // AES_256_GCM
	DEKCacheTTL     time.Duration `yaml:"dek_cache_ttl"`     // 1-5 分钟
	DEKCacheMaxSize int    `yaml:"dek_cache_max_size"`
	NonceLeaseSize  uint64 `yaml:"nonce_lease_size"`  // 单次分配区间大小
	NoncePrefetchAt float64 `yaml:"nonce_prefetch_at"` // 0.70
	NonceCriticalAt float64 `yaml:"nonce_critical_at"` // 0.90
	ResolverTimeout time.Duration `yaml:"resolver_timeout"` // 跨平面调用超时 2s
	TPMUnsealConcurrency int `yaml:"tpm_unseal_concurrency"` // 默认 1-2
}

type AuditConfig struct {
	WALDir          string        `yaml:"wal_dir"`           // 本地 WAL 目录
	WALMaxSize      int64         `yaml:"wal_max_size"`      // 单文件最大字节
	WALRetention    time.Duration `yaml:"wal_retention"`
	HighRiskOps     []string      `yaml:"high_risk_ops"`     // 高风险操作列表
}

type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug/info/warn/error
	Format string `yaml:"format"` // json/text
}

// Default 返回安全默认值配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPAddr:        ":8443",
			ShutdownTimeout: 30 * time.Second,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
		},
		Database: DatabaseConfig{
			Host: "localhost", Port: 5432, Name: "keyvault",
			User: "kv_app_rw", Password: "",
			SSLMode: "disable", MaxConns: 20, MinConns: 2,
			ConnMaxLifetime: 30 * time.Minute,
		},
		TPM: TPMConfig{
			UseSWTPM:    true,
			SWTPMSocket: "/tmp/swtpm.sock",
			NRWKHandle:  0x81010010,
			AKHandle:    0x81010100,
		},
		Auth: AuthConfig{
			JWTIssuer:     "key-vault",
			JWTAudience:   "key-vault",
			TokenMaxTTL:   15 * time.Minute,
			TimestampSkew: 300 * time.Second,
			NonceWindow:   600 * time.Second,
			JWKCacheTTL:   5 * time.Minute,
		},
		Crypto: CryptoConfig{
			DefaultSuite:         "AES_256_GCM",
			DEKCacheTTL:          3 * time.Minute,
			DEKCacheMaxSize:      10000,
			NonceLeaseSize:       1 << 16,
			NoncePrefetchAt:      0.70,
			NonceCriticalAt:      0.90,
			ResolverTimeout:      2 * time.Second,
			TPMUnsealConcurrency: 2,
		},
		Audit: AuditConfig{
			WALDir:       "/var/lib/keyvault/wal",
			WALMaxSize:   64 << 20,
			WALRetention: 7 * 24 * time.Hour,
			HighRiskOps: []string{
				"crk.create", "crk.rotate", "node.register", "node.revoke",
				"key.destroy", "policy.downgrade", "key.rotate",
			},
		},
		Logging: LoggingConfig{Level: "info", Format: "json"},
	}
}

// FromEnv 从环境变量覆盖配置（简化版，P0 支持 ENV 覆盖关键项）。
func FromEnv(cfg *Config) *Config {
	if v := os.Getenv("KV_HTTP_ADDR"); v != "" {
		cfg.Server.HTTPAddr = v
	}
	if v := os.Getenv("KV_DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("KV_DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("KV_DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("KV_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("KV_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("KV_USE_SWTPM"); v == "false" || v == "0" {
		cfg.TPM.UseSWTPM = false
	}
	return cfg
}
