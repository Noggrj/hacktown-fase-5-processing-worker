// Package metrics exposes a Prometheus /metrics endpoint plus a thin HTTP
// middleware recording request count and latency. Scraped by the
// Prometheus Agent deployed alongside the service in EKS.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests processed, labeled by path/method/status.",
	}, []string{"path", "method", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"path", "method"})

	// Business metrics — exported so internal/processing/usecase can
	// record them directly. This is the service that actually knows
	// whether ffmpeg worked, how many frames came out, and how long it
	// took — the canonical source for these, not video-service (which
	// only knows "I persisted a status update").
	VideosProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fiapx_videos_processed_total",
		Help: "Total videos successfully processed (frames extracted, zip uploaded).",
	})

	VideosFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fiapx_videos_failed_total",
		Help: "Total videos that failed processing (ffmpeg error, corrupt input, etc).",
	})

	FramesExtracted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fiapx_frames_extracted_total",
		Help: "Total frames extracted across all successfully processed videos.",
	})

	ProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "fiapx_video_processing_duration_seconds",
		Help: "Time to extract frames from a video (ffmpeg run), regardless of outcome.",
		// A 1-frame-per-second extraction on a short demo clip finishes
		// in well under a second; a real multi-minute video takes much
		// longer — buckets span both without the default's coarse tail.
		Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120, 300},
	})
)

// Handler serves the /metrics scrape endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// Middleware records one observation per request. Uses r.URL.Path
// directly (not a route-pattern) — acceptable cardinality since this
// service only exposes a handful of fixed, non-parameterized routes.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		httpRequests.WithLabelValues(r.URL.Path, r.Method, strconv.Itoa(sw.status)).Inc()
		httpDuration.WithLabelValues(r.URL.Path, r.Method).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
