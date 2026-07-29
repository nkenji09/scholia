import { describe, expect, it } from 'vitest';
import { rulesCommand } from './rulesCommand';

// カードが出す CLI コマンド（01KYJV3FYMDFRWQ939NBV2BPAC 条項3）。
//
// このコマンドは「この記録に効いている規則の全体を1つの並びで読む」唯一の経路
// （受け皿は現状 CLI だけ・同 条項1）。**画面に出す以上、コピーして動かなければ
// 開示にならない**ので、3種のレコードすべてで文字列そのものを固定する。
// フラグ名は CLI の実体（`scholia rules --tag/--tx/--vocab`）に対応する。
// **効力のフラグは付けない**——`scholia rules` は既定で効いている規則だけを本文で
// 出すので、カードが開示する件数とそのまま一致する。かつて付けていた `--current`
// は、既定が変わった今は「効いているものだけを得るには特別な指定が要る」という
// 誤解を再生産するだけになる。
//
// ここが守るのは**文字列の正しさ**まで。実際に CLI が受理して期待する出力を
// 返すことは実機で確認する（result.md §17）。

describe('rulesCommand', () => {
  it('tag: --tag（自身＋祖先タグへの decisions）', () => {
    expect(rulesCommand({ kind: 'tag', id: 'req.atoms-derive.no-spec-file' })).toBe(
      'scholia rules --tag req.atoms-derive.no-spec-file',
    );
  });

  it('transition: --tx（自身＋実効タグへの decisions・--transition ではない）', () => {
    expect(rulesCommand({ kind: 'transition', id: 'tx.viewer.search-restore' })).toBe(
      'scholia rules --tx tx.viewer.search-restore',
    );
  });

  it('vocab: --vocab（自身＋その語彙が持つタグとその祖先への decisions）', () => {
    expect(rulesCommand({ kind: 'vocab', id: 'cond.update-apply' })).toBe('scholia rules --vocab cond.update-apply');
  });

  // 既定が「効いている規則だけ」になったので、効力のフラグを足すと
  // 「特別な指定が要る」という誤解を画面が再生産する。付けないことを固定する。
  it('効力のフラグを付けない（既定が効いている規則だけ）', () => {
    for (const cmd of [
      rulesCommand({ kind: 'tag', id: 'x' }),
      rulesCommand({ kind: 'transition', id: 'y' }),
      rulesCommand({ kind: 'vocab', id: 'z' }),
    ]) {
      expect(cmd).not.toMatch(/--current/);
      expect(cmd).not.toMatch(/--all/);
    }
  });
});
