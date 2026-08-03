package viewer

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

// --- 保存失敗の失敗文言（01KYCC2TF3NW3JRSSRK9ZHN078） ---

// decision の保存は tmp を作って rename する2段構えで、rename 失敗が返す
// os.LinkError は**宛先パス** `.scholia/decisions/<新 ULID>.json` を含む。
// この文言を素通しすると、FS 障害のときだけ生 ULID が漏れる。
//
// rename 失敗はハンドラが採番する ULID に依存して再現しにくいので、報告された
// 形のエラーを直接作って描画だけを固定する（実機での再現は result に記録）。
func TestWriteFailedMessage_NoULIDButCauseKept(t *testing.T) {
	newID := "01KYHNF0EMFHA9ZC4ZTSJ0QE8T"
	cases := []struct {
		name      string
		err       error
		wantCause string
	}{
		{
			// 報告された経路そのもの: rename の宛先に新 ULID が入る。
			name: "rename 失敗（LinkError・宛先に新 ULID）",
			err: &store.RecordWriteError{Category: "decision", Err: &os.LinkError{
				Op:  "rename",
				Old: "/repo/.scholia/decisions/.tmp-260738198.json",
				New: "/repo/.scholia/decisions/" + newID + ".json",
				Err: errors.New("operation not permitted"),
			}},
			wantCause: "rename: operation not permitted",
		},
		{
			name: "tmp 作成失敗（PathError）",
			err: &store.RecordWriteError{Category: "decision", Err: &os.PathError{
				Op: "open", Path: "/repo/.scholia/decisions/.tmp-1.json", Err: errors.New("permission denied"),
			}},
			wantCause: "open: permission denied",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeStoreError(rec, tc.err)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", rec.Code)
			}
			var body struct {
				Error string `json:"error"`
				Code  string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if body.Code != "record-write-failed" {
				t.Fatalf("code = %q, want record-write-failed", body.Code)
			}
			if strings.Contains(body.Error, newID) {
				t.Fatalf("宛先の ULID が漏れている: %s", body.Error)
			}
			if leaks := ulidPattern.FindAllString(body.Error, -1); len(leaks) > 0 {
				t.Fatalf("生 ULID が漏れている: %v\nmessage: %s", leaks, body.Error)
			}
			if strings.Contains(body.Error, "/repo/") || strings.Contains(body.Error, ".tmp-") {
				t.Fatalf("パスが漏れている: %s", body.Error)
			}
			// 到達手段: 書き込み先ディレクトリと OS レベルの原因は残す。
			// ULID を消すだけにすると運用者が原因に辿り着けない。
			if !strings.Contains(body.Error, ".scholia/decisions/") {
				t.Fatalf("書き込み先ディレクトリを示すべき: %s", body.Error)
			}
			if !strings.Contains(body.Error, tc.wantCause) {
				t.Fatalf("OS レベルの原因を残すべき（%s）: %s", tc.wantCause, body.Error)
			}
			// 採用フローでは POST 失敗時に提案が残る——保存されていないことを明示する。
			if !strings.Contains(body.Error, "保存されていません") {
				t.Fatalf("保存されていないことを示すべき: %s", body.Error)
			}
		})
	}
}

// 実ハンドラを通した経路でも漏れないこと（decisions/ を書込不可にして
// tmp 作成を失敗させる。rename 失敗と違い ULID に依存せず再現できる）。
func TestPostDecision_WriteFailureCarriesNoULID(t *testing.T) {
	h, s := newTestHandler(t)
	dir := filepath.Join(s.Dir, "decisions")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	rec := doRequest(t, h, http.MethodPost, "/api/decision", []byte(`{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[]}`))
	if rec.Code != http.StatusInternalServerError {
		t.Skipf("保存が失敗しなかった（root 実行等）: status=%d", rec.Code)
	}
	got := decodeJSON[struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}](t, rec)
	if got.Code != "record-write-failed" {
		t.Fatalf("code = %q, want record-write-failed", got.Code)
	}
	if leaks := ulidPattern.FindAllString(got.Error, -1); len(leaks) > 0 {
		t.Fatalf("生 ULID が漏れている: %v\n%s", leaks, got.Error)
	}
	if strings.Contains(got.Error, s.Dir) {
		t.Fatalf("絶対パスが漏れている: %s", got.Error)
	}
	if !strings.Contains(got.Error, ".scholia/decisions/") || !strings.Contains(got.Error, "denied") {
		t.Fatalf("ディレクトリと原因を残すべき: %s", got.Error)
	}
}

// 逆側の固定: CLI 向けは元の文言のまま（full path が要る）。
func TestRecordWriteError_CLIMessageUnchanged(t *testing.T) {
	inner := &os.LinkError{Op: "rename", Old: "/a/.tmp-1.json", New: "/a/01KYHZZZZZZZZZZZZZZZZZZZZZ.json", Err: errors.New("boom")}
	err := &store.RecordWriteError{Category: "decision", Err: inner}
	if err.Error() != inner.Error() {
		t.Fatalf("CLI 向け文言は元のままであるべき:\n got = %q\nwant = %q", err.Error(), inner.Error())
	}
}
