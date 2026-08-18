package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

// 3種類の印を、**decision をどう作ったかに依存せずに**打てること
// （01M09FHEQH7PVZ2BTKGXY5YMNN 変更3・4・7）。
//
// # ここが落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:**
//   - `add-commit` から `--kind` の必須が外れた（既定値が置かれた）
//   - 是正の `add-commit` が applied[] に印を足さなくなった
//   - 矛盾・却下の口が消えた／却下の指し先の必須が外れた／指し先の実在照合が外れた
//   - 印が `decision list --json` から数えられなくなった（集計の口はこれ1つ）
//
// **落ちない:**
//   - 打ち忘れ（呼ばなければ何も起きない。矛盾・却下の口は呼ばなくても作業が終わる）
//   - 誤分類（実装 commit を是正と名乗ること）

// appliedFixture は tag 1つと decision 2つを持つ標本を作り、その id を返す。
// **git 管理下ではない**ので、commit の実在照合は形だけが効く枝を通る。
func appliedFixture(t *testing.T) (dir, drawn, landed string) {
	t.Helper()
	dir = t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern"); err != nil {
		t.Fatalf("tag create: %v", err)
	}
	drawn = decideID(t, dir, "# 引かれる側の見出し\n\n過去に決めたこと。")
	landed = decideID(t, dir, "# 着地した側の見出し\n\n却下・改訂を記録した判断。")
	return dir, drawn, landed
}

func decideID(t *testing.T, dir, why string) string {
	t.Helper()
	out, err := run(t, dir, "decide", "--on", "tag:t1", "--why", why, "--json")
	if err != nil {
		t.Fatalf("decide: %v\n%s", err, out)
	}
	var env struct {
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	return env.Record.ID
}

// loadApplied は `decision list --json`（**集計の口はこれだけ**）から印を読む。
func loadApplied(t *testing.T, dir, id string) []model.AppliedMark {
	t.Helper()
	out, err := run(t, dir, "decision", "list", "--json")
	if err != nil {
		t.Fatalf("decision list --json: %v\n%s", err, out)
	}
	var resp struct {
		Decisions []struct {
			ID      string              `json:"id"`
			Applied []model.AppliedMark `json:"applied"`
		} `json:"decisions"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	for _, d := range resp.Decisions {
		if d.ID == id {
			return d.Applied
		}
	}
	t.Fatalf("decision %s が一覧に無い:\n%s", id, out)
	return nil
}

// `add-commit` は種別の指定が無ければ落ちる。**既定値は無い。**
func TestAddCommit_KindIsRequired(t *testing.T) {
	dir, drawn, _ := appliedFixture(t)

	out, err := run(t, dir, "decision", "add-commit", drawn, "aaa1111")
	if err == nil {
		t.Fatalf("種別なしの add-commit は落ちるべき:\n%s", out)
	}
	if !strings.Contains(out, "--kind") {
		t.Errorf("何を足せばよいかが文言に無い:\n%s", out)
	}
	// 落ちたのだから commits も増えていない。
	if marks := loadApplied(t, dir, drawn); len(marks) != 0 {
		t.Errorf("落ちたのに印が付いている: %+v", marks)
	}

	if out, err := run(t, dir, "decision", "add-commit", drawn, "aaa1111", "--kind", "landing"); err == nil {
		t.Fatalf("2値の外は落ちるべき:\n%s", out)
	}
}

// 実装として結ぶだけなら印は付かない。是正として結ぶと印が1件付く。
func TestAddCommit_CorrectionLeavesAMark(t *testing.T) {
	dir, drawn, _ := appliedFixture(t)

	if out, err := run(t, dir, "decision", "add-commit", drawn, "aaa1111", "--kind", "implementation"); err != nil {
		t.Fatalf("implementation: %v\n%s", err, out)
	}
	if marks := loadApplied(t, dir, drawn); len(marks) != 0 {
		t.Fatalf("実装として結んだだけで印が付いた: %+v", marks)
	}

	out, err := run(t, dir, "decision", "add-commit", drawn, "bbb2222", "--kind", "correction")
	if err != nil {
		t.Fatalf("correction: %v\n%s", err, out)
	}
	marks := loadApplied(t, dir, drawn)
	if len(marks) != 1 {
		t.Fatalf("是正の印が1件付くはず: %+v", marks)
	}
	if marks[0].Kind != model.AppliedCorrection || marks[0].Commit != "bbb2222" {
		t.Fatalf("種別と指し先が期待と違う: %+v", marks[0])
	}
	if marks[0].At == "" {
		t.Fatalf("時刻が空: %+v", marks[0])
	}
	// 同じ commit を2度是正と印しても増えない（指し先で畳む）。
	if out, err := run(t, dir, "decision", "add-commit", drawn, "bbb2222", "--kind", "correction"); err != nil {
		t.Fatalf("2度目: %v\n%s", err, out)
	}
	if marks := loadApplied(t, dir, drawn); len(marks) != 1 {
		t.Fatalf("同じ commit の是正は畳むはず: %+v", marks)
	}
}

// 矛盾・却下の口。**指し先なしの矛盾も1件として置ける**（カウンタでは表せない
// ものを追記専用で残すための形）。
func TestDecisionApplied_ConflictAndRejection(t *testing.T) {
	dir, drawn, landed := appliedFixture(t)

	if out, err := run(t, dir, "decision", "applied", drawn, "--kind", "conflict"); err != nil {
		t.Fatalf("指し先なしの矛盾: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "decision", "applied", drawn, "--kind", "rejection", "--landed", landed); err != nil {
		t.Fatalf("却下: %v\n%s", err, out)
	}

	marks := loadApplied(t, dir, drawn)
	if len(marks) != 2 {
		t.Fatalf("印が2件のはず: %+v", marks)
	}
	if model.CountApplied(marks, model.AppliedConflict) != 1 || model.CountApplied(marks, model.AppliedRejection) != 1 {
		t.Fatalf("種別ごとに数えられるはず: %+v", marks)
	}

	// 同じ decision への却下は畳む。
	if out, err := run(t, dir, "decision", "applied", drawn, "--kind", "rejection", "--landed", landed); err != nil {
		t.Fatalf("2度目の却下: %v\n%s", err, out)
	}
	if got := loadApplied(t, dir, drawn); len(got) != 2 {
		t.Fatalf("同じ指し先の却下は畳むはず: %+v", got)
	}

	// ⚠️ 指し先の無い矛盾は畳めない——2回打てば2件になる（正直に検査へ書く）。
	if out, err := run(t, dir, "decision", "applied", drawn, "--kind", "conflict"); err != nil {
		t.Fatalf("2度目の矛盾: %v\n%s", err, out)
	}
	if got := loadApplied(t, dir, drawn); len(got) != 3 {
		t.Fatalf("指し先の無い矛盾には畳む鍵が無い（3件になるはず）: %+v", got)
	}
}

func TestDecisionApplied_RejectsBadInput(t *testing.T) {
	dir, drawn, _ := appliedFixture(t)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"種別なし", []string{"decision", "applied", drawn}, "--kind"},
		{"3値の外", []string{"decision", "applied", drawn, "--kind", "refinement"}, "--kind"},
		{"是正はこの口では受けない", []string{"decision", "applied", drawn, "--kind", "correction"}, "add-commit"},
		{"却下に指し先が無い", []string{"decision", "applied", drawn, "--kind", "rejection"}, "却下"},
		{"指し先が実在しない", []string{"decision", "applied", drawn, "--kind", "rejection", "--landed", "01NOTEXIST"}, "実在しません"},
		{"自分自身を指す", []string{"decision", "applied", drawn, "--kind", "rejection", "--landed", drawn}, "自分自身"},
		{"decision が実在しない", []string{"decision", "applied", "01NOTEXIST", "--kind", "conflict"}, "読み込めません"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := run(t, dir, c.args...)
			if err == nil {
				t.Fatalf("落ちるべき:\n%s", out)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("文言に %q が無い（何を直せばよいか読めない）:\n%s", c.want, out)
			}
		})
	}
	if marks := loadApplied(t, dir, drawn); len(marks) != 0 {
		t.Fatalf("どれも落ちたのに印が付いている: %+v", marks)
	}
}

// git 管理下でない store では、実在を照合していないことを名乗る。
// **黙って通さない**（`scholia activity` が浅い clone で数を出さないのと同じ形）。
func TestAddCommit_NamesUnverifiedWhenOutsideGit(t *testing.T) {
	dir, drawn, _ := appliedFixture(t)
	out, err := run(t, dir, "decision", "add-commit", drawn, "aaa1111", "--kind", "implementation")
	if err != nil {
		t.Fatalf("add-commit: %v\n%s", err, out)
	}
	if !strings.Contains(out, "照合していません") {
		t.Fatalf("照合できなかったことを名乗っていない:\n%s", out)
	}
}
