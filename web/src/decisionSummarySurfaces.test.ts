import { describe, expect, it } from 'vitest';
import decisionListSource from './components/decisions/DecisionList.tsx?raw';
import overviewSource from './components/overview/OverviewView.tsx?raw';
import indexCssSource from './index.css?raw';
import fontsCssSource from './styles/fonts.css?raw';
import tokensCssSource from './styles/tokens.css?raw';
import baseCssSource from './styles/base.css?raw';
import layoutCssSource from './styles/layout.css?raw';
import listItemCssSource from './styles/components/list-item.css?raw';
import chipCssSource from './styles/components/chip.css?raw';
import cardCssSource from './styles/components/card.css?raw';
import decisionRowCssSource from './styles/components/decision-row.css?raw';
import resizerCssSource from './styles/components/resizer.css?raw';
import homeCssSource from './styles/views/home.css?raw';
import overviewCssSource from './styles/views/overview.css?raw';
import browseCssSource from './styles/views/browse.css?raw';
import vocabCssSource from './styles/views/vocab.css?raw';
import configCssSource from './styles/views/config.css?raw';
import commentsCssSource from './styles/views/comments.css?raw';
import flowCssSource from './styles/views/flow.css?raw';
import decisionsCssSource from './styles/views/decisions.css?raw';
import markdownCssSource from './styles/markdown.css?raw';

// #6 で直した回帰2件のガード（result.md 参照）。
//
// ①（#5・8b5e9fb の回帰）: 「カードの展開した全文の器」に既存の別意味のクラス名
//   （一覧の要約1行・-webkit-line-clamp:2）を再利用したため、展開しても2行に
//   切られたまま隠れた。
// ②'（本ブランチの初版が持ち込んだ欠陥）: 生テキストを出していた頃の名残りの
//   white-space: pre-wrap を <Markdown> の器に残置したため、markdown-it が
//   要素間に出す `\n` が空行として描画され、同じ本文が 2.26 倍に膨らんだ。
//
// どちらも「そのクラス名に、掛かってはいけない宣言が掛かっている」という同じ型なので、
// CSS を実際にパースして宣言の有無を見る同じ仕掛けで守る（文字列の存在確認ではない）。
//
// ⚠️ このガードが**守れない**もの（称する範囲を実際に落ちる範囲より広く書かないための明示）:
//
// - **祖先に掛かった clamp / pre-wrap は検知しない。** ここが見るのは「その器自身の
//   クラス名に当たるルール」だけで、`.decision-row` など親側に line-clamp や
//   white-space が足された場合は素通りする（white-space は継承プロパティなので
//   祖先に付けば実害が出る）。DOM を起こす harness が要るため、そこは実機計測が担う。
// - **comments.css の `<Markdown>` 面（`.comment-item-text` / `.comment-decision-why`）は
//   対象に含めていない。** 両者は同じ pre-wrap × Markdown の組み合わせを持つが、本ブランチ
//   より前から存在する別項目（レビュー should-1）で、ここで落とすと未着手の欠陥で赤くなる。
//   守っているのは下の SURFACES に挙げた2面だけ。

/** css 文字列から `{ ... }` ブロックを列挙し、宣言が declRe に一致するブロックのクラス名を集める。 */
function classesWithDeclaration(cssSources: string[], declRe: RegExp): Set<string> {
  const hit = new Set<string>();
  for (const css of cssSources) {
    const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, '');
    for (const m of withoutComments.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      const [, selector, body] = m;
      if (!declRe.test(body)) continue;
      for (const part of selector.split(',')) {
        for (const cls of part.matchAll(/\.([A-Za-z0-9_-]+)/g)) {
          hit.add(cls[1]);
        }
      }
    }
  }
  return hit;
}

// 実際に読み込まれる CSS 全部。index.css の @import は Vite が1枚のバンドルにまとめる
// ＝ルートごとに分かれないので、どのファイルに書かれたルールでもこの器に当たりうる。
// **この集合が @import 一覧と一致していることは下のテストが検証する**——「全部見ている」
// と称しながら一部しか見ていない状態（実際に一度そうなった）を、宣言ではなく検査で防ぐ。
const CSS_FILES: Array<{ path: string; source: string }> = [
  { path: './styles/fonts.css', source: fontsCssSource },
  { path: './styles/tokens.css', source: tokensCssSource },
  { path: './styles/base.css', source: baseCssSource },
  { path: './styles/layout.css', source: layoutCssSource },
  { path: './styles/components/list-item.css', source: listItemCssSource },
  { path: './styles/components/chip.css', source: chipCssSource },
  { path: './styles/components/card.css', source: cardCssSource },
  { path: './styles/components/decision-row.css', source: decisionRowCssSource },
  { path: './styles/components/resizer.css', source: resizerCssSource },
  { path: './styles/views/home.css', source: homeCssSource },
  { path: './styles/views/overview.css', source: overviewCssSource },
  { path: './styles/views/browse.css', source: browseCssSource },
  { path: './styles/views/vocab.css', source: vocabCssSource },
  { path: './styles/views/config.css', source: configCssSource },
  { path: './styles/views/comments.css', source: commentsCssSource },
  { path: './styles/views/flow.css', source: flowCssSource },
  { path: './styles/views/decisions.css', source: decisionsCssSource },
  { path: './styles/markdown.css', source: markdownCssSource },
];
const ALL_CSS = CSS_FILES.map((f) => f.source);

/** decision の全文（why）を <Markdown> で出す面と、その本文に効くクラス名の取り出し方。 */
const SURFACES: Array<{ name: string; source: string; pattern: RegExp; definedIn: string }> = [
  {
    // <div class="…"><Markdown text={d.why} /></div>。white-space は継承するので、
    // Markdown 自身ではなくこの器のクラスが本文に効く。
    name: 'レコードカードの意思決定欄（DecisionList）',
    source: decisionListSource,
    pattern: /\{open && \(\s*<div class="([a-z0-9-]+)">\s*<Markdown text=\{d\.why\}/,
    definedIn: 'browse.css',
  },
  {
    // <Markdown text={d.why} class="…" />。Markdown はこれを markdown-body に足して出す。
    name: '概要タブの規則展開（OverviewView）',
    source: overviewSource,
    pattern: /\{open && <Markdown\s+text=\{d\.why\}\s+class="([a-z0-9-]+)"\s*\/>\}/,
    definedIn: 'overview.css',
  },
];

function containerClassOf(surface: (typeof SURFACES)[number]): string {
  const m = surface.pattern.exec(surface.source);
  expect(m, `${surface.name}: 全文の器が想定の形で見つからない（構造が変わった？）`).not.toBeNull();
  return m![1];
}

describe('ALL_CSS が実際に読み込まれる CSS 全部と一致している', () => {
  it('index.css の @import 一覧と過不足なく一致する', () => {
    // 「全部見ている」というコメントを、コメントではなく検査で保証する。新しい CSS を
    // @import しただけでこのテストが落ちるので、ガードの守備範囲が黙って狭まらない。
    const imported = [...indexCssSource.matchAll(/@import\s+'([^']+)'/g)].map((m) => m[1]);
    expect(imported.length).toBeGreaterThan(0);
    expect(CSS_FILES.map((f) => f.path).sort()).toEqual([...imported].sort());
  });

  it('各ファイルの中身が実体化している（vitest の CSS 空文字化に気づく）', () => {
    // vitest は既定で *.css の import を（?raw 付きでも）空文字列に潰す。空のまま
    // 通ると、以降の「clamp が無い」「pre-wrap が無い」は全部の空振りで緑になる。
    for (const f of CSS_FILES) {
      expect(f.source.length, `${f.path} が空（vite.config.ts の test.css.include を確認）`).toBeGreaterThan(0);
    }
  });
});

describe('decision の全文を出す器に line-clamp が掛からない（①の再発防止）', () => {
  const clamped = classesWithDeclaration(ALL_CSS, /line-clamp/);

  for (const surface of SURFACES) {
    it(`${surface.name}: 器のクラス名が ${surface.definedIn} で定義されている`, () => {
      const cls = containerClassOf(surface);
      const css = CSS_FILES.find((f) => f.path.endsWith('/' + surface.definedIn))!.source;
      expect(css).toMatch(new RegExp(`\\.${cls}\\s*\\{`));
    });

    it(`${surface.name}: 器のクラス名が、どの CSS ファイルの line-clamp ルールにも入っていない`, () => {
      expect([...clamped]).not.toContain(containerClassOf(surface));
    });
  }
});

describe('decision の全文を出す器に white-space: pre-wrap が掛からない（②’の再発防止）', () => {
  // <Markdown> はブロック要素の HTML を出す。pre-wrap を掛けると markdown-it が
  // 要素間に出す `\n` が空行として描画され、本文が実測 2.26 倍に膨らむ
  // （394px / 本来 174px）。生テキストを出していた頃の CSS をそのまま流用すると踏む。
  const preWrapped = classesWithDeclaration(ALL_CSS, /white-space\s*:\s*pre-wrap/);

  for (const surface of SURFACES) {
    it(`${surface.name}: 器のクラス名が、どの CSS ファイルの pre-wrap ルールにも入っていない`, () => {
      expect([...preWrapped]).not.toContain(containerClassOf(surface));
    });
  }
});

describe('概要タブの規則展開が Markdown を通っている（②の再発防止）', () => {
  it('why を出す行が <Markdown> 経由である（生の {d.why} 直書きに戻っていない）', () => {
    // 文字列 "Markdown" がファイルのどこかにあるだけでは守れない（import しただけでも
    // マッチしてしまう）。d.why を直接子要素に置く該当行そのものを見る。
    expect(overviewSource).not.toMatch(/>\{d\.why\}</);
    const m = /\{open && <Markdown\s+text=\{d\.why\}[^>]*\/>\}/.exec(overviewSource);
    expect(m, '概要タブの規則展開が <Markdown text={d.why} /> の形で見つからない').not.toBeNull();
  });
});
