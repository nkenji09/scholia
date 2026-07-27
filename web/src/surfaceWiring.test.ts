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
// 振る舞いそのものではなく**配線の有無**を見る構造ガードである点に注意。位置が正しいか・
// 開閉が実際に保たれるかまでは見ない（それは実機計測が担う）。それでも、共通配線を通らなく
// なる形の変更はここで落ちる。
//
// 面を増やしたら SURFACES に足す。新しい面が共通配線を通らない判断をするなら、その判断を
// decision に残したうえでここを更新すること（黙って外さない）。

/** 共通配線と、それが担保している既決。 */
const WIRING = {
  /** 現在地・アンカーを URL に載せる（deep-linking 01KYGYYMZSS1Y0BFEJ69Q1JC40）。 */
  routeHash: /\brouteHash\s*\(/,
  /** 面の本体スクロールを往復で保持する（view-state-continuity 01KYGYYN44MWDQMYSC5PFMVNEG）。 */
  useScrollRestore: /\buseScrollRestore\s*\(/,
  /** 面が持つ独立スクロール領域も保持する（01KYH0ESVG1D5NGDH5C4TG920J）。 */
  useElementScrollRestore: /\buseElementScrollRestore\s*\(/,
  /** ページ内の遷移リンクを実アンカーで描く（01KXFK3Q1NY9J8Q7FX14T31N7K）。 */
  HashLink: /<HashLink\b/,
  /** 開閉の保存値を既定より優先して解決する（collapsible-section 01KYGYYN8HRNFQEDMBS3DZRRX7）。 */
  collapseState: /\bloadCardSectionOpen\s*\(/,
} as const;

/** 面 → 通っているべき共通配線。独立スクロール領域を持たない面は該当分を外す。 */
const SURFACES: Array<{ name: string; source: string; wiring: Array<keyof typeof WIRING> }> = [
  {
    name: '概要タブ（構造ツリー＋コンポーネント仕様シート）',
    source: overviewSource,
    wiring: ['routeHash', 'useScrollRestore', 'useElementScrollRestore', 'HashLink', 'collapseState'],
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
