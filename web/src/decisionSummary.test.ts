import { describe, expect, it } from 'vitest';
import { SUMMARY_MAX, summaryOf } from './decisionSummary';

// 要約が要約になっていること（01KYHW54B8ZXH0NEPH2J7N1X39 条項6）。
//
// 旧実装は「1行目を句点まで」を /[。．.](\s|$)/ で探していたが、日本語は句点の
// あとに空白を置かないので**ほぼ一致せず**、第1段落がまるごと返っていた。実データ
// で1件 478 文字・695 文字が「要約」として返っていた。ここはその再発を止める。

describe('summaryOf', () => {
  it('日本語の句点で切る（句点のあとに空白は無い）', () => {
    expect(summaryOf('右配置は維持する。狭い画面では下に回す。理由は可読性。')).toBe('右配置は維持する。');
  });

  it('第1段落をまるごと返さない（旧実装の壊れ方）', () => {
    const prose = 'settled セマンティクス: 検索条件は URL に反映する。' + 'あ'.repeat(400);
    const got = summaryOf(prose);
    expect(got).toBe('settled セマンティクス: 検索条件は URL に反映する。');
    expect([...got].length).toBeLessThanOrEqual(SUMMARY_MAX);
  });

  it('句点が無い長い1行は上限で丸める', () => {
    const got = summaryOf('あ'.repeat(300));
    expect([...got].length).toBe(SUMMARY_MAX + 1); // 丸め記号 … のぶん
    expect(got.endsWith('…')).toBe(true);
  });

  it('markdown 見出しは記号を落として本文として扱う', () => {
    expect(summaryOf('## 規則を集約して見せる2つの面を廃止する\n\n### 結論\n本文…')).toBe('規則を集約して見せる2つの面を廃止する');
  });

  it('半角ピリオドは後ろに空白か行末があるときだけ文末とみなす', () => {
    // 略語・バージョン・ファイル名で切らない（`e.g.` の直後は空白なので切れてよい）。
    expect(summaryOf('viewer は v1.2 の spec.json を読む。次の文。')).toBe('viewer は v1.2 の spec.json を読む。');
    expect(summaryOf('Use it. Then stop.')).toBe('Use it.');
  });

  it('空文字・空白のみは空を返す', () => {
    expect(summaryOf('')).toBe('');
    expect(summaryOf('   \n  ')).toBe('');
  });
});
