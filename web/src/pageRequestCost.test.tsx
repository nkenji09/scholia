// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest';
import { mountApp, resetBrowserState, waitFor } from './testing/renderHarness';
import type { Mounted } from './testing/renderHarness';
import { installScaleServer, makeCorpus } from './testing/scaleHarness';
import type { Corpus, ScaleServer } from './testing/scaleHarness';
import { VIEW_NAMES } from './router';
import type { ViewName } from './router';

// ===========================================================================
// 画面1枚を開く費用の歯止め（正本 01KZ5N5CJ2VFMZAGSFPSCZAMTZ の「歯止め」節）
// ===========================================================================
//
// 正本が置き方まで定めている。守るのは**形**であって時間ではない:
//
//   ・1画面が投げる要求の本数が、レコード件数に比例して増えないこと
//   ・同じレコードを1画面で二度取りに行かないこと
//
// ## 何を、どう数えるか
//
// 🔴 **製品が自分で申告した数は使わない。** 数えるのは**実際に fetch まで届いた
// 要求**で、偽サーバ（scaleHarness）が順序つきで記録したものである。製品の
// 合成ルート（root.tsx の AppRoot）をそのまま起こすので、どの面がどう取るかを
// テスト側が写し取ってはいない——写しを作ると、製品が経路を変えたときに
// harness だけ古いまま緑になる。
//
// 🔴 **標本の上端を本番より上に置く。** 小さいほうは本 repo の `.scholia` と
// 同程度（タグ 84）、大きいほうはその 12 倍（タグ 1,008）。利用者の実機は本 repo の
// 3.17 倍（タグ ≈260）なので、**本番は標本の内側**にある。件数で分岐する形
// （「N 件までは一括、それ以上は1件ずつ」等）を標本の外に出さないため。
//
// 🔴 **面は router の一覧（VIEW_NAMES）から回る。** 面を1つ足せば型の都合で
// 必ずその配列に載るので、**新しい面がこの歯止めから漏れない**
// （CLAUDE.md 5「新しく作った面には、ガードを置き忘れる」）。
//
// ## このガードが落とさないもの（射程・CLAUDE.md 6）
//
//  1. **1本あたりの費用。** 正本の「歯止め」節が同じ限界を名指ししている——
//     本数さえ増えなければ、1本が O(N) の仕事をしていても緑になる。そちらは
//     Go 側の `internal/viewer/index_cache_test.go` が「起動後の要求がストアを
//     読み直さないこと」として別に守る。
//  2. **応答の量。** 一括にすれば本数は減るが、運ぶバイトは減らない。量の要件は
//     別の正本（req.default-output-economy・01KZ5ACN6P279S96D5M3AHY9HZ）の話で、
//     この歯止めは1バイトも見ていない。
//  3. **Go 側の答えの正しさ。** 偽サーバは corpus をそのまま返す（Go の選択規則を
//     再実装しない）。一括の口と1件ずつの口が同じ答えを返すかは
//     `internal/viewer/bulk_test.go` が全レコードで突き合わせる。
//  4. **利用者が操作した後に投げる要求。** ここが見るのは「開いて静まるまで」で、
//     絞り込みやカードを開く操作の後に増える本数は見ていない。
//  5. **利用者が画面を移った後の取り直し。** 畳んでいるのは「同時に飛んでいる
//     同じ GET」だけなので、別の画面へ移ってから同じ口を叩き直すのは重複に
//     数えない（そこは鮮度の側であって、1枚を開く費用ではない）。

const SMALL = 1; // ≈ 本 repo の `.scholia`（タグ 84 / 語彙 168 / 遷移 72 / 決定 180）
const LARGE = 12; // 本番（利用者の実機 ≈3.17×＝タグ 260）より上に置く（タグ 1,008）

/** その面を開くための hash。id を取る面は corpus の実 id を差す（素の面と両方見る）。 */
function hashesFor(view: ViewName, c: Corpus): string[] {
  const tag = c.tags.find((t) => t.kind === 'component')!.id;
  const tx = c.transitions[0].id;
  const vocabId = c.vocab[0].id;
  const action = c.transitions[0].action;
  const decisionId = c.decisions[0].id;
  switch (view) {
    case 'spec':
      return ['#/spec', `#/spec/${encodeURIComponent(tag)}`];
    case 'browse':
      return ['#/browse', `#/browse/tx/${encodeURIComponent(tx)}`];
    case 'vocab':
      return ['#/vocab', `#/vocab/${encodeURIComponent(vocabId)}`];
    case 'flow':
      return [`#/flow/${encodeURIComponent(action)}`];
    case 'decision':
      return [`#/decision/${encodeURIComponent(decisionId)}`];
    case 'overview':
      return ['#/overview', `#/overview/${encodeURIComponent(tag)}`];
    default:
      return [`#/${view}`];
  }
}

interface Opened {
  /** その hash を開いてから静まるまでに来た要求すべて。 */
  requests: string[];
  /** 画面ごとに切った要求（URL が変われば別の画面）。転送する経路
      （`#/decision/<id>` は `#/decisions?on=…` へ replace する）では
      2枚ぶんになる——**「1画面で二度取りに行かない」は1枚ごとに見る。**
      切り方は hashchange という**事象**で決めるので、どの経路が転送するかを
      テスト側が知っている必要はない。 */
  perPage: string[][];
  unhandled: string[];
}

/** その hash を開き、要求が出尽くすまで待って、来た要求を返す。 */
async function openPage(hash: string, scale: number): Promise<Opened> {
  const corpus = makeCorpus(scale);
  const server = installScaleServer(corpus);
  let mounted: Mounted | null = null;
  const bounds: number[] = [];
  const onHashChange = () => bounds.push(server.requests.length);
  window.addEventListener('hashchange', onHashChange);
  try {
    mounted = mountApp(hash);
    // 「出尽くした」の判定: 本数が増えなくなってから一定回数変化しないこと。
    // 固定の待ち時間にすると、遅い機械でだけ取りこぼす（単位AT が
    // 「6秒は再現しない」と誤読しかけたのと同じ罠）。
    let last = -1;
    let stable = 0;
    await waitFor(
      () => {
        if (server.requests.length === last) stable++;
        else {
            stable = 0;
            last = server.requests.length;
        }
        return stable >= 25 && last > 0;
      },
      `${hash} の要求が出尽くす`,
      60000,
    );
    const requests = [...server.requests];
    const cuts = [0, ...bounds, requests.length];
    const perPage: string[][] = [];
    for (let i = 0; i + 1 < cuts.length; i++) {
      const seg = requests.slice(cuts[i], cuts[i + 1]);
      if (seg.length) perPage.push(seg);
    }
    return { requests, perPage, unhandled: [...server.unhandled] };
  } finally {
    window.removeEventListener('hashchange', onHashChange);
    mounted?.unmount();
    server.restore();
    resetBrowserState();
  }
}

afterEach(() => {
  resetBrowserState();
});

// 1つの面につき **2回だけ**開く（小・大）。本数の検査と二度取りの検査を
// 別々の it に分けると同じ画面を4回起こすことになり、面の数だけ倍の時間がかかる。
describe('画面1枚を開く費用（正本 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1）', () => {
  for (const view of VIEW_NAMES) {
    for (const hash of hashesFor(view, makeCorpus(SMALL))) {
      it(
        `${hash}`,
        async () => {
          const small = await openPage(hash, SMALL);
          const large = await openPage(hash, LARGE);

          // harness が製品の経路を取りこぼしていないこと（取りこぼすと
          // 「本数が増えていない」が、答えられなかっただけの偽の緑になる）。
          expect(small.unhandled, `${hash}: harness が答えを持たない口`).toEqual([]);
          expect(large.unhandled, `${hash}: harness が答えを持たない口`).toEqual([]);

          // 条項1 前半: 本数がレコード件数に比例しない。
          expect(
            large.requests.length,
            `${hash}: レコードを ${LARGE / SMALL} 倍にしたら要求が ${small.requests.length} → ${large.requests.length} 本に増えた\n` +
              `大きいほうだけに出た口: ${JSON.stringify(diffPaths(small.requests, large.requests).slice(0, 8))}`,
          ).toBe(small.requests.length);

          // 条項1 後半: **同じ口を二度叩かない**。
          //
          // 口の種類を選り分けない。一括の口（`/api/spec`・`/api/governs?all=1`・
          // `/api/transitions?detail=1`）は**1本が全レコードを運ぶ**ので、それを
          // 二度叩くことは全レコードを二度取りに行くことと同じである。
          // 「レコードを名指しした口だけ見る」形にすると、まさにそこが抜ける
          // （実見: pendingDiff の初回解決で一括の口が2本出る変異が素通りした）。
          for (const [i, page] of small.perPage.entries()) {
            expect(duplicates(page), `${hash}: 同じ口を二度叩いている（${i + 1} 枚目）`).toEqual([]);
          }
          for (const [i, page] of large.perPage.entries()) {
            expect(duplicates(page), `${hash}: 同じ口を二度叩いている（大きい標本・${i + 1} 枚目）`).toEqual([]);
          }
        },
        // 大きい標本は 1,000 件のカードを実際に描くので、vitest の既定
        // （5 秒）では足りない。**標本を小さくして既定に収めない**——
        // 上端を本番より下げると、件数で分岐する形が標本の外に出る。
        120000,
      );
    }
  }
});

/** 2回以上来た要求（url と回数）。 */
function duplicates(requests: string[]): Array<[string, number]> {
  const seen = new Map<string, number>();
  for (const r of requests) seen.set(r, (seen.get(r) ?? 0) + 1);
  return [...seen.entries()].filter(([, n]) => n > 1);
}

/** 大きい側にだけ現れた path の種類（エラー文言を読める形にするためだけの補助）。 */
function diffPaths(small: string[], large: string[]): string[] {
  const shape = (r: string) => r.replace(/\/[^/?]+$/, '/<id>');
  const smallCounts = new Map<string, number>();
  for (const r of small) smallCounts.set(shape(r), (smallCounts.get(shape(r)) ?? 0) + 1);
  const largeCounts = new Map<string, number>();
  for (const r of large) largeCounts.set(shape(r), (largeCounts.get(shape(r)) ?? 0) + 1);
  const out: string[] = [];
  for (const [k, n] of largeCounts) {
    const m = smallCounts.get(k) ?? 0;
    if (n > m) out.push(`${k} ${m}→${n}`);
  }
  return out;
}

export type { ScaleServer };
