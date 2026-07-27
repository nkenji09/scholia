package viewer

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/review"
)

// GET /api/reviews は .scholia/reviews/ に書かれたレビューを read-only で返す（§8.4）。
func TestGetReviews(t *testing.T) {
	h, s := newTestHandler(t)

	rec := doRequest(t, h, http.MethodGet, "/api/reviews", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if empty := decodeJSON[[]review.Review](t, rec); len(empty) != 0 {
		t.Fatalf("reviews が無いときは空配列であるべき: %+v", empty)
	}

	if err := review.Add(s.Dir, review.Review{
		ID:        "r-1",
		RecordRef: review.RecordRef{Type: review.RecordTypeTag, ID: "subject.auth"},
		Body:      "AI: これはテスト提案の理由",
		Source:    review.SourceAI,
		CreatedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("review.Add: %v", err)
	}

	rec = doRequest(t, h, http.MethodGet, "/api/reviews", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[[]review.Review](t, rec)
	if len(got) != 1 || got[0].ID != "r-1" || got[0].Body != "AI: これはテスト提案の理由" {
		t.Fatalf("reviews = %+v, want [r-1]", got)
	}

	// レビューが存在しても LoadAll（lint の入力）には無影響（§8.4 grounding）。
	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(snap.Decisions) != 1 {
		t.Fatalf("LoadAll should be unaffected by reviews/: got %d decisions", len(snap.Decisions))
	}
}

// DELETE /api/reviews/{id} は adopt/reject の掃除ステップ（§35）—
// review を削除し、以後 GET /api/reviews から消える。
func TestDeleteReview(t *testing.T) {
	h, s := newTestHandler(t)

	if err := review.Add(s.Dir, review.Review{
		ID:        "r-1",
		RecordRef: review.RecordRef{Type: review.RecordTypeTag, ID: "subject.auth"},
		Body:      "AI: これはテスト提案の理由",
		Source:    review.SourceAI,
		CreatedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("review.Add: %v", err)
	}

	rec := doRequest(t, h, http.MethodDelete, "/api/reviews/r-1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(t, h, http.MethodGet, "/api/reviews", nil)
	got := decodeJSON[[]review.Review](t, rec)
	if len(got) != 0 {
		t.Fatalf("review should be gone after delete: %+v", got)
	}
}

// 存在しない id への DELETE は 404。
func TestDeleteReview_MissingIsNotFound(t *testing.T) {
	h, _ := newTestHandler(t)

	rec := doRequest(t, h, http.MethodDelete, "/api/reviews/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

// GET /api/reviews は宣言された結線先を解決して返す（derive・保存しない）。
// ドロワーが Adopt を押す前に「何を失効/改訂させるか」を人へ見せるための材料で、
// 生 ULID を読ませないために name/label・日付・why 要約まで解決する。
func TestGetReviews_ResolvesSupersedesDetail(t *testing.T) {
	h, s := newTestHandler(t)
	if err := review.Add(s.Dir, review.Review{
		ID:         "r-super",
		RecordRef:  review.RecordRef{Type: review.RecordTypeTag, ID: "subject.auth"},
		Body:       "AI: 改訂案",
		Source:     review.SourceAI,
		CreatedAt:  "2026-01-02T00:00:00Z",
		Supersedes: []model.SupersedeLink{{ID: "d1", Mode: model.ModeSupersede}, {ID: "missing-one"}},
	}); err != nil {
		t.Fatalf("review.Add: %v", err)
	}

	rec := doRequest(t, h, http.MethodGet, "/api/reviews", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[[]reviewResponse](t, rec)
	if len(got) != 1 {
		t.Fatalf("reviews = %+v, want 1", got)
	}
	d := got[0].SupersedesDetail
	if len(d) != 2 {
		t.Fatalf("supersedesDetail = %+v, want 2", d)
	}
	if d[0].ID != "d1" || d[0].Mode != model.ModeSupersede {
		t.Fatalf("detail[0] = %+v, want d1/supersede", d[0])
	}
	if d[0].TargetID != "subject.auth" || d[0].TargetName != "認証" {
		t.Fatalf("対象レコードの読める名前まで解決すべき: %+v", d[0])
	}
	if d[0].WhySummary != "認証は httpOnly cookie で発行" {
		t.Fatalf("whySummary = %q", d[0].WhySummary)
	}
	// 実在しない宣言は missing で返す（採用時にエラーになることを押す前に見せる）。
	if !d[1].Missing {
		t.Fatalf("detail[1] は missing であるべき: %+v", d[1])
	}
	// 未宣言の判断材料: 対象レコードちょうどに付いた decision の件数（d1 の1件）。
	if got[0].PriorDecisionCount != 1 {
		t.Fatalf("priorDecisionCount = %d, want 1", got[0].PriorDecisionCount)
	}
}

// 宣言が無ければ detail は出ない（omitempty）が、件数は返る。
func TestGetReviews_NoDeclarationStillReportsPriorCount(t *testing.T) {
	h, s := newTestHandler(t)
	if err := review.Add(s.Dir, review.Review{
		ID:        "r-plain",
		RecordRef: review.RecordRef{Type: review.RecordTypeTag, ID: "subject.auth"},
		Body:      "AI: 宣言なし",
		Source:    review.SourceAI,
		CreatedAt: "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("review.Add: %v", err)
	}
	rec := doRequest(t, h, http.MethodGet, "/api/reviews", nil)
	got := decodeJSON[[]reviewResponse](t, rec)
	if len(got) != 1 || len(got[0].SupersedesDetail) != 0 {
		t.Fatalf("supersedesDetail は空であるべき: %+v", got)
	}
	if got[0].PriorDecisionCount != 1 {
		t.Fatalf("priorDecisionCount = %d, want 1", got[0].PriorDecisionCount)
	}
}

// DELETE /api/reviews/{id} の失敗文言に review の生 ULID を出さない
// （01KYCC2TF3NW3JRSSRK9ZHN078）。ドロワーの Adopt は decision 昇格の直後に
// この DELETE を呼ぶので、失敗すると同じエラー欄に出る。
func TestDeleteReview_ErrorsCarryNoULID(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		wantStatus int
		wantCode   string
	}{
		{"実在しない review", "01KYHZZZZZZZZZZZZZZZZZZZZZ", http.StatusNotFound, "review-not-found"},
		// パス区切りは %2F/%5C でエンコードされたときだけハンドラまで届く
		// （生の "/" はルータが別ルートに振り、"a..b" のような形は素通りする）。
		{"パス区切りを含む不正 id", "01KYHZZZZZZZZZZZZZZZZZZZZZ%2Fx", http.StatusBadRequest, "review-invalid-id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestHandler(t)
			rec := doRequest(t, h, http.MethodDelete, "/api/reviews/"+tc.id, nil)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			got := decodeJSON[struct {
				Error string `json:"error"`
				Code  string `json:"code"`
			}](t, rec)
			if got.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", got.Code, tc.wantCode)
			}
			if leaks := ulidPattern.FindAllString(got.Error, -1); len(leaks) > 0 {
				t.Fatalf("失敗メッセージに生 ULID が漏れている: %v\nmessage: %s", leaks, got.Error)
			}
		})
	}
}

// 逆側の固定: CLI 向けの文言（review.NotFoundError.Error()）は id を含み続ける。
// 手で .scholia/reviews/ を触る人には「どの id を指したか」が要る。
func TestReviewNotFoundError_CLIMessageKeepsID(t *testing.T) {
	err := &review.NotFoundError{ID: "01KYHZZZZZZZZZZZZZZZZZZZZZ"}
	if !strings.Contains(err.Error(), "01KYHZZZZZZZZZZZZZZZZZZZZZ") {
		t.Fatalf("CLI 向け文言は id を含むべき: %q", err.Error())
	}
}
