package index

import (
	"reflect"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// testSnapshotWithDecisions reuses the fixture from index_test.go (T-1 tagged
// req.auth-happy+subject.auth, T-2 tagged concern.security, T-3 untagged) and
// adds decisions for the rules-selector tests.
func testSnapshotWithDecisions() *store.Snapshot {
	snap := testSnapshot()
	snap.Decisions = []model.Decision{
		{ID: "d-tx", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-1"}, Why: "T-1 固有の decision", At: "2026-01-01T00:00:00Z"},
		{ID: "d-tag-ancestor", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.auth"}, Why: "req.auth の cross-cutting rule", At: "2026-01-02T00:00:00Z"},
		{ID: "d-unrelated-tag", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "concern.security"}, Why: "T-1 とは無関係", At: "2026-01-03T00:00:00Z"},
	}
	return snap
}

func TestFilterTransitions_ByTagUsesAncestorExpansion(t *testing.T) {
	ix := Build(testSnapshot())
	// req.auth is T-1's tag's ancestor (req.auth-happy -> req.auth); §3.7 ancestor expansion must hit it.
	got := txIDs(FilterTransitions(ix, ix.AllTransitions(), "req.auth", ""))
	want := []string{"T-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterTransitions(tag=req.auth) = %v, want %v", got, want)
	}
}

func TestFilterTransitions_ByKind(t *testing.T) {
	ix := Build(testSnapshot())
	got := txIDs(FilterTransitions(ix, ix.AllTransitions(), "", "user"))
	want := []string{"T-1", "T-2", "T-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterTransitions(kind=user) = %v, want %v", got, want)
	}
	if got := FilterTransitions(ix, ix.AllTransitions(), "", "system"); len(got) != 0 {
		t.Fatalf("FilterTransitions(kind=system) = %v, want none", got)
	}
}

func TestBuildFacetNodes_AttachesOnlyFilteredTransitions(t *testing.T) {
	ix := Build(testSnapshot())
	filtered := FilterTransitions(ix, ix.AllTransitions(), "", "")

	nodes := BuildFacetNodes(ix, "requirement", filtered)
	if len(nodes) != 1 || nodes[0].Tag.ID != "req.auth" {
		t.Fatalf("roots = %+v, want single root req.auth", nodes)
	}
	// TransitionsByTag ancestor-expands (§3.7), so T-1 (tagged on the child
	// req.auth-happy) also lands on the req.auth root node itself.
	if len(nodes[0].Transitions) != 1 || nodes[0].Transitions[0].ID != "T-1" {
		t.Fatalf("req.auth transitions = %+v, want [T-1] via ancestor expansion", nodes[0].Transitions)
	}
	if len(nodes[0].Children) != 1 || nodes[0].Children[0].Tag.ID != "req.auth-happy" {
		t.Fatalf("req.auth children = %+v, want [req.auth-happy]", nodes[0].Children)
	}
	child := nodes[0].Children[0]
	if len(child.Transitions) != 1 || child.Transitions[0].ID != "T-1" {
		t.Fatalf("req.auth-happy transitions = %+v, want [T-1]", child.Transitions)
	}
}

func TestBuildFacetNodes_RespectsFilteredSubset(t *testing.T) {
	ix := Build(testSnapshot())
	// Pre-filter down to T-2 only; even though T-1 would otherwise attach to the requirement tree,
	// it must not appear since it's excluded from the filtered set.
	filtered := FilterTransitions(ix, ix.AllTransitions(), "concern.security", "")
	nodes := BuildFacetNodes(ix, "requirement", filtered)
	for _, root := range nodes {
		if len(root.Transitions) != 0 {
			t.Fatalf("root %s has transitions %+v, want none (T-2 has no requirement-kind tag)", root.Tag.ID, root.Transitions)
		}
	}
}

func TestUntaggedTransitions(t *testing.T) {
	ix := Build(testSnapshot())
	filtered := FilterTransitions(ix, ix.AllTransitions(), "", "")
	got := txIDs(UntaggedTransitions(ix, filtered, "requirement"))
	want := []string{"T-2", "T-3"} // neither has a requirement-kind effective tag
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UntaggedTransitions(requirement) = %v, want %v", got, want)
	}
}

func TestSelectRulesDecisions_ByTxIncludesOwnAndAncestorTagDecisions(t *testing.T) {
	snap := testSnapshotWithDecisions()
	got, err := SelectRulesDecisions(snap, "", "T-1", "")
	if err != nil {
		t.Fatalf("SelectRulesDecisions: %v", err)
	}
	var ids []string
	for _, d := range got {
		ids = append(ids, d.ID)
	}
	want := map[string]bool{"d-tx": true, "d-tag-ancestor": true}
	if len(ids) != len(want) {
		t.Fatalf("decision ids = %v, want exactly %v", ids, want)
	}
	for _, id := range ids {
		if !want[id] {
			t.Fatalf("unexpected decision %q in %v", id, ids)
		}
	}
}

func TestSelectRulesDecisions_ByTxUnknownIsError(t *testing.T) {
	snap := testSnapshotWithDecisions()
	if _, err := SelectRulesDecisions(snap, "", "does-not-exist", ""); err == nil {
		t.Fatalf("expected error for unknown transition")
	}
}

func TestSelectRulesDecisions_ByTagIncludesAncestors(t *testing.T) {
	snap := testSnapshotWithDecisions()
	got, err := SelectRulesDecisions(snap, "req.auth-happy", "", "")
	if err != nil {
		t.Fatalf("SelectRulesDecisions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "d-tag-ancestor" {
		t.Fatalf("decisions = %+v, want [d-tag-ancestor] (req.auth is req.auth-happy's parent)", got)
	}
}

func TestSelectRulesDecisions_ByTagUnknownIsError(t *testing.T) {
	snap := testSnapshotWithDecisions()
	if _, err := SelectRulesDecisions(snap, "does.not.exist", "", ""); err == nil {
		t.Fatalf("expected error for unknown tag")
	}
}

func TestSelectRulesDecisions_ByFacet(t *testing.T) {
	snap := testSnapshotWithDecisions()
	got, err := SelectRulesDecisions(snap, "", "", "requirement")
	if err != nil {
		t.Fatalf("SelectRulesDecisions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "d-tag-ancestor" {
		t.Fatalf("decisions = %+v, want [d-tag-ancestor] (only decision on a requirement-kind tag)", got)
	}
}

func TestSelectRulesDecisions_NoneReturnsAll(t *testing.T) {
	snap := testSnapshotWithDecisions()
	got, err := SelectRulesDecisions(snap, "", "", "")
	if err != nil {
		t.Fatalf("SelectRulesDecisions: %v", err)
	}
	if len(got) != len(snap.Decisions) {
		t.Fatalf("len(decisions) = %d, want %d (all)", len(got), len(snap.Decisions))
	}
}
