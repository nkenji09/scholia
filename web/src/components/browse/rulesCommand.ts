// 「この記録を支配している規則を端末で読む」ための CLI コマンドを組む
// （01KYJV3FYMDFRWQ939NBV2BPAC 条項3）。
//
// ⚠️ 同 条項1 は「全体を1つの並びで読む受け皿は現状 CLI だけ・viewer には無い」と
// 書いていたが、**それは既に事実でない**——支配方向の一覧
// （01KYKS4Y56FAHRVCWKMQJK4RT6 条項1・WholeRules.tsx）が画面側の受け皿として
// 実装されている。01KZ06SYP12ZFDG1WPNYM529D8 変更10 がこの記述を正した。
// このコマンドは「端末で読む」導線であって、唯一の経路ではない。
//
// フラグは CLI の実体（cmd/scholia rules）に合わせる:
//   --tag <id>    自身＋祖先タグへの decisions
//   --tx <id>     自身＋実効タグへの decisions
//   --vocab <id>  自身＋その語彙が持つタグとその祖先への decisions
//
// **フラグは付けない**（01KZ06SYP12ZFDG1WPNYM529D8 変更9）。素の `rules` が
// 集める集合は、カードが開示する件数（支配している規則の数・
// 01KYHW54B8ZXH0NEPH2J7N1X39 条項5 と同じ数え方）とそのまま一致する。
// ⚠️ ただし**本文が出るのはその記録自身への decision だけ**で、経由で届く分は
// 存在・経由タグ・引き方になる（端末側が `--all` を案内する）。件数は一致し、
// 本文の量だけが違う——画面と端末が同じ答えを返すようにした結果である。
//
// かつては `--current` を付けていた。既定が畳まなかった頃はそれで正しかったが、
// 既定が変わった今は**冗長なだけでなく有害**である——「正しいものを得るには
// 特別な指定が要る」という誤解を画面が再生産し続ける。`--current` は後方互換
// として受理され続けるので、外しても出力は 1 文字も変わらない。
// ⚠️ **ここに `--all` を足さない。** 足すと同じ誤解を別の綴りで作り直すことに
// なる（同 変更9）。画面で全体を読む用途は支配方向の一覧が担う。

export type RecordRef =
  | { kind: 'tag'; id: string }
  | { kind: 'transition'; id: string }
  | { kind: 'vocab'; id: string };

const FLAG: Record<RecordRef['kind'], string> = {
  tag: '--tag',
  transition: '--tx',
  vocab: '--vocab',
};

export function rulesCommand(record: RecordRef): string {
  return `scholia rules ${FLAG[record.kind]} ${record.id}`;
}
