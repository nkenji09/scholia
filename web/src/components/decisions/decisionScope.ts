import type { Decision, GovernsRef } from '../../types';

// 絞り込み条件の「どの対象か」と「どの向きか」（01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// この decision が据えたのは「絞り込み条件 → 畳まれた詳細が並ぶ一覧 → その場で
// 全文を読む」という1つの仕組みで、permalink も概要からの経路も既存フィルタも
// その上に乗る。乗せるために足りなかったのが次の2つ:
//
//   1. **「この1件」を条件として表せない。** 表せないので、1件を読むには専用の
//      画面が要った。ここで `decision:<id>` を対象の1形として扱えるようにすると、
//      permalink は「1件に絞り込んだ一覧」になり、単票が要らなくなる。
//   2. **向きが1つしかない。** タグで絞ると返るのは「そのタグと**配下**」で、
//      「この記録を**支配している**規則（自身＋祖先）」とは向きが逆だった
//      （追補 01KYJV3FYMDFRWQ939NBV2BPAC が確定した事実。実測で 8/75 のタグが
//      「継承した規則 N件」と開示した直下から 0 件の面に着いていた）。
//
// ⚠️ **`governing` の判定はここでは行わない。** 同じ選択規則を viewer 側に
// 書き直すと、CLI と画面で「この記録を支配する規則は何か」に違う答えが返る余地が
// 復活する——01KXYED61J6QBEX75H2XHVHW7Y が診断した欠陥そのもので、追補
// 01KYJV3FYMDFRWQ939NBV2BPAC の「採らなかった選択肢」が名指しで警告した形でもある。
// 判定は GET /api/governs（CLI `scholia rules` と同じ Go コア index.GovernsFor*）に
// 委ね、ここはその結果（decision id の集合）を受け取って照合するだけにしてある。
// 静的書き出しでも同じ答えが返る（api.ts の getGoverns が焼き込み済み map を引く）。

/** 対象の種別。`transition` は CLI の `--on transition:<id>` と同じ綴りに揃えてある
    （面間整合 01KXYED62CEKBY97D7X66BMC9A——同じ語彙を2通りに綴らない）。 */
export type ScopeTargetType = 'tag' | 'transition' | 'vocab' | 'decision';

export interface ScopeTarget {
  type: ScopeTargetType;
  id: string;
}

/** 向き。既定は `subtree`（現行の絞り込みが返しているもの＝その対象と配下）。 */
export type ScopeDirection = 'own' | 'governing' | 'subtree';

export const DEFAULT_SCOPE: ScopeDirection = 'subtree';

const TYPES: Record<string, ScopeTargetType> = {
  tag: 'tag',
  transition: 'transition',
  // 旧綴り／手打ちの受け口。読むときだけ受け、書くときは必ず `transition:` に
  // 正規化する（2通りの綴りが URL に出回らないように）。
  tx: 'transition',
  vocab: 'vocab',
  decision: 'decision',
};

/** `"tag:req.x"` → `{type:'tag', id:'req.x'}`。解釈できない値は undefined
    （＝条件なし。壊れた URL でも一覧そのものは開く）。 */
export function parseScopeTarget(raw: string | undefined): ScopeTarget | undefined {
  if (!raw) return undefined;
  const idx = raw.indexOf(':');
  if (idx <= 0) return undefined;
  const type = TYPES[raw.slice(0, idx)];
  const id = raw.slice(idx + 1);
  if (!type || !id) return undefined;
  return { type, id };
}

export function formatScopeTarget(target: ScopeTarget): string {
  return `${target.type}:${target.id}`;
}

export function parseScopeDirection(raw: string | undefined): ScopeDirection {
  return raw === 'own' || raw === 'governing' || raw === 'subtree' ? raw : DEFAULT_SCOPE;
}

/** その対象・向きの組で、`/api/governs` に問い合わせる必要があるか。
    `governing` のときだけ true——他の向きは手元の decision 集合だけで決まる。
    `decision` を対象にしたときは向きが意味を持たない（1件そのものを指す）。 */
export function needsGoverns(target: ScopeTarget | undefined, direction: ScopeDirection): boolean {
  return !!target && target.type !== 'decision' && direction === 'governing';
}

/** `/api/governs` の呼び出しパラメータ。呼ばない組では undefined。 */
export function governsParams(
  target: ScopeTarget | undefined,
  direction: ScopeDirection,
): { tag?: string; tx?: string; vocab?: string } | undefined {
  if (!needsGoverns(target, direction)) return undefined;
  const t = target!;
  if (t.type === 'tag') return { tag: t.id };
  if (t.type === 'transition') return { tx: t.id };
  return { vocab: t.id };
}

export interface ScopeMatcherInput {
  target: ScopeTarget | undefined;
  direction: ScopeDirection;
  /** decision id → その実効タグ集合（own タグの祖先クロージャ）。`subtree` が使う。 */
  effTagsById: Map<string, Set<string>>;
  /** `/api/governs` が返した entries。`governing` のときだけ渡る（未取得は undefined）。 */
  governs: GovernsRef[] | undefined;
}

/**
 * 「対象」と「向き」の条件に一致するかを返す述語を作る。
 *
 * - 対象が無ければ常に true（この条件は掛かっていない）。
 * - `decision:<id>` — その1件だけ。向きは見ない。
 * - `own` — その対象ちょうどを target とする decision（CLI の
 *   `scholia decision list --on <ref>` と同じ「完全一致・祖先展開なし」）。
 * - `governing` — `/api/governs` が返した集合。**未取得のあいだは1件も通さない**
 *   ——取得前に全件を見せると「支配する規則の全体」を名乗る一覧が一瞬すべてを
 *   出すことになり、名乗りと中身が食い違う。
 * - `subtree` — その対象と配下。tag のときだけ実効タグ集合への包含で判定し
 *   （現行の絞り込みと同じ・01KXZK5BWEX3HH15B78E4Z3BXK）、配下の概念を持たない
 *   transition / vocab では `own` に落ちる。
 */
export function scopeMatcher({ target, direction, effTagsById, governs }: ScopeMatcherInput): (d: Decision) => boolean {
  if (!target) return () => true;
  if (target.type === 'decision') return (d) => d.id === target.id;

  if (direction === 'governing') {
    if (!governs) return () => false;
    const ids = new Set(governs.map((g) => g.decisionId));
    return (d) => ids.has(d.id);
  }

  const exact = (d: Decision) => d.target.type === target.type && d.target.id === target.id;
  if (direction === 'own') return exact;

  // subtree
  if (target.type !== 'tag') return exact;
  return (d) => !!effTagsById.get(d.id)?.has(target.id);
}
