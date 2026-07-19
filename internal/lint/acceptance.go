// acceptance.go — typed 容認（acknowledges）の共通機構（#45 D6）。
//
// 「意図的に残す finding」を機械可読に宣言する。decision.acknowledges は
// finding の rule id を*指名*し、消費側（lint/flow）は「当該 target 宛ての
// decision の acknowledges に該当 rule 名が含まれれば、その finding を『容認済み
// （decision リンク付き）』に畳む」。
//
// 容認を untyped（対象の祖先に decision があれば緑）にしないのは、無関係な
// decision による偽陰性——信じられるようになった lint が誤った緑を返す最悪の
// 誤りモード——を作らないため（D6 の核。祖先 decision では畳まない）。
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/flow"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// ValidRuleIDs は acknowledges で名指しできる有効な rule id 集合の source of
// truth（lint.Rules の名前＋flow finding の rule 名）。decide 時の実在照合
// （gate）と dangling-acknowledges lint が同じ集合を参照する。
func ValidRuleIDs() map[string]bool {
	out := make(map[string]bool, len(Rules)+3)
	for _, r := range Rules {
		out[r.Name] = true
	}
	for _, name := range flow.RuleNames() {
		out[name] = true
	}
	return out
}

// SortedValidRuleIDs は候補提示（typo 時のヒント）用に有効 rule id を辞書順で返す。
func SortedValidRuleIDs() []string {
	valid := ValidRuleIDs()
	out := make([]string, 0, len(valid))
	for name := range valid {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// acknowledgesByTarget は target（type+id）→ その target 宛ての decision が
// acknowledges で名指しした rule 名集合 → その rule を acknowledge した decision
// の id、を索引化する。容認畳みは「当該 target 宛ての direct decision」だけを
// 見る（祖先 decision では畳まない・D6）。
type acknowledgesByTarget map[targetKey]map[string]string

type targetKey struct {
	Type string
	ID   string
}

func indexAcknowledges(decisions []model.Decision) acknowledgesByTarget {
	out := make(acknowledgesByTarget)
	for _, d := range decisions {
		if len(d.Acknowledges) == 0 {
			continue
		}
		key := targetKey{Type: d.Target.Type, ID: d.Target.ID}
		m := out[key]
		if m == nil {
			m = make(map[string]string)
			out[key] = m
		}
		for _, rule := range d.Acknowledges {
			// 同一 rule を複数 decision が容認した場合、辞書順で最初の id を残す
			// （表示の決定性のみのため——どれでも「容認済み」になる事実は同じ）。
			if prev, ok := m[rule]; !ok || d.ID < prev {
				m[rule] = d.ID
			}
		}
	}
	return out
}

// acknowledgedBy は target 宛ての decision が rule を acknowledge していれば
// その decision id と true を返す（direct のみ・祖先は見ない）。
func (a acknowledgesByTarget) acknowledgedBy(targetType, targetID, rule string) (string, bool) {
	m, ok := a[targetKey{Type: targetType, ID: targetID}]
	if !ok {
		return "", false
	}
	id, ok := m[rule]
	return id, ok
}

// --- dangling-acknowledges: rule 改名で解決しなくなった acknowledges の警告 ---
//
// decision は append-only なので acknowledges を直せない（判断欄位ではないが
// 追記専用）＝是正不能。よって acknowledge-only 扱いの info で「要再確認」を出す
// のみ（fixable ではない）。有効 rule id 集合は ValidRuleIDs（lint.Rules 名＋
// flow rule 名）で、そこに無い acknowledges を宙吊りとして列挙する。

func checkDanglingAcknowledges(snap store.Snapshot) []Finding {
	valid := ValidRuleIDs()
	var out []Finding
	for _, d := range snap.Decisions {
		var dangling []string
		for _, rule := range d.Acknowledges {
			if !valid[rule] {
				dangling = append(dangling, rule)
			}
		}
		if len(dangling) == 0 {
			continue
		}
		sort.Strings(dangling)
		q := strings.Join(dangling, "・")
		out = append(out, Finding{
			Rule:            "dangling-acknowledges",
			Severity:        SeverityInfo,
			Tier:            TierAdvisory,
			Target:          d.ID,
			TargetType:      targetDecision,
			Field:           "acknowledges",
			Quote:           q,
			AcknowledgeOnly: true,
			Message: fmt.Sprintf("decision %s: acknowledges %q が有効な rule id に解決しません（rule 改名で宙吊り・decision は append-only なので是正不能・要再確認）",
				d.ID, q),
		})
	}
	return out
}
