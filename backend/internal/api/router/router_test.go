package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"rankflow/internal/api/handler"
	"rankflow/internal/observability"
)

// TestRouterRegisters ensures all routes register without httprouter conflicts
// and the health endpoint responds.
func TestRouterRegisters(t *testing.T) {
	h := handler.New(nil, observability.NewMetrics())
	log := zap.NewNop()
	r := New(h, log) // panics here if any route path conflicts

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz want 200, got %d", w.Code)
	}
}
