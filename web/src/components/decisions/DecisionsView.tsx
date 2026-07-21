import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { api } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { useDrawer } from '../../drawer';
import type { Decision, Tag, Transition, VocabEntry } from '../../types';
import { BrowseRail } from '../browse/BrowseRail';
import type { ConditionChip, IndexItem, SuggestionItem } from '../browse/BrowseRail';
import { ancestorClosure, tagTextMatches, textMatches, transitionVocabTagIds, vocabOwnMatches } from '../browse/filters';
import { Resizer } from '../layout/Resizer';
import { RAIL_WIDTH } from '../layout/resizableWidths';
import { kindColor } from '../shared/Chip';
import { Icon } from '../shared/Icon';
import { buildCurrencyIndex, currencyOf, type Currency } from './decisionModel';

type TargetKindFilter = 'all' | 'transition' | 'tag' | 'vocab';
type CurrencyFilter = 'all' | 'current' | 'superseded';
type PeriodFilter = 'all' | '30d' | '90d' | '1y';

// All filter state (#45 D10b-4) round-trips through the URL via App. Local
// state below drives the list immediately; a debounced effect mirrors it into
// the hash (same push/adopt pattern as BrowseView/VocabView) so the combobox's
// select-then-clear-query pair composes into one URL update instead of two
// racing navigates clobbering each other.
export interface DecisionFilterState {
  query: string;
  targetKind: TargetKindFilter;
  /** Comma-joined tag ids of the active AND filter (viewer-search-consistency:
      the tag axis moved from a single native <select> to the BrowseRail
      combobox + removable AND chips). '' = no tag filter. The URL key (dt)
      is unchanged; only its value widened from one id to a list. */
  tagFilter: string;
  currency: CurrencyFilter;
  period: PeriodFilter;
}

interface Props {
  /** Free-text query (routed via the shared searchQuery hash param so it
      round-trips a shared link). */
  searchQuery: string;
  /** Filter state, restored from the URL (#45 D10b-4). */
  targetKind: TargetKindFilter;
  tagFilter: string;
  currency: CurrencyFilter;
  period: PeriodFilter;
  onFiltersChange: (f: DecisionFilterState) => void;
  onOpenDecision: (id: string) => void;
}

const PERIOD_DAYS: Record<Exclude<PeriodFilter, 'all'>, number> = { '30d': 30, '90d': 90, '1y': 365 };

// Currency → badge class + label. 'amended' rides the same "still current but
// caveated" bucket as current for the 現行/失効 filter, but keeps its own badge.
function currencyBadge(c: Currency, t: ReturnType<typeof useT>): { cls: string; label: string } {
  if (c === 'superseded') return { cls: 'decision-badge-superseded', label: t.decisions.currencySuperseded };
  if (c === 'amended') return { cls: 'decision-badge-amended', label: t.decisions.currencyAmended };
  return { cls: 'decision-badge-current', label: t.decisions.currencyCurrent };
}

const splitTags = (v: string): string[] => (v ? v.split(',').filter(Boolean) : []);

export function DecisionsView({ searchQuery, targetKind, tagFilter, currency, period, onFiltersChange, onOpenDecision }: Props) {
  const t = useT();
  const { tagName, vocabLabel, transitionLabel } = useLookups();
  const { closeDrawer } = useDrawer();
  const [decisions, setDecisions] = useState<Decision[] | null>(null);
  const [tags, setTags] = useState<Tag[]>([]);
  const [vocab, setVocab] = useState<VocabEntry[]>([]);
  const [transitions, setTransitions] = useState<Transition[]>([]);
  const [error, setError] = useState<string | null>(null);

  const cardRefs = useRef<Map<string, HTMLElement>>(new Map());

  // Local filter state seeded from the URL. The list renders from these; the
  // URL is pushed (debounced) from the effect below.
  const [query, setQuery] = useState(() => searchQuery || '');
  const [kind, setKind] = useState<TargetKindFilter>(() => targetKind);
  const [cur, setCur] = useState<CurrencyFilter>(() => currency);
  const [per, setPer] = useState<PeriodFilter>(() => period);
  const [selectedTags, setSelectedTags] = useState<string[]>(() => splitTags(tagFilter));

  // Adopt state pushed in from *outside* our own typing/clicking (Back/Forward
  // → hashchange → new props). Runs on mount too, but the seeds already match
  // so it's a no-op there.
  useEffect(() => {
    setQuery(searchQuery || '');
    setKind(targetKind);
    setCur(currency);
    setPer(period);
    setSelectedTags(splitTags(tagFilter));
  }, [searchQuery, targetKind, currency, period, tagFilter]);

  // Push local state back to the URL, but only when it genuinely diverges from
  // what the URL already encodes (echo/seed guard — the return leg of our own
  // push and the mount seed both no-op naturally, no dangling flag).
  useEffect(() => {
    const localTags = selectedTags.join(',');
    if (query === (searchQuery || '') && kind === targetKind && cur === currency && per === period && localTags === (tagFilter || '')) {
      return;
    }
    const id = setTimeout(() => onFiltersChange({ query, targetKind: kind, tagFilter: localTags, currency: cur, period: per }), 300);
    return () => clearTimeout(id);
    // Deps are LOCAL state only (URL props read in-body) so an external nav
    // doesn't schedule a spurious push of stale local state.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, kind, cur, per, selectedTags]);

  const addTag = (id: string) => {
    setSelectedTags((prev) => (prev.includes(id) ? prev : [...prev, id]));
    // Close the narrow-viewport drawer on select (same rule as BrowseView/
    // VocabView: picking a filter narrows the list, so the drawer's job is
    // done — adjusting the native selects / removing a chip doesn't close).
    closeDrawer();
  };
  const removeTag = (id: string) => setSelectedTags((prev) => prev.filter((x) => x !== id));

  useEffect(() => {
    // vocab/transitions are loaded alongside rules/tags so the tag filter can
    // resolve a decision's effective tag set for every target type (tag →
    // itself, vocab/transition → their own tags), all closed over ancestors.
    // Each is a single bulk call (no N+1); all four resolve in static mode too.
    Promise.all([api.getRules({}), api.getTags(), api.getVocab(), api.getTransitions({})])
      .then(([rules, tgs, vcb, tx]) => {
        setDecisions(rules.decisions);
        setTags(tgs);
        setVocab(vcb);
        setTransitions(tx.transitions || []);
      })
      .catch((err) => setError(String(err)));
  }, []);

  const tagById = useMemo(() => new Map(tags.map((tg) => [tg.id, tg])), [tags]);
  const vocabById = useMemo(() => new Map(vocab.map((v) => [v.id, v])), [vocab]);
  const txById = useMemo(() => new Map(transitions.map((x) => [x.id, x])), [transitions]);
  const parents = useMemo(() => new Map(tags.map((tg) => [tg.id, tg.parentIds || []])), [tags]);

  // The target's human label, covering all three target types (transitionLabel
  // only handles transitions; tag/vocab resolve through their own lookups).
  const targetLabel = (d: Decision): string => {
    if (d.target.type === 'tag') return tagName(d.target.id);
    if (d.target.type === 'vocab') return vocabLabel(d.target.id);
    return transitionLabel(d.target.id).primary;
  };
  const targetPrefix = (type: Decision['target']['type']): string =>
    type === 'tag' ? t.decisions.targetPrefixTag : type === 'vocab' ? t.decisions.targetPrefixVocab : t.decisions.targetPrefixTransition;

  const currencyIndex = useMemo(() => buildCurrencyIndex(decisions || []), [decisions]);

  // The effective tag set of each decision (viewer-search-consistency,
  // req.comfortable-viewer.decision-browse amend): the ancestor-closure of
  // the decision's target's own tags. tag targets seed with themselves;
  // vocab targets seed with their own tags; transition targets seed with
  // their own tags PLUS the tags of every vocab entry they reference
  // (action/given/then) — DESIGN §3.7's full effective-tag formula, matching
  // BrowseView. The AND tag filter matches when every selected tag is in
  // this set.
  const effTagsById = useMemo(() => {
    const m = new Map<string, Set<string>>();
    for (const d of decisions || []) {
      let own: string[] = [];
      if (d.target.type === 'tag') own = [d.target.id];
      else if (d.target.type === 'vocab') own = vocabById.get(d.target.id)?.tags || [];
      else {
        const tx = txById.get(d.target.id);
        own = tx ? transitionVocabTagIds(tx, vocabById) : [];
      }
      m.set(d.id, ancestorClosure(own, parents));
    }
    return m;
  }, [decisions, vocabById, txById, parents]);

  const q = query.trim().toLowerCase();
  const now = Date.now();

  // Base = the non-tag, non-free-text filters only (対象種別/現行性/期間). This is
  // deliberately query-independent: it's what the visible list narrows further
  // AND what the combobox gates its suggestions against — the free-text box
  // narrows the shown suggestions (by tag name, inside BrowseRail) but must not
  // shrink the candidate pool, or typing a tag name that no record's why/target
  // happens to contain would surface no suggestion (same rule as BrowseView).
  const filterBase = useMemo(() => {
    if (!decisions) return [];
    return decisions.filter((d) => {
      if (kind !== 'all' && d.target.type !== kind) return false;
      const c = currencyOf(d.id, currencyIndex);
      if (cur === 'superseded' && c !== 'superseded') return false;
      if (cur === 'current' && c === 'superseded') return false;
      if (per !== 'all') {
        const ageDays = (now - new Date(d.at).getTime()) / 86400000;
        if (!(ageDays <= PERIOD_DAYS[per])) return false;
      }
      return true;
    });
  }, [decisions, currencyIndex, kind, cur, per, now]);

  // req.comfortable-viewer.faceted-nav amend: 1=decision's own why/changed/
  // ref/acknowledges + target's own identity (tag→id/name/description・
  // vocab→id/label/description/altLabels・transition→referenced vocab's
  // same), 2=target's tag classification (tag→ancestors・vocab→own tags+
  // ancestors・transition→own tags+ancestors, name/description only — never
  // id), 3=transition targets only: referenced vocab's tags + ancestors.
  // Lower tier = more relevant; null = no match.
  const decisionTier = (d: Decision): number | null => {
    const target = d.target;
    const ownHit = textMatches(q, d.why, d.changed, d.ref, target.id, targetLabel(d), ...(d.acknowledges || []));
    if (target.type === 'tag') {
      const tg = tagById.get(target.id);
      if (ownHit || (tg && textMatches(q, tg.description))) return 1;
      if (tagTextMatches(tg?.parentIds || [], tagById, parents, q)) return 2;
      return null;
    }
    if (target.type === 'vocab') {
      const v = vocabById.get(target.id);
      if (ownHit || vocabOwnMatches(v, q)) return 1;
      if (tagTextMatches(v?.tags || [], tagById, parents, q)) return 2;
      return null;
    }
    const tx = txById.get(target.id);
    const vocabIds = tx ? [tx.action, ...tx.given, ...tx.then] : [];
    if (ownHit || vocabIds.some((vid) => vocabOwnMatches(vocabById.get(vid), q))) return 1;
    if (tx && tagTextMatches(tx.tags || [], tagById, parents, q)) return 2;
    const refTagIds = vocabIds.flatMap((vid) => vocabById.get(vid)?.tags || []);
    if (tagTextMatches(refTagIds, tagById, parents, q)) return 3;
    return null;
  };
  const matchesTags = (d: Decision): boolean => {
    if (selectedTags.length === 0) return true;
    const eff = effTagsById.get(d.id);
    return !!eff && selectedTags.every((tg) => eff.has(tg));
  };

  const filtered = useMemo(
    () => {
      const base = filterBase
        .filter((d) => !q || decisionTier(d) !== null)
        .filter(matchesTags)
        .slice()
        .reverse(); // newest-first (getRules is chronological asc)
      return q ? base.sort((a, b) => (decisionTier(a) ?? 4) - (decisionTier(b) ?? 4)) : base;
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [filterBase, q, effTagsById, selectedTags],
  );

  if (error) return <main class="decisions-view error">{error}</main>;
  if (!decisions) return <main class="decisions-view dim">{t.decisions.loading}</main>;

  const scrollToCard = (id: string) => {
    cardRefs.current.get(id)?.scrollIntoView({ block: 'start' });
    closeDrawer();
  };

  // AND condition chips (the selected tags) — same removable-chip shape the
  // other browse rails use.
  const conditions: ConditionChip[] = selectedTags.map((id) => {
    const tg = tagById.get(id);
    return { label: tg?.name || id, color: kindColor(tg?.kind), onRemove: () => removeTag(id) };
  });

  // Combobox candidates: every tag that is an effective tag of some decision
  // still passing the other filters, minus the already-selected ones, minus
  // any that would leave zero results if added (same "AND-narrow, only offer
  // what helps" rule as BrowseView/VocabView).
  const selectedSet = new Set(selectedTags);
  const corpusTagIds = new Set<string>();
  for (const d of filterBase) for (const id of effTagsById.get(d.id) || []) corpusTagIds.add(id);
  const wouldMatchAny = (candidate: string): boolean =>
    filterBase.some((d) => {
      const eff = effTagsById.get(d.id);
      return !!eff && eff.has(candidate) && selectedTags.every((tg) => eff.has(tg));
    });
  const suggestions: SuggestionItem[] = Array.from(corpusTagIds)
    .filter((id) => !selectedSet.has(id) && wouldMatchAny(id))
    .map((id) => tagById.get(id))
    .filter((tg): tg is Tag => !!tg)
    .sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id))
    .map((tg) => ({ id: tg.id, label: tg.name || tg.id, color: kindColor(tg.kind), kindLabel: t.nav.tags, onSelect: () => addTag(tg.id) }));

  // Jump index: the currently-visible decisions, keyed by target label, so the
  // rail offers scroll-to-row navigation like the other browse pages.
  const indexItems: IndexItem[] = filtered.map((d) => ({
    id: d.id,
    label: targetLabel(d),
    color: kindColor(d.target.type === 'tag' ? tagById.get(d.target.id)?.kind : undefined),
    indent: 0,
    onClick: () => scrollToCard(d.id),
  }));

  // 対象種別・現行性・期間 keep their native <select> widgets but move into the
  // shared responsive drawer (viewer-search-consistency amend). Only the tag
  // axis changed widget (→ combobox + AND chips above).
  const extraControls = (
    <div class="decisions-rail-filters">
      <label class="decisions-filter">
        <span class="decisions-filter-label dim">{t.decisions.filterTargetKind}</span>
        <select value={kind} onChange={(e) => setKind((e.target as HTMLSelectElement).value as TargetKindFilter)}>
          <option value="all">{t.decisions.filterAll}</option>
          <option value="transition">{t.decisions.targetKindTransition}</option>
          <option value="tag">{t.decisions.targetKindTag}</option>
          <option value="vocab">{t.decisions.targetKindVocab}</option>
        </select>
      </label>
      <label class="decisions-filter">
        <span class="decisions-filter-label dim">{t.decisions.filterCurrency}</span>
        <select value={cur} onChange={(e) => setCur((e.target as HTMLSelectElement).value as CurrencyFilter)}>
          <option value="all">{t.decisions.filterAll}</option>
          <option value="current">{t.decisions.currencyCurrent}</option>
          <option value="superseded">{t.decisions.currencySuperseded}</option>
        </select>
      </label>
      <label class="decisions-filter">
        <span class="decisions-filter-label dim">{t.decisions.filterPeriod}</span>
        <select value={per} onChange={(e) => setPer((e.target as HTMLSelectElement).value as PeriodFilter)}>
          <option value="all">{t.decisions.periodAll}</option>
          <option value="30d">{t.decisions.period30d}</option>
          <option value="90d">{t.decisions.period90d}</option>
          <option value="1y">{t.decisions.period1y}</option>
        </select>
      </label>
    </div>
  );

  return (
    <div class="browse-view">
      <BrowseRail
        query={query}
        onQueryChange={setQuery}
        kindFacet="all"
        kindOptions={[]}
        onKindFacetChange={() => {}}
        conditions={conditions}
        onClearConditions={() => setSelectedTags([])}
        indexItems={indexItems}
        suggestions={suggestions}
        extraControls={extraControls}
      />
      <Resizer config={RAIL_WIDTH} direction="rail" className="scholia-resizer--rail" />
      <main class="browse-main decisions-main">
        <div class="browse-main-head">
          <h1>
            <Icon name="gavel" size={20} /> {t.decisions.heading}
            <span class="decisions-count dim">{t.decisions.countLabel(filtered.length)}</span>
          </h1>
          <span class="dim">{t.decisions.intro}</span>
        </div>
        <div class="browse-card-list">
          {decisions.length === 0 ? (
            <p class="dim decisions-empty">{t.decisions.empty}</p>
          ) : filtered.length === 0 ? (
            <p class="dim decisions-empty">{t.decisions.noMatch}</p>
          ) : (
            <ul class="decisions-list">
              {filtered.map((d) => {
                const badge = currencyBadge(currencyOf(d.id, currencyIndex), t);
                return (
                  <li key={d.id}>
                    <button
                      type="button"
                      class="decision-row"
                      ref={(el) => {
                        if (el) cardRefs.current.set(d.id, el);
                        else cardRefs.current.delete(d.id);
                      }}
                      onClick={() => onOpenDecision(d.id)}
                    >
                      <div class="decision-row-top">
                        <span class="decision-row-target">
                          <span class="decision-row-target-kind dim">{targetPrefix(d.target.type)}</span>
                          {targetLabel(d)}
                        </span>
                        <span class={'decision-badge ' + badge.cls}>{badge.label}</span>
                      </div>
                      <p class="decision-row-why">{d.why}</p>
                      <span class="decision-row-at dim">{d.at.slice(0, 10)}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </main>
    </div>
  );
}
