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
	if cfg.TagKindLabels["requirement"] != "要件" {
		t.Fatalf("TagKindLabels[requirement] = %q, want 要件 (default)", cfg.TagKindLabels["requirement"])
	}
	if cfg.Display.ProductName != "pmem" {
		t.Fatalf("Display.ProductName = %q, want pmem (default)", cfg.Display.ProductName)
	}
	if cfg.Display.Tagline == "" || cfg.Display.Intro == "" {
		t.Fatalf("Display.Tagline/Intro should be seeded by default, got tagline=%q intro=%q", cfg.Display.Tagline, cfg.Display.Intro)
	}
}

// TestGetConfig_BranchEmptyOutsideGitRepo covers 2026-07-11 tweaks5 §2's
// "取得失敗は握って既定にフォールバック": newTestHandler seeds into a plain
// t.TempDir(), not a git repo, so diff.CurrentBranch must fail silently
// (empty string) rather than surfacing an error or 500ing the request.
func TestGetConfig_BranchEmptyOutsideGitRepo(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	cfg := decodeJSON[model.Config](t, rec)
	if cfg.Branch != "" {
		t.Fatalf("Branch = %q, want empty (temp dir isn't a git repo)", cfg.Branch)
	}
}

// TestPutConfig_Display covers the additive display field (2026-07-11
// tweaks5 §1/§2): round-trips through PUT/persist/GET, and Branch — even
// though it rides along in the same response — must never be written to
// config.json since it's a derived value, not a stored preference.
func TestPutConfig_Display(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"tagKinds":["subject","requirement"],"facetKinds":["subject","requirement"],"traceabilityKinds":["requirement"],"roots":[],"viewer":{"port":4577},"display":{"productName":"myproj","tagline":"カスタムタグライン","intro":"独自のイントロ文"}}`)
	rec := doRequest(t, h, http.MethodPut, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	cfg := decodeJSON[model.Config](t, rec)
	if cfg.Display.ProductName != "myproj" || cfg.Display.Tagline != "カスタムタグライン" || cfg.Display.Intro != "独自のイントロ文" {
		t.Fatalf("Display = %+v, want productName=myproj tagline=カスタムタグライン intro=独自のイントロ文", cfg.Display)
	}

	persisted, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if persisted.Display.ProductName != "myproj" {
		t.Fatalf("persisted Display.ProductName = %q, want myproj (PUT should persist)", persisted.Display.ProductName)
	}
	if persisted.Branch != "" {
		t.Fatalf("persisted Branch = %q, want empty (must never be written to config.json)", persisted.Branch)
	}
}

// TestPutConfig_TagKindLabels covers the additive tagKindLabels field
// (2026-07-11 tweaks3 §2): it must round-trip through PUT/persist/GET like
// every other configPatch field, unknown tagKind keys in the map aren't
// rejected (a stale label for a removed kind is just orphaned, not an
// error — same "no extra validation" posture store.go already has for the
// rest of Config), and — since PUT replaces the whole editable object —
// omitting the key entirely clears it, exactly like omitting roots/
// facetKinds would.
func TestPutConfig_TagKindLabels(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"tagKinds":["subject","requirement"],"facetKinds":["subject","requirement"],"traceabilityKinds":["requirement"],"roots":[],"viewer":{"port":4577},"tagKindLabels":{"requirement":"ようけん","subject":"しゅだい"}}`)
	rec := doRequest(t, h, http.MethodPut, "/api/config", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	cfg := decodeJSON[model.Config](t, rec)
	if cfg.TagKindLabels["requirement"] != "ようけん" || cfg.TagKindLabels["subject"] != "しゅだい" {
		t.Fatalf("TagKindLabels = %+v, want requirement=ようけん subject=しゅだい", cfg.TagKindLabels)
	}

	persisted, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if persisted.TagKindLabels["requirement"] != "ようけん" {
		t.Fatalf("persisted TagKindLabels[requirement] = %q, want ようけん (PUT should persist)", persisted.TagKindLabels["requirement"])
	}

	// A PUT body omitting tagKindLabels clears it (full-replace semantics,
	// same as every other configPatch field).
	body2 := []byte(`{"tagKinds":["subject","requirement"],"facetKinds":["subject","requirement"],"traceabilityKinds":["requirement"],"roots":[],"viewer":{"port":4577}}`)
	rec2 := doRequest(t, h, http.MethodPut, "/api/config", body2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}
	cfg2 := decodeJSON[model.Config](t, rec2)
	if len(cfg2.TagKindLabels) != 0 {
		t.Fatalf("TagKindLabels after omitting the key = %+v, want empty", cfg2.TagKindLabels)
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
