// gate.go — 書き込みゲート二層の検査コア（#45 U3/P3）。
//
// lint の検査を「これから保存する 1 レコード限定のスコープ」で呼べる形に
// 分離する。全書き込みコマンド（tx add/edit・tag create/edit・vocab
// add/edit/tag・decide・decision add-commit）と viewer POST /api/transition
// が保存前にこれを呼ぶ。
//
// 【reject（保存せず exit 1）】store を自己矛盾させる機械検証可能な不変条件
// のみの 3 件（決定③・拒否規則の恣意的増殖はしない）:
//
//	(a) exclusive-violation — 同一 axis kind タグの2値 condition を同一 given
//	    に名指しする transition（全軸対象・total 限定にしない。既存 lint 規則
//	    と同一 id＝同一検査コアの証。軸排他の実世界の真偽は検査しない）
//	(b) total-kind-mismatch — axis 挙動を持たない kind のタグへの total=true
//	(c) id-policy — config.idPolicy 宣言に反する新規 id（新規のみ・既存 id の
//	    edit は対象外）
//
// 逃し弁 --allow（理由必須・記録）は CLI 層（internal/cli）が担う。
//
// 【advisory（保存する・同一ターン警告）】curated set＝U2 の advisory 規則の
// うち 1,2,3,4,6,8（derived-value-in-desc・stale-tense・prose-ref・
// why-file-line・duplicate-atom・dead-doc-ref）＋desc 長。
// 含めないもの（恒常発火の advisory 疲れ＝「無視してよいもの」学習の再生産を
// 避ける・検証済みの罠）: axis-without-decision（tag create 直後は正規フロー上
// 常に未充足）・dangling-id・overlap／decision-coverage 系。
package lint

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// WriteOp は保存しようとしている 1 レコード。ちょうど 1 つのポインタを
// 設定する。IsNew は「新規 id か」（id-policy は新規のみ検査）。
type WriteOp struct {
	Transition *model.Transition
	Tag        *model.Tag
	Vocab      *model.VocabEntry
	Decision   *model.Decision
	IsNew      bool
}

// GateResult は書き込みゲート二層の検査結果。Rejections が 1 件でもあれば
// 保存しない（--allow で明示に破った場合を除く）。Advisories は保存した上で
// 同一ターンに警告する。
type GateResult struct {
	Rejections []Finding
	Advisories []Finding
}

// reject 規則の rule id（決定③＋#45 D6 の unknown-acknowledges）。
const (
	GateExclusiveViolation = "exclusive-violation"
	GateTotalKindMismatch  = "total-kind-mismatch"
	GateIDPolicy           = "id-policy"
	// GateUnknownAcknowledges（#45 D6）: 新規 decision の acknowledges が有効な
	// rule id に解決しない（typo）＝保存前に弾く。id-policy 同様、新規レコードの
	// 書き込み時のみ検査する（既存 decision の再検査はしない——rule 改名で後から
	// 宙吊りになるのは lint dangling-acknowledges の領分）。
	GateUnknownAcknowledges = "unknown-acknowledges"
)

// GateRejectRuleNames は reject 規則 id の全列挙（--allow の検証用）。
func GateRejectRuleNames() []string {
	return []string{GateExclusiveViolation, GateTotalKindMismatch, GateIDPolicy, GateUnknownAcknowledges}
}

// DescLengthThreshold は write-time advisory「desc 長」の閾値（字数・rune）。
// 定数にして変更を容易にする（U3）。長文契約 desc（実測 562 字）を検出しつつ
// ユーザーストーリー形式の tag desc は対象外（vocab.description のみ検査）。
const DescLengthThreshold = 300

// writeAdvisoryRuleNames は write-time advisory の curated set（U3。
// lint.Rules に登録済みの advisory 規則から選別）。desc-length は write-time
// 専用検査（下記 descLengthFindings）でこの列挙の外。
var writeAdvisoryRuleNames = []string{
	"derived-value-in-desc",
	"stale-tense",
	"prose-ref",
	"why-file-line",
	"duplicate-atom",
	"dead-doc-ref",
}

// CheckWrite は保存直前のレコード 1 件を二層で検査する。snap は保存前の
// snapshot（呼び出し側の LoadAll の結果）で、op を適用した後の状態に対して
// 検査し、findings は対象レコード限定に絞る。
func CheckWrite(snap store.Snapshot, op WriteOp) GateResult {
	after := applyWrite(snap, op)
	return GateResult{
		Rejections: checkRejections(after, op),
		Advisories: checkWriteAdvisories(after, op),
	}
}

// applyWrite は op を snapshot の複製へ適用する（同一 id は置換・無ければ
// 追加）。スライスヘッダのみ複製すれば十分（レコードは値型）。
func applyWrite(snap store.Snapshot, op WriteOp) store.Snapshot {
	after := snap
	switch {
	case op.Transition != nil:
		after.Transitions = replaceOrAppendByID(snap.Transitions, *op.Transition, model.Transition.GetID)
	case op.Tag != nil:
		after.Tags = replaceOrAppendByID(snap.Tags, *op.Tag, model.Tag.GetID)
	case op.Vocab != nil:
		after.Vocab = replaceOrAppendByID(snap.Vocab, *op.Vocab, model.VocabEntry.GetID)
	case op.Decision != nil:
		after.Decisions = replaceOrAppendByID(snap.Decisions, *op.Decision, model.Decision.GetID)
	}
	return after
}

func replaceOrAppendByID[T any](records []T, candidate T, getID func(T) string) []T {
	out := make([]T, len(records), len(records)+1)
	copy(out, records)
	id := getID(candidate)
	for i, r := range out {
		if getID(r) == id {
			out[i] = candidate
			return out
		}
	}
	return append(out, candidate)
}

// opTarget は op の対象 id と TargetType（rules_authoring.go の定数）を返す。
func opTarget(op WriteOp) (id, targetType string) {
	switch {
	case op.Transition != nil:
		return op.Transition.ID, targetTransition
	case op.Tag != nil:
		return op.Tag.ID, targetTag
	case op.Vocab != nil:
		return op.Vocab.ID, targetVocab
	case op.Decision != nil:
		return op.Decision.ID, targetDecision
	}
	return "", ""
}

// --- reject 層 ---

func checkRejections(after store.Snapshot, op WriteOp) []Finding {
	var out []Finding
	switch {
	case op.Transition != nil:
		// (a) exclusive-violation: 既存 lint 規則と同一の検査コア（全軸対象）
		// を対象 transition 限定で呼ぶ。gate では保存を止める違反なので
		// severity は error に引き上げる。
		for _, f := range transitionExclusiveViolations(axisValueTags(after), *op.Transition) {
			f.Severity = SeverityError
			f.TargetType = targetTransition
			out = append(out, f)
		}
	case op.Tag != nil:
		// (b) total-kind-mismatch: total=true は axis 挙動を持つ kind のタグにしか
		// 意味を持たない。#45 D9 で literal "axis" 判定を config の behaviors 宣言
		// 読取（KindHasBehavior）へ移行——別名 kind の axis 化を宣言だけで許す。
		if op.Tag.Total && !after.Config.KindHasBehavior(op.Tag.Kind, model.BehaviorAxis) {
			out = append(out, Finding{
				Rule: GateTotalKindMismatch, Severity: SeverityError,
				Target: op.Tag.ID, TargetType: targetTag,
				Message: fmt.Sprintf("tag %s: total=true は kind=axis のタグにのみ宣言できます（実際の kind %q・axis 挙動を持たない kind への --total）", op.Tag.ID, op.Tag.Kind),
			})
		}
	}
	// (c) id-policy: 新規 id のみ（既存 id の edit は対象外）。
	if op.IsNew {
		out = append(out, idPolicyViolations(after.Config, op)...)
		// (d) unknown-acknowledges（#45 D6）: 新規 decision の acknowledges が
		// 有効な rule id に解決しないなら弾く（typo は同一ターン error＋候補提示）。
		// id-policy 同様「新規のみ」——add-commit（IsNew=false）や既存 decision の
		// 再検査はしない（append-only なので後付けの宙吊りは lint が拾う）。
		if op.Decision != nil {
			out = append(out, unknownAcknowledgesViolations(*op.Decision)...)
		}
	}
	return out
}

// unknownAcknowledgesViolations は decision.acknowledges の各 rule id が有効な
// rule id 集合（lint.Rules 名＋flow rule 名）に実在するかを照合し、未知なら
// reject を返す（typo として候補提示）。
func unknownAcknowledgesViolations(d model.Decision) []Finding {
	if len(d.Acknowledges) == 0 {
		return nil
	}
	valid := ValidRuleIDs()
	var out []Finding
	for _, rule := range d.Acknowledges {
		if valid[rule] {
			continue
		}
		out = append(out, Finding{
			Rule: GateUnknownAcknowledges, Severity: SeverityError,
			Target: d.ID, TargetType: targetDecision,
			Field: "acknowledges", Quote: rule,
			Message: fmt.Sprintf("decision %s: acknowledges %q は有効な rule id ではありません（有効: %s）",
				d.ID, rule, strings.Join(SortedValidRuleIDs(), " / ")),
		})
	}
	return out
}

// idPolicyViolations は config.idPolicy の宣言に反する新規 id を返す。
// 宣言が無いスロット（nil・空文字）は何も強制しない（additive・U2）。
func idPolicyViolations(cfg model.Config, op WriteOp) []Finding {
	pol := cfg.IDPolicy
	if pol == nil {
		return nil
	}
	check := func(targetType, id, prefix, declPath string) []Finding {
		if prefix == "" || strings.HasPrefix(id, prefix) {
			return nil
		}
		return []Finding{{
			Rule: GateIDPolicy, Severity: SeverityError,
			Target: id, TargetType: targetType,
			Message: fmt.Sprintf("%s %s: 新規 id が config.idPolicy%s の宣言 prefix %q に従っていません（新規のみ検査・既存 id の edit は対象外）", targetType, id, declPath, prefix),
		}}
	}
	switch {
	case op.Transition != nil:
		return check(targetTransition, op.Transition.ID, pol.Transition, ".transition")
	case op.Vocab != nil:
		return check(targetVocab, op.Vocab.ID, pol.Vocab[op.Vocab.Category], ".vocab."+op.Vocab.Category)
	case op.Tag != nil:
		if op.Tag.Kind == "" {
			return nil
		}
		return check(targetTag, op.Tag.ID, pol.TagByKind[op.Tag.Kind], ".tagByKind."+op.Tag.Kind)
	}
	return nil
}

// --- advisory 層 ---

func checkWriteAdvisories(after store.Snapshot, op WriteOp) []Finding {
	var out []Finding
	for _, name := range writeAdvisoryRuleNames {
		r, ok := ruleByName(name)
		if !ok {
			continue
		}
		for _, f := range r.Check(after) {
			if writeFindingTargetsOp(f, op) {
				out = append(out, f)
			}
		}
	}
	if op.Vocab != nil {
		out = append(out, descLengthFindings(*op.Vocab)...)
	}
	return out
}

func ruleByName(name string) (Rule, bool) {
	for _, r := range Rules {
		if r.Name == name {
			return r, true
		}
	}
	return Rule{}, false
}

// writeFindingTargetsOp は finding が保存対象レコード自体を指すかを返す
// （対象限定スコープ）。duplicate-atom はグループ代表（辞書順先頭）を target
// にするため、保存対象がグループの一員（Quote＝構成 id の「・」連結）なら
// 該当とする。保存対象以外への波及 finding は write-time では通知しない
// （全量走査の lint/retrofit の領分）。
func writeFindingTargetsOp(f Finding, op WriteOp) bool {
	id, targetType := opTarget(op)
	if f.TargetType != targetType {
		return false
	}
	if f.Target == id {
		return true
	}
	if f.Rule == "duplicate-atom" && targetType == targetTransition {
		return contains(strings.Split(f.Quote, "・"), id)
	}
	return false
}

// descLengthFindings は write-time advisory「desc 長」（U3 curated set の
// ＋1）。vocab.description のみ対象（tag.desc はユーザーストーリー形式が
// 正当）・閾値は DescLengthThreshold（rune 数）。
func descLengthFindings(v model.VocabEntry) []Finding {
	n := utf8.RuneCountInString(v.Description)
	if n <= DescLengthThreshold {
		return nil
	}
	return []Finding{advisory("desc-length", targetVocab, v.ID, "description", "",
		"desc は要約へ痩せさせ、長文の契約・仕様本文は repo 内 versioned 文書へ置いて参照する",
		"vocab %s: description が %d 字あります（閾値 %d 字）——長文 desc は読みを重くし腐りやすい", v.ID, n, DescLengthThreshold)}
}
