// commit_deprecated.go — 「commits[] へ新しく結ぶのは非推奨」を名乗る write-time
// advisory（decision 01M1K4WPN3HXNVE0CR0T6NQK33）。
//
// 取り込み（squash merge・作業ブランチの作り直し）で hash は取り込み先の祖先から
// 外れ、結線が切れる。壊れない辿り先は URL のほうで、それは `refs[]` が持つ。
//
// # なぜ保存を止めないのか
//
// **hash には URL に無い取り柄が1つある**——外部サービスに依存せず、オフラインで
// diff が読める。PR の URL は GitHub が生きていて・権限があって・ネットがある前提で、
// 移行すれば死ぬ。だから禁止ではなく非推奨で、**選んで使う人の邪魔をしない。**
//
// # なぜ lint.Rules に登録しないのか
//
// ⚠️ **登録すると、既存レコードが一斉に鳴る。** 本 repo だけで 140 件の decision が
// hash を持ち、`commits[]` は追記専用なので**消して黙らせることができない**
// ——`acknowledges`（これも追記専用）で畳むしかなくなり、戻せない容認が積み上がる。
// これは 01KXS68HCNQ0H9QKNYFQ869J19 が「既存レコードが一斉に赤くなる移行断絶を
// 作らない」として避けた型そのものである。
//
// 名乗るのは**新しく結んだその瞬間**だけにする。過去に結んだものは名乗らない。
//
// 🔴 **したがって `acknowledges` にこの名前を書いてはいけない**（commit-unverified と
// 同じ理由——ValidRuleIDs は Rules から作られるので dangling-acknowledges が出て、
// acknowledges[] は追記専用なので永久に消えない）。
//
// # 是正（--kind correction）では名乗らない
//
// 🔴 非推奨にしたのは「**実装来歴を hash で辿ること**」であって、**是正の印ではない。**
// `applied[]` の是正は「この commit で直した」という出来事の記録で、
// `decision add-commit --kind correction` が**唯一の入口**である
// （`decision applied` は矛盾・却下しか受けない）。ここで名乗ると、代わりの口が
// 無いのに乗り換えろと言うことになり、是正の観測（01M09FHEQH7PVZ2BTKGXY5YMNN）が
// 黙って痩せる。**呼び分けるのは面の側の責任**である（この関数は件数しか見ない）。
//
// # 落ちない範囲（射程・正直に名乗る）
//
//   - **advisory を運ばない出力口を新しく作ったとき。** 判定はここ1つだが、呼ぶのは
//     面である（commit-unverified が名乗っているのと同じ穴）。
//   - 🔴 **viewer のフロントはこの advisory を画面に描かない**（`web/src` に
//     advisories を読む箇所は無い）。封筒には載るが、人の目には届かない。
package lint

// RuleCommitLinkDeprecated は「commits[] へ新しく結ぶのは非推奨」advisory の rule id。
const RuleCommitLinkDeprecated = "commit-link-deprecated"

// CommitLinkDeprecatedAdvisories は commits[] へ新しく足した件数が 1 以上のとき、
// 非推奨であることと代わりの口を名乗る。
//
// added が 0 のときは何も言わない——結んでいない呼び出しで注記だけ出ると、
// 何について言われているのか読めない（CommitUnverifiedAdvisories と同じ扱い）。
func CommitLinkDeprecatedAdvisories(added int) []Finding {
	if added <= 0 {
		return nil
	}
	return []Finding{{
		Rule:       RuleCommitLinkDeprecated,
		Severity:   SeverityInfo,
		Tier:       TierAdvisory,
		TargetType: "commit",
		Message: "commits[] へ commit hash を結ぶのは非推奨です（01M1K4WPN3HXNVE0CR0T6NQK33）。" +
			"squash merge や作業ブランチの作り直しで、この hash は取り込み先の祖先から外れて辿れなくなります" +
			"——壊れない辿り先は `scholia decision add-ref <id> <PR/issue の URL>` で結べます。" +
			"保存は止めていません（hash はオフラインで diff を読める利点があります）",
	}}
}
