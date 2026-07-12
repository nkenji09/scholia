import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { api, isStaticMode } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { useDrawer } from '../../drawer';
import { usePendingDiff } from '../../pendingDiff';
import type { Config, FacetsResponse, SpecReport, Tag, TraceabilityResponse, Transition, TransitionDetail } from '../../types';
import { BrowseRail } from './BrowseRail';
import type { ConditionChip, IndexItem, KindOption, SuggestionItem } from './BrowseRail';
import { TagCard } from './TagCard';
import { SpecCard } from './SpecCard';
import { TombstoneCard } from './TombstoneCard';
import { NewTransitionForm } from './NewTransitionForm';
import { parentsOf, childrenOf, tagMatchesFilters, specMatchesFilters } from './filters';
import type { FilterCondition } from './filters';
import { kindColor, OWNER_COLOR } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { Icon } from '../shared/Icon';

interface Props {
  facet: 'tags' | 'specs';
  initialFocusTagId?: string;
  initialFocusTxId?: string;
  onGoToSpec: (txId: string) => void;
}

function buildTagOrder(facetsData: FacetsResponse, allTags: Tag[], kindFacet: string): Array<{ id: string; depth: number }> {
  const order: Array<{ id: string; depth: number }> = [];
  const seen = new Set<string>();
  const kinds = kindFacet === 'all' ? facetsData.facetKinds : facetsData.facetKinds.filter((k) => k === kindFacet);
  const walk = (nodes: FacetsResponse['trees'][string], depth: number) => {
    for (const n of nodes) {
      if (!seen.has(n.tag.id)) {
        order.push({ id: n.tag.id, depth });
        seen.add(n.tag.id);
      }
      if (n.children) walk(n.children, depth + 1);
    }
  };
  for (const k of kinds) walk(facetsData.trees[k] || [], 0);
  // Tags whose kind isn't a declared facet kind never appear in any tree
  // above — still show them, flat, rather than silently dropping them.
  for (const t of allTags) {
    if (seen.has(t.id)) continue;
    if (kindFacet !== 'all' && t.kind !== kindFacet) continue;
    order.push({ id: t.id, depth: 0 });
    seen.add(t.id);
  }
  return order;
}

export function BrowseView({ facet, initialFocusTagId, initialFocusTxId, onGoToSpec }: Props) {
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

  const [query, setQuery] = useState('');
  const [kindFacet, setKindFacet] = useState('all');
  const [filters, setFilters] = useState<FilterCondition[]>(() => (initialFocusTagId ? [{ type: 'tag', id: initialFocusTagId }] : []));
  const [openTx, setOpenTx] = useState<Record<string, boolean>>(() => (initialFocusTxId ? { [initialFocusTxId]: true } : {}));

  const cardRefs = useRef<Map<string, HTMLElement>>(new Map());
  const scrollTarget = useRef<string | null>(initialFocusTagId || initialFocusTxId || null);

  // Per-facet reset (design's `filters: { tags: [], specs: [] }` — each
  // facet keeps its own independent filter/search/open state). This only
  // fires when the *facet itself* changes (app.tsx mounts a fresh BrowseView
  // per route anyway; this additionally covers initialFocus* changing while
  // the same facet instance is reused for a same-facet legacy-route jump).
  useEffect(() => {
    setQuery('');
    setKindFacet('all');
    setFilters(initialFocusTagId ? [{ type: 'tag', id: initialFocusTagId }] : []);
    setOpenTx(initialFocusTxId ? { [initialFocusTxId]: true } : {});
    scrollTarget.current = initialFocusTagId || initialFocusTxId || null;
    setTagsSettled(false);
    setSpecsSettled(false);
    setTagsFailedCount(0);
    setSpecsFailedCount(0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [facet, initialFocusTagId, initialFocusTxId]);

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
    const order = buildTagOrder(facetsData, tags, kindFacet);
    const visible = order.filter(({ id }) => {
      const t = tagById.get(id);
      if (!t) return false;
      if (q && !(t.id + ' ' + (t.name || '') + ' ' + (t.description || '')).toLowerCase().includes(q)) return false;
      return tagMatchesFilters(t, filters, facetsData.trees);
    });

    title = t.browse.tagsTitle;
    subtitle = t.browse.tagsSubtitle;
    indexItems = visible.map(({ id, depth }) => {
      const t = tagById.get(id)!;
      const entry = gapByTagId.get(id);
      return {
        id,
        label: t.name || t.id,
        color: kindColor(t.kind),
        indent: depth,
        isGap: entry?.gap,
        onClick: () => scrollToCard(id),
      };
    });

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
              parents={parentsOf(facetsData.trees, id, tagById)}
              children={childrenOf(facetsData.trees, id, tagById)}
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
    indexItems = visible.map((tx) => ({
      id: tx.id,
      label: txDetails[tx.id]?.actionLabel || tx.id,
      color: 'var(--t-act)',
      indent: 0,
      onClick: () => scrollToCard(tx.id),
    }));

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
      return tags.some((t) => tagMatchesFilters(t, testFilters, facetsData.trees));
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
