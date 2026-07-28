// @vitest-environment happy-dom
import { render } from 'preact';
import { afterEach, describe, expect, it } from 'vitest';
import {
  DEC,
  activeNavLabels,
  headingCount,
  installFakeServer,
  mountApp,
  railSelects,
  resetBrowserState,
  rowMarkers,
  selectValue,
  typeQuery,
  waitFor,
} from './testing/renderHarness';
import type { FakeServer, Mounted } from './testing/renderHarness';

// ===========================================================================
// 描画を1回起こして「URL に書かれた条件 → 一覧に出た行」まで通す配線ガード
// ===========================================================================
//
// ## なぜこれが要るのか
//
// 判断は純関数へ出してある（decisionFilter / decisionScope / navActive）。それらの
// **値の正しさ**は、それぞれの単体テストが守っている。守られていなかったのは
// **純関数と DOM のあいだ**——関数は正しく、正しく呼ばれてもいて、その答えを1つ
// 外側で捨てているか、痩せた材料を渡している、という層である。
//
// レビュアが当てた11通の変異のうち10通がここを通り抜けた。1トークンの書き換え
// （`governs: governs ?? undefined` → `governs: undefined`）で「支配する規則」が
// 常に0件になり、実測 8/75 のタグが 0件の面に着くという直したばかりの欠陥が復活
// するのに、テストは全部緑だった。
//
// この層はソース文字列の照合では閉じられない（同じ意味を別の綴りで書けば通る・
// `CLAUDE.md`「配線ガードの書き方」2）。**描画を起こして、出た結果を値として読む**
// のがこの file の役目である。
//
// ## このガードが落とすもの（射程・`CLAUDE.md` 6）
//
// 「URL の条件を入力にして製品の合成ルート（root.tsx）を起こし、出た**行の集合と
// 順序・見出しの件数・点灯タブ・出ている widget**を値として読む」。したがって、
// 綴りに関係なく次の型で落ちる:
//
//   ・絞った集合ではないものを並べる（見出しの件数と行が食い違う）
//   ・純関数へ渡す材料を痩せさせる（`governs` / 実効タグ集合を空にする）
//   ・外から来た条件（Back/Forward・行のチップ）を取り込まない
//   ・画面の操作を URL へ書き戻さない
//   ・判定は呼ぶが結果を捨てる（ナビの点灯・1件で開く既定）
//   ・名乗りと中身を食い違わせる（掛かっていない widget・効かない候補を出す）
//
// ## このガードが落とさないもの（名指しする）
//
//   1. **Go 側の答えそのものの正しさ。** 偽サーバは corpus に書いてある答えを返す
//      だけで、「この記録を支配する規則は何か」を導出しない（導出を viewer 側に
//      もう1本書かない・01KYKS4Y56FAHRVCWKMQJK4RT6 条項5）。ここが守るのは
//      「Go が返した答えが行まで届くか」であって「Go の答えが正しいか」ではない。
//   2. **見え方。** happy-dom はレイアウトを計算しない。CSS の当たり方・重なり・
//      色・折り返しは1つも見ていない（実機確認が担う）。
//   3. **この file が起こしていない面。** 起こすのは意思決定の一覧と、ナビの点灯を
//      見るための概要・タグの**ヘッダだけ**である。面を足したら、その面にも同じ形を
//      置き忘れる（`CLAUDE.md` 5 が名指しした型）。
//   4. **静的書き出し（`window.__SCHOLIA_STATIC__`）の経路。** api.ts は live と
//      static の2本を持つが、ここが通すのは live（HTTP）側の1本だけ。static 側だけ
//      壊す変異はここでは落ちない。
//   5. **規模・実データ特有の形。** corpus は6件の意思決定で、実データ（276レコード）
//      の規模や特殊な文字で初めて出る欠陥は範囲外。
//
// ⚠️ **「全部捕まえる」とは名乗らない。** 上の5つは、この形のままでは原理的に
// 捕まえられない（1・4・5 は corpus と経路の選び方、2 は DOM 実装の性質、3 は
// 起こしていない面）。埋めるなら別の手段が要る。

let server: FakeServer | null = null;
let mounted: Mounted | null = null;

afterEach(() => {
  mounted?.unmount();
  mounted = null;
  server?.restore();
  server = null;
  resetBrowserState();
});

/** URL を入力にして製品を起こし、一覧が描かれるまで待つ。 */
async function open(hash: string): Promise<HTMLElement> {
  server = installFakeServer();
  mounted = mountApp(hash);
  const host = mounted.host;
  await waitFor(() => !!host.querySelector('.browse-card-list'), `一覧が描かれる（${hash}）`);
  return host;
}

/** 期待の行並びに落ち着くのを待ってから、値として突き合わせる。待ちで失敗させず
    最後に expect するのは、食い違ったときに「何が出ていたか」を出すため。 */
async function expectRows(host: HTMLElement, want: string[]): Promise<void> {
  await waitFor(() => rowMarkers(host).join(',') === want.join(','), `行が ${want.join(',')} になる`, 1500).catch(() => {});
  expect(rowMarkers(host)).toEqual(want);
}

async function expectHash(match: RegExp): Promise<void> {
  await waitFor(() => match.test(window.location.hash), `URL が ${match} を含む`, 1500).catch(() => {});
  expect(window.location.hash).toMatch(match);
}

describe('URL に書かれた条件が、一覧の行になって出る（01KYKS4Y56FAHRVCWKMQJK4RT6）', () => {
  it('対象＝タグ・向き＝配下（既定）: その配下の意思決定だけが、新しい順に並ぶ', async () => {
    const host = await open('#/decisions?on=tag:req.viewer');
    // req.viewer の配下に着く4件（実効タグの祖先クロージャで届く）。req.cli 側の
    // 2件は入らない。並びは新しい順。
    await expectRows(host, ['D4', 'D3', 'D2', 'D1']);
  });

  it('見出しに出る件数と、実際の行数が一致する（フリーワードで絞ったとき）', async () => {
    const host = await open('#/decisions?q=' + encodeURIComponent('折りたたみ'));
    // 「折りたたみ」は D3 の why にしか出てこない（タグ名・語彙ラベルにも無い）。
    await expectRows(host, ['D3']);
    // 名乗る件数と中身が食い違わないこと——候補の母数を並べる変異はここで落ちる。
    expect(headingCount(host)).toBe(rowMarkers(host).length);
    expect(headingCount(host)).toBe(1);
  });

  it('向き＝支配する規則で 0件にならない（8/75 が 0件に着いていた欠陥そのもの）', async () => {
    const host = await open('#/decisions?on=tag:req.viewer.filter&scope=governing');
    // /api/governs が返した2件（自身への D2・祖先経由の D1）が、そのまま行になる。
    await expectRows(host, ['D2', 'D1']);
    expect(headingCount(host)).toBe(2);
  });

  it('タグでの絞り込み（AND）が効く', async () => {
    const host = await open('#/decisions?dt=req.viewer.filter');
    // 実効タグに req.viewer.filter を含む3件。req.viewer 直付けの D1 は含まない。
    await expectRows(host, ['D4', 'D3', 'D2']);
    expect(headingCount(host)).toBe(3);
  });

  it('外から条件が変わったら画面が追随する（Back/Forward・行のチップ）', async () => {
    const host = await open('#/decisions?on=tag:req.cli&dc=all');
    await expectRows(host, ['D6', 'D5']);
    // 画面の外で URL だけが変わる＝Back/Forward と、行のチップから移る経路が
    // 通る道。URL は正のまま、画面が追随しなければならない。
    window.location.hash = '#/decisions?on=tag:req.viewer&dc=all';
    await expectRows(host, ['D4', 'D3', 'D2', 'D1']);
  });

  it('画面の widget を操作すると URL に載る（reload/バックで消えない）', async () => {
    const host = await open('#/decisions?on=tag:req.cli');
    await expectRows(host, ['D6']); // 既定の効力＝現行のみ
    const currency = railSelects(host)[1];
    expect(currency, '現行性の select が無い').toBeTruthy();
    selectValue(currency, 'superseded');
    // 一覧には即座に効き、URL へは debounce して載る。
    await expectRows(host, ['D5']);
    await expectHash(/dc=superseded/);
  });
});

describe('面ごとに正しいタブが点灯する（01KYKS4Y56FAHRVCWKMQJK4RT6 条項6）', () => {
  // 判定そのもの（ViewName 10種）は navActive.test.ts が値で守る。ここが見るのは
  // **その答えがヘッダの class まで届いているか**——判定を呼んだうえで結果を捨てる
  // 変異は、純関数のテストでは落ちない。
  const cases: Array<[string, string]> = [
    ['#/decisions', '意思決定'],
    ['#/tags', 'タグ'],
    ['#/overview', '概要'],
  ];
  for (const [hash, label] of cases) {
    it(`${hash} では「${label}」だけが点く`, async () => {
      server = installFakeServer();
      mounted = mountApp(hash);
      const host = mounted.host;
      await waitFor(() => host.querySelectorAll('.topbar-nav-btn').length > 0, 'ヘッダが描かれる');
      expect(activeNavLabels(host)).toEqual([label]);
    });
  }
});

describe('1件を名指しした URL は、名乗りと中身が一致する（同 条項2・3）', () => {
  it('その1件だけが、開いた状態で出る', async () => {
    const host = await open(`#/decisions?on=decision:${DEC.D2}`);
    await expectRows(host, ['D2']);
    // 1件に絞られた結果は畳まれずに着地する（条項2）。全文の器が出ていること。
    expect(host.querySelector('.decision-row-body'), '1件なのに畳まれている').not.toBeNull();
  });

  it('掛かっていない絞り込み widget を出さない', async () => {
    const host = await open(`#/decisions?on=decision:${DEC.D2}`);
    await expectRows(host, ['D2']);
    expect(railSelects(host).map((s) => s.className)).toEqual([]);
    expect(host.querySelector('.decisions-named-note'), '名指し中である旨の注記が無い').not.toBeNull();
  });

  it('効かないタグ候補を出さない', async () => {
    const host = await open(`#/decisions?on=decision:${DEC.D2}`);
    await expectRows(host, ['D2']);
    // D2 の実効タグは「絞り込み」「ビューア」。名指し中はどちらも候補に出ない。
    typeQuery(host, '絞り');
    await new Promise((r) => setTimeout(r, 30));
    expect(host.querySelector('#browse-rail-listbox'), '名指し中なのに候補が開いている').toBeNull();
  });
});

describe('概要の文脈から、同じ条件の一覧へ踏める（同 条項1・概要側の入口）', () => {
  // ⚠️ この repo の実データには役割 kind（component/part）を持つタグが1件も無く、
  // 概要タブは**実機では空**になる。だから実機確認では1行も確かめられない面で、
  // ソース側の列挙検査（surfaceWiring.test.ts）だけが歯止めだった——その列挙は
  // 「入口を変数に束ねてからゲートで包む」変異を通す（レビュアの N11）。
  // ここでは**畳んだ状態で入口が出ていること**を、描画された DOM の値で見る。
  it('規則の欄を畳んだままでも入口が出ていて、その対象と向きを指している', async () => {
    server = installFakeServer();
    mounted = mountApp('#/overview');
    const host = mounted.host;
    await waitFor(() => !!host.querySelector('.overview-part-head'), '概要の構成要素が描かれる');
    (host.querySelector('.overview-part-head') as HTMLElement).click();
    await waitFor(() => !!host.querySelector('.overview-rules-toggle'), '構成要素の規則の欄が描かれる');
    // 畳んだ状態であることを先に確かめる（開いた状態で見ても検査にならない）。
    expect(host.querySelector('.overview-rules-toggle')!.getAttribute('aria-expanded')).toBe('false');
    const link = host.querySelector('.overview-rules-list-link');
    expect(link, '規則の欄を畳んだ状態では入口に辿り着けない').not.toBeNull();
    // 対象＝その構成要素・向き＝own（インライン展開と同じ集合）。
    expect(link!.getAttribute('href')).toBe('#/decisions?on=tag:part.list&scope=own');
  });
});

describe('製品の入口が、この harness と同じ合成ルートを起こす', () => {
  // ⚠️ **この harness 自身が作った穴を塞ぐ。** provider の入れ子を root.tsx へ
  // 切り出したので、main.tsx が AppRoot を通らない形に変わっても、上の検査は
  // AppRoot を直に起こしていて全部緑のままになる——「harness は緑だが製品は
  // 起動しない」。だから**製品の入口モジュールそのものを読み込んで**、それが
  // 画面を出すことまで見る。
  it('main.tsx を読み込むと、#app に配線済みの画面が出る', async () => {
    server = installFakeServer();
    window.location.hash = '#/decisions';
    const app = document.createElement('div');
    app.id = 'app';
    document.body.appendChild(app);
    try {
      // 入口は import した時点で render する副作用モジュール。provider を1つでも
      // 迂回すると、その場で useLang()/useLookups() が投げる。
      await import('./main.tsx');
      await waitFor(() => !!app.querySelector('.browse-card-list'), '製品の入口から一覧が描かれる');
      expect(app.querySelectorAll('.topbar-nav-btn').length).toBeGreaterThan(0);
      expect(rowMarkers(app).length).toBeGreaterThan(0);
    } finally {
      // 入口が起こした木は自分で畳む。畳まずに DOM から外すだけだと、この file の
      // 後始末で偽サーバを戻した後も生きた画面が残り、本物の fetch を叩きにいく。
      render(null, app);
      app.remove();
    }
  });
});

describe('harness 自身の前提', () => {
  it('製品が投げた要求を1つも取りこぼしていない（迂回していないことの確認）', async () => {
    const host = await open('#/decisions?on=tag:req.viewer.filter&scope=governing');
    await expectRows(host, ['D2', 'D1']);
    // 空でなければ、製品が新しい口を叩き始めたのに harness が答えていない
    // ＝この harness は製品の経路を1式ぶん迂回している。
    expect(server!.unhandled).toEqual([]);
    // 逆に「何も叩いていない（＝偽サーバに何も届いていない）」形も、経路を
    // 通していない証拠になる。実際に governs まで問い合わせていること。
    expect(server!.requests.some((r) => r.startsWith('/api/governs'))).toBe(true);
    expect(host.querySelector('.decisions-list')).not.toBeNull();
  });
});
