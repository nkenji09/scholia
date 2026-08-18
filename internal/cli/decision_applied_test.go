package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/gittest"
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

// ---------------------------------------------------------------------------
// 短縮 hash と完全 hash（クリーンルームレビュー 指摘1）
// ---------------------------------------------------------------------------

// gitBackedAppliedFixture は **git 管理下**の標本を作り、store の dir と
// decision id、HEAD の commit hash を返す。
func gitBackedAppliedFixture(t *testing.T) (dir, drawn, head string) {
	t.Helper()
	dir = t.TempDir()
	gittest.InitRepo(t, dir)
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern"); err != nil {
		t.Fatalf("tag create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "add", "-A")
	gittest.Run(t, dir, "commit", "-q", "-m", "seed")
	head = strings.TrimSpace(gittest.Run(t, dir, "rev-parse", "HEAD"))
	drawn = decideID(t, dir, "# 引かれる側の見出し\n\n過去に決めたこと。")
	return dir, drawn, head
}

// 🔴 **同じ commit を完全 hash と短縮 hash で是正と印しても、1件のまま。**
//
// レビュアの再現手順そのもの。直す前は `commits[]` も `applied[]` も2件になり、
// **この単位が存在する理由そのもの——是正の件数——が上振れした。**
// ⚠️ `applied[]` は追記専用なので、打ってしまうと消せない。
func TestAddCommit_ShortAndFullHashCountAsOne(t *testing.T) {
	dir, drawn, head := gitBackedAppliedFixture(t)

	if out, err := run(t, dir, "decision", "add-commit", drawn, head, "--kind", "correction"); err != nil {
		t.Fatalf("完全 hash: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "decision", "add-commit", drawn, head[:8], "--kind", "correction"); err != nil {
		t.Fatalf("短縮 hash: %v\n%s", err, out)
	}

	marks := loadApplied(t, dir, drawn)
	if n := model.CountApplied(marks, model.AppliedCorrection); n != 1 {
		t.Fatalf("同じ commit の是正は1件のはず（上振れ）: %d 件 %+v", n, marks)
	}
	if marks[0].Commit != head {
		t.Errorf("保存される値は完全 hash に寄るはず: %q", marks[0].Commit)
	}
	if got := loadCommits(t, dir, drawn); len(got) != 1 || got[0] != head {
		t.Errorf("commits[] も1件（完全 hash）のはず: %v", got)
	}

	// 逆順（短縮を先に打った decision に完全 hash を足す）でも1件のまま。
	other := decideID(t, dir, "# もう1つの見出し\n\n別の判断。")
	if out, err := run(t, dir, "decision", "add-commit", other, head[:8], "--kind", "implementation"); err != nil {
		t.Fatalf("短縮 hash: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "decision", "add-commit", other, head, "--kind", "implementation"); err != nil {
		t.Fatalf("完全 hash: %v\n%s", err, out)
	}
	if got := loadCommits(t, dir, other); len(got) != 1 {
		t.Fatalf("逆順でも1件のはず: %v", got)
	}
}

// `decide --commit` も同じ口を通るので、短縮 hash は完全 hash に寄って保存される。
func TestDecide_ShortHashIsStoredCanonical(t *testing.T) {
	dir, _, head := gitBackedAppliedFixture(t)
	id := decideIDWith(t, dir, "# 短縮で結ぶ\n\n本文。", "--commit", head[:7])
	if got := loadCommits(t, dir, id); len(got) != 1 || got[0] != head {
		t.Fatalf("保存される値は完全 hash に寄るはず: %v", got)
	}
}

// loadCommits は `decision list --json` から commits[] を読む。
func loadCommits(t *testing.T, dir, id string) []string {
	t.Helper()
	out, err := run(t, dir, "decision", "list", "--json")
	if err != nil {
		t.Fatalf("decision list --json: %v\n%s", err, out)
	}
	var resp struct {
		Decisions []struct {
			ID      string   `json:"id"`
			Commits []string `json:"commits"`
		} `json:"decisions"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	for _, d := range resp.Decisions {
		if d.ID == id {
			return d.Commits
		}
	}
	t.Fatalf("decision %s が一覧に無い", id)
	return nil
}

func decideIDWith(t *testing.T, dir, why string, extra ...string) string {
	t.Helper()
	args := append([]string{"decide", "--on", "tag:t1", "--why", why}, extra...)
	out, err := run(t, dir, append(args, "--json")...)
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

// ---------------------------------------------------------------------------
// 「照合していない」の名乗り（クリーンルームレビュー 指摘2）
// ---------------------------------------------------------------------------

// 🔴 **名乗りはテキストの面にも `--json` の面にも出る。**
//
// git が使えないとき、この道具は弾かず・素通りせず・名乗ると決めた。だが名乗りを
// テキスト出力にだけ書いていた間、**`--json` の面では名乗りがどこにも出なかった**
// ——配布スキルは AI に応答封筒を読ませる作りなので、AI は「保存された＝検査が
// 走った」と読む。**自分で採らないと決めた「素通り」が `--json` でだけ起きていた。**
func TestCommitVerifyNoticeAppearsOnBothFaces(t *testing.T) {
	dir, drawn, _ := appliedFixture(t) // git 管理下ではない標本

	t.Run("テキスト", func(t *testing.T) {
		out, err := run(t, dir, "decision", "add-commit", drawn, "aaa1111", "--kind", "implementation")
		if err != nil {
			t.Fatalf("add-commit: %v\n%s", err, out)
		}
		if !strings.Contains(out, "照合していません") {
			t.Fatalf("テキストの面に名乗りが無い:\n%s", out)
		}
	})

	t.Run("--json", func(t *testing.T) {
		out, err := run(t, dir, "decision", "add-commit", drawn, "bbb2222", "--kind", "implementation", "--json")
		if err != nil {
			t.Fatalf("add-commit --json: %v\n%s", err, out)
		}
		var env struct {
			Advisories []struct {
				Rule    string `json:"rule"`
				Message string `json:"message"`
			} `json:"advisories"`
		}
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("unmarshal: %v\n%s", err, out)
		}
		found := false
		for _, a := range env.Advisories {
			if a.Rule == RuleCommitUnverified {
				found = true
				if !strings.Contains(a.Message, "照合していません") {
					t.Errorf("文言が読めない: %q", a.Message)
				}
			}
		}
		if !found {
			t.Fatalf("`--json` の封筒に %q の advisory が無い（素通りと見分けがつかない）:\n%s",
				RuleCommitUnverified, out)
		}
	})

	t.Run("decide --commit も同じ", func(t *testing.T) {
		out, err := run(t, dir, "decide", "--on", "tag:t1",
			"--why", "# 名乗りの見出し\n\n本文。", "--commit", "ccc3333", "--json")
		if err != nil {
			t.Fatalf("decide --json: %v\n%s", err, out)
		}
		if !strings.Contains(out, RuleCommitUnverified) {
			t.Fatalf("decide の `--json` にも名乗りが要る:\n%s", out)
		}
	})
}

// git 管理下では名乗らない（照合できたのだから、言うことは無い）。
func TestCommitVerifyNoticeSilentUnderGit(t *testing.T) {
	dir, drawn, head := gitBackedAppliedFixture(t)
	out, err := run(t, dir, "decision", "add-commit", drawn, head, "--kind", "implementation", "--json")
	if err != nil {
		t.Fatalf("add-commit --json: %v\n%s", err, out)
	}
	if strings.Contains(out, RuleCommitUnverified) {
		t.Fatalf("照合できたのに名乗っている（狼少年になる）:\n%s", out)
	}
}
