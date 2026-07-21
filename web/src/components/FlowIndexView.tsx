import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { api } from '../api';
import { useT } from '../i18n';
import { useLookups } from '../lookups';
import { useDrawer } from '../drawer';
import { routeHash } from '../router';
import type { Tag, Transition, VocabEntry } from '../types';
import { BrowseRail } from './browse/BrowseRail';
import type { ConditionChip, IndexItem, KindOption, SuggestionItem } from './browse/BrowseRail';
import { ancestorClosure, tagTextMatches, textMatches, transitionVocabTagIds, vocabOwnMatches } from './browse/filters';
import { Resizer } from './layout/Resizer';
import { RAIL_WIDTH } from './layout/resizableWidths';
import { kindColor } from './shared/Chip';
import { HashLink } from './shared/HashLink';
import { Icon } from './shared/Icon';

// #/flow index（tx.viewer.flow-nav-tab）: nav の「フロー」タブの着地点。実際に
// 使われている action を一覧し、選ぶと #/flow/<action> の既存フロービューへ。
// flow の表示内容（mermaid のみ・scope-honesty の CLI 開示）は不変。
// viewer-search-consistency（flow-browse）: label/id フリーワードに加え、action
// が消費するタグ/kind の AND 絞り込み（BrowseRail の combobox＋AND チップ＋kind
// facet＋レスポンシブ・ドロワー）を新設。絞り込み状態は URL に載せて復元する。

// Filter state round-trips through the URL via App (deep-linking amend). Local
// state drives the list immediately; a debounced effect mirrors it into the
// hash (same push/adopt pattern as BrowseView/VocabView) so the combobox's
// select-then-clear-query pair composes into one URL update.
export interface FlowFilterState {
  query: string;
  kindFacet: string;
  /** Tag ids of the active AND filter. */
  tags: string[];
}

interface Props {
  onSelectAction: (actionId: string) => void;
  /** Free-text query (shared searchQuery hash param). */
  searchQuery: string;
  /** Tag-kind facet (shared searchKindFacet hash param). */
  kindFacet: string;
  /** Comma-joined tag ids of the active AND filter (dedicated ft param). */
  flowTags: string;
  onFiltersChange: (f: FlowFilterState) => void;
}

const splitTags = (v: string): string[] => (v ? v.split(',').filter(Boolean) : []);

export function FlowIndexView({ onSelectAction, searchQuery, kindFacet, flowTags, onFiltersChange }: Props) {
  const t = useT();
  const { vocabLabel, tagKindLabel } = useLookups();
  const { closeDrawer } = useDrawer();
  const [transitions, setTransitions] = useState<Transition[] | null>(null);
  const [tags, setTags] = useState<Tag[]>([]);
  const [vocab, setVocab] = useState<VocabEntry[]>([]);
  const [error, setError] = useState<string | null>(null);

  const rowRefs = useRef<Map<string, HTMLElement>>(new Map());

  // Local filter state seeded from the URL; the list renders from these, the
  // URL is pushed (debounced) from the effect below.
  const [query, setQuery] = useState(() => searchQuery || '');
  const [facet, setFacet] = useState(() => kindFacet || 'all');
  const [selectedTags, setSelectedTags] = useState<string[]>(() => splitTags(flowTags));

  // Adopt state pushed in from outside (Back/Forward → hashchange → new props).
  useEffect(() => {
    setQuery(searchQuery || '');
    setFacet(kindFacet || 'all');
    setSelectedTags(splitTags(flowTags));
  }, [searchQuery, kindFacet, flowTags]);

  // Push local state to the URL only when it diverges (echo/seed guard).
  useEffect(() => {
    const localTags = selectedTags.join(',');
    if (query === (searchQuery || '') && facet === (kindFacet || 'all') && localTags === (flowTags || '')) return;
    const id = setTimeout(() => onFiltersChange({ query, kindFacet: facet, tags: selectedTags }), 300);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, facet, selectedTags]);

  const addTag = (id: string) => {
    setSelectedTags((prev) => (prev.includes(id) ? prev : [...prev, id]));
    closeDrawer();
  };
  const removeTag = (id: string) => setSelectedTags((prev) => prev.filter((x) => x !== id));

  useEffect(() => {
    // tags carry parentIds (ancestor rollup) and kind (facet); transitions
    // carry each action's own tags; vocab carries the tags referenced
    // action/given/then entries add via vocab-tag assignment (req.comfortable-
    // viewer.flow-browse amend — vocab-derived tags count as "consumed" by the
    // action too, not just each transition's own tags). All three are single
    // bulk calls, static-safe.
    Promise.all([api.getTransitions({}), api.getTags(), api.getVocab()])
      .then(([res, tgs, vcb]) => {
        setTransitions(res.transitions ?? []);
        setTags(tgs);
        setVocab(vcb);
      })
      .catch((err) => setError(String(err)));
  }, []);

  const tagById = useMemo(() => new Map(tags.map((tg) => [tg.id, tg])), [tags]);
  const parents = useMemo(() => new Map(tags.map((tg) => [tg.id, tg.parentIds || []])), [tags]);
  const vocabById = useMemo(() => new Map(vocab.map((v) => [v.id, v])), [vocab]);

  // Distinct action ids actually used by a transition, each with a per-action
  // count and its effective tag set (ancestor-closed union of the own tags AND
  // referenced-vocab tags of every transition carrying that action —
  // "action が消費するタグ", req.comfortable-viewer.flow-browse amend).
  const actions = useMemo(() => {
    const byAction = new Map<string, Transition[]>();
    for (const tx of transitions ?? []) {
      const arr = byAction.get(tx.action) || [];
      arr.push(tx);
      byAction.set(tx.action, arr);
    }
    return Array.from(byAction.entries())
      .map(([id, txs]) => {
        const own = new Set<string>();
        const vocabIds = new Set<string>();
        const ownTagIds = new Set<string>();
        for (const tx of txs) {
          for (const tg of transitionVocabTagIds(tx, vocabById)) own.add(tg);
          vocabIds.add(tx.action);
          for (const g of tx.given) vocabIds.add(g);
          for (const th of tx.then) vocabIds.add(th);
          for (const tg of tx.tags || []) ownTagIds.add(tg);
        }
        return { id, label: vocabLabel(id), count: txs.length, tags: ancestorClosure(Array.from(own), parents), vocabIds, ownTagIds };
      })
      .sort((a, b) => a.label.localeCompare(b.label) || a.id.localeCompare(b.id));
    // vocabLabel closes over lookups (id fallback is stable enough); excluded
    // from deps intentionally.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [transitions, parents, vocabById]);

  const hasKind = (a: { tags: Set<string> }, k: string) => Array.from(a.tags).some((tid) => tagById.get(tid)?.kind === k);

  // Tag kinds present across the actions' effective tags, with a per-kind count
  // of actions carrying that kind — the facet button row.
  const kindOptions: KindOption[] = useMemo(() => {
    const kinds = new Set<string>();
    for (const a of actions) for (const tid of a.tags) { const k = tagById.get(tid)?.kind; if (k) kinds.add(k); }
    return Array.from(kinds)
      .map((k) => ({ key: k, label: tagKindLabel(k), count: actions.filter((a) => hasKind(a, k)).length }))
      .sort((x, y) => x.label.localeCompare(y.label));
    // tagKindLabel/hasKind close over lookups; excluded from deps intentionally.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [actions, tagById]);

  if (error) return <main class="flow-index-view error">{error}</main>;
  if (!transitions) return <main class="flow-index-view dim">{t.flow.loading}</main>;

  const q = query.trim().toLowerCase();
  const matchesKind = (a: { tags: Set<string> }) => facet === 'all' || hasKind(a, facet);

  // req.comfortable-viewer.faceted-nav amend: 1=action's own + given/then
  // vocab's id/label/description/altLabels (own identity), 2=the action's
  // transitions' own tags + ancestors (name/description, never id), 3=
  // referenced vocab's tags + ancestors (name/description). Lower tier =
  // more relevant; null = no match.
  const actionTier = (a: (typeof actions)[number]): number | null => {
    if (textMatches(q, a.id, a.label) || Array.from(a.vocabIds).some((vid) => vocabOwnMatches(vocabById.get(vid), q))) return 1;
    if (tagTextMatches(a.ownTagIds, tagById, parents, q)) return 2;
    const refTagIds = Array.from(a.vocabIds).flatMap((vid) => vocabById.get(vid)?.tags || []);
    if (tagTextMatches(refTagIds, tagById, parents, q)) return 3;
    return null;
  };

  // Base = the kind facet only (query-independent, like BrowseView): the free
  // text narrows the shown suggestions by tag name inside BrowseRail but must
  // not shrink the candidate pool, or typing a tag name absent from every
  // action's label/id would surface no suggestion.
  const kindBase = actions.filter(matchesKind);
  const filtered = kindBase
    .filter((a) => (!q || actionTier(a) !== null) && selectedTags.every((tg) => a.tags.has(tg)))
    .sort((a, b) => (q ? (actionTier(a) ?? 4) - (actionTier(b) ?? 4) : 0));

  const scrollToRow = (id: string) => {
    rowRefs.current.get(id)?.scrollIntoView({ block: 'start' });
    closeDrawer();
  };

  const conditions: ConditionChip[] = selectedTags.map((id) => {
    const tg = tagById.get(id);
    return { label: tg?.name || id, color: kindColor(tg?.kind), onRemove: () => removeTag(id) };
  });

  const selectedSet = new Set(selectedTags);
  const corpusTagIds = new Set<string>();
  for (const a of kindBase) for (const id of a.tags) corpusTagIds.add(id);
  const wouldMatchAny = (candidate: string): boolean =>
    kindBase.some((a) => a.tags.has(candidate) && selectedTags.every((tg) => a.tags.has(tg)));
  const suggestions: SuggestionItem[] = Array.from(corpusTagIds)
    .filter((id) => !selectedSet.has(id) && wouldMatchAny(id))
    .map((id) => tagById.get(id))
    .filter((tg): tg is Tag => !!tg)
    .sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id))
    .map((tg) => ({ id: tg.id, label: tg.name || tg.id, color: kindColor(tg.kind), kindLabel: t.nav.tags, onSelect: () => addTag(tg.id) }));

  const indexItems: IndexItem[] = filtered.map((a) => ({
    id: a.id,
    label: a.label,
    color: 'var(--t-act)',
    indent: 0,
    onClick: () => scrollToRow(a.id),
  }));

  return (
    <div class="browse-view">
      <BrowseRail
        query={query}
        onQueryChange={setQuery}
        kindFacet={facet}
        kindOptions={kindOptions}
        onKindFacetChange={setFacet}
        conditions={conditions}
        onClearConditions={() => setSelectedTags([])}
        indexItems={indexItems}
        suggestions={suggestions}
      />
      <Resizer config={RAIL_WIDTH} direction="rail" className="scholia-resizer--rail" />
      <main class="browse-main flow-index-main">
        <div class="browse-main-head">
          <h1>
            <Icon name="git-fork" size={20} /> {t.flow.indexTitle}
            <span class="flow-index-count dim">{t.flow.indexCount(filtered.length)}</span>
          </h1>
          <span class="dim">{t.flow.indexIntro}</span>
        </div>
        <div class="browse-card-list">
          {actions.length === 0 ? (
            <p class="dim flow-index-empty">{t.flow.indexEmpty}</p>
          ) : filtered.length === 0 ? (
            <p class="dim flow-index-empty">{t.flow.indexNoMatch}</p>
          ) : (
            <ul class="flow-index-list">
              {filtered.map((a) => (
                <li
                  key={a.id}
                  ref={(el) => {
                    if (el) rowRefs.current.set(a.id, el);
                    else rowRefs.current.delete(a.id);
                  }}
                >
                  <HashLink href={routeHash({ view: 'flow', actionId: a.id })} class="flow-index-row" onNavigate={() => onSelectAction(a.id)} title={a.id}>
                    <span class="flow-index-row-label">{a.label}</span>
                    <span class="flow-index-row-count dim">{t.flow.indexTxCount(a.count)}</span>
                  </HashLink>
                </li>
              ))}
            </ul>
          )}
        </div>
      </main>
    </div>
  );
}
