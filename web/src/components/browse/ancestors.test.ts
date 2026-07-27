import { describe, expect, it } from 'vitest';
import { ancestorsOf } from './filters';
import type { Tag } from '../../types';

// 祖先の連なり（01KYHW4NBNVN9BFXYZMBX8MPF8 条項4）。
//
// レビューで「ancestorsOf を直接の親1階層に退化させる」変異（M5）が配線ガードを
// 素通りした。配線ガードは「BrowseView が ancestorsOf を呼んでいるか」しか見ない
// ので、関数の中身が壊れても緑になる。ここは**関数の振る舞い**を守る。
//
// これが壊れると、祖父に書かれた規則へカードから到達する手段が画面から消える
// （実測で継承した効いている規則 82件のうち4件が祖父由来）。タグ id のドットは
// 階層ではないので、id を見ても代わりにはならない。

const mk = (pairs: Array<[string, string[]]>): Map<string, Tag> =>
  new Map(pairs.map(([id, parentIds]) => [id, { id, name: id, parentIds } as Tag]));

const ids = (tags: Tag[]) => tags.map((t) => t.id);

describe('ancestorsOf', () => {
  it('直接の親だけでなく祖父まで返す（M5 が退化させた挙動）', () => {
    const tags = mk([
      ['child', ['parent']],
      ['parent', ['grand']],
      ['grand', []],
    ]);
    expect(ids(ancestorsOf('child', tags))).toEqual(['grand', 'parent']);
  });

  it('遠い祖先が先・直接の親が最後（パンくずとして読める順）', () => {
    const tags = mk([
      ['a', ['b']],
      ['b', ['c']],
      ['c', ['d']],
      ['d', []],
    ]);
    expect(ids(ancestorsOf('a', tags))).toEqual(['d', 'c', 'b']);
  });

  it('自分自身は含まない', () => {
    const tags = mk([
      ['x', ['y']],
      ['y', []],
    ]);
    expect(ids(ancestorsOf('x', tags))).not.toContain('x');
  });

  it('DAG: 複数経路で着く祖先は1度だけ・深いほうの位置に並ぶ', () => {
    // root へは leaf→mid→root（深さ2）と leaf→root（深さ1）の2経路。
    // 深いほうを採らないと root が mid より後ろに並んでしまう。
    const tags = mk([
      ['leaf', ['mid', 'root']],
      ['mid', ['root']],
      ['root', []],
    ]);
    const got = ids(ancestorsOf('leaf', tags));
    expect(got.filter((id) => id === 'root')).toHaveLength(1);
    expect(got).toEqual(['root', 'mid']);
  });

  it('循環しても止まる', () => {
    const tags = mk([
      ['a', ['b']],
      ['b', ['a']],
    ]);
    expect(ids(ancestorsOf('a', tags))).toEqual(['b']);
  });

  it('実在しない親 id は無視する（参照だけ残った状態で落ちない）', () => {
    const tags = mk([['a', ['ghost', 'b']], ['b', []]]);
    expect(ids(ancestorsOf('a', tags))).toEqual(['b']);
  });

  it('親が無ければ空', () => {
    expect(ancestorsOf('solo', mk([['solo', []]]))).toEqual([]);
  });
});
