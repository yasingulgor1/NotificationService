package handler

import (
	"context"
	"net/http"
	"time"
)

// HealthChecker defines an interface for health checking
type HealthChecker interface {
	Health(ctx context.Context) error
}

// HealthHandler handles health check requests
type HealthHandler struct {
	checkers map[string]HealthChecker
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		checkers: make(map[string]HealthChecker),
	}
}

// AddChecker adds a health checker
func (h *HealthHandler) AddChecker(name string, checker HealthChecker) {
	h.checkers[name] = checker
}

// HealthStatus represents the health status response
type HealthStatus struct {
	Status     string                     `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Components map[string]ComponentStatus `json:"components,omitempty"`
}

// ComponentStatus represents a component's health status
type ComponentStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Health handles health check requests
// @Summary Health check
// @Description Check the health of the service and its dependencies
// @Tags health
// @Produce json
// @Success 200 {object} HealthStatus
// @Failure 503 {object} HealthStatus
// @Router /health [get]
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := HealthStatus{
		Status:     "healthy",
		Timestamp:  time.Now().UTC(),
		Components: make(map[string]ComponentStatus),
	}

	allHealthy := true

	for name, checker := range h.checkers {
		componentStatus := ComponentStatus{Status: "healthy"}

		if err := checker.Health(ctx); err != nil {
			componentStatus.Status = "unhealthy"
			componentStatus.Message = err.Error()
			allHealthy = false
		}

		status.Components[name] = componentStatus
	}

	if !allHealthy {
		status.Status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	JSON(w, http.StatusOK, status)
}

// Liveness handles liveness probe requests
// @Summary Liveness probe
// @Description Simple liveness check
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health/live [get]
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string{
		"status": "alive",
	})
}

// Readiness handles readiness probe requests
// @Summary Readiness probe
// @Description Check if the service is ready to accept traffic
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /health/ready [get]
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	for name, checker := range h.checkers {
		if err := checker.Health(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":    "not ready",
				"component": name,
				"error":     err.Error(),
			})
			return
		}
	}

	JSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}
