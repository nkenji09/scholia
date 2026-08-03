package viewer

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func TestPostDecision_CreatesDecisionFile(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"on":"transition:T-login","why":"# テスト用の見出し\n\ndangling 参照だけでなく commit の実在性も検証する","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	d := decodeJSON[model.Decision](t, rec)
	if d.ID == "" {
		t.Fatalf("Decision.ID is empty, want a fresh ULID")
	}
	if d.Target.Type != model.DecisionTargetTransition || d.Target.ID != "T-login" {
		t.Fatalf("Target = %+v, want transition:T-login", d.Target)
	}
	if d.Why != "# テスト用の見出し\n\ndangling 参照だけでなく commit の実在性も検証する" {
		t.Fatalf("Why = %q, want the posted body", d.Why)
	}
	if len(d.Commits) != 0 {
		t.Fatalf("Commits = %v, want empty at adoption time (§8.5)", d.Commits)
	}

	persisted, err := s.LoadDecision(d.ID)
	if err != nil {
		t.Fatalf("LoadDecision(%s): %v (decision file was not written)", d.ID, err)
	}
	if persisted.Why != d.Why {
		t.Fatalf("persisted Why = %q, want %q", persisted.Why, d.Why)
	}
}

func TestPostDecision_OnTag(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"# テスト用の見出し\n\n認証まわりの方針を決めた","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	d := decodeJSON[model.Decision](t, rec)
	if d.Target.Type != model.DecisionTargetTag || d.Target.ID != "subject.auth" {
		t.Fatalf("Target = %+v, want tag:subject.auth", d.Target)
	}
	if _, err := s.LoadDecision(d.ID); err != nil {
		t.Fatalf("LoadDecision(%s): %v", d.ID, err)
	}
}

// TestPostDecision_AppendOnly locks in §8.7: creating a new decision on a
// target that already has one (newTestHandler seeds "d1" on subject.auth)
// must never touch the existing decision file — every POST mints a fresh
// ULID and only ever adds a file (see postDecisionHandler's doc comment).
func TestPostDecision_AppendOnly(t *testing.T) {
	h, s := newTestHandler(t)
	before, err := s.LoadDecision("d1")
	if err != nil {
		t.Fatalf("LoadDecision(d1) before: %v", err)
	}

	body := []byte(`{"on":"tag:subject.auth","why":"# テスト用の見出し\n\n別の判断を追加する","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	created := decodeJSON[model.Decision](t, rec)
	if created.ID == "d1" {
		t.Fatalf("new decision reused id %q, want a fresh ULID distinct from the seeded d1", created.ID)
	}

	after, err := s.LoadDecision("d1")
	if err != nil {
		t.Fatalf("LoadDecision(d1) after: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("existing decision d1 changed: before=%+v after=%+v (append-only violated)", before, after)
	}
}

func TestPostDecision_UnknownTransitionRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"transition:T-does-not-exist","why":"# テスト用の見出し\n\n存在しない対象","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostDecision_UnknownTagRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"tag:does-not-exist","why":"# テスト用の見出し\n\n存在しない対象","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostDecision_MalformedOnRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"bogus","why":"# テスト用の見出し\n\n形式が不正","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostDecision_EmptyWhyRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"transition:T-login","why":"","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostDecision_UnknownFieldRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"transition:T-login","why":"# テスト用の見出し\n\n…","commits":[],"bogusField":1}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// --- 現行性リンク（adopt が supersedes まで束ねる・01KYHE08WNA8H1Q1DM2H45Y4TK） ---

// 回帰の芯（viewer 面）: POST /api/decision が supersedes を受理して保存する。
// body 型に持たせないと DisallowUnknownFields が 400 を返し、ドロワーの Adopt は
// 構造的に結線できない。
func TestPostDecision_AcceptsSupersedes(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"# テスト用の見出し\n\n改訂: cookie ではなく header で運ぶ","commits":[],"supersedes":[{"id":"d1","mode":"supersede"}]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	d := decodeJSON[model.Decision](t, rec)
	if len(d.Supersedes) != 1 || d.Supersedes[0].ID != "d1" || d.Supersedes[0].Mode != model.ModeSupersede {
		t.Fatalf("Supersedes = %+v, want [{d1 supersede}]", d.Supersedes)
	}
	persisted, err := s.LoadDecision(d.ID)
	if err != nil {
		t.Fatalf("LoadDecision: %v", err)
	}
	if len(persisted.Supersedes) != 1 || persisted.Supersedes[0].ID != "d1" {
		t.Fatalf("保存された supersedes = %+v, want [d1]", persisted.Supersedes)
	}
}

// mode 省略は既定 amend として保存される（保存値は書かれたまま＝空）。
func TestPostDecision_SupersedesDefaultMode(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"# テスト用の見出し\n\n部分改訂","commits":[],"supersedes":[{"id":"d1"}]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	d := decodeJSON[model.Decision](t, rec)
	if len(d.Supersedes) != 1 || d.Supersedes[0].SupersedeMode() != model.ModeAmend {
		t.Fatalf("既定は amend であるべき: %+v", d.Supersedes)
	}
}

// 未知フィールドは従来どおり弾く（supersedes を足しても DisallowUnknownFields は緩めない）。
func TestPostDecision_StillRejectsUnknownFields(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"superseeds":[{"id":"d1"}]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400（typo フィールドは弾く）: %s", rec.Code, rec.Body.String())
	}
}

// 検証は CLI と同じ: 実在しない旧 decision・自己参照・重複・不正 mode は 400。
func TestPostDecision_SupersedesValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"実在しない旧 decision", `{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"supersedes":[{"id":"nope"}]}`},
		{"重複指定", `{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"supersedes":[{"id":"d1","mode":"amend"},{"id":"d1","mode":"supersede"}]}`},
		{"不正な mode", `{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"supersedes":[{"id":"d1","mode":"replace"}]}`},
		{"空の id", `{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"supersedes":[{"id":""}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestHandler(t)
			rec := doRequest(t, h, http.MethodPost, "/api/decision", []byte(tc.body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// 未宣言はブロックせず、応答に supersede-unlinked advisory を添える（非ブロック）。
func TestPostDecision_UnlinkedAdvisory(t *testing.T) {
	h, _ := newTestHandler(t)
	// subject.auth には既存 decision "d1" がある。
	rec := doRequest(t, h, http.MethodPost, "/api/decision", []byte(`{"on":"tag:subject.auth","why":"# テスト用の見出し\n\n宣言なしで採用","commits":[]}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("未宣言でもブロックしないべき: status = %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[struct {
		Advisories []struct {
			Rule string `json:"rule"`
		} `json:"advisories"`
	}](t, rec)
	var found bool
	for _, a := range got.Advisories {
		if a.Rule == "supersede-unlinked" {
			found = true
		}
	}
	if !found {
		t.Fatalf("supersede-unlinked advisory が出るべき: %s", rec.Body.String())
	}

	// 既存 decision の無い対象では出ない（純粋な新規追加は正当）。
	rec = doRequest(t, h, http.MethodPost, "/api/decision", []byte(`{"on":"transition:T-login","why":"# テスト用の見出し\n\n新規","commits":[]}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	got = decodeJSON[struct {
		Advisories []struct {
			Rule string `json:"rule"`
		} `json:"advisories"`
	}](t, rec)
	if len(got.Advisories) != 0 {
		t.Fatalf("既存 decision が無いのに advisory が出た: %s", rec.Body.String())
	}
}

// --- 失敗メッセージに生 ULID を出さない（01KYCC2TF3NW3JRSSRK9ZHN078） ---

// ulidPattern は Crockford Base32 の 26 文字 ULID。
var ulidPattern = regexp.MustCompile(`\b[0-9A-HJKMNP-TV-Z]{26}\b`)

// viewer は生レコード id を表示しない（id は deep-link の href としてのみ）。
// POST /api/decision の失敗応答は人がドロワーでそのまま読む文字列になるので、
// ここに ULID が乗ると決定の字義に反する。CLI 向けの文言（どの id を直せば
// よいかを含む）を body へ素通しさせないことを、この API 面で固定する。
func TestPostDecision_SupersedeErrorsCarryNoULID(t *testing.T) {
	// 実在する旧 decision を作っておく（重複・mode 改変の材料）。
	seed := func(t *testing.T) (http.Handler, string) {
		t.Helper()
		h, s := newTestHandler(t)
		old := "01KYHZZZZZZZZZZZZZZZZZZZZZ"
		if err := s.CreateDecision(model.Decision{
			ID: old, Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.auth"},
			Why: "# テスト用の見出し\n\n旧", At: "2026-01-01T00:00:00Z",
		}, store.DecisionCreateOptions{}); err != nil {
			t.Fatalf("seed decision: %v", err)
		}
		return h, old
	}

	cases := []struct {
		name     string
		body     func(old string) string
		wantCode string
	}{
		{
			name: "実在しない旧 decision",
			body: func(string) string {
				return `{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"supersedes":[{"id":"01KYHYYYYYYYYYYYYYYYYYYYYY"}]}`
			},
			wantCode: "supersedes-missing-target",
		},
		{
			name: "3値でない mode",
			body: func(o string) string {
				return `{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"supersedes":[{"id":"` + o + `","mode":"bogus"}]}`
			},
			wantCode: "supersedes-invalid-mode",
		},
		{
			name: "同一 id の重複",
			body: func(o string) string {
				return `{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"supersedes":[{"id":"` + o + `","mode":"amend"},{"id":"` + o + `","mode":"supersede"}]}`
			},
			wantCode: "supersedes-duplicate",
		},
		{
			name: "空 id",
			body: func(string) string {
				return `{"on":"tag:subject.auth","why":"# テスト用の見出し\n\nx","commits":[],"supersedes":[{"id":""}]}`
			},
			wantCode: "supersedes-empty-id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, old := seed(t)
			rec := doRequest(t, h, http.MethodPost, "/api/decision", []byte(tc.body(old)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			got := decodeJSON[struct {
				Error string `json:"error"`
				Code  string `json:"code"`
			}](t, rec)
			if got.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q（フロントが翻訳済み文言を選ぶ鍵）", got.Code, tc.wantCode)
			}
			if leaks := ulidPattern.FindAllString(got.Error, -1); len(leaks) > 0 {
				t.Fatalf("失敗メッセージに生 ULID が漏れている: %v\nmessage: %s", leaks, got.Error)
			}
		})
	}
}

// CLI 向けの文言（model.SupersedeError.Error()）は逆に id を含み続ける——
// どの review ファイルのどの id を直せばよいかを出す必要があるため。viewer と
// CLI で見せ方を分ける前提そのものを固定する。
func TestSupersedeError_CLIMessageKeepsID(t *testing.T) {
	err := &model.SupersedeError{Kind: model.SupersedeErrMissingTarget, ID: "01KYHZZZZZZZZZZZZZZZZZZZZZ"}
	if !strings.Contains(err.Error(), "01KYHZZZZZZZZZZZZZZZZZZZZZ") {
		t.Fatalf("CLI 向け文言は id を含むべき: %q", err.Error())
	}
	if ulidPattern.MatchString(supersedeViewerMessage(err)) {
		t.Fatalf("viewer 向け文言は id を含まないべき: %q", supersedeViewerMessage(err))
	}
}

// 面3: viewer の保存系エンドポイント（01KZ06SYR3APGF3JD4NQRFTEEN 変更3）。
// **逃し弁は置かない**（エスケープは CLI の --allow --reason 経由のみ・
// transition_write.go の先例どおり）。422 で返し、ドロワーは入力中の why を
// 保持したままエラーを表示できる。
func TestPostDecision_RejectsMissingHeading(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"見出しの無い why\n\n本文","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	if !strings.Contains(got, "見出し") {
		t.Fatalf("何を直せばよいかが body にあるべき: %s", got)
	}
	// viewer は生 ULID を表示しない（01KYCC2TF3NW3JRSSRK9ZHN078）。
	if ulidPattern.MatchString(got) {
		t.Fatalf("拒否の body に生 ULID が出ている: %s", got)
	}
	if !strings.Contains(got, "reject-decision-heading") {
		t.Fatalf("機械可読な code を返すべき: %s", got)
	}
	// 保存されていない（fixture の d1 以外に decision が増えていない）。
	if n := countDecisionFiles(t, s); n != 1 {
		t.Fatalf("拒んだのに decision が増えた: %d 件", n)
	}
}

func countDecisionFiles(t *testing.T, s *store.Store) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(s.Dir, "decisions"))
	if err != nil {
		t.Fatalf("read decisions dir: %v", err)
	}
	return len(entries)
}
