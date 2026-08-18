// rules_decision_stale.go — decision-stale（info・#45 D7）と、その導出が
// 落ちたことを名乗る git-derivation-failed（info・decision
// 01M0AJDYJSEVCSYEV0HDPSTWFZ）。
//
// staleness の半分は decide イベントの外で生まれる——レコードの desc/内容は
// 後続の実装・decision で古くなるのに、それを検知する配線が decide 経路にしか
// 無かった。時刻比較型の鮮度検査は spec-first では実装 commit が常に desc より
// 新しく原理的に効かないため採らない。代わりに「レコード変更 commit に decision
// 追加が同伴しない場合のみ『要再確認』」を git から導出する（保存ゼロ）。
//
// これは info 級: 機械マイグレーション型 commit（一括 retrofit 等）の偽陽性が
// 残るため、error/warn にはせず acknowledges で容認可能にする。rename 一括 commit
// は git の rename 検出（R status）で除外する。Snapshot.Root が空（手組み
// snapshot・テスト fixture）のときは検査しない（dead-doc-ref と同型）。
//
// # 導出が落ちたときに何を報告するか（01M0AJDYJSEVCSYEV0HDPSTWFZ・
// 追補 01M0APXCFF70MBZCQT98MNQMW8）
//
// 失敗を4段に分け、**名乗るのは最後の段だけ**である。
//
//  1. git を起動できない → 黙る（既決の範囲）
//  2. git 管理下でない → 黙る（既決の範囲・01M09FHDJCV2WWFC7Z8331B0YQ）
//  3. **commit が1件も無い（unborn HEAD）** → 黙る。走査する対象がゼロで、
//     「導出できなかった」ではなく「見るものが無い」——異常ではない
//     （01M0APXCFF70MBZCQT98MNQMW8）。
//  4. **git 管理下で commit もあるのに導出そのものが落ちた** →
//     git-derivation-failed を1件出す
//
// 🔴 **段を分けるのに終了状態を使わない。** git は「repo でない」も「repo だが
// 読めない」も「commit がまだ無い」も**同じ exit 128 の fatal** で返す。
// 終了状態だけを見ると「失敗した」と「見るものが無い」は必ず混ざる——実際に
// 混ざり、`git init` 直後のストアに警告が出ていた。分けるには「走査する対象が
// そもそも存在するか」を**別の問いとして先に立てる**（第3段）。
//
// # 落ちない範囲（正直に名乗る）
//
//   - `rev-parse --show-toplevel` 自体が落ちる形の失敗（所有者が違うディレクトリの
//     安全確認など）は**第2段に寄って黙る**。文言を照合する以外に分ける手が無く、
//     文言照合は綴りが変われば外れるので採らない。
//   - git が exit 0 のまま部分的に間違った出力を返す形。
//   - 🔴 **第1段は、観測できる振る舞いとしては第2段と区別が付かない。** git が
//     無ければ第2段のリポジトリ根の解決も落ちるので、**第1段の分岐を丸ごと消しても
//     何も変わらない**（実測。クリーンルームレビューが変異で確かめ、緑のまま通った）。
//     段として書いてあるのは読み手に構造を示すためで、**検査には支えられていない。**
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/gitio"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// decisionStaleScanLimit は走査する直近 commit 数の上限（全史走査を避ける・
// 鮮度は「最近レコードを触ったのに decision を結ばなかった」に意味があるため
// 直近窓で十分）。
const decisionStaleScanLimit = 200

// RuleGitDerivationFailed は「git 管理下なのに git からの導出が落ちた」ことを
// 名乗る advisory の rule id。
//
// ⚠️ **Rules には登録しない**（commit-unverified と同型）。ValidRuleIDs は Rules
// から作られるので、この id は `acknowledges` に書けない——書くと
// dangling-acknowledges が出て、acknowledges[] は追記専用なので永久に消えない。
// 🔴 **それが狙いである。** これは記録の問題ではなく実行環境の問題で、
// レコード宛ての容認で消してよいものではない。**容認で黙らせる手段を作らない。**
// Finding に AcknowledgeOnly を立てないのも同じ理由（この repo では
// AcknowledgeOnly は「acknowledges で畳む対象」の意味で使われる）。その結果、
// この finding は既定のテキスト出力で件数に畳まれず明細が出る
// （畳まれるのは容認でしか解けない区分だけ・01KZ5AC0EJBXK3A4NYK2DJJM7P）。
const RuleGitDerivationFailed = "git-derivation-failed"

// recordSubdirs は「レコード変更」とみなすストア内のディレクトリ。
// decisions は「同伴すべき側」なので含めない。
var recordSubdirs = []string{"transitions/", "tags/", "vocab/"}

const decisionsSubdir = "decisions/"

func checkDecisionStale(snap store.Snapshot) []Finding {
	if snap.Root == "" {
		return nil // 手組み snapshot は git 履歴を持たない（dead-doc-ref と同型）
	}
	commits, failure := recordModifyingCommits(snap.Root)
	if failure != nil {
		return []Finding{{
			Rule:     RuleGitDerivationFailed,
			Severity: SeverityInfo,
			Tier:     TierAdvisory,
			Message: "git 管理下のストアですが、git からの導出に失敗したため decision-stale は検査していません" +
				"（この検査は走っていません——「問題なし」ではありません）: " + failure.Error(),
		}}
	}
	// 機械マイグレーション型の偽陽性を容認する経路（#45 D7）: いずれかの decision
	// が acknowledges で decision-stale を名指ししていれば、その decision の target
	// レコード（basename）を触った commit を畳む。commit は decision target に
	// なれないため、レコード basename 経由で照合する（recordID.json）。
	staleAcked := recordsAckingDecisionStale(snap.Decisions)

	var out []Finding
	for _, c := range commits {
		ackedBy := ""
		for _, rec := range c.records {
			if id, ok := staleAcked[rec]; ok {
				ackedBy = id
				break
			}
		}
		out = append(out, Finding{
			Rule:       "decision-stale",
			Severity:   SeverityInfo,
			Tier:       TierAdvisory,
			Target:     c.hash,
			TargetType: "commit",
			// AcknowledgedBy は対象レコード宛て acknowledges:[decision-stale] で畳んだ
			// decision id（非空なら容認済み）。AcknowledgeOnly=true: git 履歴上の
			// commit を指すため record 編集で是正できず、容認（acknowledges）でのみ
			// 解消する（retrofit の fixable に数えない）。
			AcknowledgedBy:  ackedBy,
			AcknowledgeOnly: true,
			Message: fmt.Sprintf("commit %s: 既存レコードを変更（%s）していますが decision 追加が同伴していません（要再確認・機械マイグレーション型なら対象レコード宛て acknowledges:[decision-stale] で容認可）",
				shortHash(c.hash), strings.Join(c.records, "・")),
		})
	}
	return out
}

// recordsAckingDecisionStale は「acknowledges に decision-stale を含む decision」の
// target レコード id を "<id>.json" basename → decision id で返す（commit の変更
// レコード basename と照合するため）。
func recordsAckingDecisionStale(decisions []model.Decision) map[string]string {
	out := make(map[string]string)
	for _, d := range decisions {
		for _, rule := range d.Acknowledges {
			if rule == "decision-stale" {
				out[d.Target.ID+".json"] = d.ID
			}
		}
	}
	return out
}

type staleCommit struct {
	hash    string
	records []string // 変更された（M）レコードファイルの basename
}

// recordModifyingCommits は直近 decisionStaleScanLimit commit のうち
// 「既存レコードを M（変更）したが decision を A（追加）していない」commit を
// 返す。rename（R）は除外。
//
// failure が非 nil なら「git 管理下で commit もあるのに導出が落ちた」——第1〜3段
// （git が無い／git 管理下でない／commit が1件も無い）は commits も failure も
// 返さずに黙る（package doc の4段）。
func recordModifyingCommits(projectRoot string) (commits []staleCommit, failure error) {
	if !gitio.Installed() {
		return nil, nil // 第1段: git が起動できない
	}
	gitRoot, relPrefix, err := gitio.ResolveContext(projectRoot)
	if err != nil {
		return nil, nil // 第2段: git 管理下でない
	}
	// 第3段: commit が1件も無い（`git init` した直後）。走査する対象がゼロで、
	// 「導出できなかった」ではなく「見るものが無い」——黙る。
	// err は「git を起動すらできなかった」＝第1段と同じ扱いで黙る。
	if has, err := gitio.HasAnyCommit(gitRoot); err != nil || !has {
		return nil, nil
	}
	// ⚠️ 走査する commit 集合は変えない（リポジトリ直近 decisionStaleScanLimit 件）。
	// pathspec で絞ると窓の届く先が変わる——それは検知の穴を塞ぐことと別の判断である。
	out, err := gitio.Run(gitRoot, "log",
		fmt.Sprintf("-n%d", decisionStaleScanLimit),
		"-M", "--name-status", "-z", gitio.LogFormatArg)
	if err != nil {
		return nil, err // 第4段: 導出が落ちた
	}
	parsed, err := gitio.ParseNameStatusZ(out)
	if err != nil {
		return nil, err // 読めない出力も「導出できなかった」
	}
	return staleCommits(parsed, storePathspec(relPrefix, store.DirName)), nil
}

// staleCommits は「既存レコードを M したが decision を A していない」commit を
// 選ぶ純関数（git を呼ばない・入力と出力の対で検査できる）。
func staleCommits(commits []gitio.Commit, storePrefix string) []staleCommit {
	var out []staleCommit
	for _, c := range commits {
		var modified []string
		addedDecision := false
		for _, ch := range c.Changes {
			rel, inStore := strings.CutPrefix(ch.Path, storePrefix+"/")
			if !inStore {
				continue
			}
			if strings.HasPrefix(rel, decisionsSubdir) && strings.HasPrefix(ch.Status, "A") {
				addedDecision = true
				continue
			}
			// rename（R…）は除外——レコードの実質変更ではない機械追随。
			if strings.HasPrefix(ch.Status, "R") {
				continue
			}
			// 既存レコードの変更（M）のみ数える（A=新規レコードは decision-coverage の
			// 領分・D=削除は staleness ではない）。
			if strings.HasPrefix(ch.Status, "M") && isRecordSubpath(rel) {
				modified = append(modified, baseName(rel))
			}
		}
		if len(modified) > 0 && !addedDecision {
			sort.Strings(modified)
			out = append(out, staleCommit{hash: c.Hash, records: modified})
		}
	}
	return out
}

// isRecordSubpath はストア相対のパスがレコードかを返す。
func isRecordSubpath(rel string) bool {
	for _, d := range recordSubdirs {
		if strings.HasPrefix(rel, d) {
			return true
		}
	}
	return false
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

func baseName(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}
