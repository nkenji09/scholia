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
import decisionRowFullSource from './components/decisions/DecisionRowFull.tsx?raw';
import decisionScopeSource from './components/decisions/decisionScope.ts?raw';
import decisionRedirectSource from './components/decisions/DecisionPermalinkRedirect.tsx?raw';
import decisionIdRevealSource from './components/decisions/DecisionIdReveal.tsx?raw';
import headerSource from './components/layout/Header.tsx?raw';
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
// このガードの射程（CLAUDE.md「配線ガードの書き方」2・6）:
//
// **ソース文字列の照合は、同じ意味を別の綴りで書かれれば捕まらない。** これは検査の
// 書き方を工夫しても原理的に消えない性質なので、捕まらない綴りを1つずつ数え上げる
// ことはしない（数え上げは必ず取りこぼす。実際、差し戻しのたびに新しい綴りが出た）。
// ここで名乗るのは1つ:
//
//   **このファイルが守れているのは「その形で書かれていないこと」までで、
//     「その振る舞いをしないこと」ではない。**
//
// だから、値として検査できる判断は**ここに置かない**。純関数へ出して、入力と出力の
// 対で検査する（decisionFilter.test.ts / decisionScope.test.ts / navActive.test.ts /
// inheritedSummary.test.ts）。ここに残すのは、値へ落とせない構造だけである:
//   ・その要素が描かれているか（消す変異）
//   ・どの条件ゲートの内側にも入っていないか（畳んで隠す変異）
//   ・共通の純関数を迂回していないか（面の中で判断を組み直す変異）
//
// 実機で起こしたときの見え方は、どのみちここでは分からない（実機確認が担う）。

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

  it('入口そのものは畳まれていない（01KYKS4Y56FAHRVCWKMQJK4RT6）', () => {
    // お詫びだった時代は「viewer には面が無い」という**事実**を見出し行に出していた。
    // 面ができたので出るのは実リンクだが、畳んではいけない理由は同じ——畳んだ内側に
    // 入れると、開かなかった利用者には到達手段が伝わらない。
    // 実リンク（whole-rules-link）が、畳みの見出し（whole-rules-head）より前に、
    // どの条件ゲートの内側でもない位置にあることを見る。
    const linkAt = wholeRulesSource.indexOf('class="whole-rules-link"');
    const headAt = wholeRulesSource.indexOf('class="whole-rules-head"');
    expect(linkAt, '実リンクが見つからない').toBeGreaterThan(-1);
    expect(headAt, '畳みの見出しが見つからない').toBeGreaterThan(-1);
    expect(linkAt, '実リンクが畳みの内側／後ろに移っている').toBeLessThan(headAt);
    // 行をまたぐゲートで包む変異は、上の2つ（前後関係・同一行の `&&`）では取れない
    // ——概要側には足したこの列挙が、カード側に無かった。ソース中の `{… && (` を
    // 全部回して、どれの内側にも入っていないことを見る（1つのゲートだけを見る形に
    // しない・`CLAUDE.md`「配線ガードの書き方」3）。
    const gates = [...wholeRulesSource.matchAll(/\{[^{}\n]*&&\s*\(/g)];
    expect(gates.length, '条件ゲートが1つも見つからない（列挙の正規表現が壊れている）').toBeGreaterThan(0);
    for (const m of gates) {
      const gate = extractGate(wholeRulesSource, m.index!);
      expect(gate, `ゲート ${m[0].trim()} の内側に実リンクがある`).not.toMatch(/class="whole-rules-link"/);
    }
    for (const line of wholeRulesSource.split('\n')) {
      if (line.includes('whole-rules-link')) {
        expect(line, `実リンクの行が条件付きになっている: ${line.trim()}`).not.toMatch(/&&/);
      }
    }
  });

  it('入口が「支配する向き」で絞った一覧を指している（追補 条項2・4）', () => {
    // ⚠ 向きが抜けると、実測で 8/75 のタグが「継承した規則 N件」と開示した直下から
    // **0件の面に着く**——追補 01KYJV3FYMDFRWQ939NBV2BPAC が確定した誤りに戻る。
    // 単票（廃止済み）でも、向きの無い一覧でもなく、対象＋governing を指すこと。
    expect(wholeRulesSource).toMatch(/routeHash\(\{[\s\S]{0,120}view:\s*'decisions'/);
    expect(wholeRulesSource).toMatch(/decisionOn:\s*scopeRef\(record\)/);
    expect(wholeRulesSource).toMatch(/decisionScope:\s*'governing'/);
    // 対象の綴りは共有の組み立て（formatScopeTarget）を通る。手組みすると読み側と
    // 書き側で綴りが割れる（値の正しさは decisionScope.test.ts が守る）。
    expect(wholeRulesSource).toMatch(/formatScopeTarget\(\{\s*type:\s*record\.kind,\s*id:\s*record\.id\s*\}\)/);
    // 件数は純関数を通って渡る（開示した数と行き先の件数を一致させる。値の正しさは
    // inheritedSummary.test.ts の countWholeInForce が守る）。
    expect(inheritedRulesSource).toMatch(/\bcountWholeInForce\s*\(/);
    // 純関数は通したまま**値だけずらす**変異（`{wholeCount + 1}`）も落とすため、
    // 渡す式そのものを固定する。名乗る件数と行き先の件数は一致していなければならない。
    expect(inheritedRulesSource).toMatch(/<WholeRules[\s\S]{0,120}inForceCount=\{wholeCount\}/);
  });

  it('端末で読む手段が残っている（リンクができたからといって消さない）', () => {
    // 全文を1つの並びで一気に読む／貼り付ける経路は、リンクでは代替されない
    // （01KXYED62CEKBY97D7X66BMC9A の「省略はその旨を開示する」）。
    const head = /<button[\s\S]{0,200}class="whole-rules-head"[\s\S]{0,400}<\/button>/.exec(wholeRulesSource);
    expect(head, 'whole-rules-head の見出し行が見つからない').not.toBeNull();
    expect(head![0]).toMatch(/t\.browse\.wholeRulesCliHead/);
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
// 意思決定の**一覧の行**が生成 id をどう扱うか（01KYK4YNCYGZHHXB4H90Q996T2 条項3〜5）
//
// 単票は廃止され（01KYKS4Y56FAHRVCWKMQJK4RT6）、その中身は一覧の行が吸収した。
// id の開示もそのまま行の中へ移った——条項5（消すときに到達手段を落とさない）は
// 器が変わっても同じだけ効く。以下は単票に対して書かれていたガードを、器を
// 差し替えて引き継いだもの（**外したのではなく移した**）。
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
// 条項3 は器を選ばない。当初このガードは `.decision-detail-meta` の内側だけを
// 切り出して見ていたが、それだと**器の外へ逃がす変異**——見出し行
// （`.decision-detail-title-row`）へ `<span class="dim">{decision.id}</span>` を
// 置く形——が緑のまま通り、既定の見え方に生 id が出た。器で切り出すのをやめ、
// `{decision.id}` の出現が**ファイル全体で1回だけ**（＝開示へ渡す prop のみ）で
// あることを見る。どの器へ移しても2回目の出現になるので落ちる。
//
// 射程（捕まえられない型）: 上の WholeRules ガードと同じく静的なソース照合なので、
// DOM を起こしたときの見え方は見ない。文言・コマンドの正しさも見ない（開示の中身の
// うち「既定で閉じている」「黙らない」の2点は下の DecisionIdReveal の describe が
// 受け持つ）。`decision.id` を別の変数へ束ねてから描く形（`const id = decision.id`）は
// 出現の数え方をすり抜けるので捕まらない。**整形に敏感**な点も同じ。

describe('意思決定の行が生成 id を既定に置かず、到達手段を残している（01KYK4YNCYGZHHXB4H90Q996T2 条項3〜5）', () => {
  it('求めたときに出す開示（DecisionIdReveal）が行に描かれている（条項5＝到達手段）', () => {
    // 開示ごと消す変異はここで落ちる。消すと id を得る唯一の経路が黙って失われる。
    expect(decisionRowFullSource).toMatch(/<DecisionIdReveal[\s\S]{0,60}id=\{d\.id\}/);
  });

  it('生 id が既定の見え方のどこにも描かれていない（条項3＝器を問わない）', () => {
    // 出現は「開示へ渡す prop」の1回だけ。展開の内側へ戻す変異も、見出し行など
    // 別の器へ逃がす変異も、2回目の出現になるのでここで落ちる。
    const hits = [...decisionRowFullSource.matchAll(/\{d\.id\}/g)];
    expect(hits.length, `{d.id} の出現が ${hits.length} 回ある（開示へ渡す prop の1回だけであるべき）`).toBe(1);
    // その1回が開示へ渡す prop であること（＝どこか別の場所へ移しただけ、を防ぐ）。
    expect(decisionRowFullSource).toMatch(/<DecisionIdReveal[\s\S]{0,60}id=\{d\.id\}/);
  });

  it('一覧のほうにも生 id が漏れていない（行へ移したついでに器が増えていない）', () => {
    // 一覧は key と ref に d.id を使うが、**描画**しては条項3 に反する。
    // key/ref 以外の位置に現れたらここで落ちる。
    const bare = [...decisionsViewSource.matchAll(/\{d\.id\}/g)];
    for (const m of bare) {
      const before = decisionsViewSource.slice(Math.max(0, m.index! - 40), m.index!);
      expect(before, `{d.id} が描画位置に出ている: …${before.trim()}`).toMatch(/(key|ref)=$/);
    }
  });
});

// 開示そのものが条項3・4・5 を満たしているか（DecisionIdReveal）。
//
// 上の単票側ガードは「開示が描かれている」までしか見ない。開示の中で既定を開いて
// しまえば条項3・4（既定の見え方には置かず、求めたときにだけ出す）が破れ、中で
// 黙らせれば条項5（到達手段を落とさない）が破れる——どちらも単票側からは見えない。
// 対になる WholeRules には同じ形の歯止めがあるのに単票側に無いのは非対称なので、
// 同じ2点をここで見る。
//
// 射程: 開閉の初期値と黙り込みだけを見る。文言・コマンドの正しさ・DOM を起こした
// ときの実際の見え方は見ない。**整形に敏感**。
describe('単票の開示が既定で閉じており、黙らない（01KYK4YNCYGZHHXB4H90Q996T2 条項3〜5）', () => {
  it('既定は閉じている（条項3・4＝求めたときにだけ出す）', () => {
    // useState(true) へ変える変異はここで落ちる。開いた状態が既定になると、
    // 生 id が既定の見え方に出る＝条項3 に反する。
    expect(decisionIdRevealSource).toMatch(/useState\(false\)/);
    expect(decisionIdRevealSource).not.toMatch(/useState\(true\)/);
  });

  it('開示を黙らせる早期 return が入っていない（条項5＝到達手段）', () => {
    expect(decisionIdRevealSource).not.toMatch(/return null/);
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
    // 要約の描画は行（DecisionRowFull）へ移った。
    expect(decisionRowFullSource).toMatch(/summaryOf\(d\.why\)/);
    expect(decisionRowFullSource).not.toMatch(/decision-row-why">\{d\.why\}/);
  });

  it('効力の語が2値で統一されている（条項3）', () => {
    // 同じ画面でバッジが「置き換え済み」・絞り込みが「失効」だと語が2つ同居する。
    expect(decisionRowFullSource).toMatch(/effectInForce/);
    expect(decisionRowFullSource).toMatch(/effectReplaced/);
    expect(decisionsViewSource).toMatch(/effectInForce/);
    expect(decisionsViewSource).toMatch(/effectReplaced/);
    // 語そのものを見る（識別子は 2値化で消えた）。「失効」は効いていない理由を
    // 述べないので条項3 が退けた語——1画面に2つの語を同居させない。
    expect(decisionsViewSource).not.toMatch(/失効/);
    expect(decisionRowFullSource).not.toMatch(/失効/);
  });

  it('既定では効いているものだけを出す（条項4）', () => {
    // 一覧には畳む器が無いので、既定の絞り込みがその役目を果たす。既定値そのものは
    // decisionFilter（純関数）が持ち、decisionFilter.test.ts が値として守る
    // ——ここは app がその変換を通っていることだけを見る。
    // ⚠️ これはソース照合であって、同じ意味を別の綴りで書かれれば通る（CLAUDE.md 2）。
    // 「既定が効いた結果が実際に行になる」ことは renderWiring.test.tsx が値で守る。
    expect(appSourceForList).toMatch(/conditionsFromRoute\(route\)/);
  });
});

// ---------------------------------------------------------------------------
// 絞り込まれた一覧を1つの仕組みにした（01KYKS4Y56FAHRVCWKMQJK4RT6）
//
// この decision は3つを同時に成立させて初めて成り立つ:
//   ① 一覧の行が単票の中身を全部持つ（実測 158件中 146件が、旧一覧では読めない
//      要素を1つ以上持っていた——1つ抜けると実害が出る）
//   ② 結果が1件なら開いた状態で着地する（畳まれた要約1行に着いたら今より悪い）
//   ③ 旧 URL が転送で生きる（共有済みリンクを黙って殺さない）
// どれも「無くても画面は成立してしまう」種類の配線なので、外れても誰も気づかない。
// ⚠️ この repo は「新しく作った面にガードを置き忘れる」を繰り返している
// （01KYH2533234PGSN4MDQ6ZXJHA・#3 の should-3）。同じ穴をここで作らない。
//
// 射程: これも**配線ガード**（静的なソース照合）で、DOM を起こしたときの見え方・
// 実際に開いて着地するか・転送が本当に走るかは実機確認が担う。整形に敏感。

describe('一覧の行が単票の中身を全部吸収している（①）', () => {
  // 旧一覧の行に無かった8つ。1つずつ見る——まとめて1本にすると、どれが欠けたのかが
  // 赤の理由から読めない。
  const ABSORBED: Array<[string, RegExp]> = [
    ['本文の全文（書式つき）', /<Markdown\s+text=\{d\.why\}/],
    ['変更内容', /<Markdown\s+text=\{d\.changed\}/],
    // URL のときのリンクと、そうでないときの本文の**両方**。片方だけ残す変異も落とす。
    ['参照（リンク）', /class="decision-detail-ref-link"[\s\S]{0,80}\{d\.ref\}/],
    ['参照（本文）', /<p class="decision-detail-ref">\{d\.ref\}<\/p>/],
    ['実装コミット', /commits\.map\(/],
    ['容認', /acknowledges\.map\(/],
    ['何を置き換え・改訂したか', /supersedes\.map\(/],
    ['誰に置き換え・改訂されたか', /supersededBy\.map\(/],
    ['記録を指す文字列の開示', /<DecisionIdReveal/],
  ];
  for (const [name, re] of ABSORBED) {
    it(`行が「${name}」を持つ`, () => {
      expect(decisionRowFullSource).toMatch(re);
    });
  }

  it('全文は展開の内側にあり、畳んだときは要約だけ（条項6）', () => {
    // 展開しても要約のまま／畳んでも全文が出る、のどちらへ倒れてもここで落ちる。
    expect(decisionRowFullSource).toMatch(/\{!open && <p class="decision-row-why">/);
    const gateAt = decisionRowFullSource.indexOf('{open && (');
    expect(gateAt, '展開のゲートが見つからない').toBeGreaterThan(-1);
    const gate = extractGate(decisionRowFullSource, gateAt);
    // 全文の描画は**1箇所だけ**で、それがゲートの内側にあること。ゲートを残したまま
    // 外側へ複製する変異（畳んでも全文が出る）はここで落ちる。
    const bodies = [...decisionRowFullSource.matchAll(/<Markdown text=\{d\.why\}/g)];
    expect(bodies.length, `全文の描画が ${bodies.length} 箇所ある（1箇所であるべき）`).toBe(1);
    expect(gate, '全文がゲートの内側にない').toMatch(/<Markdown text=\{d\.why\}/);
  });

  it('「併せて読む」が注記ではなく辿れる導線になっている（条項2・7）', () => {
    // 旧一覧はここがクリックできない <span class="decision-row-related-note"> だった。
    expect(decisionRowFullSource).toMatch(/readTogether/);
    expect(decisionRowFullSource).toMatch(/class="decision-row-related"[\s\S]{0,80}onOpenDecision/);
    expect(decisionRowFullSource).not.toMatch(/decision-row-related-note/);
  });

  it('置き換え関係の導線が同じ仕組みの上を移動する（別画面へ行かない）', () => {
    // チップの行き先が onOpenDecision＝「その1件に絞り込んだ一覧」であること。
    const chips = [...decisionRowFullSource.matchAll(/class="decision-link-chip"[\s\S]{0,120}/g)];
    expect(chips.length, '置き換え関係のチップが見つからない').toBeGreaterThan(0);
    for (const m of chips) expect(m[0]).toMatch(/onOpenDecision\(/);
  });

  it('1件を開く経路が一覧の絞り込み条件として組まれている（単票へ戻していない）', () => {
    const handler = /const\s+openDecision\s*=\s*\(([^)]*)\)\s*=>\s*([^;]+);/.exec(appSourceForList);
    expect(handler, 'openDecision の定義が見つからない').not.toBeNull();
    expect(handler![2]).toMatch(/view:\s*'decisions'/);
    expect(handler![2]).toMatch(/formatScopeTarget\(\{\s*type:\s*'decision',\s*id:\s*decisionId\s*\}\)/);
    // 廃止した単票へ戻す変異はここで落ちる。
    expect(handler![2]).not.toMatch(/view:\s*'decision'\b/);
  });
});

describe('URL の条件が画面へ届き、書き戻される（⑤）', () => {
  // ⚠️ **差し戻し1回目で落ちたのはここである。** 新設した条件を画面へ渡す配線に
  // ガードが1本も無く、prop を握り潰す変異（`on={''}` / `scope={''}`）も、書き戻し
  // から落とす変異も、テスト緑のまま素通りした——permalink もカードのリンクも
  // 概要の経路も、全部が素の一覧に着く状態になるのに。
  //
  // 同じファイルが `describe('app が概要タブを URL へ配線している')` を、**まさに
  // この型の回帰**のために既に持っていた。手本が同じファイルの中にあった。
  //
  // 直し方は2段構えにしてある:
  //   1. 条件の対応（URL ⇄ 条件）を純関数へ出し、**値として** decisionFilter.test.ts
  //      が守る（CLAUDE.md 1）。1つでも読み落とす／書き落とす変異はそこで落ちる。
  //   2. prop を1つずつ並べる形をやめ、口を1つにした。ここではその1つを見る。
  const opened = appSource.indexOf('<DecisionsView');
  const element = appSource.slice(opened, appSource.indexOf('/>', opened));

  it('意思決定の一覧を描画している', () => {
    expect(opened).toBeGreaterThan(-1);
  });

  it('URL から起こした条件を渡している（握り潰す口を1つに絞ってある）', () => {
    // 渡す口は1つの prop のまま。値の出所が URL であることを見る。
    // ⚠️ **毎レンダー組み直す形（`conditions={conditionsFromRoute(route)}` を JSX に
    // 直書き）は、飛んでいる応答が返った拍子に利用者の操作を消す**——実測で見つけた
    // 欠陥なので、束ねるなら「route が変わったときだけ組み直す」形であることまで見る。
    // 値としての保証（URL の条件が行になる・操作が消えない）は renderWiring.test.tsx。
    const m = /conditions=\{([A-Za-z0-9_]+|conditionsFromRoute\(route\))\}/.exec(element);
    expect(m, 'conditions に渡している式が読めない').not.toBeNull();
    const passed = m![1];
    if (passed !== 'conditionsFromRoute(route)') {
      expect(appSource, `${passed} が「route が変わったときだけ組み直す条件」ではない`).toMatch(
        new RegExp(`const ${passed} = useMemo\\(\\(\\) => conditionsFromRoute\\(route\\), \\[route\\]\\)`),
      );
    }
  });

  it('条件の変更を URL へ書き戻すハンドラを渡している', () => {
    expect(element).toMatch(/\bonConditionsChange=/);
  });

  it('そのハンドラが navigate を通り、変換の純関数を通る（＝履歴に残り、条件が落ちない）', () => {
    const m = /onConditionsChange=\{\(c\) => ([^}]+)\}/.exec(element);
    expect(m, 'onConditionsChange の中身が読めない').not.toBeNull();
    expect(m![1]).toMatch(/\bnavigate\(/);
    expect(m![1]).toMatch(/view:\s*'decisions'/);
    expect(m![1]).toMatch(/routeParamsFromConditions\(c\)/);
  });

  it('一覧が受け取った条件をそのまま照合に渡している（⑥ 適用の一行を消させない）', () => {
    // 「照合の呼び出しは残して適用の一行だけ削る」変異は、値のテスト
    // （decisionFilter.test.ts『向きが実際に適用される』）で落ちる。ここは一覧が
    // 自前で組み直さず、その純関数を通っていることだけを見る。
    expect(decisionsViewSource).toMatch(/selectDecisions\(decisions \|\| \[\], local, selectCtx\)/);
    expect(decisionsViewSource).toMatch(/selectBase\(decisions \|\| \[\], local, selectCtx\)/);
    // 一覧の中に照合を書き直す変異（純関数を迂回する）はここで落ちる。
    expect(decisionsViewSource).not.toMatch(/scopeMatcher\(/);
  });

  it('問い合わせた結果を捨てていない（⑥ 支配する規則が常に0件になる形）', () => {
    // `setGoverns([])` に差し替えると、支配する規則の一覧が常に空になる
    // ——**この decision が直そうとした欠陥そのもの**が、成功応答の裏で復活する。
    // 取得の成否は実際にネットワークを起こさないと値で検査できないので、ここは
    // 構造で押さえる（射程は下の注記のとおり）。
    const then = /\.then\(\(res\) => \{([\s\S]{0,200}?)\}\)/.exec(decisionsViewSource);
    expect(then, '取得成功時の分岐が見つからない').not.toBeNull();
    expect(then![1]).toMatch(/setGoverns\(res\.entries\)/);
    // 失敗時だけが空集合（全件へ広げない）。
    expect(decisionsViewSource).toMatch(/\.catch\([\s\S]{0,200}setGoverns\(\[\]\)/);
  });
});

describe('1件に絞られたら開いた状態で着地する（②）', () => {
  it('一覧が「結果が1件」を既定の開閉として行に渡している', () => {
    expect(decisionsViewSource).toMatch(/defaultOpen=\{filtered\.length === 1\}/);
  });

  it('それは初期既定であって、利用者が閉じた保存値を上書きしない（01KYGYYN8HRNFQEDMBS3DZRRX7）', () => {
    // `?? defaultOpen` の順序が逆（defaultOpen を優先）だと、閉じた状態の永続復元が
    // 壊れる——共通配線（collapseState）を通していること自体も併せて見る。
    expect(decisionRowFullSource).toMatch(/loadCardSectionOpen\([\s\S]{0,40}\)\s*\?\?\s*defaultOpen/);
    expect(decisionRowFullSource).toMatch(/saveCardSectionOpen\(/);
  });

  it('名指しされた1件が、他の条件で消えない（判断は純関数・値で守る）', () => {
    // 既定の効力フィルタは「効いているものだけ」なので、掛けたままにすると
    // **置き換え済みの意思決定を指す共有リンクが 0 件に着く**。改訂チェーンを辿る
    // 導線は置き換え済みの相手を指すので、ここは日常的に踏まれる経路である。
    // 判断そのものは decisionFilter.namesOneDecision / selectDecisions が持ち、
    // decisionFilter.test.ts が**値として**守る。ここは一覧がそこを通っていること
    // と、0件のときに「その記録が無い」と名指しすることだけを見る。
    expect(decisionsViewSource).toMatch(/namesOneDecision\(local\)/);
    expect(decisionsViewSource).toMatch(/isNamedOne \? t\.decisions\.notFound : t\.decisions\.noMatch/);
  });
});

describe('旧単票の URL が転送で生きる（③）', () => {
  it('app が転送を描いている（画面ごと消していない）', () => {
    expect(appSourceForList).toMatch(/view === 'decision' && <DecisionPermalinkRedirect[\s\S]{0,60}decisionId=\{route\.decisionId\}/);
  });

  it('転送先が「その1件に絞り込んだ一覧」である', () => {
    expect(decisionRedirectSource).toMatch(/view:\s*'decisions'/);
    expect(decisionRedirectSource).toMatch(/formatScopeTarget\(\{\s*type:\s*'decision',\s*id:\s*decisionId\s*\}\)/);
  });

  it('履歴を積まない形で転送する（バックが効かなくなるのを防ぐ）', () => {
    // `location.hash = …` に差し替えると旧 URL と新 URL の2エントリが並び、
    // バックした利用者が旧 URL へ戻され、そこから即座に前へ送り返される。
    expect(decisionRedirectSource).toMatch(/window\.location\.replace\(/);
    expect(decisionRedirectSource).not.toMatch(/window\.location\.hash\s*=/);
  });
});

describe('支配する向きの判定を viewer で作り直していない', () => {
  // ⚠️ 同じ選択規則を2箇所に書くと、CLI と画面で「この記録を支配する規則は何か」に
  // 違う答えが返る余地が復活する（01KXYED61J6QBEX75H2XHVHW7Y の診断）。追補
  // 01KYJV3FYMDFRWQ939NBV2BPAC の「採らなかった選択肢」が名指しで警告した形でもある。
  it('governing のとき、判定は問い合わせの結果だけを見る', () => {
    // 実効タグ（effTagsById）や祖先クロージャで導出する変異はここで落ちる。
    const branch = /if \(direction === 'governing'\) \{[\s\S]{0,400}?\n  \}/.exec(decisionScopeSource);
    expect(branch, 'governing の分岐が見つからない').not.toBeNull();
    expect(branch![0]).toMatch(/governs\.map\(\(g\) => g\.decisionId\)/);
    expect(branch![0]).not.toMatch(/effTagsById|ancestorClosure/);
  });

  it('取得前は1件も通さない（「支配する規則が全部」と一瞬でも嘘をつかない）', () => {
    expect(decisionScopeSource).toMatch(/if \(!governs\) return \(\) => false;/);
  });

  it('一覧が共有の問い合わせ（api.getGoverns）を通っている', () => {
    // 静的書き出しでも同じ答えが返る経路（api.ts が焼き込み済み map を引く）。
    expect(decisionsViewSource).toMatch(/api\s*\n?\s*\.getGoverns\(/);
    expect(decisionsViewSource).toMatch(/governsParams\(/);
    expect(decisionsViewSource).toMatch(/needsGoverns\(/);
  });

  it('viewer 側に祖先展開の再実装が入っていない', () => {
    // decisionScope は照合だけを担う。祖先を自分で辿り始めたらここで落ちる。
    expect(decisionScopeSource).not.toMatch(/parentIds|ancestorClosure/);
  });
});

describe('ナビが「概要 / タグ / 意思決定」の3つ', () => {
  // どの面でどのタブが点くかは navActive（純関数）が持ち、navActive.test.ts が
  // **全ルートを列挙して値で**守る。差し戻し1回目では、家族の定数はそのままに
  // 判定の分岐だけを潰す変異が緑で通った——定数だけを見る形をここに残さない。
  it('3つのタブを描いている', () => {
    expect(headerSource).toMatch(/\['decisions',\s*t\.nav\.decisions/);
    expect(headerSource).toMatch(/\['tags',\s*t\.nav\.tags/);
    expect(headerSource).toMatch(/\['overview',\s*t\.nav\.overview/);
  });

  it('点灯の判定を共有の純関数に委ねている（面の中で分岐を書き直さない）', () => {
    expect(headerSource).toMatch(/isNavActive\(key,\s*view\)/);
    // 家族の一覧を Header の中へ戻す変異はここで落ちる（判定が2箇所に散る）。
    expect(headerSource).not.toMatch(/TAGS_FAMILY|DECISIONS_FAMILY/);
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

describe('概要の各文脈から、同じ条件の一覧へ踏める（01KYKS4Y56FAHRVCWKMQJK4RT6）', () => {
  // ⚠️ **この注記は古くなった。** かつては「このプロジェクトでは概要タブが空（役割 kind を
  // 持つタグが0件）なので、この経路は画面では確かめられない」と書いてあったが、
  // `01KYNV5PYT6A659K8Q3X0NZ9J1` で主題種別が役割 component を宣言し、実機で17枚の
  // シートが描かれるようになった。**画面でも確かめられる。**
  // それでもこのソース照合を残すのは、実機確認が人手だからである（毎回踏むとは限らない）。
  // ⚠️ ただし**ソース照合はガードではない**（`CLAUDE.md` 2）——同じ意味を別の綴りで
  // 書けば通る。振る舞いで押さえるのは `renderWiring.test.tsx` 側の役目。
  it('3つの文脈すべてが、その文脈の対象を渡している', () => {
    // part / 制約はタグ、振る舞いは遷移。取り違えると別のレコードの規則を出す。
    expect(overviewSource).toMatch(/renderRules\('part:' \+ p\.id[\s\S]{0,90}type:\s*'tag',\s*id:\s*p\.id/);
    expect(overviewSource).toMatch(/renderRules\('tx:' \+ b\.id[\s\S]{0,90}type:\s*'transition',\s*id:\s*b\.id/);
    expect(overviewSource).toMatch(/renderRules\('prop:' \+ p\.id[\s\S]{0,90}type:\s*'tag',\s*id:\s*p\.id/);
  });

  it('リンクが実アンカーで、意思決定の一覧を指している', () => {
    expect(overviewSource).toMatch(/<HashLink[\s\S]{0,120}href=\{listHref\}/);
    expect(overviewSource).toMatch(/rulesListHref = \(scopeRef: string\) => routeHash\(\{ view: 'decisions', decisionOn: scopeRef, decisionScope: 'own' \}\)/);
  });

  it('入口がどの条件ゲートの内側にも入っていない（WholeRules と同じ歯止め）', () => {
    // 畳みの内側へ移す変異は、開かなかった利用者に到達手段が伝わらなくする。
    // 1つのゲートだけを見る形にしない——ソース中の `{… && (` を全部回す。
    const gates = [...overviewSource.matchAll(/\{[^{}\n]*&&\s*\(/g)];
    expect(gates.length, '条件ゲートが1つも見つからない（列挙の正規表現が壊れている）').toBeGreaterThan(0);
    for (const m of gates) {
      const gate = extractGate(overviewSource, m.index!);
      expect(gate, `ゲート ${m[0].trim()} の内側に「一覧で開く」がある`).not.toMatch(/class="overview-rules-list-link"/);
    }
    for (const line of overviewSource.split('\n')) {
      if (line.includes('overview-rules-list-link')) {
        expect(line, `入口の行が条件付きになっている: ${line.trim()}`).not.toMatch(/&&/);
      }
    }
    expect(overviewSource).toMatch(/class="overview-rules-list-link"/);
  });

  it('向きが own（インライン展開と同じ集合を指す）', () => {
    // governing にすると「展開して見える件数」と「踏んだ先の件数」が食い違う
    // ——インライン展開が出しているのは rulesFor（decByTarget＝その対象ちょうど）。
    const href = /const rulesListHref = [\s\S]{0,200}?;\n/.exec(overviewSource);
    expect(href, 'rulesListHref の定義が見つからない').not.toBeNull();
    expect(href![0]).toMatch(/decisionScope: 'own'/);
    expect(href![0]).not.toMatch(/governing|subtree/);
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
