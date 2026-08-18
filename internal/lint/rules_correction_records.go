// rules_correction_records.go — correction-changes-records（info・decision
// 01M09FHEQH7PVZ2BTKGXY5YMNN 変更6）。
//
// 是正とは「記録が正しく、実装のほうが違っていたので直した」ことである。だから
// 是正の commit は `.scholia/` を1バイトも変えないはずで、**変えているなら spec が
// 変わったということ**——それは定義上、是正ではなく精緻化である。
//
// # この規則が落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:**
//   - applied[] に是正（correction）の印が付いた commit が、記録ディレクトリ
//     （`.scholia/`）配下のファイルを1つでも変更しているとき。
//
// **落ちない（射程の外・正直に名乗る）:**
//   - 🔴 **「本当に実装が間違っていた」ことは確かめられない。** ここが見るのは
//     「同じ commit で `.scholia/` も触ったか」という外形だけで、
//     **ふつうの追加実装を是正と名乗る**変異は素通りする。決定本文が
//     「落とせない」として先に名乗ってある穴で、ここを厚くしても消えない。
//   - **打ち忘れ**（印を付けなかった是正）。差分の見た目がふつうの実装 commit と
//     同じで、違うのは「なぜ書いたか」だからである。
//   - **矛盾・却下の印**。それらには結ぶ commit が無いので、外形の検査が1つも無い。
//     ⚠️ **是正よりも確からしさが低い**——1つの指標に混ぜないこと。
//   - **マージコミットに付けた印**。`git diff-tree` は既定でマージの差分を出さない
//     ので、変更していても finding にならない（info 規則なので偽陰性側に倒す）。
//   - git が使えない／git 管理下でない／その commit がこの clone に無いとき。
//     浅い clone では過去の commit が手元に無く、照合そのものができない。
//
// info 級にするのは、上の偽陰性が残るためと、対象が git 履歴上の commit で
// **レコードを直しても解消できない**（applied[] は追記専用で、打った印は消せない）
// ため。解消は acknowledges でのみ行う（decision-stale と同型）。
package lint

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nkenji09/scholia/internal/activity"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// RuleCorrectionChangesRecords は規則 id（acknowledges で名指しする文字列）。
const RuleCorrectionChangesRecords = "correction-changes-records"

func checkCorrectionChangesRecords(snap store.Snapshot) []Finding {
	if snap.Root == "" {
		return nil // 手組み snapshot は git 履歴を持たない（decision-stale と同型）
	}
	// 是正の印が1つも無ければ git を呼ばない（大多数のストアで git 呼び出しゼロ）。
	if !hasCorrectionMark(snap.Decisions) {
		return nil
	}
	// ⚠️ リポジトリ根とストアの相対位置は git 自身に解決させる（単位BC の
	// activity.ResolveGitContext）。固定接頭辞で判定すると、リポジトリ根より下に
	// ストアがあるとき何も拾えない——decision-stale がその形のバグを持っている。
	gitRoot, relPrefix, err := activity.ResolveGitContext(snap.Root)
	if err != nil {
		return nil // git が使えない／git 管理下でない
	}
	storeSpec := storePathspec(relPrefix, store.DirName)

	var out []Finding
	for _, d := range snap.Decisions {
		acked := ""
		for _, rule := range d.Acknowledges {
			if rule == RuleCorrectionChangesRecords {
				acked = d.ID
			}
		}
		for _, m := range d.Applied {
			if m.Kind != model.AppliedCorrection || m.Commit == "" {
				continue
			}
			touched, ok := commitTouches(gitRoot, m.Commit, storeSpec)
			if !ok || !touched {
				continue
			}
			out = append(out, Finding{
				Rule:            RuleCorrectionChangesRecords,
				Severity:        SeverityInfo,
				Tier:            TierAdvisory,
				Target:          m.Commit,
				TargetType:      "commit",
				AcknowledgedBy:  acked,
				AcknowledgeOnly: true,
				Message: "commit " + shortHash(m.Commit) + ": 是正（correction）の印が付いていますが、" +
					"この commit は記録（" + storeSpec + "）も変更しています。" +
					"記録が変わったなら spec が変わったということで、それは是正ではなく精緻化です" +
					"（印の種別を見直すか、意図した形なら対象レコード宛て acknowledges:[" +
					RuleCorrectionChangesRecords + "] で容認できます）",
			})
		}
	}
	return out
}

func hasCorrectionMark(decisions []model.Decision) bool {
	for _, d := range decisions {
		for _, m := range d.Applied {
			if m.Kind == model.AppliedCorrection && m.Commit != "" {
				return true
			}
		}
	}
	return false
}

// storePathspec は記録ディレクトリを git の pathspec として表す。
func storePathspec(relPrefix, storeDirName string) string {
	if relPrefix == "" || relPrefix == "." {
		return storeDirName
	}
	return filepath.ToSlash(filepath.Join(relPrefix, storeDirName))
}

// commitTouches は「その commit が spec 配下を変更したか」を返す。ok=false は
// 判定できなかったこと（浅い clone でその commit が手元に無い等）。
//
// ⚠️ **`git log -1 <hash> -- <spec>` と書いてはいけない。** それは「<hash> 以前で
// <spec> を最後に触った commit」を返す（履歴の簡約）ので、**その commit 自身が
// 触っていなくても別の commit が返る**。ここで要るのは「この commit の差分」なので
// `diff-tree` を使う。`--root` は最初の commit（親が無い）でも差分を出させる。
//
// ⚠️ 単位BC が踏んだ `core.quotePath` の罠は、ここでは効かない——**出力のパスを
// 前方一致で判定していないから**である。絞り込みは pathspec として git に渡して
// おり、非 ASCII のパスが C 形式で引用されても「行が出るかどうか」は変わらない。
// 出力の中身を読む形に変えるなら、そのときは `-c core.quotePath=false` が要る。
func commitTouches(gitRoot, hash, spec string) (touched, ok bool) {
	cmd := exec.Command("git", "-C", gitRoot, "diff-tree",
		"--no-commit-id", "--name-only", "-r", "--root", hash, "--", spec)
	out, err := cmd.Output()
	if err != nil {
		return false, false
	}
	return strings.TrimSpace(string(out)) != "", true
}
