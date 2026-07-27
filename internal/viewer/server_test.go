package viewer

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/review"
	"github.com/nkenji09/scholia/internal/store"
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

func TestAPI_UnknownPathIsJSON404(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/unknown", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("body = %s, want a JSON error object", rec.Body.String())
	}
}

func TestAPI_UnregisteredMethodIsJSON405WithAllowHeader(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodPost, "/api/config", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405: %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if allow := rec.Header().Get("Allow"); allow == "" {
		t.Fatalf("Allow header missing")
	}
}

func TestAPI_MatchedWildcardRouteStillWorks(t *testing.T) {
	// Regression guard: ServeMux.Handler() alone does not populate
	// {wildcard} path values, so jsonAPIHandler must dispatch matched
	// requests back through the mux itself, not the handler Handler()
	// returns directly.
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/transitions/T-login", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"T-login"`) {
		t.Fatalf("body missing T-login: %s", rec.Body.String())
	}
}

// --- 壊れたレコードファイルの失敗文言（01KYCC2TF3NW3JRSSRK9ZHN078） ---

// decision / review のファイル名は `<ULID>.json` なので、パース失敗の文言を
// そのまま返すと viewer に生 ULID が出る。かといってファイル名を落とすだけでは
// 「どのファイルを直すか」が分からず修復できない——両方を立てるため、viewer は
// ディレクトリと壊れ方だけを読ませ、ファイル名は端末のコマンドに委ねる。
// この2つ（ULID を出さない／到達手段を必ず添える）を同時に固定する。
func TestUnreadableRecordFile_NoULIDButReachable(t *testing.T) {
	cases := []struct {
		name    string
		corrupt func(t *testing.T, scholiaDir string)
		path    string
		wantDir string
		wantCmd string
	}{
		{
			name: "decision が壊れている",
			corrupt: func(t *testing.T, dir string) {
				writeCorrupt(t, filepath.Join(dir, "decisions", "01KYHZZZZZZZZZZZZZZZZZZZZZ.json"))
			},
			path: "/api/facets", wantDir: ".scholia/decisions/", wantCmd: "scholia lint",
		},
		{
			name: "tag が壊れている",
			corrupt: func(t *testing.T, dir string) {
				writeCorrupt(t, filepath.Join(dir, "tags", "req.broken.json"))
			},
			path: "/api/facets", wantDir: ".scholia/tags/", wantCmd: "scholia lint",
		},
		{
			name: "review が壊れている",
			corrupt: func(t *testing.T, dir string) {
				writeCorrupt(t, filepath.Join(dir, "reviews", "01KYHZZZZZZZZZZZZZZZZZZZZZ.json"))
			},
			// reviews/ は store.LoadAll の対象外（§8.4）なので lint では出ない。
			path: "/api/reviews", wantDir: ".scholia/reviews/", wantCmd: "scholia review list",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, s := newTestHandler(t)
			tc.corrupt(t, s.Dir)

			rec := doRequest(t, h, http.MethodGet, tc.path, nil)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
			}
			got := decodeJSON[struct {
				Error string `json:"error"`
				Code  string `json:"code"`
			}](t, rec)
			if got.Code != "record-file-unreadable" {
				t.Fatalf("code = %q, want record-file-unreadable", got.Code)
			}
			if leaks := ulidPattern.FindAllString(got.Error, -1); len(leaks) > 0 {
				t.Fatalf("失敗メッセージに生 ULID が漏れている: %v\nmessage: %s", leaks, got.Error)
			}
			// 到達手段: どのディレクトリか＋ファイル名を出すコマンドが要る。
			// ULID を消しただけで修復不能にしていないことを、ここで担保する。
			if !strings.Contains(got.Error, tc.wantDir) {
				t.Fatalf("対象ディレクトリを示すべき（%s）: %s", tc.wantDir, got.Error)
			}
			if !strings.Contains(got.Error, tc.wantCmd) {
				t.Fatalf("ファイル名に到達するコマンドを示すべき（%s）: %s", tc.wantCmd, got.Error)
			}
		})
	}
}

func writeCorrupt(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{ broken"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// 逆側の固定: CLI 向けの文言はファイル名を含み続ける（端末が到達手段の本体）。
func TestRecordFileError_CLIMessageKeepsFileName(t *testing.T) {
	err := &store.RecordFileError{Category: "decision", Name: "01KYHZZZZZZZZZZZZZZZZZZZZZ.json", Parse: true, Err: errors.New("boom")}
	if !strings.Contains(err.Error(), "01KYHZZZZZZZZZZZZZZZZZZZZZ.json") {
		t.Fatalf("CLI 向け文言はファイル名を含むべき: %q", err.Error())
	}
	revErr := &review.FileError{Name: "01KYHZZZZZZZZZZZZZZZZZZZZZ.json", Parse: true, Err: errors.New("boom")}
	if !strings.Contains(revErr.Error(), "01KYHZZZZZZZZZZZZZZZZZZZZZ.json") {
		t.Fatalf("CLI 向け文言はファイル名を含むべき: %q", revErr.Error())
	}
}
