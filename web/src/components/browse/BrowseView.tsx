import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { api, isStaticMode } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { useDrawer } from '../../drawer';
import { usePendingDiff } from '../../pendingDiff';
import type { Config, FacetsResponse, FacetTreeNode, SpecReport, Tag, TraceabilityResponse, Transition, TransitionDetail } from '../../types';
import { BrowseRail } from './BrowseRail';
import type { ConditionChip, IndexItem, KindOption, SuggestionItem } from './BrowseRail';
import { Resizer } from '../layout/Resizer';
import { RAIL_WIDTH } from '../layout/resizableWidths';
import { TagCard } from './TagCard';
import { SpecCard } from './SpecCard';
import { TombstoneCard } from './TombstoneCard';
import { NewTransitionForm } from './NewTransitionForm';
import { parentsOf, childrenOf, tagMatchesFilters, specMatchesFilters, encodeFilters, decodeFilters } from './filters';
import type { FilterCondition } from './filters';
import { buildFolderIndex, loadCollapsed, saveCollapsed } from './indexTree';
import { kindColor, OWNER_COLOR } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { Icon } from '../shared/Icon';

export interface SearchStateChange {
  query: string;
  kindFacet: string;
  /** undefined = drop the `f` param entirely (filters equal the URL's
      no-explicit-filter default — keeps the URL clean, A-2); a string
      (incl. '') = write `f=<v>` (an explicit '' records "user cleared every
      filter", which must override the focus-tag default on reload — 条件2). */
  filtersEncoded: string | undefined;
}

interface Props {
  facet: 'tags' | 'specs';
  initialFocusTagId?: string;
  initialFocusTxId?: string;
  onGoToSpec: (txId: string) => void;
  /** Current search state as reflected in the URL (router.ts's Route.search*
      fields) — '' / 'all' / '' when the URL carries none. Read once per
      mount/reset and on external (Back/Forward) change; see the two sync
      effects below for how this composes with the legacy
      filter-on-focus-tag default. */
  searchQuery: string;
  searchKindFacet: string;
  /** undefined = no `f` param at all (legacy focus-tag default applies); ''
      = param present but empty (user explicitly cleared every filter). */
  searchFiltersEncoded: string | undefined;
  /** Fired (debounced) whenever local query/kindFacet/filters state changes,
      so the caller can push it into the URL (deep-linking, url-state-sync
      handoff). */
  onSearchChange: (state: SearchStateChange) => void;
}

// Flattens the unified parentIds forest (§3.8) into DFS order with a depth per
// tag. kind is now an attribute, not a tree axis: the whole forest is one tree
// and `kindFacet` filters it — only tags whose kind matches are emitted, and
// depth counts just the *emitted* ancestors on the path, so filtering to a
// kind re-roots its tags cleanly instead of leaving blank indentation from
// skipped cross-kind ancestors. Each tag is emitted once (first-encountered
// path) — a multi-parent tag would otherwise repeat. The single forest carries
// cross-kind nesting and kind=null tags that the old per-kind trees dropped.
function buildTagOrder(roots: FacetTreeNode[], allTags: Tag[], kindFacet: string): Array<{ id: string; depth: number }> {
  const order: Array<{ id: string; depth: number }> = [];
  const seen = new Set<string>();
  const matchKind = (kind: string | undefined) => kindFacet === 'all' || kind === kindFacet;
  const walk = (nodes: FacetTreeNode[], depth: number) => {
    for (const n of nodes) {
      const matches = matchKind(n.tag.kind);
      if (matches && !seen.has(n.tag.id)) {
        order.push({ id: n.tag.id, depth });
        seen.add(n.tag.id);
      }
      if (n.children) walk(n.children, matches ? depth + 1 : depth);
    }
  };
  walk(roots, 0);
  // Safety net: any tag not reached in the forest (shouldn't happen — the
  // unified forest nests every tag by parentIds) still shows, flat.
  for (const t of allTags) {
    if (seen.has(t.id)) continue;
    if (!matchKind(t.kind)) continue;
    order.push({ id: t.id, depth: 0 });
    seen.add(t.id);
  }
  return order;
}

interface IndexTreeNode {
  id: string;
  children: IndexTreeNode[];
}

// Rebuilds the visible tag outline (the flat, already query/condition-filtered
// (id, depth) sequence) into a tree so the 見出し index can collapse subtrees
// (依頼1). A stack tracks the current ancestor chain by depth; a node attaches
// to the nearest shallower ancestor still present — filtering can drop an
// intermediate parent, and the child then rides up to its nearest visible
// ancestor rather than vanishing.
function buildIndexTree(visible: Array<{ id: string; depth: number }>): IndexTreeNode[] {
  const roots: IndexTreeNode[] = [];
  const stack: Array<{ depth: number; node: IndexTreeNode }> = [];
  for (const { id, depth } of visible) {
    const node: IndexTreeNode = { id, children: [] };
    while (stack.length && stack[stack.length - 1].depth >= depth) stack.pop();
    if (stack.length) stack[stack.length - 1].node.children.push(node);
    else roots.push(node);
    stack.push({ depth, node });
  }
  return roots;
}

/** Filters for a (re)seed: URL's `f=` wins whenever the param is present at
    all — including an explicit empty string (the user cleared every filter
    chip, which must stick across reload) — otherwise falls back to the
    legacy "focus tag narrows to itself" default (#/spec/<id> with no `f`
    param still shows just that tag's subtree). */
function deriveFilters(searchFiltersEncoded: string | undefined, initialFocusTagId?: string): FilterCondition[] {
  if (searchFiltersEncoded !== undefined) return decodeFilters(searchFiltersEncoded);
  return initialFocusTagId ? [{ type: 'tag', id: initialFocusTagId }] : [];
}

export function BrowseView({
  facet,
  initialFocusTagId,
  initialFocusTxId,
  onGoToSpec,
  searchQuery,
  searchKindFacet,
  searchFiltersEncoded,
  onSearchChange,
}: Props) {
  const t = useT();
  const { tagById: lookupTagById, vocabById, tagKindLabel } = useLookups();
  const { closeDrawer } = useDrawer();
  const pendingDiff = usePendingDiff();
  // §8.8 P5「追加」の入口（subject の仕様一覧先頭の「＋ 新規 Transition を
  // 提案」トグル）— server-mode 限定（static export は書込不可・handoff
  // 「static では非活性/非表示」）。
  const [creatingNew, setCreatingNew] = useState(false);

  const [config, setConfig] = useState<Config | null>(null);
  const [facetsData, setFacetsData] = useState<FacetsResponse | null>(null);
  const [tags, setTags] = useState<Tag[] | null>(null);
  const [traceability, setTraceability] = useState<TraceabilityResponse | null>(null);
  const [specReports, setSpecReports] = useState<Record<string, SpecReport>>({});
  const [txList, setTxList] = useState<Transition[] | null>(null);
  const [txDetails, setTxDetails] = useState<Record<string, TransitionDetail>>({});
  const [error, setError] = useState<string | null>(null);
  // Settled-count flags + per-item failure counts (review MINOR-1): readiness
  // must not depend on "every fetch succeeded" — a single transient getSpec/
  // getTransition failure must not hang the screen on "loading…" forever.
  // Cards for failed items just don't render (report/detail stays
  // undefined); a soft banner below surfaces that something didn't load.
  const [tagsSettled, setTagsSettled] = useState(false);
  const [specsSettled, setSpecsSettled] = useState(false);
  const [tagsFailedCount, setTagsFailedCount] = useState(0);
  const [specsFailedCount, setSpecsFailedCount] = useState(0);

  const [query, setQuery] = useState(() => searchQuery || '');
  const [kindFacet, setKindFacet] = useState(() => searchKindFacet || 'all');
  const [filters, setFilters] = useState<FilterCondition[]>(() => deriveFilters(searchFiltersEncoded, initialFocusTagId));
  const [openTx, setOpenTx] = useState<Record<string, boolean>>(() => (initialFocusTxId ? { [initialFocusTxId]: true } : {}));
  // 見出しの折りたたみ状態（依頼1）— facet ごとの localStorage キーで復元。
  const [collapsedIds, setCollapsedIds] = useState<Set<string>>(() => loadCollapsed(facet));

  const cardRefs = useRef<Map<string, HTMLElement>>(new Map());
  const scrollTarget = useRef<string | null>(initialFocusTagId || initialFocusTxId || null);

  // Content-aware setFilters: bails out (returns the *same* array reference)
  // when the derived value is equivalent to what's already there. Both
  // effects below run on every mount, and a plain `setFilters(deriveFilters(
  // ...))` would hand back a freshly-allocated-but-equal array each time —
  // React can't tell that's a no-op via Object.is, so it re-renders. The push
  // effect's own echo guard already absorbs such no-op renders, but keeping
  // the reference stable avoids the wasted render in the first place.
  // Comparing the encodeFilters() wire form (already the canonical string) is
  // cheap and exact.
  const setFiltersIfChanged = (next: FilterCondition[]) =>
    setFilters((prev) => (encodeFilters(prev) === encodeFilters(next) ? prev : next));

  // Per-facet reset (design's `filters: { tags: [], specs: [] }` — each
  // facet keeps its own independent filter/search/open state). This also
  // fires when initialFocus* changes while the same facet instance is reused
  // (a same-facet re-focus: comment-panel jump to another record, or a URL
  // edit that only swaps txId/tagId — app.tsx keeps the BrowseView mounted).
  useEffect(() => {
    // Re-derive from the URL rather than hardcoding blank defaults — this
    // effect also runs on first mount (alongside the lazy useState
    // initializers above), and a reload of e.g. `#/browse?q=foo` must not
    // have this reset immediately clobber the just-restored query back to
    // '' (handoff #2).
    setQuery(searchQuery || '');
    setKindFacet(searchKindFacet || 'all');
    setFiltersIfChanged(deriveFilters(searchFiltersEncoded, initialFocusTagId));
    setOpenTx(initialFocusTxId ? { [initialFocusTxId]: true } : {});
    setCollapsedIds(loadCollapsed(facet));
    scrollTarget.current = initialFocusTagId || initialFocusTxId || null;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [facet, initialFocusTagId, initialFocusTxId]);

  // Loading (settled/failed) reset is keyed on `facet` ALONE, not on focus.
  // The fetched data (full tags list / full transition list) doesn't depend
  // on which record is focused, so a same-facet re-focus must NOT drop back
  // to `settled=false`: the settle-side fetch effects below re-run only on
  // `facet`/`tags`/`pendingDiff.version` changes, so a focus-only reset would
  // clear settled with nothing to set it true again — the infinite
  // `loading…` this fix targets. Facet changes still reset here (and the
  // fetch effect re-runs on `facet`, flipping settled back to true).
  useEffect(() => {
    setTagsSettled(false);
    setSpecsSettled(false);
    setTagsFailedCount(0);
    setSpecsFailedCount(0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [facet]);

  // Adopts search state pushed in from *outside* this component's own
  // typing/filter-clicking — i.e. Back/Forward (hashchange → new route →
  // new searchQuery/searchKindFacet/searchFiltersEncoded props) while the
  // facet/focus stay the same (the effect above only fires when those
  // change, so a pure search-state history step needs this separate one).
  // Deliberately does NOT touch openTx/scrollTarget/settled flags — going
  // back to an earlier search shouldn't refetch data or re-scroll.
  useEffect(() => {
    setQuery(searchQuery || '');
    setKindFacet(searchKindFacet || 'all');
    setFiltersIfChanged(deriveFilters(searchFiltersEncoded, initialFocusTagId));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchQuery, searchKindFacet, searchFiltersEncoded]);

  // Pushes local query/kindFacet/filters changes back out to the URL, but
  // ONLY when local state genuinely diverges from what the URL already
  // encodes (echo guard). The old skip-a-flag scheme miscounted: after our
  // own push→hashchange→adopt round-trip the adopt effect re-armed the flag,
  // but the push effect never re-ran (local state already equalled the pushed
  // value), so a stale flag lingered and swallowed the *next* real edit
  // (settle → single op lost — review A-1). Comparing the URL-equivalent
  // serialization here means the return leg of a round-trip (state == URL)
  // and the mount seed both no-op naturally, with no flag to leave dangling.
  //
  // Deps are the LOCAL state only, NOT the URL props (which are read directly
  // in the body). That's deliberate: an external navigation (Back/Forward, or
  // a programmatic navigate to a different search) changes the URL props in
  // render N while local state is still the old value — if this effect fired
  // on that render it would (old-local != new-url) schedule a spurious push of
  // the *stale* local state, fighting the navigation. Firing only on local
  // change means it re-runs in render N+1 (after the adopt effect has mirrored
  // the URL into local state), where the closure captures the already-updated
  // props and the echo guard cleanly no-ops.
  useEffect(() => {
    // What the URL currently means for filters, re-derived the same way the
    // adopt/reset effects seed local state (so undefined `f` + focus-tag
    // default compares equal to its own seeded filters — mount doesn't push).
    const urlFiltersEncoded = encodeFilters(deriveFilters(searchFiltersEncoded, initialFocusTagId));
    const localFiltersEncoded = encodeFilters(filters);
    // Echo / seed: local already matches the URL — nothing to push.
    if (query === searchQuery && kindFacet === searchKindFacet && localFiltersEncoded === urlFiltersEncoded) {
      return;
    }
    // A-2: drop `f=` when filters equal the no-explicit-filter default (clean
    // URL); keep an explicit '' only when it diverges from that default (i.e.
    // the user cleared filters a focus route would otherwise re-seed — 条件2).
    const defaultFiltersEncoded = encodeFilters(deriveFilters(undefined, initialFocusTagId));
    const nextFiltersEncoded = localFiltersEncoded === defaultFiltersEncoded ? undefined : localFiltersEncoded;
    const id = setTimeout(() => {
      onSearchChange({ query, kindFacet, filtersEncoded: nextFiltersEncoded });
    }, 350);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, kindFacet, filters]);

  useEffect(() => {
    Promise.all([api.getConfig(), api.getFacets(), api.getTags(), api.getTraceability()])
      .then(([cfg, f, t, trace]) => {
        setConfig(cfg);
        setFacetsData(f);
        setTags(t);
        setTraceability(trace);
      })
      .catch((err) => setError(String(err)));
  }, []);

  useEffect(() => {
    if (facet !== 'tags' || !tags) return;
    let cancelled = false;
    Promise.all(
      tags.map((t) =>
        api
          .getSpec(t.id)
          .then((r) => [t.id, r] as const)
          .catch(() => [t.id, undefined] as const),
      ),
    ).then((pairs) => {
      if (cancelled) return;
      const next: Record<string, SpecReport> = {};
      let failed = 0;
      for (const [id, r] of pairs) {
        if (r) next[id] = r;
        else failed++;
      }
      setSpecReports(next);
      setTagsFailedCount(failed);
      setTagsSettled(true);
    });
    return () => {
      cancelled = true;
    };
  }, [facet, tags]);

  useEffect(() => {
    if (facet !== 'specs') return;
    let cancelled = false;
    api
      .getTransitions({})
      .then((res) => {
        if (cancelled) return undefined;
        const list = res.transitions || [];
        setTxList(list);
        return Promise.all(list.map((t) => api.getTransition(t.id).catch(() => undefined)));
      })
      .then((details) => {
        if (cancelled || !details) return;
        const next: Record<string, TransitionDetail> = {};
        let failed = 0;
        for (const d of details) {
          if (d) next[d.id] = d;
          else failed++;
        }
        setTxDetails(next);
        setSpecsFailedCount(failed);
        setSpecsSettled(true);
      })
      .catch((err) => setError(String(err)));
    return () => {
      cancelled = true;
    };
    // pendingDiff.version bumps on every refresh() (§8.8 P5) — a
    // create/delete elsewhere (comment drawer's delete toggle, this view's
    // own "+ 新規 Transition" form) refreshes the diff, and this refetches
    // the live transition list/details so the affected card actually
    // appears/disappears without a full page reload.
  }, [facet, pendingDiff.version]);

  const tagsReady = facet === 'tags' && tagsSettled;
  const specsReady = facet === 'specs' && specsSettled;
  const failedCount = facet === 'tags' ? tagsFailedCount : specsFailedCount;

  useEffect(() => {
    const id = scrollTarget.current;
    if (!id || (!tagsReady && !specsReady)) return;
    const el = cardRefs.current.get(id);
    if (el) {
      el.scrollIntoView({ block: 'start' });
      scrollTarget.current = null;
    }
  });

  // Design closes the narrow-viewport drawer on every selection that
  // narrows/changes what's on screen — addFilter (self-filter, combobox
  // suggestion, vocab/tag chip) and scrolling to a different card (index
  // item, parent/child tag link) — but not on kindFacet select, removeFilter,
  // or query typing (those you'd do *from* the drawer, so closing it would
  // be self-defeating). No-op on wide viewports (drawer.tsx's closeDrawer).
  const addFilter = (f: FilterCondition) => {
    setFilters((prev) => (prev.some((p) => p.type === f.type && p.id === f.id) ? prev : [...prev, f]));
    closeDrawer();
  };
  const removeFilter = (i: number) => setFilters((prev) => prev.filter((_, idx) => idx !== i));
  const scrollToCard = (id: string) => {
    scrollTarget.current = id;
    cardRefs.current.get(id)?.scrollIntoView({ block: 'start' });
    closeDrawer();
  };
  // Fold/unfold a subtree in the 見出し index and persist it (依頼1). Doesn't
  // close the drawer — you fold *from* the rail, so closing would be
  // self-defeating (same reasoning as removeFilter/kindFacet).
  const toggleCollapse = (id: string) => {
    setCollapsedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      saveCollapsed(facet, next);
      return next;
    });
  };

  const tagById = useMemo(() => new Map((tags || []).map((t) => [t.id, t])), [tags]);
  const gapByTagId = useMemo(() => new Map((traceability?.entries || []).map((e) => [e.tag.id, e])), [traceability]);

  if (error) return <div class="browse-view error">{error}</div>;
  if (!config || !facetsData || !tags || !traceability) return <div class="browse-view dim">{t.browse.loading}</div>;

  const q = query.trim().toLowerCase();

  let title = '';
  let subtitle = '';
  let indexItems: IndexItem[] = [];
  let kindOptions: KindOption[] = [];
  let body: preact.JSX.Element;

  if (facet === 'tags') {
    kindOptions = config.tagKinds.map((k) => ({ key: k, label: tagKindLabel(k), count: tags.filter((t) => t.kind === k).length }));
    const order = buildTagOrder(facetsData.roots, tags, kindFacet);
    const visible = order.filter(({ id }) => {
      const t = tagById.get(id);
      if (!t) return false;
      if (q && !(t.id + ' ' + (t.name || '') + ' ' + (t.description || '')).toLowerCase().includes(q)) return false;
      return tagMatchesFilters(t, filters, facetsData.roots);
    });

    title = t.browse.tagsTitle;
    subtitle = t.browse.tagsSubtitle;
    // 見出しは統一ツリーを畳める階層で描く（依頼1）: visible を木に組み直し、
    // 畳まれた親の子孫はここで間引く（カード本体は visible 全件のまま）。
    const flatIndex: IndexItem[] = [];
    const emitIndex = (nodes: IndexTreeNode[], depth: number) => {
      for (const n of nodes) {
        const tg = tagById.get(n.id)!;
        const entry = gapByTagId.get(n.id);
        const hasChildren = n.children.length > 0;
        const collapsed = collapsedIds.has(n.id);
        flatIndex.push({
          id: n.id,
          label: tg.name || tg.id,
          color: kindColor(tg.kind),
          indent: depth,
          isGap: entry?.gap,
          hasChildren,
          collapsed,
          onToggle: hasChildren ? () => toggleCollapse(n.id) : undefined,
          onClick: () => scrollToCard(n.id),
        });
        if (hasChildren && !collapsed) emitIndex(n.children, depth + 1);
      }
    };
    emitIndex(buildIndexTree(visible), 0);
    indexItems = flatIndex;

    body = !tagsReady ? (
      <div class="dim">{t.browse.loading}</div>
    ) : visible.length === 0 ? (
      <div class="card-empty">{t.browse.empty}</div>
    ) : (
      <>
        {visible.map(({ id }) => {
          const t = tagById.get(id)!;
          const entry = gapByTagId.get(id);
          return (
            <TagCard
              key={id}
              tag={t}
              report={specReports[id]}
              isGap={entry?.gap}
              parents={parentsOf(facetsData.roots, id, tagById)}
              children={childrenOf(facetsData.roots, id, tagById)}
              cardRef={(el) => {
                if (el) cardRefs.current.set(id, el);
                else cardRefs.current.delete(id);
              }}
              onFilterSelf={() => addFilter({ type: 'tag', id })}
              onSelectParent={scrollToCard}
              onSelectChild={scrollToCard}
              onSelectSpec={onGoToSpec}
            />
          );
        })}
      </>
    );
  } else {
    const allKinds = Array.from(new Set(tags.map((t) => t.kind).filter((k): k is string => !!k)));
    kindOptions = allKinds.map((k) => ({
      key: k,
      label: tagKindLabel(k),
      count: Object.values(txDetails).filter((d) => (d.effectiveTags || []).some((et) => tagById.get(et.id)?.kind === k)).length,
    }));

    const list = txList || [];
    const visible = list.filter((tx) => {
      const detail = txDetails[tx.id];
      if (!detail) return false;
      if (kindFacet !== 'all' && !(detail.effectiveTags || []).some((et) => tagById.get(et.id)?.kind === kindFacet)) return false;
      if (q) {
        const hay = (
          tx.id +
          ' ' +
          (detail.actionLabel || '') +
          ' ' +
          (detail.givenLabels || []).join(' ') +
          ' ' +
          (detail.thenLabels || []).join(' ')
        ).toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return specMatchesFilters(detail, filters, vocabById);
    });

    // §8.8 P5「削除」の tombstone（§3 の 3種別表）: removed from the working
    // tree, so there's no TransitionDetail to run specMatchesFilters/
    // kindFacet against — only the free-text query narrows this list (id/
    // action match), same as the live list's own text-search clause above.
    // Tag/vocab/owner AND-filter chips and the kind facet don't apply here
    // (bounded scope: a deleted record's provenance data is gone with it).
    const visibleTombstones = pendingDiff.removedTransitions.filter((tx) => {
      if (!q) return true;
      const hay = (tx.id + ' ' + vocabById.get(tx.action)?.label || tx.action).toLowerCase();
      return hay.includes(q);
    });

    title = t.browse.specsTitle;
    subtitle = t.browse.specsSubtitle;
    // 索引をタグ階層フォルダに（依頼C-1）: 各 spec を自分の own tags
    // （Transition.tags — 確定基準）のフォルダすべてに重複して出し、タグ無しは
    // 末尾の未分類フォルダへ。折りたたみは tags facet と同じ per-facet localStorage。
    indexItems = buildFolderIndex({
      roots: facetsData.roots,
      leaves: visible.map((tx) => ({
        id: tx.id,
        label: txDetails[tx.id]?.actionLabel || tx.id,
        color: 'var(--t-act)',
        tags: txDetails[tx.id]?.tags || [],
      })),
      untaggedLabel: t.browse.uncategorized,
      folderColor: (tag) => kindColor(tag.kind),
      collapsedIds,
      onToggle: toggleCollapse,
      onSelect: scrollToCard,
    });

    const newTransitionEntry = !isStaticMode && (
      <>
        {!creatingNew && (
          <button type="button" class="new-transition-trigger" onClick={() => setCreatingNew(true)}>
            <Icon name="plus" size={13} /> {t.comments.newTransitionButton}
          </button>
        )}
        {creatingNew && (
          <NewTransitionForm
            onClose={() => setCreatingNew(false)}
            onCreated={(id) => {
              setCreatingNew(false);
              scrollTarget.current = id;
              setOpenTx((prev) => ({ ...prev, [id]: true }));
            }}
          />
        )}
      </>
    );

    body = !specsReady ? (
      <div class="dim">{t.browse.loading}</div>
    ) : visible.length === 0 && visibleTombstones.length === 0 ? (
      <>
        {newTransitionEntry}
        <div class="card-empty">{t.browse.empty}</div>
      </>
    ) : (
      <>
        {newTransitionEntry}
        {visible.map((tx) => (
          <SpecCard
            key={tx.id}
            detail={txDetails[tx.id]}
            isOpen={!!openTx[tx.id]}
            cardRef={(el) => {
              if (el) cardRefs.current.set(tx.id, el);
              else cardRefs.current.delete(tx.id);
            }}
            onToggleOpen={() => setOpenTx((prev) => ({ ...prev, [tx.id]: !prev[tx.id] }))}
            onFilterVocab={(id) => addFilter({ type: 'vocab', id })}
            onFilterTag={(id) => addFilter({ type: 'tag', id })}
            onFilterOwner={(owner) => addFilter({ type: 'owner', id: owner })}
          />
        ))}
        {visibleTombstones.map((tx) => (
          <TombstoneCard
            key={tx.id}
            transition={tx}
            cardRef={(el) => {
              if (el) cardRefs.current.set(tx.id, el);
              else cardRefs.current.delete(tx.id);
            }}
          />
        ))}
      </>
    );
  }

  const conditions: ConditionChip[] = filters.map((f, i) => {
    if (f.type === 'tag') {
      const tag = tagById.get(f.id) || lookupTagById.get(f.id);
      return { label: tag?.name || f.id, color: kindColor(tag?.kind), onRemove: () => removeFilter(i) };
    }
    if (f.type === 'owner') {
      return { label: f.id, color: OWNER_COLOR, onRemove: () => removeFilter(i) };
    }
    const v = vocabById.get(f.id);
    return { label: v?.label || f.id, color: kindColor(v?.category), onRemove: () => removeFilter(i) };
  });

  // Combobox candidates (2026-07-11 tweaks3 §3, narrowed 2026-07-11 tweaks4
  // §1): every tag/vocab entry already loaded (this view's own `tags`
  // state, and the app-wide vocab lookup — both plain lists Go already
  // returned, no relationship recomputation), minus whichever are already
  // an active AND condition, minus whichever would leave zero results if
  // added — reusing the exact tagMatchesFilters/specMatchesFilters
  // membership functions the visible-list computation above already calls
  // (§7/§9: no new relationship logic, just asking the same question — "does
  // this filter set match anything" — for one more filter than what's
  // currently applied).
  const activeFilterKeys = new Set(filters.map((f) => `${f.type}:${f.id}`));
  const wouldMatchAny = (candidate: FilterCondition): boolean => {
    const testFilters = [...filters, candidate];
    if (facet === 'tags') {
      return tags.some((t) => tagMatchesFilters(t, testFilters, facetsData.roots));
    }
    return Object.values(txDetails).some((d) => specMatchesFilters(d, testFilters, vocabById));
  };
  // Owner candidates only make sense on the specs facet (facet='tags' cards
  // have no owner field to match against — tagMatchesFilters treats 'owner'
  // conditions as a pass-through, so they'd never actually narrow there).
  const ownerValues =
    facet === 'specs' ? Array.from(new Set(Array.from(vocabById.values()).map((v) => v.owner).filter((o): o is string => !!o))) : [];
  const suggestions: SuggestionItem[] = [
    ...tags
      .filter((tag) => !activeFilterKeys.has(`tag:${tag.id}`) && wouldMatchAny({ type: 'tag', id: tag.id }))
      .map((tag) => ({ id: tag.id, label: tag.name || tag.id, color: kindColor(tag.kind), kindLabel: t.nav.tags, onSelect: () => addFilter({ type: 'tag', id: tag.id }) })),
    ...Array.from(vocabById.values())
      .filter((v) => !activeFilterKeys.has(`vocab:${v.id}`) && wouldMatchAny({ type: 'vocab', id: v.id }))
      .map((v) => ({
        id: v.id,
        label: v.label || v.id,
        color: kindColor(v.category),
        kindLabel: t.nav.vocab,
        onSelect: () => addFilter({ type: 'vocab', id: v.id }),
      })),
    ...ownerValues
      .filter((o) => !activeFilterKeys.has(`owner:${o}`) && wouldMatchAny({ type: 'owner', id: o }))
      .map((o) => ({ id: o, label: o, color: OWNER_COLOR, kindLabel: t.vocab.owner, onSelect: () => addFilter({ type: 'owner', id: o }) })),
  ];

  return (
    <div class="browse-view">
      <BrowseRail
        query={query}
        onQueryChange={setQuery}
        kindFacet={kindFacet}
        kindOptions={kindOptions}
        onKindFacetChange={setKindFacet}
        conditions={conditions}
        onClearConditions={() => setFilters([])}
        indexItems={indexItems}
        suggestions={suggestions}
      />
      <Resizer config={RAIL_WIDTH} direction="rail" className="pmem-resizer--rail" />
      <main class="browse-main">
        <div class="browse-main-head">
          <h1>
            {title}
            <CommentButton recordType="page" recordId={facet} recordTitle={title} anchor="page" anchorLabel={t.comments.pageAnchorLabel} />
          </h1>
          <span class="dim">{subtitle}</span>
        </div>
        {failedCount > 0 && <div class="browse-fetch-warning">{t.browse.fetchWarning(failedCount)}</div>}
        <div class="browse-card-list">{body}</div>
      </main>
    </div>
  );
}
