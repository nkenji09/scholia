import type { Decision } from '../../types';

/** `decision.at` (ISO 8601 UTC, always) → `YYYY-MM-DD HH:MM` rendered in
    `timezone` (an IANA zone name, e.g. "Asia/Tokyo"). Omitted/empty
    `timezone` renders UTC — the pre-timezone-config behavior, unchanged
    (req.comfortable-viewer.config-editing amend: storage stays UTC always;
    only display is configurable). Call through
    useLookups().formatDecisionAt rather than this directly in components —
    that binds config.timezone once so every screen renders the same
    wall-clock time without each caller threading config through itself. */
export function formatDecisionAt(at: string, timezone?: string): string {
  const d = new Date(at);
  if (Number.isNaN(d.getTime())) return at;
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: timezone || 'UTC',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(d);
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
  return `${get('year')}-${get('month')}-${get('day')} ${get('hour')}:${get('minute')}`;
}

// Currency of a decision relative to the full decision set (D10a).
//   - 'superseded': some OTHER decision's supersedes[] links to this id with
//     mode === 'supersede'. That link outright replaces this decision.
//   - 'amended': some OTHER decision references this id via supersedes[] but
//     only with mode 'amend'/'exception' (or an omitted mode, which is treated
//     as 'amend') — it refines/excepts but does NOT replace it, so it stays
//     current-with-a-caveat rather than dead.
//   - 'current': nothing references it.
// 'superseded' wins over 'amended' when a decision is on the receiving end of
// both kinds of link — a hard replacement dominates a mere refinement.
export type Currency = 'current' | 'superseded' | 'amended';

/** mode omitted ⇒ 'amend' (types.ts SupersedeLink doc / Go model.SupersedeLink). */
export function linkMode(mode: string | undefined): 'supersede' | 'amend' | 'exception' {
  if (mode === 'supersede' || mode === 'exception') return mode;
  return 'amend';
}

// Scan every decision's supersedes[] once and classify which prior decisions
// are superseded vs merely amended. Computed client-side from the full array
// (the list/detail pages get all decisions via api.getRules({})), so it works
// identically in live and static-export mode.
export interface CurrencyIndex {
  supersededIds: Set<string>;
  amendedIds: Set<string>;
  /** targetId -> the decisions that link to it (any mode), for the detail
      page's derived "superseded/amended by" back-references. */
  supersededByMap: Map<string, Decision[]>;
}

export function buildCurrencyIndex(decisions: Decision[]): CurrencyIndex {
  const supersededIds = new Set<string>();
  const amendedIds = new Set<string>();
  const supersededByMap = new Map<string, Decision[]>();
  for (const d of decisions) {
    for (const link of d.supersedes || []) {
      const arr = supersededByMap.get(link.id) || [];
      arr.push(d);
      supersededByMap.set(link.id, arr);
      if (linkMode(link.mode) === 'supersede') supersededIds.add(link.id);
      else amendedIds.add(link.id);
    }
  }
  return { supersededIds, amendedIds, supersededByMap };
}

export function currencyOf(id: string, index: CurrencyIndex): Currency {
  if (index.supersededIds.has(id)) return 'superseded';
  if (index.amendedIds.has(id)) return 'amended';
  return 'current';
}

// ---------------------------------------------------------------------------
// 画面に出す「効力」は2値（01KYHW54B8ZXH0NEPH2J7N1X39 条項1）
//
// 記録側の3値（supersede / amend / exception・01KXWPQDGMDB01V86KZ91M0BPQ）は不変。
// 変えるのは**画面の状態列だけ**で、そこに出せる値は「効いている」「置き換え済み」
// の2つに限る。3値をそのまま状態の3値として写していたため「現行 ⇔ 改訂」という
// 存在しない対立が生まれ、まだ効いている条項が履歴側だと誤読された——それが
// この2値化の起点。amend / exception で参照された decision は**効いている**。
//
// 「後続に部分改訂・例外が付いている」は状態ではなく**付帯情報**として、
// relatedDecisions() の件数と導線で出す（条項2）。

/** 画面に出せる効力。記録の3値とは別物なので `Currency` と混ぜない。 */
export type Effect = 'in-force' | 'replaced';

/** 効いているか。畳むのは supersede で置き換えられたものだけ（保守的な導出）。 */
export function effectOf(id: string, index: CurrencyIndex): Effect {
  return index.supersededIds.has(id) ? 'replaced' : 'in-force';
}

export function isInForce(id: string, index: CurrencyIndex): boolean {
  return effectOf(id, index) === 'in-force';
}

/** 後続で部分改訂・例外を付けた decision（＝併せて読むべきもの・条項2）。
    supersede で置き換えたものはここに含めない（それは「置き換えた側」で、
    replacedBy() が返す）。 */
export function relatedDecisions(id: string, index: CurrencyIndex): Decision[] {
  return (index.supersededByMap.get(id) || []).filter((d) =>
    (d.supersedes || []).some((l) => l.id === id && linkMode(l.mode) !== 'supersede'),
  );
}

/** この decision を全文置換した側（条項3: 置き換えられた側からそこへ辿れる）。 */
export function replacedBy(id: string, index: CurrencyIndex): Decision | undefined {
  return (index.supersededByMap.get(id) || []).find((d) =>
    (d.supersedes || []).some((l) => l.id === id && linkMode(l.mode) === 'supersede'),
  );
}
