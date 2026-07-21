package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	t.Setenv("APP_ENV", "test")

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	newMux().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var payload struct {
		Status  string `json:"status"`
		Service string `json:"service"`
		Env     string `json:"env"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Status != "ok" {
		t.Fatalf("expected status ok, got %q", payload.Status)
	}

	if payload.Service != "devlab-board-backend" {
		t.Fatalf("expected backend service name, got %q", payload.Service)
	}

	if payload.Env != "test" {
		t.Fatalf("expected env test, got %q", payload.Env)
	}
}
