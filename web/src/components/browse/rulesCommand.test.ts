import { describe, expect, it } from 'vitest';
import { rulesCommand } from './rulesCommand';

// カードが出す CLI コマンド（01KYJV3FYMDFRWQ939NBV2BPAC 条項3）。
//
// このコマンドは「この記録に効いている規則の全体を1つの並びで読む」唯一の経路
// （受け皿は現状 CLI だけ・同 条項1）。**画面に出す以上、コピーして動かなければ
// 開示にならない**ので、3種のレコードすべてで文字列そのものを固定する。
// フラグ名は CLI の実体（`scholia rules --tag/--tx/--vocab`）に対応し、
// --current は「効いている規則だけ」＝カードが開示する件数と同じ数え方。
//
// ここが守るのは**文字列の正しさ**まで。実際に CLI が受理して期待する出力を
// 返すことは実機で確認する（result.md §17）。

describe('rulesCommand', () => {
  it('tag: --tag（自身＋祖先タグへの decisions）', () => {
    expect(rulesCommand({ kind: 'tag', id: 'req.atoms-derive.no-spec-file' })).toBe(
      'scholia rules --tag req.atoms-derive.no-spec-file --current',
    );
  });

  it('transition: --tx（自身＋実効タグへの decisions・--transition ではない）', () => {
    expect(rulesCommand({ kind: 'transition', id: 'tx.viewer.search-restore' })).toBe(
      'scholia rules --tx tx.viewer.search-restore --current',
    );
  });

  it('vocab: --vocab（自身＋その語彙が持つタグとその祖先への decisions）', () => {
    expect(rulesCommand({ kind: 'vocab', id: 'cond.update-apply' })).toBe('scholia rules --vocab cond.update-apply --current');
  });

  it('効いている規則だけに絞る（--current を落とすと画面の件数と食い違う）', () => {
    expect(rulesCommand({ kind: 'tag', id: 'x' })).toMatch(/ --current$/);
  });
});
