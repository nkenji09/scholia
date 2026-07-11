import { useEffect, useRef, useState } from 'preact/hooks';
import { api } from '../api';
import { useLookups } from '../lookups';
import { useDrawer } from '../drawer';
import { strings } from '../strings';
import type { Transition, VocabEntry } from '../types';
import { BrowseRail } from './browse/BrowseRail';
import type { ConditionChip, IndexItem, KindOption, SuggestionItem } from './browse/BrowseRail';
import { VocabCard } from './browse/VocabCard';
import { CommentButton } from './comments/CommentButton';
import { kindColor } from './shared/Chip';

interface Props {
  onSelectTx: (id: string) => void;
  /** Vocab entry to scroll to on mount (router's #/vocab/<id>) — used by the
      comment panel's "位置へ移動" on vocab comments. */
  initialFocusId?: string;
}

// Order/labels match the design's きっかけ→前提→結果 grammar (2026-07-11
// tweaks3 §1), not the Go-side category name order.
const CATEGORIES: VocabEntry['category'][] = ['action', 'condition', 'effect'];

function usedBy(v: VocabEntry, transitions: Transition[]): Transition[] {
  return transitions.filter((t) => t.action === v.id || t.given.includes(v.id) || t.then.includes(v.id));
}

// Rebuilt (2026-07-11 tweaks2) to follow BrowseView's rail+card pattern —
// same BrowseRail component, same .card-based VocabCard — instead of its
// own bespoke category-tabs list (.concierge/decision.md's tweaks2 handoff
// §4, "他のタグ画面の構成を踏襲して"). Filtering/search/scroll-to-card all
// stay local state here rather than living in BrowseView/filters.ts: Vocab
// isn't a third BrowseView facet (its match rules — category instead of
// facet-tree kind, membership in v.tags instead of tagMatchesFilters'
// descendant-tree walk — don't fit that shared machinery), it just borrows
// the same rail/card *presentation*.
export function VocabView({ onSelectTx, initialFocusId }: Props) {
  const { tagById } = useLookups();
  const { closeDrawer } = useDrawer();
  const [vocab, setVocab] = useState<VocabEntry[] | null>(null);
  const [transitions, setTransitions] = useState<Transition[]>([]);
  const [error, setError] = useState<string | null>(null);

  const [query, setQuery] = useState('');
  const [categoryFacet, setCategoryFacet] = useState('all');
  const [tagFilters, setTagFilters] = useState<string[]>([]);

  const cardRefs = useRef<Map<string, HTMLElement>>(new Map());
  const scrollTarget = useRef<string | null>(initialFocusId || null);

  // Re-arm the scroll target if a comment's "位置へ移動" jumps here while
  // VocabView is already mounted (same pattern as BrowseView's per-facet
  // reset effect for initialFocusTagId/initialFocusTxId).
  useEffect(() => {
    scrollTarget.current = initialFocusId || null;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialFocusId]);

  useEffect(() => {
    Promise.all([api.getVocab(), api.getTransitions({})])
      .then(([v, tx]) => {
        setVocab(v);
        setTransitions(tx.transitions || []);
      })
      .catch((err) => setError(String(err)));
  }, []);

  useEffect(() => {
    const id = scrollTarget.current;
    if (!id || !vocab) return;
    const el = cardRefs.current.get(id);
    if (el) {
      el.scrollIntoView({ block: 'start' });
      scrollTarget.current = null;
    }
  });

  // Same close-on-select rule as BrowseView.tsx: addFilter/scroll-to close
  // the narrow-viewport drawer, kindFacet/removeFilter/query don't.
  const addTagFilter = (id: string) => {
    setTagFilters((prev) => (prev.includes(id) ? prev : [...prev, id]));
    closeDrawer();
  };
  const removeTagFilter = (i: number) => setTagFilters((prev) => prev.filter((_, idx) => idx !== i));

  if (error) return <div class="browse-view error">{error}</div>;
  if (!vocab) return <div class="browse-view dim">{strings.vocab.loading}</div>;

  const q = query.trim().toLowerCase();
  const kindOptions: KindOption[] = CATEGORIES.map((c) => ({
    key: c,
    label: strings.vocab.categoryLabel(c),
    count: vocab.filter((v) => v.category === c).length,
  }));

  const visible = vocab
    .filter((v) => categoryFacet === 'all' || v.category === categoryFacet)
    .filter((v) => !q || (v.id + ' ' + v.label + ' ' + (v.description || '')).toLowerCase().includes(q))
    .filter((v) => tagFilters.every((id) => (v.tags || []).includes(id)))
    .sort((a, b) => a.id.localeCompare(b.id));

  const indexItems: IndexItem[] = visible.map((v) => ({
    id: v.id,
    label: v.label,
    color: kindColor(v.category),
    indent: 0,
    onClick: () => {
      scrollTarget.current = v.id;
      cardRefs.current.get(v.id)?.scrollIntoView({ block: 'start' });
      closeDrawer();
    },
  }));

  const conditions: ConditionChip[] = tagFilters.map((id, i) => {
    const t = tagById.get(id);
    return { label: t?.name || id, color: kindColor(t?.kind), onRemove: () => removeTagFilter(i) };
  });

  // Combobox candidates (2026-07-11 tweaks3 §3): tags only, not other vocab
  // entries — Vocab's own entries have no self-filter affordance (see
  // VocabCard.tsx's onFilterSelf comment), so there's nothing meaningful a
  // vocab suggestion would do here that clicking the card itself doesn't
  // already do. Narrowed further (2026-07-11 tweaks4 §1) to only tags that
  // would leave ≥1 entry visible if added — reusing the same category-facet
  // + tagFilters membership test `visible` above already runs, just with
  // one more candidate tag appended (no new relationship logic, §7/§9).
  const categoryPool = vocab.filter((v) => categoryFacet === 'all' || v.category === categoryFacet);
  const tagWouldMatchAny = (tagId: string): boolean => {
    const testFilters = [...tagFilters, tagId];
    return categoryPool.some((v) => testFilters.every((id) => (v.tags || []).includes(id)));
  };
  const suggestions: SuggestionItem[] = Array.from(tagById.values())
    .filter((t) => !tagFilters.includes(t.id) && tagWouldMatchAny(t.id))
    .map((t) => ({ id: t.id, label: t.name || t.id, color: kindColor(t.kind), kindLabel: strings.nav.tags, onSelect: () => addTagFilter(t.id) }));

  return (
    <div class="browse-view">
      <BrowseRail
        query={query}
        onQueryChange={setQuery}
        kindFacet={categoryFacet}
        kindOptions={kindOptions}
        onKindFacetChange={setCategoryFacet}
        conditions={conditions}
        onClearConditions={() => setTagFilters([])}
        indexItems={indexItems}
        suggestions={suggestions}
      />
      <main class="browse-main">
        <div class="browse-main-head">
          <h1>
            {strings.vocab.heading}
            <CommentButton recordType="page" recordId="vocab" recordTitle={strings.vocab.heading} anchor="page" anchorLabel="ページ全体" />
          </h1>
          <span class="dim">{strings.vocab.intro}</span>
        </div>
        <div class="browse-card-list">
          {visible.length === 0 ? (
            <div class="card-empty">{strings.vocab.empty}</div>
          ) : (
            visible.map((v) => (
              <VocabCard
                key={v.id}
                entry={v}
                uses={usedBy(v, transitions)}
                cardRef={(el) => {
                  if (el) cardRefs.current.set(v.id, el);
                  else cardRefs.current.delete(v.id);
                }}
                onFilterTag={addTagFilter}
                onSelectTx={onSelectTx}
              />
            ))
          )}
        </div>
      </main>
    </div>
  );
}
