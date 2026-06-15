package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config 描述服务全部可配置项,优先级:CLI flag > env > remote > file > default。
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	TPM       TPMConfig       `mapstructure:"tpm"`
	Keystore  KeystoreConfig  `mapstructure:"keystore"`
	Deploy    DeployConfig    `mapstructure:"deploy"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Log       LogConfig       `mapstructure:"log"`
	Crypto    CryptoConfig    `mapstructure:"crypto"`
}

type ServerConfig struct {
	GRPCAddr        string        `mapstructure:"grpc_addr"`
	HTTPAddr        string        `mapstructure:"http_addr"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	TLS             TLSConfig     `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

type TPMConfig struct {
	DevicePath  string `mapstructure:"device_path"`
	Simulator   bool   `mapstructure:"simulator"`
	PCRSel      []int  `mapstructure:"pcr_selection"`
	SealedARK   string `mapstructure:"sealed_ark_path"`
	ReadOnly    bool   `mapstructure:"read_only"`
}

type KeystoreConfig struct {
	Backend   string   `mapstructure:"backend"` // bolt | etcd | memory
	Path      string   `mapstructure:"path"`
	Endpoints []string `mapstructure:"endpoints"`
	Prefix    string   `mapstructure:"prefix"`
}

type DeployConfig struct {
	Mode string `mapstructure:"mode"` // isolated | cluster
}

type RateLimitConfig struct {
	QPS   int `mapstructure:"qps"`
	Burst int `mapstructure:"burst"`
}

type LogConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
	Output   string `mapstructure:"output"`
}

type CryptoConfig struct {
	Backend    string                  `mapstructure:"backend"` // auto | std | circl | openssl
	OpenSSL    OpenSSLConfig           `mapstructure:"openssl"`
	Benchmark  BenchmarkConfig         `mapstructure:"benchmark"`
}

type OpenSSLConfig struct {
	LibPath    string `mapstructure:"lib_path"`
	MinVersion string `mapstructure:"min_version"`
}

type BenchmarkConfig struct {
	EnforceBaseline     bool    `mapstructure:"enforce_baseline"`
	RegressionThreshold float64 `mapstructure:"regression_threshold_pct"`
	BaselineFile        string  `mapstructure:"baseline_file"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			GRPCAddr:        ":9090",
			HTTPAddr:        ":8080",
			ShutdownTimeout: 30 * time.Second,
		},
		TPM: TPMConfig{
			DevicePath: "/dev/tpm0",
			Simulator:  false,
			PCRSel:     []int{0, 1, 2, 3, 4, 5, 6, 7},
			SealedARK:  "/var/lib/tpm-crypto/sealed-ark.blob",
		},
		Keystore: KeystoreConfig{
			Backend: "bolt",
			Path:    "/var/lib/tpm-crypto/data.bolt",
			Prefix:  "/tpm-crypto/dek/",
		},
		Deploy: DeployConfig{Mode: "isolated"},
		RateLimit: RateLimitConfig{
			QPS:   1000,
			Burst: 2000,
		},
		Log: LogConfig{
			Level:    "info",
			Encoding: "json",
			Output:   "stdout",
		},
		Crypto: CryptoConfig{
			Backend: "auto",
			OpenSSL: OpenSSLConfig{
				LibPath:    "/usr/lib/x86_64-linux-gnu/libcrypto.so",
				MinVersion: "3.0.0",
			},
			Benchmark: BenchmarkConfig{
				EnforceBaseline:     true,
				RegressionThreshold: 5.0,
				BaselineFile:        "bench/baseline.txt",
			},
		},
	}
}

// BindFlags 把配置项注册到 pflag;支持 flag > env > file 优先级。
func BindFlags(fs *pflag.FlagSet) {
	fs.String("config", "", "配置文件路径(yaml)")
	fs.String("server.grpc_addr", ":9090", "gRPC 监听地址")
	fs.String("server.http_addr", ":8080", "HTTP 监听地址")
	fs.Duration("server.shutdown_timeout", 30*time.Second, "优雅关停超时")
	fs.String("tpm.device_path", "/dev/tpm0", "TPM 设备路径")
	fs.Bool("tpm.simulator", false, "使用软件模拟器")
	fs.String("tpm.sealed_ark_path", "", "ARK SealedBlob 持久化路径")
	fs.String("keystore.backend", "bolt", "密钥库后端:bolt | etcd | memory")
	fs.String("keystore.path", "", "bolt 路径")
	fs.StringSlice("keystore.endpoints", nil, "etcd endpoints")
	fs.String("deploy.mode", "isolated", "isolated | cluster")
	fs.String("crypto.backend", "auto", "auto | std | circl | openssl")
	fs.String("log.level", "info", "日志级别")
	fs.String("log.encoding", "json", "日志编码:json | console")
	fs.String("log.output", "stdout", "日志输出:stdout | stderr | 文件路径")
}

// Load 合并配置:flag > env > file > default。
func Load(fs *pflag.FlagSet) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("TCS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfgFile, _ := fs.GetString("config")
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := Default()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// flag > viper: 用 pflag 显式设置的值覆盖 viper 解出的值
	fs.VisitAll(func(f *pflag.Flag) {
		if !f.Changed {
			return
		}
		v.Set(f.Name, f.Value.String())
	})
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("re-unmarshal after flags: %w", err)
	}
	return cfg, nil
}
