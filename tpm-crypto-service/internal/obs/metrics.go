package obs

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusRegistry 集中保存 default Prometheus registry,
// 供 NewMetrics 注册和 MetricsHandler 暴露使用。
var PrometheusRegistry = prometheus.NewRegistry()

// Metrics 聚合所有 Prometheus 指标,集中注册到 PrometheusRegistry。
type Metrics struct {
	OpDuration      *prometheus.HistogramVec
	RateLimited     prometheus.Counter
	CPUFeatures     *prometheus.GaugeVec
	ServiceStartDur prometheus.Histogram
	Health          *prometheus.GaugeVec
	BackendInfo     *prometheus.GaugeVec
}

// NewMetrics 注册并返回指标集合;幂等。
func NewMetrics() *Metrics {
	return NewMetricsWith(PrometheusRegistry)
}

// NewMetricsWith 允许调用方传入自定义 registry(测试隔离用)。
func NewMetricsWith(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		OpDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "crypto_op_duration_seconds",
			Help:    "加解密 / 密钥管理操作耗时(秒)。",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 14), // 100us .. ~1.6s
		}, []string{"op", "algorithm", "mode", "result"}),
		RateLimited: factory.NewCounter(prometheus.CounterOpts{
			Name: "crypto_rate_limited_total",
			Help: "被限流的请求总数。",
		}),
		CPUFeatures: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "crypto_cpu_features_info",
			Help: "当前进程可用的 CPU 特性(1=支持)。",
		}, []string{"feature"}),
		ServiceStartDur: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "service_start_duration_seconds",
			Help:    "服务启动耗时。",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		}),
		Health: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "service_health",
			Help: "健康检查分项状态(1=ok,0=fail)。",
		}, []string{"component"}),
		BackendInfo: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "crypto_backend_info",
			Help: "当前算法后端(恒为 1,label 标识)。",
		}, []string{"backend"}),
	}
}

// SetBackend 把当前选定的算法后端写入 BackendInfo 指标(label=1)。
func (m *Metrics) SetBackend(backend string) {
	if m == nil {
		return
	}
	m.BackendInfo.WithLabelValues(backend).Set(1)
}

// SetCPUFeature 把 CPU 特性信息写入 CPUFeatures 指标(1=支持)。
func (m *Metrics) SetCPUFeature(name string, ok bool) {
	if m == nil {
		return
	}
	v := 0.0
	if ok {
		v = 1.0
	}
	m.CPUFeatures.WithLabelValues(name).Set(v)
}
