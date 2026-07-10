package viewer

import (
	"net/http"
	"strings"
	"testing"
)

func TestSPA_ServesIndexAtRoot(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("body does not look like index.html: %s", rec.Body.String())
	}
}

func TestSPA_FallsBackToIndexForUnknownPath(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/browse/some/deep/route", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (client-side-routing fallback): %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatalf("fallback body does not look like index.html: %s", rec.Body.String())
	}
}

func TestSPA_ServesStaticAsset(t *testing.T) {
	h, _ := newTestHandler(t)
	// Discover an actual asset path from the rendered index.html rather than
	// hardcoding the content-hashed filename Vite generates.
	indexRec := doRequest(t, h, http.MethodGet, "/", nil)
	body := indexRec.Body.String()
	start := strings.Index(body, `src="/assets/`)
	if start == -1 {
		t.Fatalf("index.html has no /assets/ script reference: %s", body)
	}
	start += len(`src="`)
	end := strings.Index(body[start:], `"`)
	if end == -1 {
		t.Fatalf("could not parse asset path from index.html: %s", body)
	}
	assetPath := body[start : start+end]

	rec := doRequest(t, h, http.MethodGet, assetPath, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for %s", rec.Code, assetPath)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Fatalf("Content-Type = %q, want javascript for %s", ct, assetPath)
	}
}
