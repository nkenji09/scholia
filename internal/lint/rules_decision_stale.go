// rules_decision_stale.go — decision-stale（info・#45 D7）。
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
package lint

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// decisionStaleScanLimit は走査する直近 commit 数の上限（全史走査を避ける・
// 鮮度は「最近レコードを触ったのに decision を結ばなかった」に意味があるため
// 直近窓で十分）。
const decisionStaleScanLimit = 200

// recordDirs は「レコード変更」とみなすディレクトリ（.scholia 相対）。
// decisions は「同伴すべき側」なので含めない。
var recordDirs = []string{".scholia/transitions/", ".scholia/tags/", ".scholia/vocab/"}

func checkDecisionStale(snap store.Snapshot) []Finding {
	if snap.Root == "" {
		return nil // 手組み snapshot は git 履歴を持たない（dead-doc-ref と同型）
	}
	commits, ok := recordModifyingCommits(snap.Root)
	if !ok {
		return nil // git が使えない store では検査しない
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
// 返す。rename（R）は除外。git が使えなければ ok=false。
func recordModifyingCommits(root string) (commits []staleCommit, ok bool) {
	// --name-status -M で各 commit の変更ファイルと status を取る。
	// フォーマット: commit 行（%H で始まる）＋ status\tpath 行群。
	cmd := exec.Command("git", "-C", root, "log",
		fmt.Sprintf("-n%d", decisionStaleScanLimit),
		"-M", "--name-status", "--format=%H")
	outBytes, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	lines := strings.Split(string(outBytes), "\n")

	var curHash string
	var modifiedRecords []string
	var addedDecision bool
	flush := func() {
		if curHash == "" {
			return
		}
		if len(modifiedRecords) > 0 && !addedDecision {
			sort.Strings(modifiedRecords)
			commits = append(commits, staleCommit{hash: curHash, records: modifiedRecords})
		}
		modifiedRecords = nil
		addedDecision = false
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		// commit ハッシュ行（40 hex・タブなし）。
		if !strings.ContainsRune(line, '\t') && len(line) >= 7 && isHexLine(line) {
			flush()
			curHash = line
			continue
		}
		// status\tpath[\tpath2]（R は "R100\told\tnew"）。
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		path := fields[len(fields)-1] // rename は新パスを見る

		if strings.HasPrefix(path, ".scholia/decisions/") && strings.HasPrefix(status, "A") {
			addedDecision = true
			continue
		}
		// rename（R…）は除外——レコードの実質変更ではない機械追随。
		if strings.HasPrefix(status, "R") {
			continue
		}
		// 既存レコードの変更（M）のみ数える（A=新規レコードは decision-coverage の
		// 領分・D=削除は staleness ではない）。
		if strings.HasPrefix(status, "M") && isRecordPath(path) {
			modifiedRecords = append(modifiedRecords, baseName(path))
		}
	}
	flush()
	return commits, true
}

func isRecordPath(path string) bool {
	for _, d := range recordDirs {
		if strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}

func isHexLine(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
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
