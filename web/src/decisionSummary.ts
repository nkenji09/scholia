// 意思決定の「要約」を1行ぶん切り出す（01KYHW54B8ZXH0NEPH2J7N1X39 条項6）。
//
// 条項6 は「要約は一目で走査できる長さであること。全文の第1段落をそのまま出す
// ことを要約と呼ばない。要約だけを縦に読んで、その記録にどんな判断があるかを
// 走査できること」と定める。ここはその1点を担う。
//
// 直そうとしている実際の壊れ方: 旧実装は「1行目を句点まで」を
// `/[。．.](\s|$)/` で探していたが、**日本語は句点のあとに空白を置かない**ので
// この正規表現はほぼ一致せず、1行目がまるごと返っていた。実データで1件 478 文字・
// 695 文字が「要約」として返っていた（本 repo の req.comfortable-viewer.deep-linking）。
// 見出し（`## …`）で始まる新しい decision だけがたまたま短くなっていた。
//
// 半角ピリオドだけは後ろに空白か行末を要求する——`e.g.` / `v1.2` / `file.ts` /
// `01K…` のようなトークンで切ってしまわないため。日本語の句点にその条件は付けない。

/** 要約の上限。これを超えたら丸めて `…` を付ける（走査できる長さの担保）。 */
export const SUMMARY_MAX = 68;

/** 1行目・最初の文の終わりまで・上限で丸め、の順で要約を作る。 */
export function summaryOf(text: string): string {
  const s = (text || '').trim();
  if (!s) return '';
  const nl = s.search(/\n/);
  let line = (nl >= 0 ? s.slice(0, nl) : s).trim();
  // markdown 見出しの記号は本文ではないので落とす（`## 結論` → `結論`）。
  line = line.replace(/^#{1,6}\s+/, '').trim();
  // 最初の文末で切る。`。`『．』は直後に空白を要求しない／`.` は要求する。
  const end = line.search(/[。．]|\.(?=\s|$)/);
  if (end >= 0) line = line.slice(0, end + 1);
  return clamp(line, SUMMARY_MAX);
}

/** 文字数で丸める（丸めたときだけ `…` を付ける）。 */
export function clamp(text: string, max: number): string {
  const s = (text || '').trim();
  if ([...s].length <= max) return s;
  return [...s].slice(0, max).join('').trimEnd() + '…';
}
