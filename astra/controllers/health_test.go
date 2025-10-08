package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	hc := NewHealthController()
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	hc.HealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	expectedBody := `{"status": "ok"}`
	if rr.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rr.Body.String())
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %v", rr.Header().Get("Content-Type"))
	}
}
