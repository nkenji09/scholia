// commit_unverified.go — 「結ぶ commit の実在を照合できなかった」ことを名乗る
// write-time advisory（decision 01M09FHEQH7PVZ2BTKGXY5YMNN・commitcheck の package doc）。
//
// # なぜ面ごとに書かないのか（CLAUDE.md「配線ガードの書き方」6）
//
// git が使えないとき、この道具は**弾かず・素通りせず・名乗る**と決めた。ところが
// 名乗りを `fmt.Fprintln` でテキスト出力にだけ書いていた間、**`--json` の面では
// 名乗りがどこにも出なかった**——配布スキルは AI に応答封筒を読ませる作りなので、
// AI は「保存された＝検査が走った」と読む。**自分で採らないと決めた「素通り」が、
// `--json` の面でだけ実際に起きていた**（クリーンルームレビュー 指摘2）。
//
// だから名乗りを advisory 1本に寄せる。**commits[] を受け取れる面はすべてこれを運ぶ**:
//
//	scholia decide --commit …            テキスト（printWriteGateText）と --json 封筒
//	scholia decision add-commit …        同上
//	viewer の POST /api/decision         応答 JSON の advisories
//
// ⚠️ **この判定は internal/lint に置く。** commitcheck に置くと
// commitcheck → lint → store → commitcheck の循環になり、cli に置くと
// viewer から呼べない（cli が viewer を import している）。
//
// # 落ちない範囲（射程・正直に名乗る）
//
//   - **advisory を運ばない出力口を新しく作ったとき。** 判定はここ1つだが、
//     呼ぶのは面である——呼ばない面を足せば名乗りは出ない。
//   - 🔴 **viewer のフロントはこの advisory を画面に描かない**（実測: `web/src` に
//     `advisories` を読む箇所は無い）。**封筒には載るが、人の目には届かない。**
//     届くのは API を直に叩く消費者と、応答を読む AI である。
package lint

// RuleCommitUnverified は「結ぶ commit の実在を照合できなかった」advisory の rule id。
//
// ⚠️ **Rules には登録しない。** これは「保存したその瞬間に git が使えたか」という
// 事実で、**後から全走査で判定できない**（保存後に git が入っても、入る前に保存された
// 値は照合されていない）。lint の規則は snapshot から導けるものだけを載せる。
//
// 🔴 **したがって `acknowledges` にこの名前を書いてはいけない。** ValidRuleIDs は
// Rules から作られるのでこの id を含まず、書くと `dangling-acknowledges` が出る
// ——そして `acknowledges[]` は追記専用なので**永久に消えない**。
// Finding に AcknowledgeOnly を立てないのは、そう誤読させないためである
// （AcknowledgeOnly はこの repo では「acknowledges で畳む対象」の意味で使われる）。
const RuleCommitUnverified = "commit-unverified"

// CommitUnverifiedAdvisories は「実在を照合していない」ことを advisory として返す。
//
// gitManaged が true（照合できた）なら何も言わない——照合できたのに名乗ると
// 狼少年になる。hashes が 0 のときも言わない（照合する相手が無い呼び出しで注記
// だけ出ると、何について言われているのか読めない）。
func CommitUnverifiedAdvisories(gitManaged bool, hashes int) []Finding {
	if hashes == 0 || gitManaged {
		return nil
	}
	return []Finding{{
		Rule:       RuleCommitUnverified,
		Severity:   SeverityInfo,
		Tier:       TierAdvisory,
		TargetType: "commit",
		Message: "結ぶ commit の実在は照合していません（git 管理下ではありません）。" +
			"形が commit hash であることだけを確かめました——" +
			"実在しない hash が保存されている可能性があります",
	}}
}
