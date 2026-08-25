package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/noggrj/hacktown-fase-5-processing-worker/internal/platform/metrics"
)

func TestMiddleware_PassesThroughStatusAndBody(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("ok"))
	})
	h := metrics.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected status to pass through, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body to pass through, got %q", rec.Body.String())
	}
}

func TestMiddleware_DefaultsStatusTo200WhenHandlerNeverWritesHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("implicit-200"))
	})
	h := metrics.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/implicit", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected implicit 200, got %d", rec.Code)
	}
}

func TestHandler_ServesPrometheusExposition(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("expected non-empty metrics exposition body")
	}
}
