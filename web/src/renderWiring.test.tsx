// @vitest-environment happy-dom
import { render } from 'preact';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  DEC,
  DEFAULT_CONFIG,
  TAGS,
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
import type { FakeServer, HarnessConfig, Mounted } from './testing/renderHarness';

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
//   ・絞った集合ではないものを並べる（見出しの件数と行が食い違う。**0件の境界も踏む**）
//   ・条件の軸を1つだけ落とす／握り潰す。**5軸すべて**（フリーワード `q`・対象の種別
//     `dk`・タグ `dt`・効力 `dc`・期間 `dp`）と対象・向き（`on`/`scope`）を、
//     それぞれ URL から起こして行で確かめている
//   ・純関数へ渡す材料を痩せさせる（`governs`・実効タグ集合・効力の判定・期間の「いま」）
//   ・外から来た条件（Back/Forward・行のチップ）を取り込まない。**軸を1つだけ
//     取り込まない形も含む**（タグの軸も外から動かしている）
//   ・画面の操作を URL へ書き戻さない／**飛んでいる応答が返った拍子に操作が消える**
//   ・判定は呼ぶが結果を捨てる（ナビの点灯・1件で開く既定）
//   ・名乗りと中身を食い違わせる（掛かっていない widget・効かない候補を出す）
//   ・起こしている面のどれかで、harness が答えを持たない口を叩き始める
//   ・**設定が決めるはずの語を、プログラムに literal で書く**（役割の呼び名）。
//     config の呼び名を変えて画面の文言が動くかを見るので、**どう綴っても**落ちる
//   ・**コンポーネントに直接付いた遷移を、シートが描かない／二重に描く**
//
// ## このガードが落とさないもの（名指しする）
//
//   1. **Go 側の答えそのものの正しさ。** 偽サーバは corpus に書いてある答えを返す
//      だけで、「この記録を支配する規則は何か」を導出しない（導出を viewer 側に
//      もう1本書かない・01KYKS4Y56FAHRVCWKMQJK4RT6 条項5）。ここが守るのは
//      「Go が返した答えが行まで届くか」であって「Go の答えが正しいか」ではない。
//   2. **見え方。** happy-dom はレイアウトを計算しない。CSS の当たり方・重なり・
//      色・折り返しは1つも見ていない（実機確認が担う）。
//   3. **この file が起こしていない面。** 起こすのは意思決定の一覧・概要（構成要素の
//      規則欄／直下の振る舞いの欄／空状態3種）と、ナビの点灯を見るための タグ・
//      語彙の**ヘッダだけ**である。`#/flow` `#/browse` `#/spec` `#/config` は1本も
//      通っていない。面を足したら、その面にも同じ形を置き忘れる
//      （`CLAUDE.md` 5 が名指しした型）。
//      ⚠️ **役割の呼び名の検査も、この射程の中にしかない。** 概要タブの外
//      （タグ・意思決定・語彙・設定）に役割名を literal で書いても落ちない。
//      ⚠️ 概要の中でも、**描かれた枝しか見ていない**——畳まれた欄の中身や、
//      条件を満たさずに描かれなかった分岐に literal を書いても落ちない。
//      この射程は実際に狭かった: 最初は既定で選ばれる1枚のシートしか見ておらず、
//      **構成要素を持たないコンポーネント側の欄に literal を書いた変異が素通りした。**
//      いまは両方の状態を踏むが、「全部の状態を踏んでいる」とは名乗らない。
//   4. **静的書き出し（`window.__SCHOLIA_STATIC__`）の経路。** api.ts は live と
//      static の2本を持つが、ここが通すのは live（HTTP）側の1本だけ。static 側だけ
//      壊す変異はここでは落ちない。
//   5. **規模・実データ特有の形。** corpus は意思決定7件・タグ8件。**`config` の形も
//      実データとは別物である**——実データは主題種別に役割 component を**宣言して**
//      いる（`.scholia/config.json`）のに対し、corpus の既定 config は**宣言せず**
//      リテラル id `component`/`part` に当たる形にしてある（宣言していない
//      プロジェクトの経路を常時踏ませるため）。宣言した側の経路は
//      `installFakeServer({ config })` で個別に起こす。`roots` の中身も
//      `facetKinds` の数も実データとは違う。実データの規模・形でだけ出る欠陥は範囲外。
//   6. **入口が provider の「正しい複製」を持つ形。** `main.tsx` が `AppRoot` を通らず
//      同じ入れ子を書き写した場合、振る舞いは同じなのでここは緑のまま通り、
//      `root.tsx` は黙って死に枝になる。**ソース照合（「`AppRoot` と書いてあるか」）で
//      縛ることはしない**——それは `CLAUDE.md` 2 が「ガードと呼ばない」と定めた形で、
//      別の綴りで書けば通るものを1つ増やすだけだから。**振る舞いが壊れる形**
//      （provider の脱落）は下の「製品の入口」の検査が実際に落とす。
//   7. **ソース照合が肩代わりしている型。** 「カードの入口を行またぎのゲートで包む」等、
//      **描かれる場所の構造**を見る検査は `surfaceWiring.test.ts` にあり、この file は
//      落とさない。描画ガードを入れてもソース照合が要らなくなったわけではない。
//
// ⚠️ **「全部捕まえる」とは名乗らない。** 上の7つは、この形のままでは原理的に
// 捕まえられない（1・4・5 は corpus と経路の選び方、2 は DOM 実装の性質、3 は
// 起こしていない面、6・7 は意図的に置かなかった歯止め）。埋めるなら別の手段が要る。

let server: FakeServer | null = null;
let mounted: Mounted | null = null;

afterEach(async () => {
  const unhandled = server?.unhandled ?? [];
  mounted?.unmount();
  mounted = null;
  server?.restore();
  server = null;
  vi.useRealTimers();
  resetBrowserState();
  // ⚠️ **URL の後始末が次のテストへ漏れないようにする。** hash の代入で生まれる
  // hashchange は次の macrotask で届くので、ここで流しておかないと**次のテストが
  // 起こした画面**に届く。届いた先で何が起きるかは製品側の作りの問題（それは
  // 上の検査が値で守る）だが、テストが互いの URL 操作を受け取る形自体を残さない。
  await new Promise((r) => setTimeout(r, 0));
  // ⚠️ **起こした面すべてで見る。** 1つの URL の中だけで見ていた頃は、別の面でだけ
  // 新しい口を叩き始める形が素通りした。空でなければ、製品が叩いた口に harness が
  // 答えていない＝この harness はその面の経路を1式ぶん迂回している。
  expect(unhandled, '製品が投げた要求に harness が答えていない').toEqual([]);
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
    最後に expect するのは、食い違ったときに「何が出ていたか」を出すため。
    ⚠️ 「まだ来ていない」と「来ない」を赤の文面で見分けられるよう、**何 ms 待って
    何が出ていたか**を添える（追跡がここで止まらないように）。 */
async function expectRows(host: HTMLElement, want: string[]): Promise<void> {
  const started = performance.now();
  await waitFor(() => rowMarkers(host).join(',') === want.join(','), `行が ${want.join(',')} になる`, 1500).catch(() => {});
  const waited = Math.round(performance.now() - started);
  expect(rowMarkers(host), `${waited}ms 待った時点の行`).toEqual(want);
}

async function expectHash(match: RegExp): Promise<void> {
  const started = performance.now();
  await waitFor(() => match.test(window.location.hash), `URL が ${match} を含む`, 1500).catch(() => {});
  const waited = Math.round(performance.now() - started);
  expect(window.location.hash, `${waited}ms 待った時点の URL`).toMatch(match);
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

  it('1件も当たらないときは 0 件と名乗る（境界）', async () => {
    // ⚠️ **境界を踏まない検査は、境界でだけ嘘をつく変異を通す。** 件数に下駄を履かせる
    // （0件のとき「1件」と名乗る）変異は、行が1件の検査だけでは落ちない。
    const host = await open('#/decisions?q=' + encodeURIComponent('ぬりかべ'));
    await expectRows(host, []);
    expect(headingCount(host)).toBe(0);
    expect(headingCount(host)).toBe(rowMarkers(host).length);
  });

  it('対象の種別で絞り込める（dk）', async () => {
    const host = await open('#/decisions?dk=vocab');
    // 語彙を対象にした意思決定は D4 だけ。
    await expectRows(host, ['D4']);
    expect(headingCount(host)).toBe(1);
  });

  it('期間で絞り込める（dp）', async () => {
    // 期間は「いま」を材料に取る。実時計のままだと**日が変われば結果が変わる**検査に
    // なるので、Date だけを固定する（setTimeout は本物のまま＝書き戻しの debounce は
    // 実時間で動く）。
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-07-20T00:00:00Z'));
    const host = await open('#/decisions?dp=30d');
    // 30日以内は D7（2026-07-05・15日前）だけ。D6（45日前）以前は入らない。
    await expectRows(host, ['D7']);
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

  it('外から変わったのがタグの軸でも追随する（軸を1つだけ落とす形を通さない）', async () => {
    // ⚠️ **取り込みを「1つの軸だけ」落とす変異は、その軸を外から動かさない限り
    // 落ちない。** 対象と効力だけを動かしていた頃は、タグ AND の軸だけ取り込まない
    // 書き換え（利用者から見れば「Back でタグの絞り込みだけ戻らない」うえ、
    // 300ms 後の書き戻しで URL 側が古いタグに上書きされる）が素通りした。
    const host = await open('#/decisions');
    await expectRows(host, ['D7', 'D6', 'D4', 'D3', 'D2', 'D1']);
    window.location.hash = '#/decisions?dt=req.viewer.filter';
    await expectRows(host, ['D4', 'D3', 'D2']);
    // 取り込んだ条件が、そのまま書き戻しの材料になっていること（古い値で URL を
    // 上書きし返さない）。
    await new Promise((r) => setTimeout(r, 400));
    expect(window.location.hash).toBe('#/decisions?dt=req.viewer.filter');
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

  it('飛んでいる要求が返ってきても、その間の操作が消えない', async () => {
    // ⚠️ **これは harness が実際に見つけた欠陥の再発防止である。**
    // 実サーバは即答しない。応答が1つ飛んでいる最中に利用者が widget を触ると、
    // 遅れて返ってきた応答が App を再描画し、その拍子に「外から来た条件の取り込み」が
    // 走って**利用者の操作を URL 側の古い値で上書き**していた（一覧にも URL にも
    // 残らない）。速さの問題ではなく順序の問題なので、待ち時間ではなく**順序**で
    // 再現する——応答を握ったまま操作し、そのあとで返す。
    server = installFakeServer({ hold: ['/api/reviews'] });
    mounted = mountApp('#/decisions?on=tag:req.cli');
    const host = mounted.host;
    await waitFor(() => !!host.querySelector('.browse-card-list'), '一覧が描かれる');
    await expectRows(host, ['D6']);
    selectValue(railSelects(host)[1], 'superseded');
    await expectRows(host, ['D5']);
    server.release(); // 握っていた応答がここで返る＝App が再描画される
    await new Promise((r) => setTimeout(r, 50));
    expect(rowMarkers(host), '飛んでいた応答が返った拍子に操作が消えた').toEqual(['D5']);
    await expectHash(/dc=superseded/);
  });

  it('URL が変わっていない hashchange が届いても、その間の操作が消えない', async () => {
    // ⚠️ **これも harness が実際に見つけた欠陥の再発防止である。**
    // `hashchange` は**中身が同じでも届きうる**——URL を代入してから listener が
    // 張られるまでの分や、1つの処理の中で2回代入したときの2発目は、listener が読む
    // 時点では既に現在の hash と同じになっている。それを無条件に取り込むと、内容の
    // 同じ**新しい Route** が作られ、下流の「外から来た条件の取り込み」が「URL が
    // 変わった」と受け取って、**その瞬間に利用者が操作していた値を古い値で上書き**する。
    //
    // 実測では、この順序が偶然噛み合って 60回に2回だけ赤くなる形で出ていた。
    // **たまたま噛み合うのを待つ検査にはしない**——届く event を明示的に届けて、
    // 順序として固定する。
    const host = await open('#/decisions?on=tag:req.cli');
    await expectRows(host, ['D6']);
    selectValue(railSelects(host)[1], 'superseded');
    await expectRows(host, ['D5']);
    const hashBefore = window.location.hash;
    window.dispatchEvent(new Event('hashchange'));
    await new Promise((r) => setTimeout(r, 50));
    expect(window.location.hash, 'この検査は URL を変えない（変えたら別の検査になる）').toBe(hashBefore);
    expect(rowMarkers(host), '取り込むものが無い hashchange で操作が消えた').toEqual(['D5']);
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
    // タブを名乗らないレンズ（語彙）も、deep link で来たら点灯先がある。
    // ⚠️ **起こしていない面では、その面でだけ点灯先を取り違える変異が通る。**
    ['#/vocab', 'タグ'],
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
  // ⚠️ かつてこの repo の実データには役割 kind を持つタグが1件も無く、概要タブは
  // 実機で空だった（＝実機確認では1行も確かめられない面だった）。いまは
  // `.scholia/config.json` が主題種別に役割 component を宣言しており、実機でも
  // シートが出る。ただし **corpus の config は実データと別物**である点は変わらない
  // （下の「射程」5 を参照）。
  // ソース側の列挙検査（surfaceWiring.test.ts）だけが歯止めだった時期があり、その
  // 列挙は「入口を変数に束ねてからゲートで包む」変異を通した（レビュアの N11）。
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

// ---------------------------------------------------------------------------
// 役割の呼び名は config が決める（01KYCC2THS5RX3HB27SQGFWSA5）
// ---------------------------------------------------------------------------
//
// **原理を1つ置く**（綴りを1つずつ列挙しない・`CLAUDE.md` 2）:
// **呼び名を config で変えたら、画面の文言も変わる。**
// 役割名をプログラムに literal で書いた実装は、**どう綴っても**この検査を通れない
// ——config を変えても出力が動かないからである。だから「『コンポーネント』と
// 書いていないか」を探すのではなく、**動くかどうか**を見る。
//
// **何に落ちるか**: 概要タブの下に描かれる**どの面**であれ、役割名を literal で
// 抱えていれば落ちる（新しく足した面も、そこが描かれる限り自動的に射程に入る——
// `CLAUDE.md` 5 が名指しした「新しい面に置き忘れる」型への手当て）。
// **何に落ちないか**: (1) 概要タブの外（タグ・意思決定・語彙・フロー・設定）。
// (2) 描かれない枝（畳まれた欄の中身・条件を満たさない分岐）。
// (3) 属性やクラス名の中の role 名（textContent しか見ない）。
// (4) 呼び名を config から取ってはいるが**間違った kind の**呼び名を取っている形。
const ROLE_LITERALS = ['コンポーネント', 'component', 'Component'];

/** 役割 component を `kind` が担う、と宣言した config を作る。 */
function configDeclaringComponent(kind: string, label: string): HarnessConfig {
  return {
    ...DEFAULT_CONFIG,
    tagKinds: DEFAULT_CONFIG.tagKinds.map((d) => (d === kind ? { id: kind, behaviors: ['component'] } : d)),
    tagKindLabels: { ...DEFAULT_CONFIG.tagKindLabels, [kind]: label },
  };
}

async function openOverview(opts: { config?: HarnessConfig; tags?: typeof TAGS } = {}): Promise<HTMLElement> {
  server = installFakeServer(opts);
  mounted = mountApp('#/overview');
  const host = mounted.host;
  await waitFor(() => !!host.querySelector('.overview-view'), '概要が描かれる');
  // ⚠️ 読み込み中も `.overview-empty` で描かれる。**その状態を空状態と読むと、
  // 何を検査しても「読み込み中…」に対する検査になる**（実際にこの罠を踏んだ）。
  // 中身が決まるまで待ってから読む。
  await waitFor(
    () => !!host.querySelector('.overview-sheet') || (host.querySelector('.overview-empty')?.textContent || '') !== '読み込み中…',
    '概要の中身が決まる（読み込み中を抜ける）',
  );
  return host;
}

/** 構造ツリーの行を名前で押す（別のコンポーネントのシートへ移る）。 */
async function selectComponent(host: HTMLElement, name: string): Promise<void> {
  await waitFor(() => !!host.querySelector('.overview-tree'), '構造ツリーが描かれる');
  const row = Array.from(host.querySelectorAll<HTMLElement>('.overview-tree a, .overview-tree button')).find((el) => (el.textContent || '').includes(name));
  expect(row, `構造ツリーに「${name}」の行が無い`).toBeTruthy();
  row!.click();
  await waitFor(() => (host.querySelector('.overview-sheet')?.textContent || '').includes(name), `${name} のシートが出る`);
}

describe('画面が名乗る役割の呼び名は、プロジェクトの設定に追随する', () => {
  it('呼び名を変えると、シートが出ている状態の文言も変わる（literal は追随できない）', async () => {
    const host = await openOverview({ config: configDeclaringComponent('component', 'ZZ役割ZZ') });
    // ⚠️ **1つのシートだけ見ても足りない。** 構成要素を持つコンポーネントと持たない
    // コンポーネントでは描かれる欄が違い、片方だけ見ると**もう片方に literal を
    // 書いた変異が素通りする**（実際に素通りした）。両方の状態の文言を見る。
    const seen: string[] = [];
    await selectComponent(host, 'ビューア画面'); // 構成要素あり
    seen.push(host.querySelector('.overview-sheet')!.textContent || '');
    await selectComponent(host, '端末'); // 構成要素なし＝直下の振る舞いの欄が出る
    seen.push(host.querySelector('.overview-sheet')!.textContent || '');
    expect(seen.some((s) => s.includes('振る舞い')), '直下の振る舞いの欄が1つも描かれていない＝検査が空振りしている').toBe(true);
    // 役割名を literal で抱えた実装は、config をどう変えてもこの語を出し続ける。
    for (const text of seen) {
      for (const lit of ROLE_LITERALS) {
        expect(text, `設定の呼び名を変えたのに、画面がまだ「${lit}」と言っている`).not.toContain(lit);
      }
    }
  });

  it('役割は宣言されているがそのタグが1件も無いとき、呼び名つきで「まだ無い」と言う', async () => {
    const host = await openOverview({
      config: configDeclaringComponent('component', 'ZZ役割ZZ'),
      tags: TAGS.filter((t) => t.kind !== 'component'),
    });
    const text = host.querySelector('.overview-empty')?.textContent || '';
    expect(text, '空状態が、設定した呼び名で語っていない').toContain('ZZ役割ZZ');
    for (const lit of ROLE_LITERALS) expect(text).not.toContain(lit);
  });

  it('役割が宣言されていないときは、呼び名を捏造せず「役割を宣言する」と案内する', async () => {
    // DEFAULT_CONFIG は behaviors を宣言していない＝リテラル id へのフォールバック。
    // タグ側も役割 kind を持たないので、空状態になる。
    const host = await openOverview({ tags: TAGS.filter((t) => t.kind !== 'component') });
    const text = host.querySelector('.overview-empty')?.textContent || '';
    expect(text, '空状態が描かれていない').not.toBe('');
    // 生の kind id をそのまま画面に出さない（01KYCC2TF3NW3JRSSRK9ZHN078）。
    for (const lit of ROLE_LITERALS) expect(text, `宣言が無いのに「${lit}」という呼び名を出している`).not.toContain(lit);
    // 利用者がやることは「タグを作る」ではなく「役割を宣言する」。
    expect(text).toContain('宣言');
  });
});

// ---------------------------------------------------------------------------
// 構成要素を持たないコンポーネントの、直下の遷移
// ---------------------------------------------------------------------------
//
// **何に落ちるか**: 直下の遷移がカードとして1枚も描かれない形（＝この単位が直した
// 欠陥そのもの）、および構成要素があるのに直下の分まで描いて二重に出す形。
// **何に落ちないか**: カードの中身の正しさ（きっかけ／前提／結果の対応は
// sheetModel.test.ts と既存の検査が見る）。見え方・重なり・折り返し（実機確認）。
describe('遷移がコンポーネントに直接付いていても、シートに振る舞いとして出る', () => {
  it('構成要素を持たないコンポーネントで、直下の遷移がカードになる', async () => {
    // comp.cli は構成要素を持たず、T-run-cli が直接付いている。
    const host = await openOverview();
    await selectComponent(host, '端末');
    await waitFor(() => (host.querySelector('.overview-own-behaviors')?.querySelectorAll('.overview-behavior').length ?? 0) > 0, '直下の振る舞いカードが描かれる');
    const cards = host.querySelectorAll('.overview-own-behaviors .overview-behavior');
    // ⚠️ 1枚ちょうど。comp.cli は子コンポーネント comp.cli.sub を持ち、そちらにも
    // 遷移が1本ある——**祖先展開込みの索引を使うと2枚になる**（子の振る舞いが親の
    // シートに再掲される）。件数だけでその混入を落とす。
    expect(cards.length, '直下に付いた遷移がカードになっていない／子の分まで混ざっている').toBe(1);
    // カードの中身が、その遷移のものであること（空のカードを数えても検査にならない）。
    expect(cards[0].textContent).toContain('コマンドを実行する');
    expect(cards[0].textContent).toContain('結果が印字される');
    expect(host.querySelector('.overview-own-behaviors')!.textContent, '子コンポーネントの振る舞いが親のシートに出ている').not.toContain('下位を呼ぶ');
  });

  it('構成要素を持つコンポーネントでは、直下の欄を出さない（同じ遷移を2度出さない）', async () => {
    const host = await openOverview();
    // 既定の選択は comp.viewer（構成要素 part.list を持つ）。
    await waitFor(() => !!host.querySelector('.overview-part-head'), '構成要素の欄が描かれる');
    expect(host.querySelector('.overview-own-behaviors'), '構成要素があるのに直下の欄も出ている').toBeNull();
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
