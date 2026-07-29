// @vitest-environment happy-dom
import { render } from 'preact';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  ALIAS_CONFIG,
  ALIAS_TAGS,
  DEC,
  DEFAULT_CONFIG,
  NO_ROOTS_CONFIG,
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
//   ・**役割の解決そのものをリテラル kind id に戻す**（`01KYCC2THS5RX3HB27SQGFWSA5`）。
//     慣用 id のタグが1件も無い corpus（`ALIAS_TAGS`/`ALIAS_CONFIG`）でシートが
//     描かれることを見るので、**この repo が乗っている経路そのもの**が射程に入る
//   ・**コンポーネントに直接付いた遷移を、シートが描かない／二重に描く**
//   ・**遷移の実効タグの合成に vocab を渡し忘れる**（純関数は正しいが材料が痩せる型）
//   ・**見出しの「現行ルール N」と、シートの中で開いて読める規則の数が食い違う**
//     ——ただし **`sheetRuleCount` の4系統のうち2系統だけ**（直下の振る舞い／構成要素
//     の自身の規則）。残り2系統は下記 8 を参照
//   ・**空状態が「タグがまだ無い」と「役割を宣言せよ」を取り違える**
//   ・**新しい欄を初期全開で置く**（段階的開示・01KYCC2TK3BEDA43TA61TPT4R5／
//     01KXDFD2SRHJJ0E551V240JMKT 条項3）。畳んだ状態で走査できる数が出ることも見る
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
//   3b. **役割 part / constraint / group の解決を、リテラル id へ戻す形。**
//      component については別 id の corpus（`ALIAS_*`）が落とすが、残る3役割は
//      corpus が慣用 id のままなので、宣言を読まずリテラルに戻しても答えが変わらない。
//      値としては `roleKinds.test.ts` が守るが、**配線は守られていない。**
//      3役割ぶんの別 id corpus を足せば埋まる（今回は足していない）。
//   4. **静的書き出し（`window.__SCHOLIA_STATIC__`）の経路。** api.ts は live と
//      static の2本を持つが、ここが通すのは live（HTTP）側の1本だけ。static 側だけ
//      壊す変異はここでは落ちない。**概要タブについては実機で確認した**が、機械の
//      歯止めは無い（`scholia export --html` を起こす harness が要る）。
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
//   8. **「現行ルール N」の突き合わせのうち、残り2系統。** `sheetRuleCount` は4系統
//      （構成要素の自身の規則／その配下の振る舞いの規則／直下の振る舞いの規則／制約の規則）
//      を足すが、**描画で突き合わせているのは前者2つだけ**である。
//      ⚠️ **`構成要素配下の振る舞い` と `制約` のスロットに痩せた材料を渡しても、
//      この file は落とさない**——corpus の part.list は振る舞いを持たず、制約タグは
//      1件も無いので、そのスロットを落としても見出しの数が動かない（実測: どちらの
//      変異も `sheetModel.test.ts`（純関数）だけが red で、描画側は緑のままだった）。
//      埋めるなら corpus に「規則を持つ振る舞いを配下に持つ構成要素」と「規則を持つ
//      制約タグ」を足す必要がある。**今は足していない。**
//
// ⚠️ **「全部捕まえる」とは名乗らない。** 上の8つは、この形のままでは原理的に
// 捕まえられない（1・4・5・8 は corpus と経路の選び方、2 は DOM 実装の性質、3 は
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
    // D9/D8 は概要シートの規則欄を数えるために足した corpus で、日付は最も古い
    // （絞り込みの振る舞いは何も変えていない・並びの末尾に付くだけ）。
    await expectRows(host, ['D7', 'D6', 'D4', 'D3', 'D2', 'D1', 'D9', 'D8']);
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

/** 直下の振る舞いの欄を開く（初期は畳まれている）。開く前後の枚数も返す。 */
async function openOwnBehaviors(host: HTMLElement): Promise<{ before: number; after: number; head: HTMLElement }> {
  await waitFor(() => !!host.querySelector('.overview-own-behaviors'), '直下の欄が描かれる');
  const box = host.querySelector('.overview-own-behaviors')!;
  const before = box.querySelectorAll('.overview-behavior').length;
  const head = box.querySelector('.overview-part-head') as HTMLElement;
  expect(head, '直下の欄に開閉の見出しが無い').toBeTruthy();
  head.click();
  await waitFor(() => (host.querySelector('.overview-own-behaviors')?.querySelectorAll('.overview-behavior').length ?? 0) > 0, '直下の振る舞いカードが描かれる');
  return { before, after: host.querySelector('.overview-own-behaviors')!.querySelectorAll('.overview-behavior').length, head };
}

/** 構造ツリーの行を名前で押す（別のコンポーネントのシートへ移る）。 */
async function selectComponent(host: HTMLElement, name: string): Promise<void> {
  await waitFor(() => !!host.querySelector('.overview-tree'), '構造ツリーが描かれる');
  // ⚠️ **行が出るまで待つ。** コンポーネントは束ねる段の下にあり、束ねる段が既定で開くのは
  // 最初の描画の**後**に走る効果なので、ツリーの器が出た瞬間にはまだ子が並んでいない。
  // ここを即時の assert にしていたときは「行が無い」で落ちた（corpus を実データと
  // 同じ形＝起点が束ねる段だけ、にした時点で顕在化した）。
  const find = () => Array.from(host.querySelectorAll<HTMLElement>('.overview-tree a, .overview-tree button')).find((el) => (el.textContent || '').includes(name));
  await waitFor(() => !!find(), `構造ツリーに「${name}」の行が無い`);
  find()!.click();
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
    // ⚠️ **テスト名が名乗る「まだ無い」まで検査する。** 呼び名の有無しか見ていなかった
    // ときは、「タグがまだ無い」と「役割を宣言せよ」を取り違える変異が素通りした
    // （テスト名が中身より広い＝`CLAUDE.md` 6 の型）。
    expect(text, '「タグがまだ無い」と言っていない').toContain('まだありません');
    expect(text, '宣言済みなのに「役割を宣言せよ」と案内している').not.toContain('宣言');
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
    await openOwnBehaviors(host);
    const own = host.querySelector('.overview-own-behaviors')!;
    const cards = own.querySelectorAll('.overview-behavior');
    // ⚠️ 2枚ちょうど。comp.cli には直接タグ付けされた T-run-cli と、**参照する vocab
    // 側にだけタグが付いた** T-cli-vocab がある。comp.cli は子コンポーネント
    // comp.cli.sub も持ち、そちらにも遷移が1本ある——**祖先展開込みの索引を使うと
    // 3枚になる**（子の振る舞いが親のシートに再掲される）。件数だけで両方を落とす。
    expect(cards.length, '直下の遷移がカードになっていない／子の分まで混ざっている').toBe(2);
    // カードの中身が、その遷移のものであること（空のカードを数えても検査にならない）。
    expect(own.textContent).toContain('コマンドを実行する');
    expect(own.textContent).toContain('結果が印字される');
    // ⚠️ **遷移の実効タグは tx.tags だけではない**——参照する vocab のタグも含む。
    // 純関数はその合成を守っているが、**配線が vocab を渡し忘れる**形は、この行が
    // 無ければ落ちない（「判断は正しいが1つ外側で痩せた材料を渡す」型）。
    expect(own.textContent, '語彙側にだけタグが付いた遷移が拾えていない').toContain('語彙経由で束ねる');
    expect(own.textContent, '子コンポーネントの振る舞いが親のシートに出ている').not.toContain('下位を呼ぶ');
  });

  /** 見出しの「現行ルール N」と、シートの中で実際に開いて読める規則の数を突き合わせる
   *  （01KYHW54B8ZXH0NEPH2J7N1X39 条項5）。⚠️ **数える対象が0件だと検査が空振りする**ので、
   *  0 でないことを先に確かめる。 */
  function expectRuleCountMatchesReadable(host: HTMLElement): void {
    const headline = /現行ルール\s*(\d+)/.exec(host.querySelector('.overview-cov-meta')?.textContent || '');
    expect(headline, '見出しに「現行ルール N」が出ていない').not.toBeNull();
    const toggles = Array.from(host.querySelectorAll('.overview-sheet .overview-rules-toggle'));
    const readable = toggles.reduce((n, el) => n + Number(/\((\d+)\)/.exec(el.textContent || '')?.[1] ?? 0), 0);
    expect(readable, '数える対象が0件では検査にならない').toBeGreaterThan(0);
    expect(Number(headline![1]), '見出しの件数と、シートの中で読める規則の数が食い違っている').toBe(readable);
  }

  it('見出しの「現行ルール N」と、シートの中で開いて読める規則の数が一致する（直下の振る舞い）', async () => {
    // 01KYHW54B8ZXH0NEPH2J7N1X39 条項5。⚠️ **これは decision の条項なのに、直下の
    // 振る舞いカードの分については機械の歯止めが1つも無かった**——算入を落としても
    // 全部緑だった。数える対象（T-run-cli / T-cli-vocab への決定）を corpus に置き、
    // 見出しの数と、実際に読める規則の数を突き合わせる。
    const host = await openOverview();
    await selectComponent(host, '端末');
    // 畳んだままでは規則のトグルが描かれない（＝数える対象がゼロになって検査が空振りする）。
    await openOwnBehaviors(host);
    expectRuleCountMatchesReadable(host);
  });

  it('見出しの「現行ルール N」と、開いて読める規則の数が一致する（構成要素を持つシート）', async () => {
    // ⚠️ **上の1本だけでは足りなかった。** 突き合わせが「構成要素0・制約0 のシート」
    // でしか走らないので、**構成要素のスロットに痩せた材料を渡しても落ちなかった**
    // ——`sheetRuleCount` は4系統（構成要素／その配下の振る舞い／直下の振る舞い／制約）
    // を足すのに、検査が踏むのは直下の1系統だけで、射程の名乗りが実際より広かった。
    // comp.viewer は構成要素 part.list を持ち、そこに規則（D7）が付いている。
    const host = await openOverview();
    await selectComponent(host, 'ビューア画面');
    // 構成要素の欄も初期は畳まれている（畳んだままだと規則のトグルが描かれない）。
    await waitFor(() => !!host.querySelector('.overview-part-head'), '構成要素の欄が描かれる');
    (host.querySelector('.overview-part-head') as HTMLElement).click();
    await waitFor(() => !!host.querySelector('.overview-rules-toggle'), '構成要素の規則の欄が描かれる');
    expectRuleCountMatchesReadable(host);
  });

  it('直下の欄は初期表示で畳まれていて、押すと開く（段階的開示）', async () => {
    // 01KYCC2TK3BEDA43TA61TPT4R5（下位セクションは初期折りたたみ・初期表示は走査に留める）と
    // 01KXDFD2SRHJJ0E551V240JMKT 条項3（5件以上は既定で畳む）。兄弟の欄（構成要素）は
    // 最初からこれを守っており、**新設した欄だけが全開だった**（`CLAUDE.md` 5 が名指しした
    // 「新しく作った面に規律を持ち込み忘れる」型）。
    const host = await openOverview();
    await selectComponent(host, '端末');
    const { before, after, head } = await openOwnBehaviors(host);
    expect(before, '初期表示で振る舞いカードが開いたまま出ている').toBe(0);
    expect(after, '押しても開かない').toBeGreaterThan(0);
    expect(head.getAttribute('aria-expanded'), '開いたのに aria-expanded が追随していない').toBe('true');
  });

  it('畳んだままでも、何がどれだけあるかを走査できる', async () => {
    // 「初期表示は**全体を走査できる概要に留める**」（同 decision）。畳んだ状態で
    // 中身が読めないのは正しいが、**何がどれだけあるか**まで消えると走査にならない。
    const host = await openOverview();
    await selectComponent(host, '端末');
    await waitFor(() => !!host.querySelector('.overview-own-behaviors'), '直下の欄が描かれる');
    const head = host.querySelector('.overview-own-behaviors .overview-part-head')!;
    expect(head.getAttribute('aria-expanded'), '初期表示で開いている').toBe('false');
    // 遷移の数が読めること（2本ある）。数字そのものを見るので、言い回しを変えても通る。
    const count = /(\d+)/.exec(head.querySelector('.overview-part-count')?.textContent || '');
    expect(count, '畳んだ見出しに数が出ていない').not.toBeNull();
    expect(Number(count![1]), '畳んだ見出しの数が、開いて出るカードの数と違う').toBe(2);
  });

  it('直下の欄の見出しが、構成要素の欄の見出しと取り違えられていない', async () => {
    // ⚠️ 両方に「振る舞い」が含まれるので、「振る舞いという語が出ているか」では
    // 取り違えを落とせない。**2つの欄の見出しが互いに違うこと**を見る（文言そのものを
    // ここに書き写すと、言い回しを変えるたびに嘘になる）。
    const host = await openOverview();
    await selectComponent(host, 'ビューア画面'); // 構成要素あり
    const partHeading = host.querySelector('.overview-section-title')?.textContent || '';
    await selectComponent(host, '端末'); // 構成要素なし＝直下の欄
    const ownSection = host.querySelector('.overview-own-behaviors')!.closest('.overview-section')!;
    const ownHeading = ownSection.querySelector('.overview-section-title')?.textContent || '';
    expect(partHeading, '構成要素の欄の見出しが読めていない').not.toBe('');
    expect(ownHeading, '直下の欄の見出しが読めていない').not.toBe('');
    expect(ownHeading, '直下の欄が、構成要素の欄の見出しを名乗っている').not.toBe(partHeading);
  });
});

// ---------------------------------------------------------------------------
// 役割 component を、フォールバックと違う id が担う形
// ---------------------------------------------------------------------------
//
// ⚠️ **これがこの単位の成立条件そのものである。** `.scholia/config.json` は主題種別
// `subject` に役割 component を宣言しており、慣用 id `component` のタグは1件も無い。
// つまり「シート単位の kind をリテラル `'component'` に固定する」変異を入れると、
// **この repo の概要タブは丸ごと空に戻る**。それが全部緑で通っていた。
//
// **何に落ちるか**: 役割の解決を宣言から読まずリテラルに戻す形（`01KYCC2THS5RX3HB27SQGFWSA5`
// の眼目）。4役割のうち component について落ちる。
// **何に落ちないか**: part / constraint / group の解決を同様にリテラルへ戻す形——
// この corpus はそれらを慣用 id のまま宣言していない（フォールバック経路を常時
// 踏ませるため）。そこは `roleKinds.test.ts` が値で守るが、**配線は守られていない。**
describe('役割 component を別 id が担うプロジェクトでも、仕様シートが出る', () => {
  const aliasWorld = { config: ALIAS_CONFIG, tags: ALIAS_TAGS };

  it('慣用 id のタグが1件も無くても、シートが描かれる（空状態に落ちない）', async () => {
    const host = await openOverview(aliasWorld);
    expect(host.querySelector('.overview-empty'), '別 id が役割を担うのに空状態へ落ちている').toBeNull();
    expect(host.querySelector('.overview-sheet'), '仕様シートが描かれていない').not.toBeNull();
    // 役割の呼び名も、その別 id の設定ラベルから来る。
    expect(host.querySelector('.overview-sheet')!.textContent, '別 id の呼び名がバッジに出ていない').toContain('ZZ主題ZZ');
  });

  it('別 id の世界でも、直下の遷移が振る舞いカードになる', async () => {
    const host = await openOverview(aliasWorld);
    await selectComponent(host, '端末');
    const { before, after } = await openOwnBehaviors(host);
    expect(before, '別 id の世界だけ初期表示で開いている').toBe(0);
    expect(after, '別 id の世界では直下の振る舞いが出ていない').toBe(2);
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

// ---------------------------------------------------------------------------
// 構造ツリーに並ぶもの・並んだ行を押した結果
// ---------------------------------------------------------------------------
//
// ## この節が落とすもの（射程・`CLAUDE.md` 6）
//
//   ・**役割を持たないタグが構造ツリーに並ぶ。** 行に付いた種類を値として読むので、
//     どの経路（起点／子）から混ざっても落ちる。**綴りには依らない。**
//   ・**並ぶ行の集合が変わる。** 件数ではなく**集合そのもの**を固定してあるので、
//     **起点と子のどちらか一方にだけ痩せた役割集合を渡す**形（純関数は1つのままでも
//     呼び出し側が非対称にできる）も落ちる。corpus の起点は**束ねる段だけ**にしてある
//     ——コンポーネントを起点に置くと、束ねる段の資格を外す変異が「まだ行がある」で通る。
//   ・**起点を決める2分岐の、どちらの側も踏む。** 起点は「設定が指定した集合」と
//     「親を持たないタグ（フォールバック）」で決まる。⚠️ **既定 corpus は宣言している側しか
//     踏まない**ので、フォールバック側は `NO_ROOTS_CONFIG`（`roots: []`＝**実データと同じ側**）で
//     別に起こしている。片側だけだと、**フォールバックの起点候補を痩せさせる変異**が
//     素通りする——実測で 288/288 が緑のまま、実データではツリーが0行になった。
//   ・**押しても何も起きない行ができる。** 2つの形の**両方**を見る:
//     (a) 開閉の三角も無く、リンクでもない行（`!isAnchor && !hasToggle`）
//     (b) **三角はあるが、押しても中身が1つも変わらない行**（三角を1つずつ押して確かめる）
//     (a) だけを見ていたときは (b) の変異が素通りした。
//   ・**純関数（`treeModel`）を呼ばない／呼んで答えを捨てる／痩せた材料を渡す。**
//     ここは描画を起こすので、判定が正しくても配線が外れていれば落ちる。
//     **転送についても同じ**（下の「共有済み URL の転送」節）。
//
// ## この節が落とさないもの（名指しする）
//
//   1. **行き先が正しいか。** アンカーが付いていることは見るが、その URL が
//      **意図した相手を指しているか**は見ていない（`treeModel.test.ts` が値で見る）。
//   2. **見え方。** 並び順・段の付き方・余白は1つも見ていない（happy-dom はレイアウトを
//      計算しない）。「階層に見えるか」はここでは答えられない。
//   3. **構成要素の入れ子。** corpus に構成要素の下の構成要素は無い。
//      入れ子を入れる変異はここでは落ちない（別単位で扱うと決めた範囲）。
//   4. ⚠️ **パンくず（シート上部の祖先の並び）。** あれは `parentIds[0]` をそのまま
//      遡っており、**役割の資格判定を1つも通していない**——「構造上どこに居るか」という
//      同じ問いを、集約した述語とは別の規則で答えている**5箇所目**である。
//      実データでは正しい答えが出るので欠陥は見えておらず、描画も `<span>` で
//      リンクではないため行き先の無い URL にはならないが、**ここは守っていない。**
describe('構造ツリーは役割を持つタグだけを並べ、並んだ行はすべて押した意味を持つ', () => {
  /** 行に付いた種類（`kind-<id>` クラス）と、押した結果を決める形を値として読む。 */
  function treeRows(host: HTMLElement) {
    return Array.from(host.querySelectorAll<HTMLElement>('.overview-tree-row')).map((row) => {
      const label = row.querySelector<HTMLElement>('.overview-tree-label')!;
      return {
        name: (row.querySelector('.overview-tree-name')?.textContent || '').trim(),
        kind: (Array.from(label.classList).find((c) => c.startsWith('kind-')) || '').replace(/^kind-/, ''),
        isAnchor: label.tagName === 'A' && !!label.getAttribute('href'),
        hasToggle: !!row.querySelector('.overview-tree-toggle'),
      };
    });
  }

  it('役割（コンポーネント／構成要素／束ねる段）を持たないタグは1行も並ばない', async () => {
    const host = await openOverview();
    const rows = treeRows(host);
    expect(rows.length, '構造ツリーが1行も描かれていない＝検査が空振りしている').toBeGreaterThan(0);
    // corpus には親を持たない要件（`req.viewer` / `req.cli`）が居り、config.roots にも
    // 入っている。**是正前はこの2件が最上段に並んでいた**（`req.viewer` は要件の子を
    // 持つので、押しても何も起きない行にもなっていた）。
    const strays = rows.filter((r) => !['component', 'part', 'group'].includes(r.kind));
    expect(strays.map((r) => `${r.name}(${r.kind})`), '役割を持たないタグが並んでいる').toEqual([]);
  });

  it('押しても何も起きない行が0件（開閉の三角も無く、リンクでもない行）', async () => {
    const host = await openOverview();
    const rows = treeRows(host);
    expect(rows.length).toBeGreaterThan(0);
    // ⚠️ **これが是正の本体である。** 「アンカーである」か「開閉できる」か、
    // 少なくとも一方は必ず成り立つ。両方を欠いた行は、押しても何も起きない。
    const dead = rows.filter((r) => !r.isAnchor && !r.hasToggle);
    expect(dead.map((r) => `${r.name}(${r.kind})`), '押しても何も起きない行がある').toEqual([]);
  });

  it('並ぶ行の集合そのものを値で固定する（起点・子のどちらかだけ材料を痩せさせる変異を落とす）', async () => {
    // ⚠️ **「1行以上ある」では足りない。** 純関数を1つに集約しても、**呼び出し側が
    // 起点と子で違う材料を渡せば非対称は復活する**——そのとき行は減るが0にはならない
    // ことが多く、件数だけ見ていると素通りする（レビュアの変異 R1/R2 がこの形）。
    // 集合そのものを固定して、**どこか1つでも欠けたら落ちる**ようにする。
    const host = await openOverview();
    expect(treeRows(host).map((r) => r.name)).toEqual([
      '主要なまとまり', // 束ねる段（起点。実データと同じく起点は束ねる段だけ）
      'ビューア画面', //   コンポーネント（既定の現在地なので開いている）
      '意思決定の一覧', //   その構成要素
      '端末', //           コンポーネント（畳んだまま＝子は出ない）
      '道具のまとまり', // 子は居るが役割を持つ子が居ない束ねる段
    ]);
  });

  it('設定が起点を宣言していないとき（＝実データ側）も、同じ集合が並ぶ', async () => {
    // ⚠️ **分岐の片側を踏ませるための検査。** 起点は「設定が指定した集合」と
    // 「親を持たないタグ（フォールバック）」の2分岐で決まる。既定 corpus は `roots` が
    // 非空なので**宣言している側しか踏まない**——実測では、フォールバック側の起点候補を
    // コンポーネントだけに絞る変異で 288/288 が緑のままだった。実データでは
    // コンポーネントは全件が束ねる段の子なので、その変異で**ツリーは0行になる**。
    const host = await openOverview({ config: NO_ROOTS_CONFIG });
    // 並ぶものは宣言した側と同じ（順序だけタグの並び順に従う）。
    expect(treeRows(host).map((r) => r.name)).toEqual([
      '道具のまとまり', // 子は居るが役割を持つ子が居ない束ねる段
      '主要なまとまり',
      'ビューア画面',
      '意思決定の一覧',
      '端末',
    ]);
    expect(treeRows(host).filter((r) => !r.isAnchor && !r.hasToggle).map((r) => r.name), '押しても何も起きない行がある').toEqual([]);
  });

  it('開閉の三角がある行は、押すと必ず中身が変わる（開いても何も出ない三角を落とす）', async () => {
    // ⚠️ 「押しても何も起きない行」を `アンカーでもなく三角も無い` とだけ定義すると、
    // **三角はあるが開いても何も出ない行**が漏れる（レビュアの変異 M5/M5b）。
    // 三角の有無ではなく**押した結果**を見る。
    const host = await openOverview();
    const rowCount = () => host.querySelectorAll('.overview-tree-row').length;
    const toggles = () => Array.from(host.querySelectorAll<HTMLElement>('.overview-tree-row')).filter((r) => r.querySelector('.overview-tree-toggle'));
    expect(toggles().length, '三角のある行が1つも無い＝検査が空振りしている').toBeGreaterThan(0);
    for (let i = 0; i < toggles().length; i++) {
      const row = toggles()[i];
      const name = row.querySelector('.overview-tree-name')?.textContent || '';
      const before = rowCount();
      row.querySelector<HTMLElement>('.overview-tree-toggle')!.click();
      await waitFor(() => rowCount() !== before, `「${name}」の三角を押しても行が1つも増減しない（開いても何も出ない三角）`);
      // 元に戻して次の行を同じ初期状態で見る
      const reopened = toggles().find((r) => (r.querySelector('.overview-tree-name')?.textContent || '') === name);
      reopened?.querySelector<HTMLElement>('.overview-tree-toggle')!.click();
      await waitFor(() => rowCount() === before, `「${name}」の三角がもう一度押しても戻らない`);
    }
  });

  it('役割を別 id が担うプロジェクトでも同じ答えになる（リテラル id に戻す変異を落とす）', async () => {
    // `ALIAS_*` の世界に `component` という id は無い。役割の解決をリテラルへ戻すと
    // ツリーが空になるか、逆に役割を持たないタグが混ざる。
    const host = await openOverview({ config: ALIAS_CONFIG, tags: ALIAS_TAGS });
    const rows = treeRows(host);
    expect(rows.length, 'ツリーが空＝役割の解決が宣言を読んでいない').toBeGreaterThan(0);
    // ALIAS の世界で宣言し直しているのは component だけ。part / group は宣言が無く
    // 慣用 id へフォールバックするので、その2つはこの世界でも役割を持つ。
    const strays = rows.filter((r) => !['subject', 'part', 'group'].includes(r.kind));
    expect(strays.map((r) => `${r.name}(${r.kind})`), '役割を持たないタグが並んでいる').toEqual([]);
    expect(rows.filter((r) => !r.isAnchor && !r.hasToggle).map((r) => r.name), '押しても何も起きない行がある').toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// 共有済み URL の転送（`01KYKS4Y56FAHRVCWKMQJK4RT6` 条項4 の射程を概要の現在地へ）
// ---------------------------------------------------------------------------
//
// ⚠️ **値のガード（`treeModel.test.ts`）だけでは足りない。** 転送先を計算する純関数が
// 正しくても、**その答えを使わない／痩せた材料を渡す**と転送は起きない。
// レビュアの変異3通（答えを計算するが `location.replace` を呼ばない／転送先から
// 構成要素の id を落とす／親を探す材料を常に null にする）は**すべて値のガードを
// 素通りした**。ここは描画を起こして、**URL がどこへ着いたか**を値として読む。
//
// ## この節が落とすもの
//   ・転送の配線を外す（答えを捨てる）／転送先の材料を痩せさせる／着地先から構成要素を落とす
// ## この節が落とさないもの
//   ・履歴を積まないこと（`location.replace` か `hash=` かの区別は happy-dom では見ていない）
//   ・寄せ先までスクロールすること（happy-dom はレイアウトを計算しない）
describe('構成要素になったタグを指す古い URL は、転送で生きる', () => {
  it('コンポーネントとして解決できない現在地が、親コンポーネント＋その構成要素へ着地する', async () => {
    server = installFakeServer({});
    mounted = mountApp('#/overview/part.list');
    const host = mounted.host;
    await waitFor(() => !!host.querySelector('.overview-tree'), '概要が描かれる');
    await waitFor(
      () => window.location.hash === '#/overview/comp.viewer/part/part.list',
      `古い URL が転送されない（いま: ${window.location.hash}）`,
    );
    // 着地したシートが親コンポーネントのものであること（既定へ黙って落ちていない）。
    await waitFor(() => (host.querySelector('.overview-title')?.textContent || '') === 'ビューア画面', '親コンポーネントのシートに着いていない');
    // 寄せ先の器が在ること（構成要素の id を落とすと URL は変わってもここが無くなる）。
    expect(host.querySelector('[data-part="part.list"]'), '寄せ先の構成要素が描かれていない').not.toBeNull();
  });

  it('コンポーネントを指す現在地は転送しない（射程を広げない）', async () => {
    // ⚠️ **親がコンポーネントであるコンポーネントを使う。** `comp.cli` は親が束ねる段なので、
    // 「転送しない」が**「親コンポーネントが見つからない」だけで満たされてしまう**
    // ——種別の判定をまるごと外しても緑のままになる（空振り）。`comp.cli.sub` は
    // 親が `comp.cli`（コンポーネント）なので、**種別で弾いていることだけが**
    // 転送しない理由になる。
    server = installFakeServer({});
    mounted = mountApp('#/overview/comp.cli.sub');
    const host = mounted.host;
    await waitFor(() => (host.querySelector('.overview-title')?.textContent || '') === '端末: 下位', '端末: 下位のシートが出る');
    expect(window.location.hash, 'コンポーネントの現在地まで転送している').toBe('#/overview/comp.cli.sub');
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
