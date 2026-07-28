import { useEffect, useRef, useState } from 'preact/hooks';

// Hash-based routing so Back/Forward work in both `scholia view` (served over
// HTTP) and a `scholia export --html` file opened via file:// or a plain static
// file server. History.pushState is unreliable on file:// in some browsers;
// assigning `location.hash` is not — it always both updates the visible URL
// and pushes a browser history entry, with no server round-trip, which is
// exactly the "static export with working Back/Forward" behavior this needs.

// 'traceability' removed (2026-07-11, user request): not covered by the
// Claude Design mock and dropped from the nav for now — trivially
// restorable from git history (internal/viewer's /api/traceability endpoint
// is untouched; only this frontend surface is gone).
//
// 'compare' (diff-viz / 評価コックピット, G-5) was reinstated 2026-07-12 as a
// purpose-built read-only comparison view (change-cockpit-design-v2.md §2)
// but removed again the same day per change-cockpit-design-v3.md §5 P1:
// evaluation moves inline into each Transition's comment drawer instead of
// living on its own route. `getDiff` (api.ts) and the `/api/diff` backend
// endpoint stay for that inline reuse (P2) — only this standalone view goes.
// 'overview' is the IA-rework landing (viewer-overview-browser): the structure
// tree + component spec sheet. It's the new DEFAULT_ROUTE. 'home' (the previous
// landing) is kept as a valid view so old #/home bookmarks still resolve —
// app.tsx renders OverviewView for both. Every other legacy route
// (#/spec/#/vocab/#/flow/#/decisions/#/tags) is untouched: they're no longer
// top-level nav tabs but stay reachable as internal lenses/detail routes of
// 概要/ブラウザ (per the IA-rework: nothing is deleted, only demoted).
export type ViewName = 'overview' | 'home' | 'browse' | 'vocab' | 'spec' | 'tags' | 'config' | 'flow' | 'decisions' | 'decision';

export interface Route {
  view: ViewName;
  tagId?: string;
  txId?: string;
  /** Vocab entry to scroll to on mount (#/vocab/<id>) — same "focus on one
      record within this view's route" pattern as spec's tagId, added for
      comment-panel "位置へ移動" on vocab comments (2026-07-11 コメント拡張4件). */
  vocabId?: string;
  /** Action id whose given→then flow is shown (#/flow/<action>,
      tx.viewer.action-flow-render). Opened in a separate tab from the spec
      card's action kebab (tx.viewer.action-flow-link) — a standalone route,
      same "focus id in the path" shape as spec/vocab above. */
  actionId?: string;
  /** Decision to show on the permalink detail page (#/decision/<ulid>,
      tx.viewer.decision-detail). Same "focus id in the path" shape as
      spec/vocab/flow above — the ulid is the shareable permalink key. */
  decisionId?: string;
  /** 概要タブの現在地（deep-linking の適用面拡張・01KYGYYMZSS1Y0BFEJ69Q1JC40）:
      いま仕様シートに開いているコンポーネント（#/overview/<componentId>）と、
      そこでアンカーした構成要素（#/overview/<componentId>/part/<partId>）。
      spec/vocab/flow/decision と同じ「focus id を path に置く」形で、componentId
      は位置引数・partId は browse の tag/tx と同じ key/value ペアで続く（構成要素は
      コンポーネントの中でしか意味を持たないので、単独では URL に現れない）。
      省略時は「既定のコンポーネント」を意味する——初期選択を URL に書き戻すと
      入口が汚れるうえ履歴が1件増えるので、既定は載せない（kindFacet の 'all' を
      省くのと同じ扱い）。 */
  componentId?: string;
  partId?: string;
  /** BrowseView's search state (query/kindFacet/filters), carried as a query
      string appended to the hash path (e.g. #/browse/tag/<id>?q=..&f=..) so
      it composes with the existing path-segment routes above instead of
      replacing them (url-state-sync handoff #4/#5). router.ts treats
      searchFilters as an opaque wire string — filters.ts's encodeFilters/
      decodeFilters own its FilterCondition[] codec; router.ts only knows it
      as one more query param. */
  searchQuery?: string;
  searchKindFacet?: string;
  searchFilters?: string;
  /** VocabView's コンポ別モード subject (a tag id), carried as the `s` query
      param so vocab's browse state round-trips like tags/specs do
      (view-state-continuity). Vocab-only — tags/specs BrowseView never sets
      it, so the param is simply absent there. */
  searchSubject?: string;
  /** DecisionsView's filter state (#/decisions・#45 D10b-4): target 種別・
      タグ・現行/失効・期間。free-text search（searchQuery）と同じく URL query
      に載せ、reload/ブラウザバックで復元する（既存 settled セマンティクス）。
      キー名は router.ts の実装詳細（decision には書かない）。既定値（'all'）の
      ときは省略して URL を汚さない（q/k と同じ扱い）。 */
  decisionTargetKind?: string;
  decisionTag?: string;
  decisionCurrency?: string;
  decisionPeriod?: string;
  /** 意思決定の一覧の「どの対象か」「どの向きか」（01KYKS4Y56FAHRVCWKMQJK4RT6）。
      `on` は `tag:<id>` / `transition:<id>` / `vocab:<id>` / `decision:<ulid>` の
      いずれか1件、`scope` は own / governing / subtree。上の5条件（dk/dt/dc/dp と
      共有の q）とは AND で合成される別の条件で、置き換えではない。
      `decision:<ulid>` の形が単票の代わり——1件に絞り込んだ一覧が permalink に
      なる。既定（subtree）は省いて URL を汚さない（dk/dp と同じ扱い）。
      キー名そのものは実装詳細（decision には書かない・01KXYED63EKN9DXTZ3XT498Q3M
      と同じ規律）。 */
  decisionOn?: string;
  decisionScope?: string;
  /** #/flow 一覧の絞り込み状態（viewer-search-consistency・flow-browse／
      deep-linking amend）。フリーワードは共有 searchQuery（q）、kind facet は
      共有 searchKindFacet（k）に相乗りし、タグ AND だけ専用キー ft に comma
      区切りで載せる。reload/バック/タブ切替で復元。空は省略して URL を汚さない。
      キー名は実装詳細（decision には書かない）。actionId present（フロー図）の
      ときは付かない。 */
  flowTags?: string;
}

const VIEWS: ViewName[] = ['overview', 'home', 'browse', 'vocab', 'spec', 'tags', 'config', 'flow', 'decisions', 'decision'];
// OVERVIEW is the IA-rework landing (viewer-overview-browser): default route
// moves from 'home' to 'overview'. An empty/unknown hash still falls back to
// DEFAULT_ROUTE below, so bookmarks of `#` or the bare page URL land on 概要
// now — every other existing route (#/browse/..., #/spec/..., #/home) is
// unaffected since parseRoute only consults DEFAULT_ROUTE when the hash's view
// segment is absent or unknown.
const DEFAULT_ROUTE: Route = { view: 'overview' };

function isViewName(s: string): s is ViewName {
  return (VIEWS as string[]).includes(s);
}

export function parseRoute(hash: string): Route {
  const withoutPrefix = hash.replace(/^#\/?/, '');
  // Search-state query string (?q=..&k=..&f=..) is a suffix of the whole
  // hash, after the path segments parsed below — split it off first so it
  // never gets swept into the '/'-separated path parsing.
  const qsIdx = withoutPrefix.indexOf('?');
  const raw = qsIdx === -1 ? withoutPrefix : withoutPrefix.slice(0, qsIdx);
  const queryString = qsIdx === -1 ? '' : withoutPrefix.slice(qsIdx + 1);
  if (!raw) return DEFAULT_ROUTE;
  const parts = raw.split('/').filter((p) => p.length > 0).map(decodeURIComponent);
  const view = parts[0];
  if (!isViewName(view)) return DEFAULT_ROUTE;

  const route: Route = { view };
  switch (view) {
    case 'overview':
      // #/overview/<componentId>[/part/<partId>]。componentId は位置引数、構成要素
      // アンカーは browse の tag/tx と同じ key/value ペアで続ける。'home'（旧ランディング）
      // は同じ OverviewView を描くが、現在地を持たない素の入口として据え置く
      // ——既存の #/home ブックマークの意味を変えないため。
      if (parts[1]) route.componentId = parts[1];
      for (let i = 2; i < parts.length - 1; i += 2) {
        if (parts[i] === 'part') route.partId = parts[i + 1];
      }
      break;
    case 'browse':
      for (let i = 1; i < parts.length - 1; i += 2) {
        if (parts[i] === 'tag') route.tagId = parts[i + 1];
        else if (parts[i] === 'tx') route.txId = parts[i + 1];
      }
      break;
    case 'spec':
      if (parts[1]) route.tagId = parts[1];
      break;
    case 'vocab':
      if (parts[1]) route.vocabId = parts[1];
      break;
    case 'flow':
      if (parts[1]) route.actionId = parts[1];
      break;
    case 'decision':
      // #/decision/<ulid> permalink — same single-path-segment focus shape
      // as spec/vocab/flow above; the ulid rides in the path so the URL is
      // directly shareable (tx.viewer.decision-detail).
      if (parts[1]) route.decisionId = parts[1];
      break;
  }
  if (queryString) {
    // URLSearchParams decodes each value on .get() — plain text (q/k) needs
    // no extra decode step; searchFilters is handed to filters.ts's
    // decodeFilters as-is, which owns its own inner ':'/',' unescaping.
    const params = new URLSearchParams(queryString);
    const q = params.get('q');
    const k = params.get('k');
    const s = params.get('s');
    if (q) route.searchQuery = q;
    if (k) route.searchKindFacet = k;
    // `s` (vocab subject) is a plain tag id — truthy-check like q/k (an
    // absent/empty subject means グローバル mode, which carries no param).
    if (s) route.searchSubject = s;
    // `f` uses has()/empty-string, not truthy-check like q/k above: an
    // explicit `f=` (user cleared every filter chip) must round-trip as ''
    // and stay distinct from "no `f` param at all" (BrowseView's
    // filter-on-focus-tag default applies only in the latter case) — see
    // BrowseView.tsx's deriveFilters.
    if (params.has('f')) route.searchFilters = params.get('f') || '';
    // DecisionsView filters (#45 D10b-4) — plain truthy-check like q/k: an
    // absent param means "default" (the view resolves 'all'/'' itself), so
    // the default value is simply omitted from the URL.
    const dk = params.get('dk');
    const dt = params.get('dt');
    const dc = params.get('dc');
    const dp = params.get('dp');
    if (dk) route.decisionTargetKind = dk;
    if (dt) route.decisionTag = dt;
    if (dc) route.decisionCurrency = dc;
    if (dp) route.decisionPeriod = dp;
    // 対象と向き（01KYKS4Y56FAHRVCWKMQJK4RT6）。値の解釈（`tag:` 等の綴り・
    // 既定の向き）は decisionScope.ts が1箇所で持つので、router は不透明な
    // 文字列として運ぶだけ（searchFilters を filters.ts に委ねているのと同じ形）。
    const on = params.get('on');
    const scope = params.get('scope');
    if (on) route.decisionOn = on;
    if (scope) route.decisionScope = scope;
    // #/flow filter tags (viewer-search-consistency) — comma-joined tag id
    // list; plain truthy-check like q/k (absent = no tag filter).
    const ft = params.get('ft');
    if (ft) route.flowTags = ft;
  }
  return route;
}

export function routeHash(route: Route): string {
  const seg: string[] = [route.view];
  switch (route.view) {
    case 'overview':
      // partId は componentId の下でしか意味を持たない（どのコンポーネントの構成要素か
      // が決まらないと解決できない）ので、componentId が無ければ何も積まない＝
      // 「既定のコンポーネント」を指す素の #/overview に落ちる。
      if (route.componentId) {
        seg.push(encodeURIComponent(route.componentId));
        if (route.partId) seg.push('part', encodeURIComponent(route.partId));
      }
      break;
    case 'browse':
      if (route.tagId) seg.push('tag', encodeURIComponent(route.tagId));
      if (route.txId) seg.push('tx', encodeURIComponent(route.txId));
      break;
    case 'spec':
      if (route.tagId) seg.push(encodeURIComponent(route.tagId));
      break;
    case 'vocab':
      if (route.vocabId) seg.push(encodeURIComponent(route.vocabId));
      break;
    case 'flow':
      if (route.actionId) seg.push(encodeURIComponent(route.actionId));
      break;
    case 'decision':
      if (route.decisionId) seg.push(encodeURIComponent(route.decisionId));
      break;
  }
  let hash = `#/${seg.join('/')}`;
  // 'all' is kindFacet's default (BrowseView) — omitting it here is what
  // keeps a facet-less search state from dirtying the URL (handoff #6).
  const params = new URLSearchParams();
  if (route.searchQuery) params.set('q', route.searchQuery);
  if (route.searchKindFacet && route.searchKindFacet !== 'all') params.set('k', route.searchKindFacet);
  // '' (グローバル mode) is subject's default — omitting it keeps a mode-less
  // vocab state from dirtying the URL, same treatment as 'all' kindFacet.
  if (route.searchSubject) params.set('s', route.searchSubject);
  // Explicit '' must still emit `f=` (see parseRoute) — only a fully-absent
  // searchFilters (undefined) omits the param.
  if (route.searchFilters !== undefined) params.set('f', route.searchFilters);
  // DecisionsView filters (#45 D10b-4) — omit default values so the URL stays
  // clean until a filter is actually applied (parseRoute treats absent as
  // default). App passes these already-normalized (undefined when default).
  if (route.decisionTargetKind) params.set('dk', route.decisionTargetKind);
  if (route.decisionTag) params.set('dt', route.decisionTag);
  if (route.decisionCurrency) params.set('dc', route.decisionCurrency);
  if (route.decisionPeriod) params.set('dp', route.decisionPeriod);
  // 対象と向き。既定の向き（subtree）は App が undefined に正規化して渡すので、
  // ここは「値があれば出す」だけ（dk/dp と同じ扱い）。
  if (route.decisionOn) params.set('on', route.decisionOn);
  if (route.decisionScope) params.set('scope', route.decisionScope);
  // #/flow filter tags (viewer-search-consistency) — omit when empty.
  if (route.flowTags) params.set('ft', route.flowTags);
  // `:` はクエリ部分でエスケープが要らない文字（RFC 3986 の pchar）なのに、
  // URLSearchParams は %3A に畳んでしまう。絞り込み条件は「対象:id」の形を持つので
  // （`on=tag:req.x` / `f=tag:req.x`）、そのままだと URL が `on=tag%3Areq.x` になる
  // ——利用者が読んで組み立てる前提の条件なので、読める形に戻す。
  //
  // 値の中に含まれる本物の `:` は encodeURIComponent 済みで `%253A` になっており
  // （`%`+`2`+`5`+`3`+`A`）、この置換の対象文字列 `%3A` を含まないので巻き込まない。
  const qs = params.toString().replace(/%3A/g, ':');
  if (qs) hash += `?${qs}`;
  return hash;
}

function currentRoute(): Route {
  return parseRoute(window.location.hash);
}

export function useHashRoute(): [Route, (route: Route) => void] {
  const [route, setRoute] = useState<Route>(currentRoute);
  // いま取り込み済みの hash（正規化した綴り）。**listener 側も内容で短絡する**ための
  // 目印で、下の navigate が持っていた短絡と対になる。
  //
  // ⚠️ **この非対称が実害を出した。** navigate は「同じ hash なら何もしない」と
  // 比較していたのに、listener は届いた event を無条件に state へ流していた。
  // hashchange は**中身が同じでも届きうる**——URL を代入してから listener が張られる
  // までの分や、1つの処理の中で2回代入したときの2発目は、listener が読む時点では
  // 既に現在の hash と同じである。その1発ごとに setRoute が**内容の同じ新しい
  // Route オブジェクト**を作り、下流（意思決定の一覧の「外から来た条件の取り込み」）が
  // 「URL が変わった」と受け取って、**利用者がその瞬間に操作していた値を URL 側の
  // 古い値で上書き**していた。実測では 60回に2回この順序が噛み合って再現した。
  //
  // 取り込むものが無い event では state を差し替えない。genuine な遷移（綴りが
  // 変わる）は従来どおり全部通る。
  const appliedHash = useRef<string>(routeHash(route));

  useEffect(() => {
    const onHashChange = () => {
      const next = currentRoute();
      const hash = routeHash(next);
      if (hash === appliedHash.current) return;
      appliedHash.current = hash;
      setRoute(next);
    };
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  const navigate = (next: Route) => {
    const hash = routeHash(next);
    // No-op when nothing observable changes: `hash` is a full serialization
    // of `next` (routeHash/parseRoute round-trip), so an unchanged hash
    // means unchanged route content. Skipping setRoute here — rather than
    // calling it with a same-content-but-new-reference `next` — keeps
    // `route.searchFilters` (and any other array/object field) reference-
    // stable across renders that don't actually navigate; BrowseView's URL
    // sync effect (search state → hash) depends on that stability to avoid
    // re-triggering itself every time an unrelated re-render hands it a
    // fresh-but-equal object.
    if (window.location.hash === hash) return;
    // Triggers the 'hashchange' listener above, which updates `route`; a
    // new browser history entry is pushed as a side effect of the
    // assignment itself (see module comment).
    window.location.hash = hash;
  };

  return [route, navigate];
}
