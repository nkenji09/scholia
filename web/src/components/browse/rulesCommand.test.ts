import { describe, expect, it } from 'vitest';
import { rulesCommand } from './rulesCommand';

// カードが出す CLI コマンド（01KYJV3FYMDFRWQ939NBV2BPAC 条項3）。
//
// このコマンドは「この記録を支配している規則を端末で読む」導線である。
// **画面に出す以上、コピーして動かなければ開示にならない**ので、3種のレコード
// すべてで文字列そのものを固定する。フラグ名は CLI の実体
// （`scholia rules --tag/--tx/--vocab`）に対応する。
// **フラグは付けない**（01KZ06SYP12ZFDG1WPNYM529D8 変更9）——素の `rules` が
// 集める集合は、カードが開示する件数とそのまま一致する。かつて付けていた
// `--current` も、いま足したくなる `--all` も、「正しいものを得るには特別な指定が
// 要る」という誤解を画面が再生産することになる。
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
