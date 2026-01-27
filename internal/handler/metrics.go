package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/insider-one/notification-service/internal/domain"
)

// Metrics holds Prometheus metrics
type Metrics struct {
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	notificationsSent   *prometheus.CounterVec
	notificationsFailed *prometheus.CounterVec
	queueDepth          *prometheus.GaugeVec
	processingLatency   *prometheus.HistogramVec
}

// NewMetrics creates new Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		notificationsSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notifications_sent_total",
				Help: "Total number of notifications sent successfully",
			},
			[]string{"channel"},
		),
		notificationsFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notifications_failed_total",
				Help: "Total number of failed notifications",
			},
			[]string{"channel", "reason"},
		),
		queueDepth: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "notification_queue_depth",
				Help: "Current depth of the notification queue",
			},
			[]string{"channel"},
		),
		processingLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_processing_latency_seconds",
				Help:    "Time from creation to successful send",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"channel"},
		),
	}
}

// RecordRequest records HTTP request metrics
func (m *Metrics) RecordRequest(method, path, status string, duration time.Duration) {
	m.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
	m.httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordNotificationSent records a successful notification send
func (m *Metrics) RecordNotificationSent(channel string) {
	m.notificationsSent.WithLabelValues(channel).Inc()
}

// RecordNotificationFailed records a failed notification
func (m *Metrics) RecordNotificationFailed(channel, reason string) {
	m.notificationsFailed.WithLabelValues(channel, reason).Inc()
}

// SetQueueDepth sets the current queue depth
func (m *Metrics) SetQueueDepth(channel string, depth float64) {
	m.queueDepth.WithLabelValues(channel).Set(depth)
}

// RecordProcessingLatency records the time from creation to send
func (m *Metrics) RecordProcessingLatency(channel string, latency time.Duration) {
	m.processingLatency.WithLabelValues(channel).Observe(latency.Seconds())
}

// MetricsHandler handles metrics endpoints
type MetricsHandler struct {
	metrics *Metrics
	queue   domain.Queue
}

// NewMetricsHandler creates a new MetricsHandler
func NewMetricsHandler(metrics *Metrics, queue domain.Queue) *MetricsHandler {
	return &MetricsHandler{
		metrics: metrics,
		queue:   queue,
	}
}

// Handler returns the Prometheus HTTP handler
func (h *MetricsHandler) Handler() http.Handler {
	return promhttp.Handler()
}

// QueueMetrics represents real-time queue metrics
type QueueMetrics struct {
	SMS   QueueChannelMetrics `json:"sms"`
	Email QueueChannelMetrics `json:"email"`
	Push  QueueChannelMetrics `json:"push"`
}

// QueueChannelMetrics represents metrics for a single channel
type QueueChannelMetrics struct {
	Depth       int64 `json:"depth"`
	CurrentRate int64 `json:"current_rate_per_sec"`
}

// RealtimeMetrics handles real-time metrics requests
// @Summary Real-time metrics
// @Description Get real-time metrics including queue depth and rates
// @Tags metrics
// @Produce json
// @Success 200 {object} QueueMetrics
// @Failure 500 {object} Response
// @Router /metrics/realtime [get]
func (h *MetricsHandler) RealtimeMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	depths, err := h.queue.GetAllQueueDepths(ctx)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "METRICS_ERROR", "Failed to get queue depths", nil)
		return
	}

	// Update Prometheus gauges
	for channel, depth := range depths {
		h.metrics.SetQueueDepth(string(channel), float64(depth))
	}

	metrics := QueueMetrics{
		SMS: QueueChannelMetrics{
			Depth: depths[domain.ChannelSMS],
		},
		Email: QueueChannelMetrics{
			Depth: depths[domain.ChannelEmail],
		},
		Push: QueueChannelMetrics{
			Depth: depths[domain.ChannelPush],
		},
	}

	JSON(w, http.StatusOK, metrics)
}
