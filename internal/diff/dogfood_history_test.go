package diff

// dogfood 実履歴（この repo の git 履歴）に対する欄位単位正規化の受け入れ固定
// テスト（#45 U4）:
//   - 29e817c … `scholia decision add-commit` による commits 結線の decide
//     コミット。旧実装（decision 全体一致比較）では違反扱いだった正規操作で、
//     欄位単位の再定義後は緑になること。
//   - 65cb5a4 … #42 の全店 prose retrofit（"pmem"→"scholia"・decisions 24 件の
//     判断欄位改変）。再定義後も赤のまま（判断欄位は不可侵）であること。
//
// shallow clone 等で対象コミットが解決できない環境では skip する。

import (
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

const (
	dogfoodAddCommitRef = "29e817c" // decide(#43②) add-commit のみの変更
	dogfoodRetrofitRef  = "65cb5a4" // rename(#42 P5a) 判断欄位 retrofit
)

func dogfoodStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Discover(".")
	if err != nil {
		t.Fatalf("dogfood store が見つかりません（repo checkout が壊れている）: %v", err)
	}
	return s
}

func requireDogfoodRef(t *testing.T, s *store.Store, ref string) {
	t.Helper()
	root, _, err := repoRootAndRelDir(s)
	if err != nil {
		t.Skipf("git repo が解決できないため skip: %v", err)
	}
	if _, err := runGit(root, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
		t.Skipf("dogfood 履歴 %s が解決できないため skip（shallow clone？）: %v", ref, err)
	}
}

func TestDogfoodHistory_AddCommitCommitIsGreen(t *testing.T) {
	s := dogfoodStore(t)
	requireDogfoodRef(t, s, dogfoodAddCommitRef+"^")

	r, err := DiffRefs(s, dogfoodAddCommitRef+"^", dogfoodAddCommitRef)
	if err != nil {
		t.Fatalf("DiffRefs(%s^, %s): %v", dogfoodAddCommitRef, dogfoodAddCommitRef, err)
	}
	if r.DecisionViolation() {
		t.Fatalf("add-commit の正規コミット %s が違反扱いのまま（偽陽性未解消）: %+v",
			dogfoodAddCommitRef, r.Decisions.Changed)
	}
	// 変更されたのは decision 01KXN6G0R4DSXEVV86K8W0CZYW の commits 追記のみ。
	if len(r.Decisions.Changed) != 1 {
		t.Fatalf("Decisions.Changed = %d 件, want 1", len(r.Decisions.Changed))
	}
	c := r.Decisions.Changed[0]
	if c.ID != "01KXN6G0R4DSXEVV86K8W0CZYW" {
		t.Fatalf("changed decision = %s, want 01KXN6G0R4DSXEVV86K8W0CZYW", c.ID)
	}
	if len(c.AllowedFields) != 1 || c.AllowedFields[0] != "commits(+1)" {
		t.Fatalf("AllowedFields = %v, want [commits(+1)]", c.AllowedFields)
	}
}

func TestDogfoodHistory_RetrofitCommitStaysRed(t *testing.T) {
	s := dogfoodStore(t)
	requireDogfoodRef(t, s, dogfoodRetrofitRef+"^")

	r, err := DiffRefs(s, dogfoodRetrofitRef+"^", dogfoodRetrofitRef)
	if err != nil {
		t.Fatalf("DiffRefs(%s^, %s): %v", dogfoodRetrofitRef, dogfoodRetrofitRef, err)
	}
	if !r.DecisionViolation() {
		t.Fatalf("#42 判断欄位 retrofit コミット %s が緑になった（不可侵欄位が緩んでいる）", dogfoodRetrofitRef)
	}
	// kit-bundle1 の実測: decisions 24 件（why 23・changed 2、うち 1 件は changed 欄のみ）。
	violations := 0
	whyCount, changedCount := 0, 0
	for _, c := range r.Decisions.Changed {
		if !c.Violation() {
			continue
		}
		violations++
		for _, f := range c.ViolatedFields {
			switch f {
			case "why":
				whyCount++
			case "changed":
				changedCount++
			}
		}
	}
	if violations != 24 {
		t.Fatalf("違反 decision = %d 件, want 24（kit-bundle1 実測）", violations)
	}
	if whyCount != 23 || changedCount != 2 {
		t.Fatalf("欄位内訳 why=%d changed=%d, want why=23 changed=2（kit-bundle1 実測）", whyCount, changedCount)
	}
}
