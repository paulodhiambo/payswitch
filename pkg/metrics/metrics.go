package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	PaymentsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_total",
			Help: "Total number of payments processed",
		},
		[]string{"status"},
	)
	SagaStepsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "saga_steps_total",
			Help: "Total number of saga steps executed",
		},
		[]string{"step", "result"},
	)
	OutboxEventsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "outbox_events_total",
			Help: "Total number of outbox events published",
		},
	)
	ActivePayments = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_payments",
			Help: "Number of active payments in flight",
		},
	)
)

func Init() {
	prometheus.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		HTTPRequestsTotal,
		HTTPRequestDuration,
		PaymentsTotal,
		SagaStepsTotal,
		OutboxEventsTotal,
		ActivePayments,
	)
}

func Listen(addr string) {
	Init()
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("metrics listening on %s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("metrics server: %v", err)
		}
	}()
}
