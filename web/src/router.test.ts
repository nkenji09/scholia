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

// 絞り込み条件の「どの対象か」「どの向きか」（01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// この2条件が URL に載り往復することが、「permalink＝1件に絞り込んだ一覧」と
// 「カードから支配する規則の一覧へ踏める」の両方が成り立つ前提そのもの。往復が
// 壊れると、共有されたリンクが別の集合を開く／リロードで条件が黙って外れる。
describe('意思決定の一覧の対象と向き', () => {
  it('対象を載せて往復する', () => {
    expect(parseRoute('#/decisions?on=tag:req.x')).toEqual({ view: 'decisions', decisionOn: 'tag:req.x' });
    expectRoundTrip('#/decisions?on=tag:req.x');
  });

  it('向きまで載せて往復する（支配する規則の一覧＝カードのリンクが指す形）', () => {
    expect(parseRoute('#/decisions?on=tag:req.x&scope=governing')).toEqual({
      view: 'decisions',
      decisionOn: 'tag:req.x',
      decisionScope: 'governing',
    });
    expectRoundTrip('#/decisions?on=tag:req.x&scope=governing');
  });

  it('1件を名指しした形（旧単票の permalink）が往復する', () => {
    expect(parseRoute('#/decisions?on=decision:01KXDFD2S9MWHKH4RC52SJ2A8N')).toEqual({
      view: 'decisions',
      decisionOn: 'decision:01KXDFD2S9MWHKH4RC52SJ2A8N',
    });
    expectRoundTrip('#/decisions?on=decision:01KXDFD2S9MWHKH4RC52SJ2A8N');
  });

  it('既存の5条件と AND で合成できる（置き換えではない）', () => {
    const hash = '#/decisions?q=viewer&dk=tag&dt=req.x&dc=all&dp=30d&on=tag:req.y&scope=own';
    expect(parseRoute(hash)).toEqual({
      view: 'decisions',
      searchQuery: 'viewer',
      decisionTargetKind: 'tag',
      decisionTag: 'req.x',
      decisionCurrency: 'all',
      decisionPeriod: '30d',
      decisionOn: 'tag:req.y',
      decisionScope: 'own',
    });
    expectRoundTrip(hash);
  });

  it('条件が無ければ URL を汚さない', () => {
    expect(routeHash({ view: 'decisions' })).toBe('#/decisions');
  });

  it('「対象:id」の区切りが読める形で出る（%3A に畳まない）', () => {
    // 利用者が読んで組み立てる前提の条件なので、`on=tag%3Areq.x` にはしない。
    expect(routeHash({ view: 'decisions', decisionOn: 'tag:req.x' })).toBe('#/decisions?on=tag:req.x');
    // 既存の絞り込み（f=）も同じ形を持つので、そちらも読める形になる。
    expect(routeHash({ view: 'browse', searchFilters: 'tag:req.x' })).toBe('#/browse?f=tag:req.x');
    expectRoundTrip('#/browse?f=tag:req.x');
  });

  it('id の中の本物のコロンは巻き込まれず往復する', () => {
    // 区切りの `:` だけを戻す置換が、値の中の `:`（encodeURIComponent 済み＝%253A）
    // まで戻してしまうと、区切りが2つになって対象の解釈が壊れる。
    const route: Route = { view: 'decisions', decisionOn: 'tag:req.a:b' };
    expect(parseRoute(routeHash(route))).toEqual(route);
    const withFilters: Route = { view: 'browse', searchFilters: 'tag:' + encodeURIComponent('req.a:b') };
    expect(parseRoute(routeHash(withFilters))).toEqual(withFilters);
  });

  it('旧単票のルートは引き続き解決する（転送で生かすため）', () => {
    // 転送そのものは app.tsx（DecisionPermalinkRedirect）が行うが、ルートが
    // 解釈できなくなると転送する先が決まらない。
    expect(parseRoute('#/decision/01KXDFD2S9MWHKH4RC52SJ2A8N')).toEqual({
      view: 'decision',
      decisionId: '01KXDFD2S9MWHKH4RC52SJ2A8N',
    });
  });
});
