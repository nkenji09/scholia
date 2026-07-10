package viewer

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nkenji09/product-memory/internal/model"
)

func TestGetConfig(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	cfg := decodeJSON[model.Config](t, rec)
	if cfg.Viewer.Port != 4577 {
		t.Fatalf("Viewer.Port = %d, want 4577 (default)", cfg.Viewer.Port)
	}
}

func TestPutConfig_Valid(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"tagKinds":["subject","requirement"],"facetKinds":["subject","requirement"],"traceabilityKinds":["requirement"],"roots":[],"viewer":{"port":4580}}`)
	rec := doRequest(t, h, http.MethodPut, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	cfg := decodeJSON[model.Config](t, rec)
	if cfg.Viewer.Port != 4580 {
		t.Fatalf("Viewer.Port = %d, want 4580", cfg.Viewer.Port)
	}

	persisted, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if persisted.Viewer.Port != 4580 {
		t.Fatalf("persisted Viewer.Port = %d, want 4580 (PUT should persist)", persisted.Viewer.Port)
	}
}

func TestPutConfig_RejectsUnknownKey(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"tagKinds":["subject"],"facetKinds":[],"traceabilityKinds":[],"roots":[],"viewer":{"port":4577},"bogus":1}`)
	rec := doRequest(t, h, http.MethodPut, "/api/config", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPutConfig_RejectsTagKindInUse(t *testing.T) {
	h, _ := newTestHandler(t)
	// req.auth-happy が kind=requirement を使用中のため、tagKinds から requirement を外すのは拒否される。
	body := []byte(`{"tagKinds":["subject"],"facetKinds":["subject"],"traceabilityKinds":[],"roots":[],"viewer":{"port":4577}}`)
	rec := doRequest(t, h, http.MethodPut, "/api/config", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "req.auth-happy") {
		t.Fatalf("error body should name the blocking tag: %s", rec.Body.String())
	}
}

func TestPutConfig_RejectsNonNumericPort(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"tagKinds":["subject","requirement"],"facetKinds":["subject","requirement"],"traceabilityKinds":["requirement"],"roots":[],"viewer":{"port":"abcd"}}`)
	rec := doRequest(t, h, http.MethodPut, "/api/config", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
