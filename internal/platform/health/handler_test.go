package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/noggrj/hacktown-fase-5-processing-worker/internal/platform/health"
)

func TestLive_AlwaysOK(t *testing.T) {
	h := health.New("v1", nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.Live(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestReady_AllHealthy_Returns200(t *testing.T) {
	h := health.New("v1", map[string]health.Probe{
		"postgres": func() health.Check { return health.Check{Status: "healthy"} },
	})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	h.Ready(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp health.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
}

func TestReady_OneUnhealthy_Returns503(t *testing.T) {
	h := health.New("v1", map[string]health.Probe{
		"postgres": func() health.Check { return health.Check{Status: "unhealthy", Detail: "connection refused"} },
	})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	h.Ready(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
