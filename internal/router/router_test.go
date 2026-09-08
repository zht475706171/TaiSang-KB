package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	// db == nil: router fast-path returns engine with only /api/health registered,
	// so NewServices is never called and no encryption key / DB plumbing is needed.
	r := New(nil, Config{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("body = %s, want contains 'ok'", w.Body.String())
	}
}