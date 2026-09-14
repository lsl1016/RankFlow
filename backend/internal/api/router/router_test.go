package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"rankflow/internal/api/handler"
	"rankflow/internal/api/middleware"
	"rankflow/internal/observability"
)

// TestRouterRegisters ensures all routes register without httprouter conflicts
// and the health endpoint responds.
func TestRouterRegisters(t *testing.T) {
	h := handler.New(nil, observability.NewMetrics())
	log := zap.NewNop()
	r := New(h, log, middleware.AuthConfig{}) // panics here if any route path conflicts

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz want 200, got %d", w.Code)
	}
}

func TestRouterProtectsAdminAndWriterRoutes(t *testing.T) {
	h := handler.New(nil, observability.NewMetrics())
	log := zap.NewNop()
	auth := middleware.AuthConfig{Enabled: true, AdminToken: "admin-secret", WriterToken: "writer-secret"}
	r := New(h, log, auth)

	cases := []struct {
		method string
		path   string
		want   int
	}{
		{method: http.MethodGet, path: "/api/ranks", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/ranks/10001/score/add", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/swagger/index.html", want: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s %s: got=%d want=%d", tc.method, tc.path, w.Code, tc.want)
		}
	}
}
