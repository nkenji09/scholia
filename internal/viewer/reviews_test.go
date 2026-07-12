package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/product-memory/internal/review"
)

// GET /api/reviews は .pmem/reviews/ に書かれたレビューを read-only で返す（§8.4）。
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
