// 「この記録に効いている規則の全体を1つの並びで読む」ための CLI コマンドを組む
// （01KYJV3FYMDFRWQ939NBV2BPAC 条項3）。
//
// その用途の受け皿は**現状 CLI だけ**である（同 条項1）。viewer の意思決定の
// 一覧はタグの**配下**方向にしか絞れず、支配する規則＝自身＋**祖先**とは向きが
// 逆なので受け皿にならない。よってカードが出すこのコマンドは、利用者が全体へ
// 到達する唯一の経路——**フラグを間違えると開示そのものが嘘になる**ので、
// 3種のレコードすべてを rulesCommand.test.ts で固定する。
//
// フラグは CLI の実体（cmd/scholia rules）に合わせる:
//   --tag <id>    自身＋祖先タグへの decisions
//   --tx <id>     自身＋実効タグへの decisions
//   --vocab <id>  自身＋その語彙が持つタグとその祖先への decisions
//
// **フラグは付けない。** `scholia rules` は既定で効いている規則だけを本文で出す
// ので、カードが開示する件数（効いている規則の数・01KYHW54B8ZXH0NEPH2J7N1X39
// 条項5 と同じ数え方）とそのまま一致する。
//
// かつては `--current` を付けていた。既定が畳まなかった頃はそれで正しかったが、
// 既定が変わった今は**冗長なだけでなく有害**である——「効いているものだけを得るには
// 特別な指定が要る」という誤解を画面が再生産し続ける。その誤解を消すのが既定を
// 変えた理由そのものだった。`--current` は後方互換として受理され続けるので、
// 外しても出力は 1 文字も変わらない。

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
