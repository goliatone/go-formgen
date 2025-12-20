package timezones

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type handlerResponse struct {
	Data []Option `json:"data"`
}

func TestNewHandler_EmptyQueryReturnsEmptyDataArray(t *testing.T) {
	h := NewHandler(
		WithZones([]string{"UTC"}),
		WithEmptySearchMode(EmptySearchNone),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/timezones", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if ct := strings.TrimSpace(res.Header.Get("Content-Type")); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %q", ct)
	}

	var payload handlerResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Data == nil || len(payload.Data) != 0 {
		t.Fatalf("expected empty data array, got %#v", payload.Data)
	}
}

func TestNewHandler_SearchAndLimitClamped(t *testing.T) {
	h := NewHandler(
		WithZones([]string{"America/Chicago", "America/New_York", "Europe/Paris", "UTC"}),
		WithMaxLimit(2),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/timezones?q=America&limit=10", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	var payload handlerResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("expected 2 results, got %d: %#v", len(payload.Data), payload.Data)
	}
	if payload.Data[0].Value != "America/Chicago" || payload.Data[0].Label != "America/Chicago" {
		t.Fatalf("unexpected first option: %#v", payload.Data[0])
	}
	if payload.Data[1].Value != "America/New_York" || payload.Data[1].Label != "America/New_York" {
		t.Fatalf("unexpected second option: %#v", payload.Data[1])
	}
}

func TestNewHandler_CustomQueryParams(t *testing.T) {
	h := NewHandler(
		WithZones([]string{"UTC", "Europe/Paris"}),
		WithSearchParam("search"),
		WithLimitParam("l"),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/timezones?search=utc&l=5", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	var payload handlerResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Value != "UTC" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestNewHandler_GuardRejects(t *testing.T) {
	h := NewHandler(
		WithZones([]string{"UTC"}),
		WithGuard(func(r *http.Request) error {
			return StatusError{Code: http.StatusUnauthorized}
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/timezones?q=utc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestNewHandler_MethodNotAllowed(t *testing.T) {
	h := NewHandler(WithZones([]string{"UTC"}))

	req := httptest.NewRequest(http.MethodPost, "/api/timezones?q=utc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
}

func TestNewHandler_NegativeLimitReturnsEmptyDataArray(t *testing.T) {
	h := NewHandler(
		WithZones([]string{"UTC"}),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/timezones?q=utc&limit=-1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload handlerResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Data == nil || len(payload.Data) != 0 {
		t.Fatalf("expected empty data array, got %#v", payload.Data)
	}
}
