import type { Decision, GovernsRef } from '../../types';
import { DEFAULT_SCOPE, parseScopeDirection, parseScopeTarget, scopeMatcher } from './decisionScope';

// 意思決定の一覧の「絞り込み条件」を、画面から切り離して呼べる形にしたもの
// （01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// **なぜ純関数にするか。** CLAUDE.md「配線ガードの書き方」1 の適用である。
// 差し戻し1回目のレビューで、次の4つが**テスト緑のまま**素通りした:
//
//   ・URL の条件を画面へ渡す prop を握り潰す（`on={''}` / `scope={''}`）
//     → permalink もカードのリンクも概要の経路も、全部 158 件の素の一覧に着く
//   ・書き戻しから対象と向きを落とす
//     → 他の条件を1つ触った瞬間に、対象と向きが URL から黙って消える
//   ・照合の呼び出しだけ残して**適用の一行を削る**
//     → `scope=governing` が全件を返す（この decision が直そうとした欠陥そのもの）
//
// どれも「ソース文字列がそこに在るか」を見る検査では捕まらない型である
// （同じ意味を別の綴りで書けば通る・CLAUDE.md 2）。**判断を値として検査できる形に
// 移せば、綴りに関係なく落ちる**——ここがその移し先。
//
// 画面側（DecisionsView / app）はこの3つを呼ぶだけにしてある:
//   conditionsFromRoute   URL → 条件（読み）
//   routeParamsFromConditions  条件 → URL（書き戻し）
//   selectDecisions       条件 → 見せる集合
// prop を1つずつ渡す形をやめたのも同じ理由で、握り潰せる口を減らすため。

export type TargetKindFilter = 'all' | 'transition' | 'tag' | 'vocab';
export type CurrencyFilter = 'all' | 'current' | 'superseded';
export type PeriodFilter = 'all' | '30d' | '90d' | '1y';

/** 一覧に掛かっている条件の全体。URL とも画面の widget とも1:1に対応する。 */
export interface DecisionConditions {
  query: string;
  targetKind: TargetKindFilter;
  /** タグ AND（widget 由来・複数）。 */
  tags: string[];
  currency: CurrencyFilter;
  period: PeriodFilter;
  /** 対象（リンク由来・単一）。`tag:<id>` 等。'' = 条件なし。 */
  on: string;
  /** 向き。'' = 既定（subtree）。 */
  scope: string;
}

/** router.Route のうち、この一覧が使う分だけ。 */
export interface DecisionRouteParams {
  searchQuery?: string;
  decisionTargetKind?: string;
  decisionTag?: string;
  decisionCurrency?: string;
  decisionPeriod?: string;
  decisionOn?: string;
  decisionScope?: string;
}

const TARGET_KINDS: TargetKindFilter[] = ['all', 'transition', 'tag', 'vocab'];
const CURRENCIES: CurrencyFilter[] = ['all', 'current', 'superseded'];
const PERIODS: PeriodFilter[] = ['all', '30d', '90d', '1y'];

/** 効いていないものは既定で本文に混ぜない（01KYHW54B8ZXH0NEPH2J7N1X39 条項4）。
    一覧には畳む器が無いので、既定の絞り込みがその役目を果たす——なので効力だけ
    既定が 'all' ではなく 'current'。 */
export const DEFAULT_CURRENCY: CurrencyFilter = 'current';

export const EMPTY_CONDITIONS: DecisionConditions = {
  query: '',
  targetKind: 'all',
  tags: [],
  currency: DEFAULT_CURRENCY,
  period: 'all',
  on: '',
  scope: '',
};

const oneOf = <T extends string>(allowed: T[], raw: string | undefined, fallback: T): T =>
  allowed.includes((raw || '') as T) ? ((raw || '') as T) : fallback;

export const splitTags = (v: string | undefined): string[] => (v ? v.split(',').filter(Boolean) : []);

/** URL → 条件。省略された値は既定に落ちる（既定は URL に書かないので、逆向きと対になる）。 */
export function conditionsFromRoute(p: DecisionRouteParams): DecisionConditions {
  return {
    query: p.searchQuery || '',
    targetKind: oneOf(TARGET_KINDS, p.decisionTargetKind, 'all'),
    tags: splitTags(p.decisionTag),
    currency: oneOf(CURRENCIES, p.decisionCurrency, DEFAULT_CURRENCY),
    period: oneOf(PERIODS, p.decisionPeriod, 'all'),
    on: p.decisionOn || '',
    scope: p.decisionScope || '',
  };
}

/** 条件 → URL。**既定値は省く**（q/k と同じ扱いで URL を汚さない）。 */
export function routeParamsFromConditions(c: DecisionConditions): DecisionRouteParams {
  return {
    searchQuery: c.query || undefined,
    decisionTargetKind: c.targetKind === 'all' ? undefined : c.targetKind,
    decisionTag: c.tags.length ? c.tags.join(',') : undefined,
    decisionCurrency: c.currency === DEFAULT_CURRENCY ? undefined : c.currency,
    decisionPeriod: c.period === 'all' ? undefined : c.period,
    decisionOn: c.on || undefined,
    decisionScope: c.scope && c.scope !== DEFAULT_SCOPE ? c.scope : undefined,
  };
}

/** 2つの条件が同じか（URL への書き戻しを、本当に変わったときだけ走らせるため）。 */
export function sameConditions(a: DecisionConditions, b: DecisionConditions): boolean {
  return (
    a.query === b.query &&
    a.targetKind === b.targetKind &&
    a.currency === b.currency &&
    a.period === b.period &&
    a.on === b.on &&
    a.scope === b.scope &&
    a.tags.length === b.tags.length &&
    a.tags.every((t, i) => t === b.tags[i])
  );
}

/**
 * この URL が「1件の意思決定を名指ししている」か。
 *
 * 名指しのときは他の条件を掛けない。これは旧単票（`#/decision/<id>`）を引き継ぐ形で、
 * 「その1件を見せる」以外の意味を持たない URL だからである。掛けたままにすると
 * **置き換え済みの意思決定を指す共有リンクが0件に着く**——改訂チェーンを辿る導線
 * （置き換え／改訂チップ）は置き換え済みの相手を指すので、日常的に踏まれる経路。
 */
export function namesOneDecision(c: DecisionConditions): boolean {
  return parseScopeTarget(c.on || undefined)?.type === 'decision';
}

export interface SelectContext {
  /** 効力は2値（01KYHW54B8ZXH0NEPH2J7N1X39 条項1）。 */
  effectOf: (id: string) => 'in-force' | 'replaced';
  /** decision id → 実効タグ集合（own タグの祖先クロージャ）。 */
  effTagsById: Map<string, Set<string>>;
  /** `/api/governs` の結果。`governing` のときだけ渡る（未取得は undefined）。 */
  governs: GovernsRef[] | undefined;
  /** 期間の基準時刻。 */
  now: number;
  /** フリーワードの関連度（低いほど関連が強い・null＝不一致）。索引が要るので呼び手が持つ。 */
  tierOf: (d: Decision) => number | null;
}

const PERIOD_DAYS: Record<Exclude<PeriodFilter, 'all'>, number> = { '30d': 30, '90d': 90, '1y': 365 };

/**
 * フリーワードとタグ AND を**掛けない**段階の集合。
 *
 * 一覧が更に絞る母数であると同時に、タグの候補（combobox）を出す母数でもある
 * ——フリーワードで候補の母数まで縮めると、どの記録の本文にも出てこないタグ名を
 * 打った瞬間に候補が消える（BrowseView と同じ規則）。
 */
export function selectBase(decisions: Decision[], c: DecisionConditions, ctx: SelectContext): Decision[] {
  const matchesScope = scopeMatcher({
    target: parseScopeTarget(c.on || undefined),
    direction: parseScopeDirection(c.scope || undefined),
    effTagsById: ctx.effTagsById,
    governs: ctx.governs,
  });
  if (namesOneDecision(c)) return decisions.filter(matchesScope);
  return decisions.filter((d) => {
    if (c.targetKind !== 'all' && d.target.type !== c.targetKind) return false;
    const e = ctx.effectOf(d.id);
    if (c.currency === 'superseded' && e !== 'replaced') return false;
    if (c.currency === 'current' && e !== 'in-force') return false;
    if (c.period !== 'all') {
      const ageDays = (ctx.now - new Date(d.at).getTime()) / 86400000;
      if (!(ageDays <= PERIOD_DAYS[c.period])) return false;
    }
    if (!matchesScope(d)) return false;
    return true;
  });
}

/** タグ AND の一致（実効タグ集合が選択タグを全部含む・01KXZK5BWEX3HH15B78E4Z3BXK）。 */
export function matchesTags(d: Decision, tags: string[], effTagsById: Map<string, Set<string>>): boolean {
  if (tags.length === 0) return true;
  const eff = effTagsById.get(d.id);
  return !!eff && tags.every((tg) => eff.has(tg));
}

/** 画面に並べる集合（新しい順・フリーワードがあるときは関連度順）。 */
export function selectDecisions(decisions: Decision[], c: DecisionConditions, ctx: SelectContext): Decision[] {
  const base = selectBase(decisions, c, ctx);
  // 名指しの1件には、フリーワードもタグ AND も掛けない（namesOneDecision と同じ理由）。
  if (namesOneDecision(c)) return base;
  const q = c.query.trim();
  const narrowed = base
    .filter((d) => !q || ctx.tierOf(d) !== null)
    .filter((d) => matchesTags(d, c.tags, ctx.effTagsById))
    .slice()
    .reverse(); // getRules は時系列昇順なので、反転して新しい順
  return q ? narrowed.sort((a, b) => (ctx.tierOf(a) ?? 4) - (ctx.tierOf(b) ?? 4)) : narrowed;
}
