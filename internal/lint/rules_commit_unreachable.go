// rules_commit_unreachable.go — commit-unreachable（info・decision
// 01M1JY0APWXHFZ1TKWST7VPS9N）。
//
// `commits[]` は「この判断をどの変更で実装したか」を後から辿るための欄だが、
// **squash merge のリポジトリでは、PR を取り込んだ瞬間に記録したハッシュが
// 取り込み先の祖先でなくなる。** 手元にはブランチのオブジェクトが残るので
// `git cat-file` は通り、保存の口（internal/commitcheck）も「実在する」と答える
// ——**新しく clone した人には存在しない**のに、である。
// 作業ブランチの作り直し（rebase）でも機序は違うが結果は同じで、実測では
// この repo 自身のユニークハッシュ 234 件のうち 91 件が HEAD から辿れなかった。
//
// # 何のときに出すのか（01M1K4WPN3HXNVE0CR0T6NQK33 で条件が1つ増えた）
//
// 出すのは「**commits がどれも辿れず、かつ `refs` も空**」のときだけである。
// 来歴の正本は `refs[]`（URL・取り込みで壊れない）へ移り、`commits[]` は非推奨に
// なった——**壊れない辿り先が別にあるなら、切れた hash が残っていること自体は
// 実害ではない。** `commits[]` は追記専用で消せないので、ここで黙らないと
// 「refs へ移したのに永久に鳴り続ける」finding になる。移行が進むほど静かになる。
//
// 残るのは「この判断が、どの変更で実装されたのかを辿る手立てが1つも無い」場合で、
// これは非推奨のあとも意味を持つ。
//
// # なぜ「1つも辿れない」で判定するのか
//
// `commits[]` は追記専用で、**切れたハッシュは永久に消せない**
// （01KXS4BB6B8FNFXQV9PEDK336S）。だから「1件でも辿れなければ出す」形にすると、
// 着地ハッシュを足しても finding が消えない——消す手段が `acknowledges` だけに
// なり、それも追記専用で永久に残る。**直せない finding を作らないために、
// 「辿れるものが1つも無い」でだけ出す。** 着地ハッシュを1つ足せば消える。
//
// # 落ちない範囲（射程・正直に名乗る）
//
//   - 🔴 **手元にオブジェクトが残っていなければ、この finding が出ても救済できない。**
//     新しく clone した後・GC の後は、元の commit の見出しすら読めない
//     （`scholia decision relink-commits` が候補を探せなくなる）。**この規則の本体は
//     「切れてから直す」ではなく「切れたその場で気づく」ことである。**
//   - **浅い clone では何も出さない。** `rev-list` が浅い境界で切れるため、境界より
//     古い祖先が「辿れない」に見える——**黙るのは、偽の finding を出さないためである。**
//     ⚠️ したがって浅い clone では「異常なし」と「検査していない」が区別できない。
//   - **git が無い／git 管理下でない／commit が1件も無いときは黙る**
//     （decision-stale と同型・01M09FHDJCV2WWFC7Z8331B0YQ / 01M0APXCFF70MBZCQT98MNQMW8）。
//     **導出そのものが落ちたときだけ git-derivation-failed で名乗る**
//     （01M0AJDYJSEVCSYEV0HDPSTWFZ の4段と同型）。
//   - **ストアとコードが別リポジトリのとき**、ハッシュはこの clone で解決しない。
//     全件が finding になるが、それは正しい——手元では辿れないからである
//     （保存時点の照合は commit-unverified が扱う）。
//   - **「その commit が本当にその判断の実装か」は見ていない。** 見ているのは
//     祖先かどうかだけで、無関係な commit を結んだ変異は素通りする。
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/gitio"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// RuleCommitUnreachable は規則 id（acknowledges で名指しする文字列）。
const RuleCommitUnreachable = "commit-unreachable"

func checkCommitUnreachable(snap store.Snapshot) []Finding {
	if snap.Root == "" {
		return nil // 手組み snapshot は git 履歴を持たない（decision-stale と同型）
	}
	// 結ぶ先が1つも無ければ git を呼ばない（commits を使わないストアで git 呼び出しゼロ）。
	if !anyDecisionHasCommits(snap.Decisions) {
		return nil
	}
	reachable, derived, failure := gitio.ReachableFromHead(snap.Root)
	if failure != nil {
		return []Finding{{
			Rule:     RuleGitDerivationFailed,
			Severity: SeverityInfo,
			Tier:     TierAdvisory,
			Message: "git 管理下のストアですが、git からの導出に失敗したため commit-unreachable は検査していません" +
				"（この検査は走っていません——「問題なし」ではありません）: " + failure.Error(),
		}}
	}
	if !derived {
		return nil // 黙る段（git が無い／管理下でない／commit が無い／浅い clone）
	}
	return commitUnreachableFindings(snap.Decisions, reachable)
}

// commitUnreachableFindings は「commits[] に載っているどのハッシュも reachable に
// 無い」decision を finding にする純関数（git を呼ばない・入力と出力の対で検査できる）。
func commitUnreachableFindings(decisions []model.Decision, reachable gitio.ReachableSet) []Finding {
	var out []Finding
	for _, d := range decisions {
		if len(d.Commits) == 0 {
			continue
		}
		// refs[] を持つ decision では出さない（01M1K4WPN3HXNVE0CR0T6NQK33）。
		// 来歴の正本は refs へ移り、commits は非推奨になった——**壊れない辿り先が
		// 別にあるなら、切れた hash が残っていること自体は実害ではない。**
		// commits[] は追記専用で消せないので、ここで黙らないと「refs へ移したのに
		// 永久に鳴り続ける」finding になる。移行が進むほど静かになる形にする。
		if len(d.Refs) > 0 {
			continue
		}
		var unreachable []string
		for _, h := range d.Commits {
			if !reachable.Contains(h) {
				unreachable = append(unreachable, h)
			}
		}
		// 1つでも辿れるものがあれば出さない——着地ハッシュを足した時点で消える
		// finding にするため（package doc「なぜ1つも辿れないで判定するのか」）。
		if len(unreachable) < len(d.Commits) {
			continue
		}
		short := make([]string, 0, len(unreachable))
		for _, h := range unreachable {
			short = append(short, shortHash(h))
		}
		out = append(out, Finding{
			Rule:       RuleCommitUnreachable,
			Severity:   SeverityInfo,
			Tier:       TierAdvisory,
			Target:     d.ID,
			TargetType: "decision",
			Field:      "commits",
			Quote:      strings.Join(short, "・"),
			Suggestion: "scholia decision add-ref <id> <PR/issue の URL> で壊れない辿り先を結ぶ（または scholia decision relink-commits で着地先の候補を探す）",
			Message: fmt.Sprintf("decision %s: 結んだ commit %d 件のいずれも HEAD から辿れず、refs も空です（%s）。この判断が、どの変更で実装されたのかを辿る手立てがありません",
				d.ID, len(unreachable), strings.Join(short, "・")),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

// anyDecisionHasCommits は「git を呼ぶ価値があるか」の早期打ち切り。
//
// ⚠️ **ここを緩めても答えは変わらない**（実測: `&& len(d.Refs) == 0` を外す変異を
// 入れてもテストは緑のまま）。緩めれば git を無駄に呼ぶだけで、finding は下の
// ループが同じ条件で絞る。**値で落ちる検査が当たらない範囲**なので、そう名乗る
// （CLAUDE.md「配線ガードの書き方」2）。逆向き——不当に false を返して黙る——は
// 起きない。commits を持ち refs も持つ decision は、どのみちループでも飛ばされる。
func anyDecisionHasCommits(decisions []model.Decision) bool {
	for _, d := range decisions {
		if len(d.Commits) > 0 && len(d.Refs) == 0 {
			return true
		}
	}
	return false
}
