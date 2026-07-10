package index

import (
	"testing"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func searchSnapshot() *store.Snapshot {
	return &store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "act.submit", Category: model.CategoryAction, Label: "フォーム送信", Kind: "user"},
			{ID: "eff.token", Category: model.CategoryEffect, Label: "トークン発行"},
		},
		Tags: []model.Tag{
			{ID: "req.auth", Name: "認証要件", Kind: "requirement"},
		},
		Transitions: []model.Transition{
			{ID: "T-login", Action: "act.submit", Then: []string{"eff.token"}, Tags: []string{"req.auth"}},
			{ID: "T-other", Action: "act.submit", Then: []string{"eff.token"}},
		},
	}
}

func TestSearch_MatchesEffectiveTagIDOrName(t *testing.T) {
	ix := Build(searchSnapshot())

	byID := Search(ix, "req.auth")
	assertTxIDs(t, byID.Transitions, []string{"T-login"})

	byName := Search(ix, "認証")
	assertTxIDs(t, byName.Transitions, []string{"T-login"})
}

func TestSearch_MatchesVocabIDOrLabel(t *testing.T) {
	ix := Build(searchSnapshot())

	byID := Search(ix, "eff.token")
	assertTxIDs(t, byID.Transitions, []string{"T-login", "T-other"})

	byLabel := Search(ix, "送信")
	assertTxIDs(t, byLabel.Transitions, []string{"T-login", "T-other"})
}

func TestSearch_MatchesTransitionID(t *testing.T) {
	ix := Build(searchSnapshot())

	res := Search(ix, "T-login")
	assertTxIDs(t, res.Transitions, []string{"T-login"})
}

func TestSearch_MatchesKindName(t *testing.T) {
	ix := Build(searchSnapshot())

	res := Search(ix, "user")
	assertTxIDs(t, res.Transitions, []string{"T-login", "T-other"})
}

func TestSearch_IsCaseInsensitive(t *testing.T) {
	ix := Build(searchSnapshot())

	res := Search(ix, "USER")
	assertTxIDs(t, res.Transitions, []string{"T-login", "T-other"})
}

func TestSearch_NoHitsReturnsEmptyNotNil(t *testing.T) {
	ix := Build(searchSnapshot())

	res := Search(ix, "nope-nothing-matches")
	if res.Transitions == nil {
		t.Fatal("Transitions is nil, want empty slice")
	}
	if len(res.Transitions) != 0 {
		t.Fatalf("Transitions = %v, want empty", res.Transitions)
	}
	if res.MatchedOn == nil {
		t.Fatal("MatchedOn is nil, want empty map")
	}
}

func TestSearch_EmptyQueryMatchesNothing(t *testing.T) {
	ix := Build(searchSnapshot())

	res := Search(ix, "   ")
	if len(res.Transitions) != 0 {
		t.Fatalf("Transitions = %v, want empty for blank query", res.Transitions)
	}
}

func assertTxIDs(t *testing.T, got []model.Transition, want []string) {
	t.Helper()
	gotIDs := make([]string, len(got))
	for i, tx := range got {
		gotIDs[i] = tx.ID
	}
	if len(gotIDs) != len(want) {
		t.Fatalf("tx ids = %v, want %v", gotIDs, want)
	}
	wantSet := make(map[string]bool, len(want))
	for _, id := range want {
		wantSet[id] = true
	}
	for _, id := range gotIDs {
		if !wantSet[id] {
			t.Fatalf("tx ids = %v, want %v", gotIDs, want)
		}
	}
}
