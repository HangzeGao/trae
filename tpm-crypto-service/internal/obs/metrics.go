package obs

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 聚合所有 Prometheus 指标,集中注册到 default registry。
type Metrics struct {
	OpDuration      *prometheus.HistogramVec
	RateLimited     prometheus.Counter
	CPUFeatures     *prometheus.GaugeVec
	ServiceStartDur prometheus.Histogram
	Health          *prometheus.GaugeVec
	BackendInfo     *prometheus.GaugeVec
}

// NewMetrics 注册并返回指标集合;幂等。
func NewMetrics(reg prometheus.Registerer) *Metrics {
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
