package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"livepoll/backend/internal/config"
)

func TestOriginAndPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	api := New(nil, nil, config.Config{PublicOrigin: "http://localhost:5173", AllowedOrigins: []string{"https://preview.example.test"}})
	router := api.Router()

	for _, test := range []struct {
		name, origin string
		status       int
	}{
		{"no-origin health", "", http.StatusOK},
		{"primary origin", "http://localhost:5173", http.StatusOK},
		{"explicit preview origin", "https://preview.example.test", http.StatusOK},
		{"attacker origin", "https://evil.example", http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("health returned %d; want %d", recorder.Code, test.status)
			}
			if test.status == http.StatusOK && test.origin != "" && recorder.Header().Get("Access-Control-Allow-Origin") != test.origin {
				t.Fatalf("CORS origin %q; want %q", recorder.Header().Get("Access-Control-Allow-Origin"), test.origin)
			}
		})
	}

	missingCSRF := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/polls", nil)
	request.Header.Set("Origin", "https://preview.example.test")
	router.ServeHTTP(missingCSRF, request)
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF returned %d", missingCSRF.Code)
	}

	preflight := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodOptions, "/api/polls", nil)
	request.Header.Set("Origin", "https://preview.example.test")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	router.ServeHTTP(preflight, request)
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("allowed preflight returned %d", preflight.Code)
	}

	blockedPreflight := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodOptions, "/api/polls", nil)
	request.Header.Set("Origin", "https://evil.example")
	router.ServeHTTP(blockedPreflight, request)
	if blockedPreflight.Code != http.StatusForbidden {
		t.Fatalf("attacker preflight returned %d", blockedPreflight.Code)
	}
}
