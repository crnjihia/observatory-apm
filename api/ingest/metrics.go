// path: api/ingest/metrics.go
package ingest

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// IngestReceivedTotal counts total incoming requests by telemetry type and status.
	IngestReceivedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observatory",
			Subsystem: "ingest",
			Name:      "received_total",
			Help:      "Total number of OTLP requests received by telemetry type and acceptance status.",
		},
		[]string{"type", "status"},
	)

	// IngestPointsTotal counts total ingested telemetry points (spans, metrics, logs).
	IngestPointsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "observatory",
			Subsystem: "ingest",
			Name:      "points_total",
			Help:      "Total number of individual data points processed by telemetry type.",
		},
		[]string{"type"},
	)

	// IngestQueueDepth tracks current queue fill level.
	IngestQueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "observatory",
			Subsystem: "ingest",
			Name:      "queue_depth",
			Help:      "Current number of pending telemetry items in the bounded in-memory buffer.",
		},
		[]string{"type"},
	)

	// IngestBatchFlushDuration measures time taken to flush a batch to storage.
	IngestBatchFlushDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "observatory",
			Subsystem: "ingest",
			Name:      "batch_flush_duration_seconds",
			Help:      "Time taken to commit a batch to storage via pgx.CopyFrom.",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
		},
		[]string{"type"},
	)

	// IngestBatchSize observes the distribution of batch sizes flushed.
	IngestBatchSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "observatory",
			Subsystem: "ingest",
			Name:      "batch_size",
			Help:      "Number of items included in each committed batch.",
			Buckets:   []float64{1, 10, 50, 100, 250, 500, 1000, 2000, 5000},
		},
		[]string{"type"},
	)
)

