package diff

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

func TestCompute_VocabAddedAndRemoved(t *testing.T) {
	before := refSnapshot{Vocab: []model.VocabEntry{{ID: "cond.a", Category: "condition", Label: "a"}}}
	after := refSnapshot{Vocab: []model.VocabEntry{{ID: "cond.b", Category: "condition", Label: "b"}}}

	r := compute("HEAD", before, after)
	if len(r.Vocab.Added) != 1 || r.Vocab.Added[0].ID != "cond.b" {
		t.Fatalf("Vocab.Added = %+v, want [cond.b]", r.Vocab.Added)
	}
	if len(r.Vocab.Removed) != 1 || r.Vocab.Removed[0].ID != "cond.a" {
		t.Fatalf("Vocab.Removed = %+v, want [cond.a]", r.Vocab.Removed)
	}
	if len(r.Vocab.Changed) != 0 {
		t.Fatalf("Vocab.Changed = %+v, want none", r.Vocab.Changed)
	}
	if r.Empty() {
		t.Fatalf("Empty() = true, want false")
	}
}

func TestCompute_VocabChangedSameID(t *testing.T) {
	before := refSnapshot{Vocab: []model.VocabEntry{{ID: "cond.a", Category: "condition", Label: "旧"}}}
	after := refSnapshot{Vocab: []model.VocabEntry{{ID: "cond.a", Category: "condition", Label: "新"}}}

	r := compute("HEAD", before, after)
	if len(r.Vocab.Changed) != 1 || r.Vocab.Changed[0].Before.Label != "旧" || r.Vocab.Changed[0].After.Label != "新" {
		t.Fatalf("Vocab.Changed = %+v", r.Vocab.Changed)
	}
}

func TestCompute_NoChangesIsEmpty(t *testing.T) {
	snap := refSnapshot{
		Vocab:       []model.VocabEntry{{ID: "cond.a", Category: "condition", Label: "a"}},
		Tags:        []model.Tag{{ID: "t.a", Name: "a"}},
		Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Given: []string{"cond.a"}, Then: []string{"eff.a"}}},
		Decisions:   []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "why", At: "2026-01-01T00:00:00Z"}},
	}
	r := compute("HEAD", snap, snap)
	if !r.Empty() {
		t.Fatalf("Empty() = false, want true for identical snapshots: %+v", r)
	}
}

func TestCompute_ThenReorderIsDetectedAsChangeNotAddRemove(t *testing.T) {
	before := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a", "eff.b"}}}}
	after := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.b", "eff.a"}}}}

	r := compute("HEAD", before, after)
	if len(r.Transitions.Added) != 0 || len(r.Transitions.Removed) != 0 {
		t.Fatalf("expected no add/remove for reorder, got added=%v removed=%v", r.Transitions.Added, r.Transitions.Removed)
	}
	if len(r.Transitions.Changed) != 1 {
		t.Fatalf("Transitions.Changed = %+v, want 1 entry", r.Transitions.Changed)
	}
	c := r.Transitions.Changed[0]
	if !c.ThenChanged || !c.ThenReordered {
		t.Fatalf("ThenChanged=%v ThenReordered=%v, want both true", c.ThenChanged, c.ThenReordered)
	}
}

func TestCompute_ThenElementChangeIsNotReordered(t *testing.T) {
	before := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a", "eff.b"}}}}
	after := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a", "eff.c"}}}}

	r := compute("HEAD", before, after)
	c := r.Transitions.Changed[0]
	if !c.ThenChanged || c.ThenReordered {
		t.Fatalf("ThenChanged=%v ThenReordered=%v, want changed=true reordered=false", c.ThenChanged, c.ThenReordered)
	}
}

func TestCompute_GivenIsSetComparisonNotOrderSensitive(t *testing.T) {
	before := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Given: []string{"cond.a", "cond.b"}, Then: []string{"eff.a"}}}}
	after := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Given: []string{"cond.b", "cond.a"}, Then: []string{"eff.a"}}}}

	r := compute("HEAD", before, after)
	if len(r.Transitions.Changed) != 0 {
		t.Fatalf("given reordering must not count as a change (given is a set, §3.2), got %+v", r.Transitions.Changed)
	}
}

func TestCompute_DecisionAddedIsNormalNotViolation(t *testing.T) {
	before := refSnapshot{}
	after := refSnapshot{Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "w", At: "2026-01-01T00:00:00Z"}}}

	r := compute("HEAD", before, after)
	if len(r.Decisions.Added) != 1 {
		t.Fatalf("Decisions.Added = %+v, want 1", r.Decisions.Added)
	}
	if r.DecisionViolation() {
		t.Fatalf("DecisionViolation() = true for a pure append, want false")
	}
}

func TestCompute_DecisionRemovedIsViolation(t *testing.T) {
	before := refSnapshot{Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "w", At: "2026-01-01T00:00:00Z"}}}
	after := refSnapshot{}

	r := compute("HEAD", before, after)
	if len(r.Decisions.Removed) != 1 {
		t.Fatalf("Decisions.Removed = %+v, want 1", r.Decisions.Removed)
	}
	if !r.DecisionViolation() {
		t.Fatalf("DecisionViolation() = false after a decision was removed, want true (append-only violation)")
	}
}

func TestCompute_DecisionModifiedIsViolation(t *testing.T) {
	before := refSnapshot{Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "旧", At: "2026-01-01T00:00:00Z"}}}
	after := refSnapshot{Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "改変", At: "2026-01-01T00:00:00Z"}}}

	r := compute("HEAD", before, after)
	if len(r.Decisions.Changed) != 1 {
		t.Fatalf("Decisions.Changed = %+v, want 1", r.Decisions.Changed)
	}
	if !r.DecisionViolation() {
		t.Fatalf("DecisionViolation() = false after a decision was modified, want true (append-only violation)")
	}
	if got := r.Decisions.Changed[0].ViolatedFields; len(got) != 1 || got[0] != "why" {
		t.Fatalf("ViolatedFields = %v, want [why]", got)
	}
}

// --- #45 U4: 欄位単位正規化 ---

func baseDecision() model.Decision {
	return model.Decision{
		ID:     "d1",
		Target: model.DecisionTarget{Type: "transition", ID: "T-old"},
		Why:    "why", Changed: "changed", Ref: "PR#1",
		At:      "2026-01-01T00:00:00Z",
		Commits: []string{"aaa111"},
	}
}

func TestCompute_CommitsAppendIsAllowed(t *testing.T) {
	b := baseDecision()
	a := baseDecision()
	a.Commits = []string{"aaa111", "bbb222"}
	before := refSnapshot{Decisions: []model.Decision{b}}
	after := refSnapshot{Decisions: []model.Decision{a}}

	r := compute("HEAD", before, after)
	if len(r.Decisions.Changed) != 1 {
		t.Fatalf("Decisions.Changed = %+v, want 1", r.Decisions.Changed)
	}
	c := r.Decisions.Changed[0]
	if c.Violation() {
		t.Fatalf("commits の追記が違反扱いされた（正規操作 add-commit の偽陽性）: %+v", c)
	}
	if len(c.AllowedFields) != 1 || c.AllowedFields[0] != "commits(+1)" {
		t.Fatalf("AllowedFields = %v, want [commits(+1)]", c.AllowedFields)
	}
	if r.DecisionViolation() {
		t.Fatalf("DecisionViolation() = true for a pure commits append, want false")
	}
}

func TestCompute_CommitsRemovalOrReorderIsViolation(t *testing.T) {
	for name, commits := range map[string][]string{
		"removal": {},
		"replace": {"zzz999"},
		"reorder": {"bbb222", "aaa111"},
	} {
		b := baseDecision()
		b.Commits = []string{"aaa111", "bbb222"}
		a := baseDecision()
		a.Commits = commits
		r := compute("HEAD", refSnapshot{Decisions: []model.Decision{b}}, refSnapshot{Decisions: []model.Decision{a}})
		if !r.DecisionViolation() {
			t.Fatalf("%s: commits の削除・改変・並べ替えは違反のはず: %+v", name, r.Decisions.Changed)
		}
		if got := r.Decisions.Changed[0].ViolatedFields; len(got) != 1 || got[0] != "commits" {
			t.Fatalf("%s: ViolatedFields = %v, want [commits]", name, got)
		}
	}
}

// --- supersedes（#45 D7）の append-only 分類（correctness critical） ---
//
// model に Supersedes を足すと未知フィールドでなくなり、diffDecisions の
// reflect.DeepEqual が supersedes 変更を検出する。分類を足さないと既存 link の
// 改変が allowed にも violated にも入らず黙認される（append-only 破れ）。以下は
// その穴が開いていないことの反証テスト。

func TestCompute_SupersedesAppendIsAllowed(t *testing.T) {
	b := baseDecision()
	a := baseDecision()
	a.Supersedes = []model.SupersedeLink{{ID: "d0", Mode: "supersede"}}
	r := compute("HEAD", refSnapshot{Decisions: []model.Decision{b}}, refSnapshot{Decisions: []model.Decision{a}})
	if len(r.Decisions.Changed) != 1 {
		t.Fatalf("Decisions.Changed = %+v, want 1", r.Decisions.Changed)
	}
	c := r.Decisions.Changed[0]
	if c.Violation() || r.DecisionViolation() {
		t.Fatalf("supersede link の追記が違反扱いされた（追記専用 link の偽陽性）: %+v", c)
	}
	if len(c.AllowedFields) != 1 || c.AllowedFields[0] != "supersedes(+1)" {
		t.Fatalf("AllowedFields = %v, want [supersedes(+1)]", c.AllowedFields)
	}
}

func TestCompute_SupersedesModeChangeIsViolation(t *testing.T) {
	// 既存 link の mode 改変は append-only 破れ（判断の書き換え）。
	b := baseDecision()
	b.Supersedes = []model.SupersedeLink{{ID: "d0", Mode: "amend"}}
	a := baseDecision()
	a.Supersedes = []model.SupersedeLink{{ID: "d0", Mode: "supersede"}}
	r := compute("HEAD", refSnapshot{Decisions: []model.Decision{b}}, refSnapshot{Decisions: []model.Decision{a}})
	if !r.DecisionViolation() {
		t.Fatalf("既存 supersede link の mode 改変が violation にならない（黙認される穴）: %+v", r.Decisions.Changed)
	}
	if got := r.Decisions.Changed[0].ViolatedFields; len(got) != 1 || got[0] != "supersedes" {
		t.Fatalf("ViolatedFields = %v, want [supersedes]", got)
	}
}

func TestCompute_SupersedesRemovalOrReorderIsViolation(t *testing.T) {
	for name, links := range map[string][]model.SupersedeLink{
		"removal": {},
		"replace": {{ID: "dX", Mode: "supersede"}},
		"reorder": {{ID: "d2", Mode: "amend"}, {ID: "d1prev", Mode: "supersede"}},
	} {
		b := baseDecision()
		b.Supersedes = []model.SupersedeLink{{ID: "d1prev", Mode: "supersede"}, {ID: "d2", Mode: "amend"}}
		a := baseDecision()
		a.Supersedes = links
		r := compute("HEAD", refSnapshot{Decisions: []model.Decision{b}}, refSnapshot{Decisions: []model.Decision{a}})
		if !r.DecisionViolation() {
			t.Fatalf("%s: 既存 supersede link の削除・改変・並べ替えは違反のはず: %+v", name, r.Decisions.Changed)
		}
		if got := r.Decisions.Changed[0].ViolatedFields; len(got) != 1 || got[0] != "supersedes" {
			t.Fatalf("%s: ViolatedFields = %v, want [supersedes]", name, got)
		}
	}
}

func TestCompute_AcknowledgesAppendOnly(t *testing.T) {
	// 追記は allowed・既存要素の削除（容認の取り消し）は violation。
	appended := baseDecision()
	appended.Acknowledges = []string{"requirement-gap"}
	rAdd := compute("HEAD", refSnapshot{Decisions: []model.Decision{baseDecision()}}, refSnapshot{Decisions: []model.Decision{appended}})
	if rAdd.DecisionViolation() {
		t.Fatalf("acknowledges の追記が違反扱いされた: %+v", rAdd.Decisions.Changed)
	}
	if got := rAdd.Decisions.Changed[0].AllowedFields; len(got) != 1 || got[0] != "acknowledges(+1)" {
		t.Fatalf("AllowedFields = %v, want [acknowledges(+1)]", got)
	}

	b := baseDecision()
	b.Acknowledges = []string{"requirement-gap", "overlap"}
	a := baseDecision()
	a.Acknowledges = []string{"requirement-gap"}
	rDel := compute("HEAD", refSnapshot{Decisions: []model.Decision{b}}, refSnapshot{Decisions: []model.Decision{a}})
	if !rDel.DecisionViolation() {
		t.Fatalf("acknowledges の削除（容認取り消し）が violation にならない: %+v", rDel.Decisions.Changed)
	}
	if got := rDel.Decisions.Changed[0].ViolatedFields; len(got) != 1 || got[0] != "acknowledges" {
		t.Fatalf("ViolatedFields = %v, want [acknowledges]", got)
	}
}

func TestCompute_JudgmentFieldChangesAreViolations(t *testing.T) {
	mutate := map[string]func(*model.Decision){
		"why":         func(d *model.Decision) { d.Why = "改変" },
		"changed":     func(d *model.Decision) { d.Changed = "改変" },
		"ref":         func(d *model.Decision) { d.Ref = "改変" },
		"at":          func(d *model.Decision) { d.At = "2027-01-01T00:00:00Z" },
		"target.type": func(d *model.Decision) { d.Target.Type = "tag" },
	}
	for field, fn := range mutate {
		b := baseDecision()
		a := baseDecision()
		fn(&a)
		r := compute("HEAD", refSnapshot{Decisions: []model.Decision{b}}, refSnapshot{Decisions: []model.Decision{a}})
		if !r.DecisionViolation() {
			t.Fatalf("%s: 判断欄位の改変は違反のはず: %+v", field, r.Decisions.Changed)
		}
		found := false
		for _, v := range r.Decisions.Changed[0].ViolatedFields {
			if v == field {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: ViolatedFields = %v に欄位名が無い", field, r.Decisions.Changed[0].ViolatedFields)
		}
	}
}

func TestCompute_TxRenamePairAllowsTargetIDRepoint(t *testing.T) {
	tx := model.Transition{ID: "T-old", Action: "act.a", Given: []string{"cond.a"}, Then: []string{"eff.a"}, Tags: []string{"subject.x"}}
	renamed := tx
	renamed.ID = "T-new"

	b := baseDecision()
	a := baseDecision()
	a.Target.ID = "T-new"
	before := refSnapshot{Transitions: []model.Transition{tx}, Decisions: []model.Decision{b}}
	after := refSnapshot{Transitions: []model.Transition{renamed}, Decisions: []model.Decision{a}}

	r := compute("HEAD", before, after)
	if r.DecisionViolation() {
		t.Fatalf("tx rename ペア照合が取れる target.id 張替えが違反扱い: %+v", r.Decisions.Changed)
	}
	if got := r.Decisions.Changed[0].AllowedFields; len(got) != 1 || got[0] != "target.id(rename T-old→T-new)" {
		t.Fatalf("AllowedFields = %v", got)
	}
}

func TestCompute_TxRenameWithSameCommitEditIsStillAllowed(t *testing.T) {
	// 同一 PR 内で rename 後にレコード自体がさらに編集された場合、内容同一照合は
	// 破れるが「旧 id 消滅＋新 id 出現」のペアで許容する（result.md P2 検証補正）。
	tx := model.Transition{ID: "T-old", Action: "act.a", Then: []string{"eff.a"}}
	renamedAndEdited := model.Transition{ID: "T-new", Action: "act.a", Then: []string{"eff.a", "eff.b"}}

	b := baseDecision()
	a := baseDecision()
	a.Target.ID = "T-new"
	r := compute("HEAD",
		refSnapshot{Transitions: []model.Transition{tx}, Decisions: []model.Decision{b}},
		refSnapshot{Transitions: []model.Transition{renamedAndEdited}, Decisions: []model.Decision{a}})
	if r.DecisionViolation() {
		t.Fatalf("rename+edit の張替えが違反扱い: %+v", r.Decisions.Changed)
	}
	if got := r.Decisions.Changed[0].AllowedFields; len(got) != 1 || got[0] != "target.id(rename+edit T-old→T-new)" {
		t.Fatalf("AllowedFields = %v", got)
	}
}

func TestCompute_TxMergePairAllowsRepointToSurvivor(t *testing.T) {
	dup := model.Transition{ID: "T-dup", Action: "act.a", Then: []string{"eff.a"}}
	survivor := model.Transition{ID: "T-surv", Action: "act.a", Then: []string{"eff.a"}}

	b := baseDecision()
	b.Target.ID = "T-dup"
	a := baseDecision()
	a.Target.ID = "T-surv"
	before := refSnapshot{Transitions: []model.Transition{dup, survivor}, Decisions: []model.Decision{b}}
	after := refSnapshot{Transitions: []model.Transition{survivor}, Decisions: []model.Decision{a}}

	r := compute("HEAD", before, after)
	if r.DecisionViolation() {
		t.Fatalf("merge ペア照合（旧消滅＋現存宛）が違反扱い: %+v", r.Decisions.Changed)
	}
	if got := r.Decisions.Changed[0].AllowedFields; len(got) != 1 || got[0] != "target.id(merge T-dup→T-surv)" {
		t.Fatalf("AllowedFields = %v", got)
	}
}

func TestCompute_RepointWithoutPairIsViolation(t *testing.T) {
	// 旧 transition が残ったままの張替え（rename でも merge でもない）は違反。
	tx1 := model.Transition{ID: "T-old", Action: "act.a", Then: []string{"eff.a"}}
	tx2 := model.Transition{ID: "T-other", Action: "act.b", Then: []string{"eff.b"}}

	b := baseDecision()
	a := baseDecision()
	a.Target.ID = "T-other"
	before := refSnapshot{Transitions: []model.Transition{tx1, tx2}, Decisions: []model.Decision{b}}
	after := refSnapshot{Transitions: []model.Transition{tx1, tx2}, Decisions: []model.Decision{a}}

	r := compute("HEAD", before, after)
	if !r.DecisionViolation() {
		t.Fatalf("ペア照合の取れない target.id 張替えは違反のはず: %+v", r.Decisions.Changed)
	}
	if got := r.Decisions.Changed[0].ViolatedFields; len(got) != 1 || got[0] != "target.id" {
		t.Fatalf("ViolatedFields = %v, want [target.id]", got)
	}
}

func TestCompute_TagCascadeRenameAllowsSubtreeRepoints(t *testing.T) {
	// req.foo → req.baz の cascade: 子 req.foo.bar → req.baz.bar は parentIds の
	// 親 id 置換（req.foo → req.baz）も追随扱いでサブツリー単位に照合される。
	root := model.Tag{ID: "req.foo", Name: "foo", Kind: "requirement"}
	child := model.Tag{ID: "req.foo.bar", Name: "bar", Kind: "requirement", ParentIDs: []string{"req.foo"}}
	newRoot := model.Tag{ID: "req.baz", Name: "foo", Kind: "requirement"}
	newChild := model.Tag{ID: "req.baz.bar", Name: "bar", Kind: "requirement", ParentIDs: []string{"req.baz"}}

	dRoot := baseDecision()
	dRoot.ID = "d-root"
	dRoot.Target = model.DecisionTarget{Type: "tag", ID: "req.foo"}
	dChild := baseDecision()
	dChild.ID = "d-child"
	dChild.Target = model.DecisionTarget{Type: "tag", ID: "req.foo.bar"}

	aRoot := dRoot
	aRoot.Target.ID = "req.baz"
	aChild := dChild
	aChild.Target.ID = "req.baz.bar"

	before := refSnapshot{Tags: []model.Tag{root, child}, Decisions: []model.Decision{dRoot, dChild}}
	after := refSnapshot{Tags: []model.Tag{newRoot, newChild}, Decisions: []model.Decision{aRoot, aChild}}

	r := compute("HEAD", before, after)
	if r.DecisionViolation() {
		t.Fatalf("tag cascade rename の追随張替えが違反扱い: %+v", r.Decisions.Changed)
	}
	want := map[string]string{
		"d-child": "target.id(rename req.foo.bar→req.baz.bar)",
		"d-root":  "target.id(rename req.foo→req.baz)",
	}
	for _, c := range r.Decisions.Changed {
		if len(c.AllowedFields) != 1 || c.AllowedFields[0] != want[c.ID] {
			t.Fatalf("%s: AllowedFields = %v, want [%s]", c.ID, c.AllowedFields, want[c.ID])
		}
	}
}

func TestCompute_TagRepointToPreexistingTagIsViolation(t *testing.T) {
	// タグには merge の正規操作が無い: 旧タグ消滅でも既存タグ宛の張替えは違反。
	oldTag := model.Tag{ID: "req.old", Name: "old"}
	existing := model.Tag{ID: "req.existing", Name: "existing"}

	b := baseDecision()
	b.Target = model.DecisionTarget{Type: "tag", ID: "req.old"}
	a := b
	a.Target.ID = "req.existing"
	before := refSnapshot{Tags: []model.Tag{oldTag, existing}, Decisions: []model.Decision{b}}
	after := refSnapshot{Tags: []model.Tag{existing}, Decisions: []model.Decision{a}}

	r := compute("HEAD", before, after)
	if !r.DecisionViolation() {
		t.Fatalf("既存タグ宛の張替えは違反のはず: %+v", r.Decisions.Changed)
	}
}

func TestCompute_VocabRenamePairForwardCompat(t *testing.T) {
	// P5 前方互換: vocab-target decision（未導入）の rename 追随も判定できる。
	v := model.VocabEntry{ID: "cond.old", Category: "condition", Label: "c"}
	renamed := v
	renamed.ID = "cond.new"

	b := baseDecision()
	b.Target = model.DecisionTarget{Type: "vocab", ID: "cond.old"}
	a := b
	a.Target.ID = "cond.new"
	before := refSnapshot{Vocab: []model.VocabEntry{v}, Decisions: []model.Decision{b}}
	after := refSnapshot{Vocab: []model.VocabEntry{renamed}, Decisions: []model.Decision{a}}

	r := compute("HEAD", before, after)
	if r.DecisionViolation() {
		t.Fatalf("vocab rename ペア照合が取れる張替えが違反扱い: %+v", r.Decisions.Changed)
	}
	if got := r.Decisions.Changed[0].AllowedFields; len(got) != 1 || got[0] != "target.id(rename cond.old→cond.new)" {
		t.Fatalf("AllowedFields = %v", got)
	}
}

func TestCompute_VocabTargetDecisionRepointAllowedWithNewFields(t *testing.T) {
	// #45 D5: vocab-target decision の target.id が vocab rename で張替わった
	// diff は append-only OK（exit 0 相当・違反にならない）。新スロット
	// （establishes/ref/altLabels）を持つ vocab の rename も内容照合が通る。
	v := model.VocabEntry{ID: "eff.old", Category: "effect", Label: "save",
		Ref: "https://example/spec", AltLabels: []string{"別名"}, Establishes: []string{"cond.x"}}
	renamed := v
	renamed.ID = "eff.new"

	b := baseDecision()
	b.Target = model.DecisionTarget{Type: "vocab", ID: "eff.old"}
	a := b
	a.Target.ID = "eff.new"
	before := refSnapshot{Vocab: []model.VocabEntry{v}, Decisions: []model.Decision{b}}
	after := refSnapshot{Vocab: []model.VocabEntry{renamed}, Decisions: []model.Decision{a}}

	r := compute("HEAD", before, after)
	if r.DecisionViolation() {
		t.Fatalf("新スロット付き vocab rename 追随が違反扱い: %+v", r.Decisions.Changed)
	}
	if got := r.Decisions.Changed[0].AllowedFields; len(got) != 1 || got[0] != "target.id(rename eff.old→eff.new)" {
		t.Fatalf("AllowedFields = %v", got)
	}
}

func TestCompute_RenamePairDoesNotUnlockJudgmentFields(t *testing.T) {
	// 悪用対策: rename ペアが存在しても判断欄位（why）は不可侵。
	tx := model.Transition{ID: "T-old", Action: "act.a", Then: []string{"eff.a"}}
	renamed := tx
	renamed.ID = "T-new"

	b := baseDecision()
	a := baseDecision()
	a.Target.ID = "T-new"
	a.Why = "書き換え"
	r := compute("HEAD",
		refSnapshot{Transitions: []model.Transition{tx}, Decisions: []model.Decision{b}},
		refSnapshot{Transitions: []model.Transition{renamed}, Decisions: []model.Decision{a}})
	if !r.DecisionViolation() {
		t.Fatalf("rename ペアの存在下でも why 改変は違反のはず: %+v", r.Decisions.Changed)
	}
	c := r.Decisions.Changed[0]
	if len(c.ViolatedFields) != 1 || c.ViolatedFields[0] != "why" {
		t.Fatalf("ViolatedFields = %v, want [why]", c.ViolatedFields)
	}
	if len(c.AllowedFields) != 1 {
		t.Fatalf("AllowedFields = %v（target.id 張替え自体は許容として記録される）", c.AllowedFields)
	}
}

// 前方互換（将来の additive フィールド）: 未知 additive フィールドの追記は
// decode 層で無視されるため Changed に現れない＝violation にならない。
// supersedes は #45 D7 で既知フィールドになったため、ここでは「本実装がまだ
// 知らない架空の将来フィールド」で前方互換の不変条件を守る（supersedes の
// append-only 分類は TestSupersedesAppendOnly 系が担う）。
func TestUnknownAdditiveFieldIsIgnoredByDecode(t *testing.T) {
	var withField, without model.Decision
	if err := json.Unmarshal([]byte(`{"id":"d1","target":{"type":"tag","id":"req.a"},"why":"w","at":"2026-01-01T00:00:00Z","someFutureP12Field":[{"id":"d0"}]}`), &withField); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"id":"d1","target":{"type":"tag","id":"req.a"},"why":"w","at":"2026-01-01T00:00:00Z"}`), &without); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withField, without) {
		t.Fatalf("未知 additive フィールドが decode に影響している: %+v vs %+v", withField, without)
	}
	r := compute("HEAD", refSnapshot{Decisions: []model.Decision{without}}, refSnapshot{Decisions: []model.Decision{withField}})
	if len(r.Decisions.Changed) != 0 || r.DecisionViolation() {
		t.Fatalf("未知 additive フィールドの追記が Changed/violation になった: %+v", r.Decisions)
	}
}

func TestSetDiff(t *testing.T) {
	added, removed := setDiff([]string{"a", "b"}, []string{"b", "c"})
	if !reflect.DeepEqual(added, []string{"c"}) {
		t.Fatalf("added = %v, want [c]", added)
	}
	if !reflect.DeepEqual(removed, []string{"a"}) {
		t.Fatalf("removed = %v, want [a]", removed)
	}
}
