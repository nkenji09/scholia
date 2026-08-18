package cli

import (
	"github.com/nkenji09/scholia/internal/commitcheck"
	"github.com/nkenji09/scholia/internal/lint"
)

// RuleCommitUnverified は「結ぶ commit の実在を照合できなかった」ことを名乗る
// advisory の rule id（decision 01M09FHEQH7PVZ2BTKGXY5YMNN・commitcheck の
// package doc）。
//
// ⚠️ **lint.Rules には登録しない。** これは「保存したその瞬間に git が使えたか」
// という事実で、**後から全走査で判定できない**（保存後に git が入っても、
// 入る前に保存された値は照合されていない）。lint の規則は snapshot から導ける
// ものだけを載せる。
const RuleCommitUnverified = "commit-unverified"

// commitVerifyAdvisories は「実在を照合していない」ことを advisory として返す。
//
// # なぜテキスト専用の分岐にしないのか（CLAUDE.md「配線ガードの書き方」6）
//
// git が使えないとき、この道具は**弾かず・素通りせず・名乗る**と決めた
// （commitcheck の package doc）。ところが名乗りを `fmt.Fprintln` でテキスト出力
// にだけ書いていた間、**`--json` の面では名乗りがどこにも出なかった**
// ——配布スキルは AI に応答封筒を読ませる作りなので、AI は「保存された＝検査が
// 走った」と読む。**自分で採らないと決めた「素通り」が、`--json` の面でだけ
// 実際に起きていた**（クリーンルームレビュー 指摘2）。
//
// だから名乗りを advisory 1本に寄せる。advisory はテキスト面
// （printWriteGateText）と JSON 面（emitWriteJSON の封筒）の**両方が同じものを
// 運ぶ**ので、面ごとに書き分ける余地が無くなる。
//
// **落ちない範囲:** advisory を運ばない面を新しく作ったとき。封筒に advisories を
// 持たない出力口を足せば、そこには載らない。
func commitVerifyAdvisories(repo commitcheck.Repo, hashes int) []lint.Finding {
	// hashes が 0 のときは何も言わない（照合する相手が無い呼び出しで注記だけ
	// 出ると、何について言われているのか読めない）。
	if hashes == 0 || repo.Managed() {
		return nil
	}
	return []lint.Finding{{
		Rule:       RuleCommitUnverified,
		Severity:   lint.SeverityInfo,
		Tier:       lint.TierAdvisory,
		TargetType: "commit",
		// AcknowledgeOnly: レコードを直しても解消しない（git 管理下に置くか
		// どうかの話で、記録の書き方の問題ではない）。
		AcknowledgeOnly: true,
		Message: "結ぶ commit の実在は照合していません（git 管理下ではありません）。" +
			"形が commit hash であることだけを確かめました——" +
			"実在しない hash が保存されている可能性があります",
	}}
}
