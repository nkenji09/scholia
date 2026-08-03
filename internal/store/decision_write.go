// decision_write.go — decision の書き込み口を「新規作成」と「更新」に型で分ける
// （01KZ06SYR3APGF3JD4NQRFTEEN 変更3）。
//
// **なぜ口を 2 つに割るのか。**
// 保存時ゲートを「decision を作る面」ごとに配線する形は、この repo で繰り返し
// 抜けている（CLAUDE.md 5「新しく作った面には、ガードを置き忘れる」）。実際、
// 本 decision の直前まで、decision を新規に作る 3 面のうち保存前検査を通って
// いたのは `scholia decide` だけだった（`review adopt` と viewer は素通り）。
//
// だから面を列挙して塞ぐのではなく、**書き込みの口そのものを 2 つに割る**:
//
//	CreateDecision — その id のファイルが「まだ無いこと」を要求する
//	UpdateDecision — その id のファイルが「既に在ること」を要求する
//
// この形にすると、**新規作成の口を通らずに decision を作ることができない。**
// 4 面目を足した誰かが UpdateDecision で新規を作ろうとしても、実行時に落ちる。
// したがって歯止めは「新規作成の口が見出し無しを拒む」1 本で書ける——
// 面の数だけ検査を並べなくてよい（CLAUDE.md 1・3）。
//
// ⚠️ **この歯止めが落ちる範囲**（CLAUDE.md 6）:
// 「`store.Store` を経由して decision ファイルを作る」経路のすべてに落ちる。
// `.scholia/decisions/*.json` を store を通さず直接書く（エディタ・別ツール・
// `os.WriteFile`）ものには落ちない——保存時ゲートは保存の口に置く歯止めであり、
// ファイルシステムそのものは守れない。そこは `scholia lint` の領分である。
package store

import (
	"fmt"
	"os"

	"github.com/nkenji09/scholia/internal/model"
)

// DecisionCreateOptions は新規作成の口に渡す明示の逃し弁。
//
// **省略できない引数にしてある。** 既定値（逃し弁なし）を暗黙に選べる形だと、
// 呼ぶ側が「何も指定しない」を選んだのか「逃し弁を検討しなかった」のかが
// 呼び出し箇所から読めない。`DecisionCreateOptions{}` と書かせることで、
// 逃し弁を渡していないことが各面のソースに残る。
type DecisionCreateOptions struct {
	// AllowRules は保存時拒否規則を明示に破る指定（CLI の `--allow <rule>`）。
	// 理由の必須・記録は CLI 層（internal/cli/write_gate.go）が担う。
	AllowRules []string
}

func (o DecisionCreateOptions) allows(rule string) bool {
	for _, r := range o.AllowRules {
		if r == rule {
			return true
		}
	}
	return false
}

// RuleDecisionHeading は「why の 1 行目が見出しであること」の拒否規則 id。
// lint 側の拒否規則列挙（lint.GateRejectRuleNames）と同じ文字列を指す
// ——`--allow` に渡せる名前と、ここが見る名前が別物にならないようにする。
const RuleDecisionHeading = "decision-heading"

// DecisionRejectError は新規作成の口が保存を拒んだこと。
//
// Reason は「何を満たしていないか」だけを持つ（レコード id を含まない）。
// viewer は生 id を表示しない（01KYCC2TF3NW3JRSSRK9ZHN078）ので、Message を
// そのまま body に載せられない——Reason から文言を組み直せるようにしてある。
type DecisionRejectError struct {
	Rule    string
	Reason  string
	Message string
}

func (e *DecisionRejectError) Error() string { return e.Message }

// CreateDecision は decision を新規に 1 件作る**唯一の口**。
//
// その id のファイルが既に在れば落ちる（append-only の decision を、新規作成の
// 顔をして踏み潰すことはできない）。そのうえで保存時の拒否規則を当てる。
//
// 拒否規則（01KZ06SYR3APGF3JD4NQRFTEEN 変更1）:
//   - decision-heading — `why` の 1 行目が見出しでない（判定は
//     model.CheckDecisionHeading・3 条件）
//
// ⚠️ **既存 decision の更新には効かない**（UpdateDecision は通らない）。
// `why` は append-only で保存後に直せないので、遡って課す手段がそもそも無い。
func (s *Store) CreateDecision(d model.Decision, opts DecisionCreateOptions) error {
	path := s.decisionPath(d.ID)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("decision %s は既に存在します（新規作成の口では既存レコードを書き換えられません。更新は UpdateDecision）", d.ID)
	} else if !os.IsNotExist(err) {
		return &RecordWriteError{Category: "decision", Err: err}
	}

	if !opts.allows(RuleDecisionHeading) {
		if r := model.CheckDecisionHeading(d.Why); !r.OK {
			return &DecisionRejectError{
				Rule:   RuleDecisionHeading,
				Reason: r.Reason,
				Message: fmt.Sprintf(
					"decision %s: why の 1 行目を見出しにしてください（%s）。"+
						"形式は `# <1〜%d 字の見出し>` に続けて空行、そして本文。"+
						"why は append-only で保存後に直せないため、保存前に止めています",
					d.ID, r.Reason, model.DecisionHeadingMaxRunes),
			}
		}
	}

	return s.writeDecision(d)
}

// UpdateDecision は既存 decision を書き戻す口（`decision add-commit` の追記・
// `decision link` の結線・改名に伴う target 追随）。
//
// その id のファイルが無ければ落ちる——**この口から新規を作らせない**のが、
// CreateDecision を唯一の新規作成口に保つ仕掛けの片側である。
//
// 保存時の拒否規則は当てない。`why` を作っていない書き戻しだからで、
// 既存 173 件の書き方を遡って壊さないための境界でもある。
func (s *Store) UpdateDecision(d model.Decision) error {
	if _, err := os.Stat(s.decisionPath(d.ID)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("decision %s は存在しません（更新の口では新規に作れません。新規作成は CreateDecision）", d.ID)
		}
		return &RecordWriteError{Category: "decision", Err: err}
	}
	return s.writeDecision(d)
}
