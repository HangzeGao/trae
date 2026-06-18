package observability

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
)

// Logger 是全局结构化日志器，内置脱敏（HA-03）。
type Logger struct {
	*slog.Logger
}

var defaultLogger atomic.Pointer[Logger]

func init() {
	l := &Logger{slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))}
	defaultLogger.Store(l)
}

// SetLogger 替换全局日志器。
func SetLogger(l *Logger) {
	defaultLogger.Store(l)
}

// Get 返回全局日志器。
func Get() *Logger {
	return defaultLogger.Load()
}

// Init 根据配置初始化日志器。
func Init(level, format string) {
	var sl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		sl = slog.LevelDebug
	case "warn":
		sl = slog.LevelWarn
	case "error":
		sl = slog.LevelError
	default:
		sl = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     sl,
		AddSource: true,
	}
	if strings.ToLower(format) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	l := &Logger{slog.New(redactingHandler{handler})}
	defaultLogger.Store(l)
}

// 敏感数据脱敏正则（HA-03）。
var (
	hexKeyPattern    = regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`)
	base64KeyPattern = regexp.MustCompile(`\b[A-Za-z0-9+/]{32,}={0,2}\b`)
)

// redactingHandler 包装 slog.Handler，对日志值做脱敏。
type redactingHandler struct {
	inner slog.Handler
}

func (h redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	// 对 record 的所有 attr 值做脱敏
	attrs := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, redactAttr(a))
		return true
	})
	r = slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	for _, a := range attrs {
		r.AddAttrs(a)
	}
	return h.inner.Handle(ctx, r)
}

func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = redactAttr(a)
	}
	return redactingHandler{h.inner.WithAttrs(redacted)}
}

func (h redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{h.inner.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() != slog.KindString {
		return a
	}
	s := a.Value.String()
	// 已知敏感字段名直接脱敏
	name := strings.ToLower(a.Key)
	if strings.Contains(name, "key") || strings.Contains(name, "secret") ||
		strings.Contains(name, "token") || strings.Contains(name, "password") ||
		strings.Contains(name, "dek") || strings.Contains(name, "crk") ||
		strings.Contains(name, "wrapped") || strings.Contains(name, "hmac") {
		return slog.String(a.Key, "[REDACTED]")
	}
	// 对长 hex/base64 串脱敏
	if len(s) >= 32 {
		s = hexKeyPattern.ReplaceAllString(s, "[REDACTED]")
		s = base64KeyPattern.ReplaceAllString(s, "[REDACTED]")
	}
	return slog.String(a.Key, s)
}

// RequestIDFromContext 提取请求 ID。
type ctxKey string

const requestIDKey ctxKey = "request_id"

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// HexShort 返回 hex 串的前 8 字符用于日志标识（不泄露完整密钥）。
func HexShort(b []byte) string {
	if len(b) <= 4 {
		return "[SHORT]"
	}
	return hex.EncodeToString(b[:4]) + "..."
}

// FmtHexShort 格式化辅助。
func FmtHexShort(b []byte) string {
	return fmt.Sprintf("%s", HexShort(b))
}
