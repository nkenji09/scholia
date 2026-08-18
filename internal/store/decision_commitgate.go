// decision_commitgate.go — 「結ぶ commit が実在すること」「印（applied[]）の形が
// 正しいこと」を**保存の口**で当てる（decision 01M09FHEQH7PVZ2BTKGXY5YMNN
// 変更5・「落とせる範囲」②）。
//
// # なぜ面ではなく口に置くのか
//
// 正本は「`add-commit` と `decide --commit` に入れる」と書いている。だが
// commits[] を書ける面はその2つではない——viewer の `POST /api/decision` も
// commits を受け取る。**面を数えて配線する形は、この repo が繰り返し落として
// きた型**である（CLAUDE.md 5「新しく作った面には、ガードを置き忘れる」・
// 実際 decision-heading の保存時ゲートも、面ごとに配線していた時期は3面のうち
// 1面にしか効いていなかった）。
//
// だから 01KZ06SYR3APGF3JD4NQRFTEEN 変更3 が割った2つの口（CreateDecision /
// UpdateDecision）にそのまま置く。**4面目を足した誰かが照合を書き忘れても落ちる。**
// 口が2つしかないことは TestNoThirdDecisionWritePort が別途守っている。
//
// # 何に対して落ちるのか（CLAUDE.md 6）
//
// **落ちる:**
//   - store を通して decision を保存するどの経路でも、**新しく足された**
//     commit hash が形不正・実在しないなら落ちる（CLI・viewer・将来の面を問わない）。
//   - 同じく、新しく足された applied[] の印が3値でない・時刻が空・種別と指し先の
//     組み合わせが噛み合っていないなら落ちる。
//
// **落ちない（射程の外・正直に名乗る）:**
//   - 🔴 **既に保存されている commit hash と印。** 更新の口では「増えた分」しか
//     見ない。既存 187 件の decision には形の合わない値が入っていることがあり、
//     改名の追随（target 張替え）でそれを再検査すると、無関係の操作が落ちる。
//   - git が無い／git 管理下でないときの**実在**（形の検査だけが残る）。
//     照合していないことは呼んだ側が名乗る（commitcheck の package doc）。
//   - `.scholia/decisions/*.json` を store を通さず直接書く経路。
package store

import (
	"path/filepath"
	"reflect"

	"github.com/nkenji09/scholia/internal/commitcheck"
	"github.com/nkenji09/scholia/internal/model"
)

// projectRoot は .scholia の親（commitcheck が git リポジトリを解決する起点）。
func (s *Store) projectRoot() string { return filepath.Dir(s.Dir) }

// CommitRepo は「この store が git 管理下にあるか」を解決した照合相手を返す。
// 面は Managed() を見て「実在は照合していない」と名乗るかどうかを決める。
func (s *Store) CommitRepo() commitcheck.Repo {
	return commitcheck.Open(s.projectRoot())
}

// checkDecisionAdditions は「今回の保存で新しく足された」commit hash と印だけを
// 検査し、**通ったものを完全 hash へ寄せる**（正規化）。prev が nil のときは
// 全件が新規（＝新規作成）。
//
// ⚠️ **d をポインタで受けるのは、保存する値をここで書き換えるからである。**
// 保存ゲートは 16 進 7〜64 文字を通すので短縮 hash も正当な入力だが、同じ
// 1 commit が短縮と完全で2つの文字列として保存されると、**完全一致で畳む
// 仕組み（applied[] の重複判定）が効かない**——実測で是正が2件に上振れした。
// 正規化も検査と同じく**口**に置く: 面ごとに書くと、新しい面が忘れる。
func (s *Store) checkDecisionAdditions(d *model.Decision, prev *model.Decision) error {
	var prevCommits []string
	var prevMarks []model.AppliedMark
	if prev != nil {
		prevCommits = prev.Commits
		prevMarks = prev.Applied
	}

	newMarks := addedMarks(prevMarks, d.Applied)
	for _, m := range newMarks {
		if err := model.ValidateAppliedMark(m, d.ID); err != nil {
			return err
		}
	}

	hashes := addedStrings(prevCommits, d.Commits)
	for _, m := range newMarks {
		if m.Commit != "" {
			hashes = append(hashes, m.Commit)
		}
	}

	// ⚠️ **「新しい hash がゼロなら何もしない」で早期に抜けてはいけない。**
	// 同じ値が2回並ぶ形（既に保存済みの完全 hash を、もう一度足す呼び出し）は
	// 新規 hash ゼロだが**要素は増えている**——ここで抜けると正規化に届かず、
	// 同じ commit が2件のまま保存された（実測）。抜けてよいのは
	// **来歴も印も1バイトも変わっていないとき**だけである（改名の追随がこれ）。
	if len(hashes) == 0 &&
		reflect.DeepEqual(prevCommits, d.Commits) &&
		reflect.DeepEqual(prevMarks, d.Applied) {
		return nil
	}

	repo := s.CommitRepo()
	if len(hashes) > 0 {
		if err := repo.Check(hashes); err != nil {
			return err
		}
	}
	// ここまで来た＝止めるべきものは無い。**通った値だけを寄せる。**
	// 解決できない値（git 管理外・git 不在）は canon が "" を返すので、
	// 渡されたままの値が残る（「照合していない」と名乗る領域）。
	canon := model.Canonicalizer(repo.Canonical)
	d.Commits = model.NormalizeCommits(prevCommits, d.Commits, canon)
	d.Applied = model.NormalizeAppliedMarks(prevMarks, d.Applied, canon)
	return nil
}

// addedStrings は after のうち before に無い値を返す（順序保存）。
func addedStrings(before, after []string) []string {
	had := make(map[string]bool, len(before))
	for _, v := range before {
		had[v] = true
	}
	var out []string
	for _, v := range after {
		if !had[v] {
			out = append(out, v)
		}
	}
	return out
}

// addedMarks は after のうち before に無い印を返す（全欄一致で見る・順序保存）。
func addedMarks(before, after []model.AppliedMark) []model.AppliedMark {
	had := make(map[model.AppliedMark]bool, len(before))
	for _, m := range before {
		had[m] = true
	}
	var out []model.AppliedMark
	for _, m := range after {
		if !had[m] {
			out = append(out, m)
		}
	}
	return out
}
