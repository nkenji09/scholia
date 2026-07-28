import { describe, expect, it } from 'vitest';
import decisionListSource from './components/decisions/DecisionList.tsx?raw';
import overviewSource from './components/overview/OverviewView.tsx?raw';
import browseCssSource from './styles/views/browse.css?raw';
import decisionsCssSource from './styles/views/decisions.css?raw';
import overviewCssSource from './styles/views/overview.css?raw';
import vocabCssSource from './styles/views/vocab.css?raw';
import configCssSource from './styles/views/config.css?raw';
import commentsCssSource from './styles/views/comments.css?raw';
import flowCssSource from './styles/views/flow.css?raw';
import homeCssSource from './styles/views/home.css?raw';

// #6 フェーズ2で直した回帰2件のガード（result.md 参照）。
//
// 前任（#5・8b5e9fb）が「カードの展開した全文の器」に既存の別意味のクラス名
// （一覧の要約1行・-webkit-line-clamp:2）を再利用したことで、展開しても
// 2行に切られたまま隠れる回帰が起きた。ここは「クラス名がまた衝突したら
// 落ちる」形で守る——文字列の存在確認ではなく、CSS を実際にパースして
// line-clamp が掛かる集合にそのクラスが入っていないかを見る。

/** css 文字列から `{ ... }` ブロックを列挙し、line-clamp を含むブロックのセレクタ一覧を返す。 */
function classesWithLineClamp(cssSources: string[]): Set<string> {
  const hit = new Set<string>();
  for (const css of cssSources) {
    const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, '');
    for (const m of withoutComments.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
      const [, selector, body] = m;
      if (!/line-clamp/.test(body)) continue;
      for (const part of selector.split(',')) {
        for (const cls of part.matchAll(/\.([A-Za-z0-9_-]+)/g)) {
          hit.add(cls[1]);
        }
      }
    }
  }
  return hit;
}

// 実際に読み込まれる CSS 全部（index.css の @import 一覧と同じ集合）。特定の2ファイルだけを
// 見ると「別ファイルに同名クランプが増えた」を見逃す。
const ALL_CSS = [
  browseCssSource,
  decisionsCssSource,
  overviewCssSource,
  vocabCssSource,
  configCssSource,
  commentsCssSource,
  flowCssSource,
  homeCssSource,
];

describe('カードの展開した全文の器に line-clamp が掛からない（①の再発防止）', () => {
  it('DecisionList が展開時に使うコンテナのクラス名を取り出せる', () => {
    const m = /\{open && \(\s*<div class="([a-z0-9-]+)">\s*<Markdown text=\{d\.why\}/.exec(decisionListSource);
    expect(m, 'DecisionList の展開コンテナが見つからない（構造が変わった？）').not.toBeNull();
  });

  it('そのクラス名は browse.css で定義されている', () => {
    const m = /\{open && \(\s*<div class="([a-z0-9-]+)">\s*<Markdown text=\{d\.why\}/.exec(decisionListSource);
    const cls = m![1];
    expect(browseCssSource).toMatch(new RegExp(`\\.${cls}\\s*\\{`));
  });

  it('そのクラス名は、どの CSS ファイルの line-clamp ルールにも入っていない', () => {
    const m = /\{open && \(\s*<div class="([a-z0-9-]+)">\s*<Markdown text=\{d\.why\}/.exec(decisionListSource);
    const cls = m![1];
    const clamped = classesWithLineClamp(ALL_CSS);
    expect([...clamped]).not.toContain(cls);
  });
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
