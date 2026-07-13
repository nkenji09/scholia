package index

import (
	"reflect"
	"testing"
	"time"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func txIDs(ts []model.Transition) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

func testSnapshot() *store.Snapshot {
	return &store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "act.submit", Category: model.CategoryAction, Label: "送信", Kind: "user"},
			{ID: "cond.valid", Category: model.CategoryCondition, Label: "正当"},
			{ID: "eff.token", Category: model.CategoryEffect, Label: "トークン発行"},
			{ID: "eff.redirect", Category: model.CategoryEffect, Label: "リダイレクト"},
		},
		Tags: []model.Tag{
			{ID: "subject.auth", Name: "認証", Kind: "subject"},
			{ID: "req.auth", Name: "認証要件", Kind: "requirement"},
			{ID: "req.auth-happy", Name: "正常系", Kind: "requirement", ParentIDs: []string{"req.auth"}},
			{ID: "concern.security", Name: "セキュリティ", Kind: "concern"},
		},
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.submit", Given: []string{"cond.valid"}, Then: []string{"eff.token", "eff.redirect"},
				Tags: []string{"req.auth-happy", "subject.auth"}},
			{ID: "T-2", Action: "act.submit", Given: []string{}, Then: []string{"eff.token"},
				Tags: []string{"concern.security"}},
			{ID: "T-3", Action: "act.submit", Given: []string{}, Then: []string{"eff.redirect"}},
		},
	}
}

func TestBuild_EffectiveTagsMaterialized(t *testing.T) {
	ix := Build(testSnapshot())

	got := ix.EffectiveTags["T-1"]
	want := []string{"req.auth", "req.auth-happy", "subject.auth"} // 祖先展開込み
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveTags[T-1] = %v, want %v", got, want)
	}
}

func TestBuild_TagTransitionsReverseIncludesDescendantHits(t *testing.T) {
	ix := Build(testSnapshot())

	// req.auth は T-1 の親タグ（req.auth-happy 経由）。祖先展開の帰結でヒットするはず（§3.7）。
	got := txIDs(ix.TransitionsByTag("req.auth"))
	want := []string{"T-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitionsByTag(req.auth) = %v, want %v", got, want)
	}

	if !ix.HasEffectiveTag("T-1", "req.auth") {
		t.Fatalf("HasEffectiveTag(T-1, req.auth) = false, want true")
	}
	if ix.HasEffectiveTag("T-2", "req.auth") {
		t.Fatalf("HasEffectiveTag(T-2, req.auth) = true, want false")
	}
}

func TestBuild_VocabTransitionsReverse(t *testing.T) {
	ix := Build(testSnapshot())

	got := txIDs(ix.TransitionsByVocab("eff.token"))
	want := []string{"T-1", "T-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitionsByVocab(eff.token) = %v, want %v", got, want)
	}

	got = txIDs(ix.TransitionsByVocab("eff.redirect"))
	want = []string{"T-1", "T-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitionsByVocab(eff.redirect) = %v, want %v", got, want)
	}
}

func TestBuild_AllTransitionsSortedByID(t *testing.T) {
	ix := Build(testSnapshot())
	got := txIDs(ix.AllTransitions())
	want := []string{"T-1", "T-2", "T-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllTransitions ids = %v, want %v", got, want)
	}
}

func TestFacetTree_BuildsNestedTreeWithinKindOnly(t *testing.T) {
	ix := Build(testSnapshot())

	tree := ix.FacetTree("requirement")
	if len(tree) != 1 {
		t.Fatalf("expected 1 root for requirement facet, got %d", len(tree))
	}
	root := tree[0]
	if root.Tag.ID != "req.auth" {
		t.Fatalf("root = %s, want req.auth", root.Tag.ID)
	}
	if len(root.Children) != 1 || root.Children[0].Tag.ID != "req.auth-happy" {
		t.Fatalf("root.Children = %+v, want single child req.auth-happy", root.Children)
	}

	// subject/concern kind のタグは requirement facet には出ない。
	subjectTree := ix.FacetTree("subject")
	if len(subjectTree) != 1 || subjectTree[0].Tag.ID != "subject.auth" {
		t.Fatalf("subject facet tree = %+v, want single root subject.auth", subjectTree)
	}
}

func TestFacetTree_MultiParentAppearsInMultipleBranches(t *testing.T) {
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "req.a", Name: "a", Kind: "requirement"},
			{ID: "req.b", Name: "b", Kind: "requirement"},
			{ID: "req.child", Name: "child", Kind: "requirement", ParentIDs: []string{"req.a", "req.b"}},
		},
	}
	ix := Build(snap)
	tree := ix.FacetTree("requirement")
	if len(tree) != 2 {
		t.Fatalf("expected 2 roots (a, b), got %d: %+v", len(tree), tree)
	}
	for _, root := range tree {
		if len(root.Children) != 1 || root.Children[0].Tag.ID != "req.child" {
			t.Fatalf("root %s children = %+v, want single child req.child", root.Tag.ID, root.Children)
		}
	}
}

func TestFacetTree_ToleratesCycleWithinKind(t *testing.T) {
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "a", Name: "a", Kind: "requirement", ParentIDs: []string{"b"}},
			{ID: "b", Name: "b", Kind: "requirement", ParentIDs: []string{"a"}},
		},
	}
	ix := Build(snap)
	done := make(chan []*TagNode, 1)
	go func() { done <- ix.FacetTree("requirement") }()
	select {
	case tree := <-done:
		_ = tree // 無限ループしなければ十分（lint が別途循環を error にする・§5）
	case <-time.After(2 * time.Second):
		t.Fatal("FacetTree did not terminate on cyclic parentIds")
	}
}

// rootIDs / findNode help assert against the unified TagForest shape.
func rootIDs(forest []*TagNode) []string {
	out := make([]string, len(forest))
	for i, n := range forest {
		out[i] = n.Tag.ID
	}
	return out
}

func findNode(forest []*TagNode, id string) *TagNode {
	for _, n := range forest {
		if n.Tag.ID == id {
			return n
		}
		if got := findNode(n.Children, id); got != nil {
			return got
		}
	}
	return nil
}

func childIDs(node *TagNode) []string {
	if node == nil {
		return nil
	}
	out := make([]string, len(node.Children))
	for i, c := range node.Children {
		out[i] = c.Tag.ID
	}
	return out
}

func TestTagForest_CrossKindNestingAndNullKind(t *testing.T) {
	// subject 親の下に requirement 子（cross-kind）、さらに kind=null のタグも
	// parentIds 通りに現れることを確認する — per-kind FacetTree では両方脱落する。
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "subject.auth", Name: "認証", Kind: "subject"},
			{ID: "req.login", Name: "ログイン要件", Kind: "requirement", ParentIDs: []string{"subject.auth"}},
			{ID: "note.free", Name: "無分類メモ", ParentIDs: []string{"req.login"}}, // kind=null
		},
	}
	ix := Build(snap)

	forest := ix.TagForest()
	if got := rootIDs(forest); !reflect.DeepEqual(got, []string{"subject.auth"}) {
		t.Fatalf("TagForest roots = %v, want [subject.auth]", got)
	}
	if got := childIDs(findNode(forest, "subject.auth")); !reflect.DeepEqual(got, []string{"req.login"}) {
		t.Fatalf("subject.auth children = %v, want [req.login] (cross-kind nesting)", got)
	}
	if got := childIDs(findNode(forest, "req.login")); !reflect.DeepEqual(got, []string{"note.free"}) {
		t.Fatalf("req.login children = %v, want [note.free] (kind=null nests, not dropped)", got)
	}

	// 対照: per-kind FacetTree は cross-kind の親子を繋がない（挙動不変の確認）。
	reqTree := ix.FacetTree("requirement")
	if got := rootIDs(reqTree); !reflect.DeepEqual(got, []string{"req.login"}) {
		t.Fatalf("FacetTree(requirement) roots = %v, want [req.login] (re-rooted, subject parent invisible)", got)
	}
}

func TestTagForest_MultiParentAppearsUnderEachParent(t *testing.T) {
	// 多親タグは各親の下に重複して現れる（多重所属可・§3.8）。cross-kind でも同様。
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "subject.a", Name: "A", Kind: "subject"},
			{ID: "concern.b", Name: "B", Kind: "concern"},
			{ID: "req.shared", Name: "共有", Kind: "requirement", ParentIDs: []string{"subject.a", "concern.b"}},
		},
	}
	ix := Build(snap)
	forest := ix.TagForest()
	if got := rootIDs(forest); !reflect.DeepEqual(got, []string{"concern.b", "subject.a"}) {
		t.Fatalf("TagForest roots = %v, want [concern.b subject.a]", got)
	}
	for _, parent := range []string{"subject.a", "concern.b"} {
		if got := childIDs(findNode(forest, parent)); !reflect.DeepEqual(got, []string{"req.shared"}) {
			t.Fatalf("%s children = %v, want [req.shared]", parent, got)
		}
	}
}

func TestTagForest_ToleratesCycle(t *testing.T) {
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "a", Name: "a", Kind: "requirement", ParentIDs: []string{"b"}},
			{ID: "b", Name: "b", Kind: "subject", ParentIDs: []string{"a"}},
		},
	}
	ix := Build(snap)
	done := make(chan []*TagNode, 1)
	go func() { done <- ix.TagForest() }()
	select {
	case tree := <-done:
		_ = tree // 無限ループしなければ十分（lint が循環を別途 error にする・§5）
	case <-time.After(2 * time.Second):
		t.Fatal("TagForest did not terminate on cyclic parentIds")
	}
}
