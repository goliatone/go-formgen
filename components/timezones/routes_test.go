package timezones

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMountPath_JoinsBasePath(t *testing.T) {
	if got := MountPath("/admin"); got != "/admin/api/timezones" {
		t.Fatalf("unexpected mount path: %q", got)
	}
	if got := MountPath("admin"); got != "/admin/api/timezones" {
		t.Fatalf("unexpected mount path: %q", got)
	}
	if got := MountPath("/admin/", WithRoutePath("api/tz")); got != "/admin/api/tz" {
		t.Fatalf("unexpected mount path: %q", got)
	}
}

func TestRegisterRoutes_RegistersHandler(t *testing.T) {
	mux := http.NewServeMux()
	pattern, err := RegisterRoutes(mux, "/admin", WithZones([]string{"UTC"}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pattern != "/admin/api/timezones" {
		t.Fatalf("unexpected registered pattern: %q", pattern)
	}

	req := httptest.NewRequest(http.MethodGet, pattern+"?q=utc&limit=1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}
