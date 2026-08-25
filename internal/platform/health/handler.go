// Package health exposes liveness (/health) and readiness (/ready)
// endpoints. /health always returns 200 once the process is up; /ready
// runs every registered probe and returns 503 if any dependency is
// unhealthy — this is what a load balancer / Kubernetes readinessProbe
// should point at, so a pod that lost its DB connection stops receiving
// traffic instead of failing every request silently.
package health

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

type Response struct {
	Status    string           `json:"status"`
	Version   string           `json:"version"`
	GoVersion string           `json:"go_version"`
	Uptime    string           `json:"uptime"`
	Timestamp string           `json:"timestamp"`
	Checks    map[string]Check `json:"checks,omitempty"`
}

type Check struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type Probe func() Check

type Handler struct {
	version string
	startAt time.Time
	probes  map[string]Probe
}

func New(version string, probes map[string]Probe) *Handler {
	return &Handler{version: version, startAt: time.Now(), probes: probes}
}

func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) Ready(w http.ResponseWriter, _ *http.Request) {
	checks := make(map[string]Check, len(h.probes))
	status := "ok"
	for name, probe := range h.probes {
		c := probe()
		checks[name] = c
		if c.Status != "healthy" {
			status = "degraded"
		}
	}
	resp := Response{
		Status:    status,
		Version:   h.version,
		GoVersion: runtime.Version(),
		Uptime:    time.Since(h.startAt).Truncate(time.Second).String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Checks:    checks,
	}
	w.Header().Set("Content-Type", "application/json")
	if status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(resp)
}
