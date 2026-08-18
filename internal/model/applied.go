// applied.go — 「記録が結論を決めた」出来事の印（applied[]・decision
// 01M09FHEQH7PVZ2BTKGXY5YMNN）のモデル層。
//
// scholia は「変更を過去の意図と突き合わせて評価する」道具である。突き合わせの
// 結果は5つに分かれる（是正・精緻化・矛盾・新規・却下）が、そのうち
// **是正・矛盾・却下の3つは「記録があったから結論が変わった」場合**であり、
// どれも記録の側には何も残らなかった。applied[] はその3つを1件1要素で残す。
//
// # ここに置く理由
//
// 印を書く口は2つある（`decision add-commit --kind correction` と
// `decision applied`）。面ごとに検査を書き分けると、どれか1つだけが緩む
// ——supersedes[] の検証が実際にそうなった（adopt が持ち上げる宣言だけが
// mode 3値と重複の検査を素通りしていた）。だから**要素が満たすべき不変条件は
// ここ1つ**に置き、各面はここを呼ぶ。
//
// # 検証は純関数（CLAUDE.md「配線ガードの書き方」1）
//
// ValidateAppliedMark / AppendAppliedMarks はどちらも入力と出力の対で検査できる
// ——git もファイルシステムも触らない。commit hash が**実在するか**は git を要る
// ので、ここではなく保存の口（store）で当たる。
package model

import "fmt"

// AppliedMark は applied[] の1要素。
//
// 種別・時刻・指し先の3つを持つ。**指し先は種別で決まる**——是正は commit
// hash、矛盾・却下は着地した decision の id、何も着地しなかった矛盾は指し先なし。
//
// ⚠️ **数を数えるフィールド（カウンタ）にはできない。** 「記録が正しかったので
// 何も変わらなかった矛盾」には payload が何も無いが、カウンタを増やすことは
// 既存の値の書き換えであり、追記専用にできない。だから payload が無くても
// 1件として置ける形にしてある。
//
// 全欄が文字列なので、この型は比較可能（== が書ける）。diff の欄位分類が
// 「既存要素の改変」を要素単位で見るのに使う。
type AppliedMark struct {
	// Kind は3値（AppliedCorrection / AppliedConflict / AppliedRejection）。
	Kind string `json:"kind"`
	// At は印を打った時刻（RFC3339）。**判定した時点ではなく、結論が着地した
	// 時点**を記録する——判定は覆りうるし、判定だけして着地しなかったものを
	// 数えると「検討した回数」になる。
	At string `json:"at"`
	// Commit は是正の指し先（この decision に書いてあるとおりに実装されて
	// いなかったのを直した commit の hash）。是正以外では空。
	Commit string `json:"commit,omitempty"`
	// Decision は矛盾・却下の指し先（着地した decision の id）。是正では空。
	// 何も着地しなかった矛盾でも空になる。
	Decision string `json:"decision,omitempty"`
}

// AppliedMark.Kind の3値。
const (
	// AppliedCorrection は是正——記録が正しく、実装のほうが違っていたので直した。
	// 着地するのは実装コミットで、.scholia/ は1バイトも変わらない（変われば
	// 是正ではなく精緻化である）。
	AppliedCorrection = "correction"
	// AppliedConflict は矛盾——記録と衝突したので止めた。改訂の decision が残る
	// こともあれば、「指摘のほうが誤りで記録が正しかった」として何も残らない
	// こともある。
	AppliedConflict = "conflict"
	// AppliedRejection は却下——記録が既に決めていたので採らなかった。却下を
	// 記録する decision が残る。
	AppliedRejection = "rejection"
)

// AppliedKinds は3値の一覧（CLI のフラグ説明・分類表がここから引く）。
// **名前を書き写さない**——書き写すと種別を足したときに黙ってずれる。
func AppliedKinds() []string {
	return []string{AppliedCorrection, AppliedConflict, AppliedRejection}
}

// ValidAppliedKind は種別が3値のいずれかかを返す。
func ValidAppliedKind(kind string) bool {
	for _, k := range AppliedKinds() {
		if k == kind {
			return true
		}
	}
	return false
}

// AppliedError は applied[] の検証違反。SupersedeError と同型に、文字列ではなく
// 型で返す——面ごとに見せ方が違う（CLI は id を含む文言を出すが、viewer は生の
// レコード id を表示しない・01KYCC2TF3NW3JRSSRK9ZHN078）。
type AppliedError struct {
	Kind string // 違反の種類（下の AppliedErr* ）
	Mark AppliedMark
}

// AppliedError.Kind の値。
const (
	AppliedErrInvalidKind      = "invalid-kind"
	AppliedErrMissingAt        = "missing-at"
	AppliedErrMissingCommit    = "missing-commit"
	AppliedErrUnexpectedCommit = "unexpected-commit"
	AppliedErrMissingDecision  = "missing-decision"
	AppliedErrUnexpectedTarget = "unexpected-decision"
	AppliedErrSelfReference    = "self-reference"
	AppliedErrMissingTarget    = "missing-target"
)

func (e *AppliedError) Error() string {
	switch e.Kind {
	case AppliedErrInvalidKind:
		return fmt.Sprintf("applied: 種別 %q は %s のいずれかである必要があります",
			e.Mark.Kind, joinKinds())
	case AppliedErrMissingAt:
		return "applied: 時刻（at）が空です"
	case AppliedErrMissingCommit:
		return "applied: 是正（correction）には直した commit の hash が要ります"
	case AppliedErrUnexpectedCommit:
		return fmt.Sprintf("applied: 種別 %q に commit は書けません（指し先は着地した decision の id です）", e.Mark.Kind)
	case AppliedErrMissingDecision:
		return "applied: 却下（rejection）には、却下を記録した decision の id が要ります（却下は必ず decision を1件残すため）"
	case AppliedErrUnexpectedTarget:
		return "applied: 是正（correction）に decision の指し先は書けません（指し先は直した commit の hash です）"
	case AppliedErrSelfReference:
		return fmt.Sprintf("applied: decision は自分自身（%s）を指せません", e.Mark.Decision)
	case AppliedErrMissingTarget:
		return fmt.Sprintf("applied: 指し先の decision %q が実在しません", e.Mark.Decision)
	}
	return "applied: 印が不正です"
}

func joinKinds() string {
	out := ""
	for i, k := range AppliedKinds() {
		if i > 0 {
			out += "|"
		}
		out += k
	}
	return out
}

// ValidateAppliedMark は1件の印が満たすべき不変条件を検査する純関数。
//
// selfID が空でないときは自己参照も弾く（引かれた decision 自身を指し先には
// できない——「この記録が結論を決めた」の指し先は、その結論が着地した先である）。
func ValidateAppliedMark(m AppliedMark, selfID string) error {
	if !ValidAppliedKind(m.Kind) {
		return &AppliedError{Kind: AppliedErrInvalidKind, Mark: m}
	}
	if m.At == "" {
		return &AppliedError{Kind: AppliedErrMissingAt, Mark: m}
	}
	switch m.Kind {
	case AppliedCorrection:
		if m.Commit == "" {
			return &AppliedError{Kind: AppliedErrMissingCommit, Mark: m}
		}
		if m.Decision != "" {
			return &AppliedError{Kind: AppliedErrUnexpectedTarget, Mark: m}
		}
	case AppliedConflict:
		// 矛盾だけは指し先なしを許す（記録が正しかったので何も変わらなかった場合）。
		if m.Commit != "" {
			return &AppliedError{Kind: AppliedErrUnexpectedCommit, Mark: m}
		}
	case AppliedRejection:
		if m.Commit != "" {
			return &AppliedError{Kind: AppliedErrUnexpectedCommit, Mark: m}
		}
		if m.Decision == "" {
			return &AppliedError{Kind: AppliedErrMissingDecision, Mark: m}
		}
	}
	if selfID != "" && m.Decision == selfID {
		return &AppliedError{Kind: AppliedErrSelfReference, Mark: m}
	}
	return nil
}

// ValidateAppliedTargets は指し先の decision が実在するかを検査する
// （ValidateSupersedeTargets と同型）。指し先を持たない印は素通りする。
func ValidateAppliedTargets(all []Decision, marks []AppliedMark) error {
	if len(marks) == 0 {
		return nil
	}
	exists := make(map[string]bool, len(all))
	for _, d := range all {
		exists[d.ID] = true
	}
	for _, m := range marks {
		if m.Decision == "" {
			continue
		}
		if !exists[m.Decision] {
			return &AppliedError{Kind: AppliedErrMissingTarget, Mark: m}
		}
	}
	return nil
}

// AppendAppliedMarks は existing に candidates を追記し、追加分のみ返す純関数。
//
// **重複を畳む鍵は指し先である。** 同じ commit を2度是正と印した場合、同じ
// decision を2度却下と印した場合は冪等に skip する。
//
// ⚠️ **指し先を持たない矛盾は畳めない。** 「記録が正しかったので何も変わら
// なかった矛盾」には畳む鍵が無いので、同じ出来事に2回打てば2件になる。
// これは検出できない——正直に名乗る（決定本文「落とせない」節）。
func AppendAppliedMarks(existing, candidates []AppliedMark) (added []AppliedMark) {
	seen := make(map[string]bool, len(existing)+len(candidates))
	for _, m := range existing {
		if k, ok := appliedDedupeKey(m); ok {
			seen[k] = true
		}
	}
	for _, c := range candidates {
		k, hasKey := appliedDedupeKey(c)
		if hasKey && seen[k] {
			continue // 冪等 skip
		}
		if hasKey {
			seen[k] = true
		}
		added = append(added, c)
	}
	return added
}

// appliedDedupeKey は「同じ出来事」を見分ける鍵。指し先が無ければ鍵は作れない。
func appliedDedupeKey(m AppliedMark) (string, bool) {
	switch {
	case m.Commit != "":
		return m.Kind + "\x00commit\x00" + m.Commit, true
	case m.Decision != "":
		return m.Kind + "\x00decision\x00" + m.Decision, true
	}
	return "", false
}

// CountApplied は marks のうち種別が kind のものを数える純関数（消費側が
// 「印が何件付いているか」を数えるのに使う。集計のためのサブコマンド・フラグは
// 足さない——数えるのは `decision list --json` である）。
func CountApplied(marks []AppliedMark, kind string) int {
	n := 0
	for _, m := range marks {
		if m.Kind == kind {
			n++
		}
	}
	return n
}
