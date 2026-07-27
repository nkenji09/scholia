import { describe, expect, it } from 'vitest';
// ソースを文字列として読む（node の fs ではなくバンドラの ?raw を使う——app の tsconfig は
// node 型を持たない＝アプリ側が node API を触れないようにしてあるので、そこを崩さない）。
import appSource from './app.tsx?raw';
import overviewSource from './components/overview/OverviewView.tsx?raw';
import tagCardSource from './components/browse/TagCard.tsx?raw';
import specCardSource from './components/browse/SpecCard.tsx?raw';
import vocabCardSource from './components/browse/VocabCard.tsx?raw';
import browseViewSource from './components/browse/BrowseView.tsx?raw';
import inheritedRulesSource from './components/browse/InheritedRules.tsx?raw';
import rulesListLinkSource from './components/browse/RulesListLink.tsx?raw';
import decisionListSource from './components/decisions/DecisionList.tsx?raw';
import inheritedSummarySource from './components/browse/inheritedSummary.ts?raw';
import decisionsViewSource from './components/decisions/DecisionsView.tsx?raw';
import appSourceForList from './app.tsx?raw';

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

// ---------------------------------------------------------------------------
// 集約2面の廃止と、その代替の配線（01KYHW4NBNVN9BFXYZMBX8MPF8）
//
// この decision は「この記録を支配する規則」欄と概要タブの「このコンポーネントの
// 規則」欄を**廃止する**と同時に、廃止したままにしない条件を課している——継承した
// 規則の存在の開示（条項3）と祖先リンクの完備（条項4）。廃止だけが実装されて代替が
// 落ちると、01KXYED61J6QBEX75H2XHVHW7Y が診断した欠陥（親に決定を持ち子に無い
// レコードの why が viewer から不可視）がそのまま復活する——実測でタグ9件・
// transition 36本・vocab 9件のカードが規則ゼロ表示になる。
//
// 代替は「無くても画面は成立してしまう」種類の配線なので、外れても誰も気づかない。
// #3 のレビュー指摘（should-3「web 側の配線に回帰テストが皆無で、結線を丸ごと
// 外しても緑のまま」）と同じ穴をここで作らないためのガード。
//
// 例によってこれは**配線ガード**で、見た目や件数の正しさは実機計測が担う。

const CARDS: Array<{ name: string; source: string; kind: string; id: string }> = [
  { name: 'TagCard', source: tagCardSource, kind: 'tag', id: 'tag.id' },
  { name: 'SpecCard', source: specCardSource, kind: 'transition', id: 'detail.id' },
  { name: 'VocabCard', source: vocabCardSource, kind: 'vocab', id: 'entry.id' },
];

describe('継承した規則の開示がカードに配線されている（条項3）', () => {
  for (const card of CARDS) {
    it(`${card.name}: そのレコードを指して InheritedRules を描いている`, () => {
      // 「ファイルのどこかで InheritedRules に触れているか」では守れない——
      // import だけ残して描画を消せる。record にそのカードのレコードが渡って
      // いるところまで見る。
      const re = new RegExp(`<InheritedRules[\\s\\S]{0,120}kind:\\s*'${card.kind}'[\\s\\S]{0,40}id:\\s*${card.id.replace('.', '\\.')}`);
      expect(card.source).toMatch(re);
    });
  }

  it('件数の計算が純関数を通っている（値の正しさは inheritedSummary.test.ts が守る）', () => {
    // own 除外・効いているものだけ・継承元ごとの束ね方は summarizeInherited の
    // 振る舞いテストが守る。ここはそこを通っていることだけを見る。
    expect(inheritedRulesSource).toMatch(/\bsummarizeInherited\s*\(/);
    expect(inheritedRulesSource).toMatch(/\bisInForce\s*\(/);
    expect(inheritedSummarySource).toMatch(/provenance\s*!==\s*'own'/);
  });

  it('継承元が実際に描かれている（配線だけ残して中身を消させない）', () => {
    expect(inheritedRulesSource).toMatch(/sources\.map\(/);
    expect(inheritedRulesSource).toMatch(/inherited-rules-sources/);
  });

  it('開示を黙らせる早期 return が入っていない', () => {
    // 「配線もフィルタも JSX も残したまま先頭で return null する」変異は、
    // ソース文字列を見るだけのガードでは原理的に捕まらない（DOM を起こす
    // harness が要る）。そこで**早期 return の条件そのもの**を固定する:
    // 出てよいのは「まだ取得していない」と「継承0件」の2つだけ。
    const guards = [...inheritedRulesSource.matchAll(/^\s*if \(([^)]*)\) return null;/gm)].map((m) => m[1].trim());
    expect(guards).toEqual(['!entries', 'total === 0']);
    // 条件の付いていない裸の return null も塞ぐ。
    expect(inheritedRulesSource).not.toMatch(/^\s*return null;\s*$/m);
  });
});

describe('規則の全体を読む入口がカードにある（条項5）', () => {
  it('タグのカードは継承0件でも入口を出す', () => {
    // 条項5 は条項3・4 と同格の「廃止するなら課す条件」。継承が無いカードでも
    // own の規則を通覧する入口は要る。
    expect(tagCardSource).toMatch(/<RulesListLink[\s\S]{0,60}tagId=\{tag\.id\}[\s\S]{0,20}exact/);
  });

  it('transition / vocab は開示ブロック側から入口を出す', () => {
    expect(inheritedRulesSource).toMatch(/record\.kind !== 'tag' && <RulesListLink/);
  });

  it('入口が意思決定の一覧を対象で絞って指している', () => {
    // 単票（#/decision/<id>）ではなく一覧（#/decisions?dt=…）を指すこと。
    expect(rulesListLinkSource).toMatch(/view:\s*'decisions'/);
    expect(rulesListLinkSource).toMatch(/decisionTag:\s*tagId/);
  });
});

describe('意思決定の一覧が提案B の3面のひとつとして揃っている', () => {
  it('要約が共有の切り出しを通る（条項6）', () => {
    // 生の why を CSS の line-clamp で切ると markdown 記法のまま第1段落が流れる。
    expect(decisionsViewSource).toMatch(/summaryOf\(d\.why\)/);
    expect(decisionsViewSource).not.toMatch(/decision-row-why">\{d\.why\}/);
  });

  it('効力の語が2値で統一されている（条項3）', () => {
    // 同じ画面でバッジが「置き換え済み」・絞り込みが「失効」だと語が2つ同居する。
    expect(decisionsViewSource).toMatch(/effectInForce/);
    expect(decisionsViewSource).toMatch(/effectReplaced/);
    expect(decisionsViewSource).not.toMatch(/currencySuperseded/);
  });

  it('既定では効いているものだけを出す（条項4）', () => {
    // 一覧には畳む器が無いので、既定の絞り込みがその役目を果たす。
    expect(appSourceForList).toMatch(/route\.decisionCurrency \|\| 'current'/);
  });
});

describe('祖先リンクが直接の親で止まっていない（条項4）', () => {
  it('BrowseView が祖先の連なり全体を渡している', () => {
    // parentsOf は直接の親1階層しか返さない。実測で継承した効いている規則 82件の
    // うち4件が祖父由来で、直接の親だけではカードから到達手段が無かった。
    expect(browseViewSource).toMatch(/ancestors=\{ancestorsOf\(/);
    expect(browseViewSource).not.toMatch(/parents=\{parentsOf\(/);
  });

  it('TagCard が受け取った祖先を全部リンクにしている', () => {
    // 先頭だけ描く（ancestors[0]）に差し替えられたらここで落ちる。
    expect(tagCardSource).toMatch(/ancestors\.map\(/);
    expect(tagCardSource).toMatch(/<HashLink[\s\S]{0,200}tagId:\s*p\.id/);
  });
});

describe('廃止した集約2面が戻っていない（条項1・2）', () => {
  it('どのカードにも「この記録を支配する規則」欄が無い', () => {
    for (const card of CARDS) {
      expect(card.source, card.name).not.toMatch(/GovernsSection/);
    }
  });

  it('概要タブにコンポーネント本体の規則欄が無い', () => {
    // 欄そのもの（componentRules の構築とトグル）が消えていること。構成要素・
    // 振る舞い・制約のインライン展開は残すので renderRules 自体は残る。
    expect(overviewSource).not.toMatch(/componentRules/);
    expect(overviewSource).not.toMatch(/componentRulesToggle/);
  });

  it('概要タブは構成要素・振る舞い・制約のインライン展開を残している（消したら行き過ぎ）', () => {
    expect(overviewSource).toMatch(/renderRules\('part:'/);
    expect(overviewSource).toMatch(/renderRules\('tx:'/);
    expect(overviewSource).toMatch(/renderRules\('prop:'/);
  });
});

describe('意思決定欄が共有の描画口を通っている（01KYHW54B8ZXH0NEPH2J7N1X39）', () => {
  for (const card of CARDS) {
    it(`${card.name}: DecisionList を使っている`, () => {
      // 面ごとに書き分けると、また面ごとに違う答えが出る（面間整合）。効力2値・
      // 付帯情報・履歴の畳みは DecisionList が1箇所で担う。
      expect(card.source).toMatch(/<DecisionList[\s\S]{0,200}decisions=\{/);
    });
  }

  it('状態列に出る効力が2値である（3値をそのまま写さない）', () => {
    // effectOf は in-force / replaced の2値。currencyOf（3値）を状態列に
    // 直接使うと「現行 ⇔ 改訂」の対立が戻る。
    expect(decisionListSource).toMatch(/\beffectOf\s*\(/);
    expect(decisionListSource).not.toMatch(/currencyAmended/);
    expect(overviewSource).toMatch(/\beffectOf\s*\(/);
    expect(overviewSource).not.toMatch(/currencyAmended/);
  });

  it('「後続に部分改訂・例外がある」を付帯情報として出している（条項2）', () => {
    expect(decisionListSource).toMatch(/relatedDecisions\s*\(/);
    expect(decisionListSource).toMatch(/readTogether/);
  });

  it('置き換え済みを畳む口がある（条項4）', () => {
    expect(decisionListSource).toMatch(/replacedHeading/);
    expect(overviewSource).toMatch(/replacedHeading/);
  });

  it('要約は共有の切り出しを通る（条項6）', () => {
    // 各面が独自に slice すると、また面ごとに違う長さの「要約」が出る。
    expect(decisionListSource).toMatch(/from '\.\.\/\.\.\/decisionSummary'/);
    expect(overviewSource).toMatch(/from '\.\.\/\.\.\/decisionSummary'/);
  });
});
