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
import wholeRulesSource from './components/browse/WholeRules.tsx?raw';
import { DICTS } from './strings';
import decisionListSource from './components/decisions/DecisionList.tsx?raw';
import inheritedSummarySource from './components/browse/inheritedSummary.ts?raw';
import decisionsViewSource from './components/decisions/DecisionsView.tsx?raw';
import decisionDetailSource from './components/decisions/DecisionDetailView.tsx?raw';
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
    // 出てよいのは「まだ取得していない」と「効いている規則が1件も無い」の2つだけ。
    // 後者は値として検査できる純関数 shouldDiscloseWhole に委ねてある
    // （01KYK4YTB8087JT5GNV5QB26T2）。「継承0件」で黙る形は追補 条項3 で退けた
    // （下の describe）。
    // 条件そのものに括弧を含む（shouldDiscloseWhole(...)）ので、非貪欲ではなく
    // 「行末の `) return null;` の直前まで」を貪欲に取る。
    const guards = [...inheritedRulesSource.matchAll(/^\s*if \((.*)\) return null;/gm)].map((m) => m[1].trim());
    expect(guards).toEqual(['!entries', '!shouldDiscloseWhole(entries, (id) => isInForce(id, currencyIndex))']);
    // 条件の付いていない裸の return null も塞ぐ。
    expect(inheritedRulesSource).not.toMatch(/^\s*return null;\s*$/m);
  });
});

// ---------------------------------------------------------------------------
// 全体をどこで読めるかの開示（追補 01KYJV3FYMDFRWQ939NBV2BPAC 条項3）
//
// 件数と継承元の開示（条項3）だけでは「全体を通しで読む」用途に答えていない。
// その受け皿は現状 CLI だけで、viewer には無い——**その事実と、いま使える手段を
// カードが開示する**というのが追補の条項3。ここも「無くても画面は成立してしまう」
// 種類の配線なので、外れても誰も気づかない形にしない。
//
// 出し分けの判断（継承0・own ありのカードでも出す）は値として検査できる純関数
// shouldDiscloseWhole へ切り出してある（01KYK4YTB8087JT5GNV5QB26T2）。値の正しさは
// inheritedSummary.test.ts が守るので、ここでは (1) その純関数を実際に通っている
// こと、(2) <WholeRules> の描画が**どの条件ゲートの内側にも入っていない**こと
// ——の2つだけを見る。(2) は「`{total > 0 && (` という文字列がある」「`<WholeRules>`
// という文字列がある」を別々に確認するだけでは守れない。呼び出しをゲートの内側へ
// 移す変異は両方の文字列をそのまま残すので、それでは緑のまま通る。ゲートを開き
// 括弧の対応で切り出し、その範囲の**内と外**を見る。
//
// 特定の1ゲート（`{total > 0 && (`）だけを見る形にしないこと。レビューで、別条件で
// 包む変異（`{sources.length > 0 && <WholeRules …/>}`）と、同型ゲートを増設して内側へ
// 移す変異の2つが緑のまま通った。`sources` は own を除いて作るので、前者は own のみ・
// 継承0 のカード（実測 tag 21件）の開示を丸ごと消す——このガードが守ると称する
// まさにその性質を壊す。よってソース中の `{… && (` を**全部**列挙して回し、同一行で
// 条件付きにする形（`{x && <WholeRules …/>}`）も併せて塞ぐ。
//
// このガードの射程（捕まえられない変異の型）:
//   - shouldDiscloseWhole 自身の中身の正しさは見ない（値の正しさは
//     inheritedSummary.test.ts の役目）。
//   - DOM を実際に起こしたときの見え方（本当に描画されるか）は見ない。ソースの
//     静的な構造だけを見る配線ガードである。
//   - `&&` を使わない形で包む変異（三項演算子・early return で JSX を差し替える・
//     ヘルパー関数の中へ隠す等）は、ゲートとして列挙できないので捕まらない。
//   - **整形に敏感である。** ゲートの列挙も下の早期 return の検査も、条件式が1行に
//     閉じていることを前提にしたソース文字列の照合なので、振る舞いを変えない整形
//     （prettier 風の複数行化・変数名の変更・コールバックの括り出し）で落ちうる。
//     落ち方は安全側（緑にならない）だが、意味のない赤が出たときは、ガードを緩める
//     前にこの射程の記述を疑うこと。

/** `{cond && (` で始まる JSX ゲートを、開き括弧の対応が取れる終端まで切り出す。
    複数のゲートを回せるよう、marker 文字列ではなく**開始位置**を受け取る形にして
    ある——`indexOf(marker)` で探す形だと同じ marker の2つめ以降を取れず、同型
    ゲートを増設する変異がそこから素通りする。 */
function extractGate(source: string, start: number): string {
  const openParenIndex = source.indexOf('(', start);
  if (openParenIndex < 0) throw new Error(`gate has no '(' after index ${start}`);
  let depth = 0;
  let i = openParenIndex;
  for (; i < source.length; i++) {
    if (source[i] === '(') depth++;
    else if (source[i] === ')') {
      depth--;
      if (depth === 0) break;
    }
  }
  if (depth !== 0) throw new Error(`unbalanced parens for gate at index ${start}`);
  return source.slice(start, i + 1);
}

describe('全体をどこで読めるかがカードから読める（追補 条項3）', () => {
  it('開示ブロックがそのレコードを渡して WholeRules を描いている', () => {
    expect(inheritedRulesSource).toMatch(/<WholeRules[\s\S]{0,60}record=\{record\}/);
  });

  it('出し分けの判断が値として検査できる純関数を通っている（値の正しさは inheritedSummary.test.ts の shouldDiscloseWhole が守る）', () => {
    expect(inheritedRulesSource).toMatch(/if \(!shouldDiscloseWhole\(/);
  });

  it('WholeRules がどの条件ゲートの内側にも入っていない（構造そのものを見る）', () => {
    // 継承ブロック側は total で出し分ける（継承0で「継承した規則 0件」とは言わない）が、
    // その1ゲートだけを見ると「別条件で包む」「同型ゲートを増設して内側へ移す」変異が
    // 素通りする。ソース中の `{… && (` を全部回して、どれの内側にも入っていないことを見る。
    const gates = [...inheritedRulesSource.matchAll(/\{[^{}\n]*&&\s*\(/g)];
    expect(gates.length, '条件ゲートが1つも見つからない（列挙の正規表現が壊れている）').toBeGreaterThan(0);
    for (const m of gates) {
      const gate = extractGate(inheritedRulesSource, m.index!);
      expect(gate, `ゲート ${m[0].trim()} の内側に WholeRules がある`).not.toMatch(/<WholeRules/);
    }
    // 同一行で条件付きにする形（`{x && <WholeRules …/>}`）は上の括弧対応では
    // 取れないので、行単位でも塞ぐ。
    for (const line of inheritedRulesSource.split('\n')) {
      if (line.includes('<WholeRules')) {
        expect(line, `WholeRules の行が条件付きになっている: ${line.trim()}`).not.toMatch(/&&/);
      }
    }
    // どのゲートの内側にも無いだけでは「そもそも描かれていない」形も通るので、
    // 記録を渡した呼び出しが実在することを併せて見る（上の it と重なるが、この
    // 検査が「無いから内側にも無い」で緑になるのを防ぐ）。
    expect(inheritedRulesSource).toMatch(/<WholeRules[\s\S]{0,60}record=\{record\}/);
  });

  it('事実そのものは畳まれていない', () => {
    // 「viewer には全体を通しで読む面が無い」は見出し行に出る。畳んだ内側に入れると、
    // 開かなかった利用者には省略が伝わらない＝開示にならない。
    const head = /<button[\s\S]{0,200}class="whole-rules-head"[\s\S]{0,400}<\/button>/.exec(wholeRulesSource);
    expect(head, 'whole-rules-head の見出し行が見つからない').not.toBeNull();
    expect(head![0]).toMatch(/t\.browse\.wholeRulesFact/);
  });

  it('手段（CLI コマンド）が共有の組み立てを通り、コピーできる形で出ている', () => {
    // コマンド文字列そのものの正しさは rulesCommand.test.ts が値として守る。
    // ここは「そこを通って画面に出て、コピーできる」ことだけを見る。
    expect(wholeRulesSource).toMatch(/rulesCommand\(record\)/);
    expect(wholeRulesSource).toMatch(/<code>\{cmd\}<\/code>/);
    expect(wholeRulesSource).toMatch(/copyText\(cmd/);
  });

  it('開示を黙らせる早期 return が入っていない', () => {
    expect(wholeRulesSource).not.toMatch(/return null/);
  });
});

// ---------------------------------------------------------------------------
// 意思決定の単票が生成 id をどう扱うか（01KYK4YNCYGZHHXB4H90Q996T2 条項3〜5）
//
// 条項3 は「不透明な id を機能を持たないメタ情報として既定の見え方に置かない」、
// 条項5 は「消すときに到達手段を落とさない」を求める。単票にはこれ以外に id へ
// 到達する経路が無いので、**両方が同時に成り立っていないと決定を満たさない**——
// 開示を消せば条項5 が破れ、見出し下へ id を戻せば条項3 が破れる。
//
// この面はこの差し戻しまでガードを1件も持っておらず、開示を丸ごと消す変異も
// 見出し下へ生 id を戻す変異も緑のまま通っていた。ここがまさに、この repo が
// 4度繰り返している「外れても誰も気づかない配線」である。
//
// 射程（捕まえられない型）: 上の WholeRules ガードと同じく静的なソース照合なので、
// DOM を起こしたときの見え方は見ない。文言・コマンドの正しさも見ない（開示の中身は
// DecisionIdReveal 側の責務）。**整形に敏感**な点も同じ。

describe('意思決定の単票が生成 id を既定に置かず、到達手段を残している（01KYK4YNCYGZHHXB4H90Q996T2 条項3〜5）', () => {
  it('求めたときに出す開示（DecisionIdReveal）が単票に描かれている（条項5＝到達手段）', () => {
    // 開示ごと消す変異はここで落ちる。消すと id を得る唯一の経路が黙って失われる。
    expect(decisionDetailSource).toMatch(/<DecisionIdReveal[\s\S]{0,60}id=\{decision\.id\}/);
  });

  it('見出し下のメタに生 id が戻っていない（条項3＝既定の見え方に置かない）', () => {
    // `.decision-detail-meta` の器を括弧の対応ではなく閉じタグで切り出す（JSX の
    // div なので `</div>` まで）。この中に decision.id を描く形が復活したら落ちる。
    const start = decisionDetailSource.indexOf('class="decision-detail-meta');
    expect(start, 'decision-detail-meta が見つからない').toBeGreaterThan(-1);
    const end = decisionDetailSource.indexOf('</div>', start);
    expect(end, 'decision-detail-meta の閉じタグが見つからない').toBeGreaterThan(-1);
    const meta = decisionDetailSource.slice(start, end);
    expect(meta, `meta に生 id が描かれている: ${meta.trim()}`).not.toMatch(/\{decision\.id\}/);
  });
});

describe('一覧への入口が指す範囲を名乗っている（追補 条項2）', () => {
  // 一覧のタグ絞り込みは「そのタグと配下」方向で、「この記録を支配する規則」＝
  // 自身＋祖先とは向きが逆。支配する集合の名前をラベルに使うと、逆向きの集合を
  // 指す入口になる——実測で9タグが「継承3件」と開示した直下から0件の面に着く。
  for (const [lang, d] of Object.entries(DICTS)) {
    it(`${lang}: 支配する規則の名前をラベルに使わない`, () => {
      const labels = [d.browse.rulesListLinkExact, d.browse.rulesListLinkScoped('X')];
      for (const label of labels) {
        expect(label, label).not.toMatch(/効く|効いている|支配|governing|in force|applies/i);
      }
    });

    it(`${lang}: 自身と配下の両方を名乗る（dt= は自身への decision も返す）`, () => {
      // 「配下」だけだと、そのタグ自身に付いた意思決定を名乗り落とす。一覧が返すのは
      // 「T を実効タグに持つ decision」＝ T 自身への decision も含む。
      const labels = [d.browse.rulesListLinkExact, d.browse.rulesListLinkScoped('X')];
      for (const label of labels) {
        expect(label, label).toMatch(/と配下|and below/i);
      }
    });
  }
});

describe('配下の意思決定の一覧への入口がカードにある', () => {
  // この入口は条項5 が言う集合（この記録を支配する規則＝自身＋祖先）を**指していない**
  // ——一覧は配下方向にしか絞れない、というのが追補 01KYJV3FYMDFRWQ939NBV2BPAC の
  // 確定事項（条項2）。それ自体は使える眺めなので置いたままにし、範囲を名乗らせる。
  // 条項5 が言う用途に答えるのは WholeRules（追補 条項3）。
  it('タグのカードは継承0件でも入口を出す', () => {
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
    // 語そのものを見る（識別子は 2値化で消えた）。「失効」は効いていない理由を
    // 述べないので条項3 が退けた語——1画面に2つの語を同居させない。
    expect(decisionsViewSource).not.toMatch(/失効/);
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
