import { describe, expect, it } from 'vitest';
// ソースを文字列として読む（node の fs ではなくバンドラの ?raw を使う——app の tsconfig は
// node 型を持たない＝アプリ側が node API を触れないようにしてあるので、そこを崩さない）。
import appSource from './app.tsx?raw';
import overviewSource from './components/overview/OverviewView.tsx?raw';

// 「新しい面が共通配線を通っているか」を機械化するガード（A是正 01KYH2533234PGSN4MDQ6ZXJHA）。
//
// あの回帰の根因は、新しい面を作るときに既存の共通部品・共通配線を再利用せず素の局所状態で
// 組み直したことだった。個々の振る舞い（URL 同期・往復での位置保持・開閉の永続・アンカー化）は
// 要件の言葉で書かれていて「どの共通部品が担保しているか」は書かれていないため、部品を迂回
// しても違反として見えなかった——そこを見えるようにするのがこのテスト。
//
// 振る舞いそのものではなく**配線の有無**を見る構造ガードである点に注意。守れているのは
// 「共通配線が、守りたい対象に結び付いた形で使われていること」までで、次は守れていない:
//
// - 寄せた位置・復元した位置が正しいか（clamp されていないか・main と同じ位置か）
// - 開閉や位置が実際に往復で保たれるか
// - 修飾クリックが別タブになるか
//
// これらは DOM を起こす harness が要るため実機計測が担っている。ここが緑でも「動いている」
// ことの証明にはならない——「配線が外れていない」ことの証明に留まる。
//
// 面を増やしたら SURFACES に足す。新しい面が共通配線を通らない判断をするなら、その判断を
// decision に残したうえでここを更新すること（黙って外さない）。

/** 共通配線と、それが担保している既決。面をまたいで同じ形で使えるもの。 */
const WIRING = {
  /** 面の本体スクロールを往復で保持する（view-state-continuity 01KYGYYN44MWDQMYSC5PFMVNEG）。 */
  useScrollRestore: /\buseScrollRestore\s*\(/,
  /** 面が持つ独立スクロール領域も保持する（01KYH0ESVG1D5NGDH5C4TG920J）。 */
  useElementScrollRestore: /\buseElementScrollRestore\s*\(/,
  /** その領域の「器の形」も位置と対で保持する（01KYH8GX987GQX08C56G58JP2N）。位置だけを
      覚えても、戻ったときに器が縮んでいればその位置は存在しない。 */
  regionShape: /\bloadRegionShape\s*[<(]/,
  /** 開閉の保存値を既定より優先して解決する（collapsible-section 01KYGYYN8HRNFQEDMBS3DZRRX7）。 */
  collapseState: /\bloadCardSectionOpen\s*\(/,
} as const;

/** 面 → 通っているべき共通配線。独立スクロール領域を持たない面は該当分を外す。 */
const SURFACES: Array<{ name: string; source: string; wiring: Array<keyof typeof WIRING> }> = [
  {
    name: '概要タブ（構造ツリー＋コンポーネント仕様シート）',
    source: overviewSource,
    wiring: ['useScrollRestore', 'useElementScrollRestore', 'regionShape', 'collapseState'],
  },
];

describe('面が共通配線を通っている', () => {
  for (const surface of SURFACES) {
    for (const key of surface.wiring) {
      it(`${surface.name}: ${key}`, () => {
        expect(surface.source).toMatch(WIRING[key]);
      });
    }
  }
});

describe('概要タブのナビが URL とアンカーで組まれている', () => {
  // ここは「ファイルのどこかで使われているか」では守れない。OverviewView は回帰していた
  // 時期から、規則リンク・gap chip・制約名など他の7箇所で HashLink / routeHash を使って
  // いたので、ファイル全体への正規表現は**回帰していたソースでも緑になる**。守りたいのは
  // 「構造ツリーの行が」実アンカーで描かれ、「現在地が」URL として組み立てられること
  // なので、その2点に結び付いた形だけを見る。

  it('ツリー行の指し先が実アンカーで描かれている（01KXFK3Q1NY9J8Q7FX14T31N7K）', () => {
    // 行が持つ指し先（row.href）が HashLink の href に渡っていること。ツリー行を button に
    // 戻すと row.href の渡し先が消えるのでここで落ちる。
    expect(overviewSource).toMatch(/<HashLink[\s\S]{0,120}href=\{row\.href\}/);
  });

  it('現在地を URL として組み立てている（01KYGYYMZSS1Y0BFEJ69Q1JC40）', () => {
    // 概要タブの現在地を指す route を作っていること。他所の HashLink が使う
    // routeHash({ view: 'spec' … }) / ({ view: 'browse' … }) とは別物なので取り違えない。
    expect(overviewSource).toMatch(/routeHash\(\{\s*view:\s*'overview'/);
  });
});

describe('app が概要タブを URL へ配線している', () => {
  // ここが今回の回帰そのもの: OverviewView は route の props を1つも受け取らず、選択が
  // ローカル state に閉じていた。props を渡す配線と、選択を navigate へ流す配線の両方を見る。
  const opened = appSource.indexOf('<OverviewView');
  const overviewElement = appSource.slice(opened, appSource.indexOf('/>', opened));

  it('概要タブを描画している', () => {
    expect(opened).toBeGreaterThan(-1);
  });

  it('現在地の props（componentId / partId）を渡している', () => {
    expect(overviewElement).toMatch(/\bcomponentId=/);
    expect(overviewElement).toMatch(/\bpartId=/);
  });

  it('現在地の移動を受け取るハンドラを渡している', () => {
    expect(overviewElement).toMatch(/\bonSelectComponent=/);
  });

  it('そのハンドラが navigate を呼ぶ（＝履歴に残る経路を通る）', () => {
    // no-op や setState 直書きに差し替えられたらここで落ちる。履歴粒度の条項(3)は
    // 「現在地の移動がそのつど履歴に残る」ことを前提にしており、それは navigate 経由で成立する。
    const handler = /const\s+openOverviewAt\s*=\s*\(([^)]*)\)\s*=>\s*([^;]+);/.exec(appSource);
    expect(handler, 'openOverviewAt の定義が見つからない').not.toBeNull();
    expect(handler![2]).toMatch(/\bnavigate\s*\(/);
    expect(handler![2]).toMatch(/view:\s*'overview'/);
  });
});
