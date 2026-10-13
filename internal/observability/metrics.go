package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "requests_total", Help: "Total requests",
	}, []string{"method", "path", "status"})
	RequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "requests_in_flight", Help: "In flight",
	})
	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_duration_seconds", Help: "Duration",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	}, []string{"path"})
	DownloadDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "download_duration_seconds", Help: "Download",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	})
	DecodeDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "decode_duration_seconds", Help: "Decode",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1},
	})
	PreprocessDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "preprocess_duration_seconds", Help: "Preprocess",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1},
	})
	InferenceDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "classifier_inference_duration_seconds", Help: "Inference",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2},
	})
	InferenceInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "classifier_inference_in_flight", Help: "Inference in flight",
	})
	InferenceQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "inference_queue_depth", Help: "Queue depth",
	})
	ClassificationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "classification_total", Help: "Classification",
	}, []string{"classification"})
	PredictionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "predictions_total", Help: "Top class",
	}, []string{"top_class"})
	CacheHits           = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_hits_total"})
	CacheMisses         = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_misses_total"})
	ErrorsTotal         = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "errors_total"}, []string{"type"})
	ModelInfo           = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "model_info"}, []string{"name", "version"})
	AIRequestsTotal     = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ai_requests_total", Help: "AI requests by capability and model"}, []string{"capability", "model"})
	AIInferenceDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ai_inference_duration_seconds", Help: "AI inference duration by capability and model", Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 15, 60}}, []string{"capability", "model"})
	AIErrorsTotal       = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ai_errors_total", Help: "AI errors by capability and category"}, []string{"capability", "category"})
	AIInFlight          = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "ai_inference_in_flight", Help: "AI inferences executing by capability and model"}, []string{"capability", "model"})
)

func Register(reg prometheus.Registerer) {
	reg.MustRegister(
		RequestsTotal,
		RequestsInFlight,
		RequestDuration,
		DownloadDuration,
		DecodeDuration,
		PreprocessDuration,
		InferenceDuration,
		InferenceInFlight,
		InferenceQueueDepth,
		ClassificationTotal,
		PredictionsTotal,
		CacheHits,
		CacheMisses,
		ErrorsTotal,
		ModelInfo,
		AIRequestsTotal,
		AIInferenceDuration,
		AIErrorsTotal,
		AIInFlight,
	)
}
