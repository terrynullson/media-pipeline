package observability

import "github.com/prometheus/client_golang/prometheus"

var (
	// JobsProcessedTotal counts completed pipeline jobs by type and final status.
	JobsProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipeline_jobs_processed_total",
			Help: "Total number of pipeline jobs completed, labelled by type and status (done|failed).",
		},
		[]string{"type", "status"},
	)

	// JobDurationSeconds records the wall-clock duration of each pipeline job.
	JobDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pipeline_job_duration_seconds",
			Help:    "Wall-clock duration of pipeline jobs in seconds.",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1800},
		},
		[]string{"type"},
	)

	// ActiveJobs tracks the number of jobs currently running per type.
	ActiveJobs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pipeline_active_jobs",
			Help: "Number of pipeline jobs currently being processed, labelled by type.",
		},
		[]string{"type"},
	)
)

// RegisterMetrics registers all pipeline metrics with the default Prometheus
// registry. Call once at application startup.
func RegisterMetrics() {
	prometheus.MustRegister(JobsProcessedTotal)
	prometheus.MustRegister(JobDurationSeconds)
	prometheus.MustRegister(ActiveJobs)
}
