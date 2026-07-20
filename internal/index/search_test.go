package index

import (
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
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

func TestSearch_MatchesVocabAltLabels(t *testing.T) {
	snap := searchSnapshot()
	snap.Vocab[1].AltLabels = []string{"セッション発行", "auth token"} // eff.token
	ix := Build(snap)

	byAlt := Search(ix, "auth token")
	assertTxIDs(t, byAlt.Transitions, []string{"T-login", "T-other"})

	byAltJa := Search(ix, "セッション発行")
	assertTxIDs(t, byAltJa.Transitions, []string{"T-login", "T-other"})
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

// SearchRecords (unified core, #45 D10b-3) tests.

func recordSnapshot() *store.Snapshot {
	snap := searchSnapshot()
	snap.Tags = append(snap.Tags, model.Tag{ID: "req.auth-happy", Name: "正常系", Kind: "requirement", ParentIDs: []string{"req.auth"}})
	// Retag T-login onto the child so req.auth is an effective (ancestor) tag.
	snap.Transitions[0].Tags = []string{"req.auth-happy"}
	snap.Vocab[0].Description = "フォーム送信の説明文" // act.submit
	snap.Decisions = []model.Decision{
		{ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.auth"}, Why: "認証は httpOnly", Changed: "cookie 発行を変更", At: "2026-01-01T00:00:00Z"},
	}
	return snap
}

func matchByType(matches []RecordMatch, typ string) []RecordMatch {
	var out []RecordMatch
	for _, m := range matches {
		if m.Type == typ {
			out = append(out, m)
		}
	}
	return out
}

func TestSearchRecords_AllFourTypes(t *testing.T) {
	ix := Build(recordSnapshot())

	// tag: name/description; vocab: label/description; transition: id/tag/vocab;
	// decision: why/changed.
	if got := matchByType(SearchRecords(ix, []string{"認証"}, nil), RecordTag); len(got) == 0 {
		t.Fatalf("expected a tag match for 認証, got none")
	}
	if got := matchByType(SearchRecords(ix, []string{"説明文"}, nil), RecordVocab); len(got) == 0 {
		t.Fatalf("expected a vocab description match, got none")
	}
	if got := matchByType(SearchRecords(ix, []string{"T-login"}, nil), RecordTransition); len(got) == 0 {
		t.Fatalf("expected a transition id match, got none")
	}
	if got := matchByType(SearchRecords(ix, []string{"httpOnly"}, nil), RecordDecision); len(got) == 0 {
		t.Fatalf("expected a decision why match, got none")
	}
	if got := matchByType(SearchRecords(ix, []string{"cookie 発行"}, nil), RecordDecision); len(got) == 0 {
		t.Fatalf("expected a decision changed match, got none")
	}
}

func TestSearchRecords_TransitionHitViaEffectiveTagAndKind(t *testing.T) {
	ix := Build(recordSnapshot())
	// The sanctioned CLI behavior change (#45 D10b-3): a transition matches on
	// its ancestor (effective) tag name and its action kind name.
	if got := matchByType(SearchRecords(ix, []string{"認証要件"}, []string{RecordTransition}), RecordTransition); len(got) == 0 {
		t.Fatalf("expected T-login to match its effective (ancestor) tag req.auth's name")
	}
	if got := matchByType(SearchRecords(ix, []string{"user"}, []string{RecordTransition}), RecordTransition); len(got) == 0 {
		t.Fatalf("expected T-login to match its action kind name 'user'")
	}
}

func TestSearchRecords_TypeFilter(t *testing.T) {
	ix := Build(recordSnapshot())
	got := SearchRecords(ix, []string{"認証"}, []string{RecordTag})
	for _, m := range got {
		if m.Type != RecordTag {
			t.Fatalf("type filter leaked %q", m.Type)
		}
	}
	if len(got) == 0 {
		t.Fatalf("expected tag matches for 認証 with --type tag")
	}
}

func TestSearchRecords_OrAcrossKeywords(t *testing.T) {
	ix := Build(recordSnapshot())
	// "T-login" hits a transition; "説明文" hits a vocab — OR gets both types.
	got := SearchRecords(ix, []string{"T-login", "説明文"}, nil)
	if len(matchByType(got, RecordTransition)) == 0 || len(matchByType(got, RecordVocab)) == 0 {
		t.Fatalf("OR across keywords should match both transition and vocab, got %+v", got)
	}
}

func TestSearchRecords_EmptyKeywordsEmpty(t *testing.T) {
	ix := Build(recordSnapshot())
	if got := SearchRecords(ix, []string{"   "}, nil); len(got) != 0 {
		t.Fatalf("blank keyword should match nothing, got %+v", got)
	}
	if got := SearchRecords(ix, nil, nil); len(got) != 0 {
		t.Fatalf("nil keywords should match nothing, got %+v", got)
	}
}

func TestSearchCorpus_CoversAllTypes(t *testing.T) {
	docs := SearchCorpus(Build(recordSnapshot()))
	types := map[string]bool{}
	for _, d := range docs {
		types[d.Type] = true
	}
	for _, want := range []string{RecordTag, RecordTransition, RecordVocab, RecordDecision} {
		if !types[want] {
			t.Fatalf("SearchCorpus missing type %q; got %v", want, types)
		}
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
