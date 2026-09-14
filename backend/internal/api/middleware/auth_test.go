package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func authTestEngine(cfg AuthConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin", RequireAdmin(cfg), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.POST("/write", RequireWriter(cfg), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return r
}

func authRequest(t *testing.T, r http.Handler, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestAuthDisabledAllowsProtectedRoutes(t *testing.T) {
	r := authTestEngine(AuthConfig{})
	if got := authRequest(t, r, http.MethodGet, "/admin", "").Code; got != http.StatusNoContent {
		t.Fatalf("admin route with disabled auth: got=%d want=%d", got, http.StatusNoContent)
	}
}

func TestAdminAuthorization(t *testing.T) {
	cfg := AuthConfig{Enabled: true, AdminToken: "admin-secret", WriterToken: "writer-secret"}
	r := authTestEngine(cfg)

	if got := authRequest(t, r, http.MethodGet, "/admin", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("missing token: got=%d want=%d", got, http.StatusUnauthorized)
	}
	if got := authRequest(t, r, http.MethodGet, "/admin?token=admin-secret", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("query token must not authenticate: got=%d want=%d", got, http.StatusUnauthorized)
	}
	if got := authRequest(t, r, http.MethodGet, "/admin", "wrong").Code; got != http.StatusUnauthorized {
		t.Fatalf("wrong token: got=%d want=%d", got, http.StatusUnauthorized)
	}
	if got := authRequest(t, r, http.MethodGet, "/admin", "writer-secret").Code; got != http.StatusForbidden {
		t.Fatalf("writer accessing admin: got=%d want=%d", got, http.StatusForbidden)
	}
	if got := authRequest(t, r, http.MethodGet, "/admin", "admin-secret").Code; got != http.StatusNoContent {
		t.Fatalf("admin token: got=%d want=%d", got, http.StatusNoContent)
	}
}

func TestWriterAuthorizationAcceptsWriterAndAdmin(t *testing.T) {
	cfg := AuthConfig{Enabled: true, AdminToken: "admin-secret", WriterToken: "writer-secret"}
	r := authTestEngine(cfg)

	if got := authRequest(t, r, http.MethodPost, "/write", "writer-secret").Code; got != http.StatusNoContent {
		t.Fatalf("writer token: got=%d want=%d", got, http.StatusNoContent)
	}
	if got := authRequest(t, r, http.MethodPost, "/write", "admin-secret").Code; got != http.StatusNoContent {
		t.Fatalf("admin token on writer route: got=%d want=%d", got, http.StatusNoContent)
	}
}
