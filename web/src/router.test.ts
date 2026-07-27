import { describe, expect, it } from 'vitest';
import { parseRoute, routeHash } from './router';
import type { Route } from './router';

// 概要タブの現在地が URL に載ることの回帰テスト（deep-linking の適用面拡張・
// 01KYGYYMZSS1Y0BFEJ69Q1JC40）。2タブ再設計の直後、概要タブは現在地をローカル
// state だけで持っていて URL に何も載らず、reload・ブラウザバックで必ず既定へ
// 落ちていた。ここが再び落ちると同じ回帰に戻るので、往復を固定する。
//
// reload / ブラウザバックでの復元は、どちらも「URL の hash を parseRoute した
// 結果からビューを組み直す」経路に乗る（router.ts の useHashRoute）。つまり
// hash ⇄ Route の往復が壊れていないことが、両者の復元が成り立つ条件そのもの。

/** hash → Route → hash が元に戻る（往復で情報が落ちない）。 */
function expectRoundTrip(hash: string) {
  expect(routeHash(parseRoute(hash))).toBe(hash);
}

describe('概要タブの現在地', () => {
  it('コンポーネントを path に載せて往復する', () => {
    expect(parseRoute('#/overview/cmp.alpha')).toEqual({ view: 'overview', componentId: 'cmp.alpha' });
    expect(routeHash({ view: 'overview', componentId: 'cmp.alpha' })).toBe('#/overview/cmp.alpha');
    expectRoundTrip('#/overview/cmp.alpha');
  });

  it('構成要素アンカーまで載せて往復する', () => {
    expect(parseRoute('#/overview/cmp.alpha/part/part.alpha.p2')).toEqual({
      view: 'overview',
      componentId: 'cmp.alpha',
      partId: 'part.alpha.p2',
    });
    expectRoundTrip('#/overview/cmp.alpha/part/part.alpha.p2');
  });

  it('既定のコンポーネントは URL に書かない（素の #/overview が既定を指す）', () => {
    expect(parseRoute('#/overview')).toEqual({ view: 'overview' });
    expect(routeHash({ view: 'overview' })).toBe('#/overview');
  });

  it('コンポーネントが無ければ構成要素アンカーも書かない（単独では解決できないため）', () => {
    expect(routeHash({ view: 'overview', partId: 'part.alpha.p2' })).toBe('#/overview');
  });

  it('id に含まれる記号をエスケープして往復する', () => {
    const route: Route = { view: 'overview', componentId: 'cmp/a b', partId: 'part#1' };
    const hash = routeHash(route);
    expect(parseRoute(hash)).toEqual(route);
  });

  it('旧ランディング #/home は現在地を持たない素の入口のまま', () => {
    // 既存ブックマークの意味を変えないための据え置き（router.ts）。
    expect(parseRoute('#/home')).toEqual({ view: 'home' });
    expect(parseRoute('#/home/cmp.alpha')).toEqual({ view: 'home' });
  });
});

describe('既存ルートへの回帰が無いこと', () => {
  // 概要タブの追加で他のルートの解釈が変わっていないこと（受入7 の静的版）。
  it.each([
    '#/browse',
    '#/browse/tag/req.x',
    '#/browse/tx/tx.y',
    '#/spec/req.x',
    '#/vocab/act.user.z',
    '#/flow/act.user.z',
    '#/decision/01KXDFD2S9MWHKH4RC52SJ2A8N',
    '#/browse?q=viewer',
    '#/browse?q=viewer&k=tag',
    '#/decisions?dk=tag&dt=req.x',
    '#/flow?ft=req.x',
  ])('%s が往復する', (hash) => {
    expectRoundTrip(hash);
  });

  it('未知のビューは既定ルートへ落ちる', () => {
    expect(parseRoute('#/nope')).toEqual({ view: 'overview' });
    expect(parseRoute('')).toEqual({ view: 'overview' });
  });
});
