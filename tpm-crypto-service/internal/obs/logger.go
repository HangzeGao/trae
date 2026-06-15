package obs

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger 构造 zap 结构化日志器。
// level: debug|info|warn|error;encoding: json|console;output: stdout|stderr|file path。
func NewLogger(level, encoding, output, service, instanceID, tpmMode string, cpuFeats map[string]bool) (*zap.Logger, error) {
	lvl := zap.NewAtomicLevel()
	if err := lvl.UnmarshalText([]byte(strings.ToLower(level))); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}

	var enc zapcore.Encoder
	if strings.EqualFold(encoding, "console") {
		enc = zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig())
	} else {
		enc = zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	}

	var ws zapcore.WriteSyncer
	switch strings.ToLower(output) {
	case "stdout", "":
		ws = zapcore.AddSync(os.Stdout)
	case "stderr":
		ws = zapcore.AddSync(os.Stderr)
	default:
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		ws = zapcore.AddSync(f)
	}

	core := zapcore.NewCore(enc, ws, lvl)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))

	// 启动字段
	fields := []zap.Field{
		zap.String("service", service),
		zap.String("instance_id", instanceID),
	}
	if tpmMode != "" {
		fields = append(fields, zap.String("tpm_mode", tpmMode))
	}
	if len(cpuFeats) > 0 {
		fields = append(fields, zap.Bool("cpu_features", true))
		// 把每个特性也单独打,便于日志检索
		for k, v := range cpuFeats {
			fields = append(fields, zap.Bool("cpu_"+k, v))
		}
	}
	return logger.With(fields...), nil
}
