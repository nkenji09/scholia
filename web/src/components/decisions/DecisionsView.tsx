import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import { api } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { useDrawer } from '../../drawer';
import type { Decision, FacetsResponse, GovernsRef, Tag, Transition, VocabEntry } from '../../types';
import { BrowseRail } from '../browse/BrowseRail';
import type { ConditionChip, IndexItem, SuggestionItem } from '../browse/BrowseRail';
import { ancestorClosure, tagTextMatches, textMatches, transitionVocabTagIds, vocabOwnMatches } from '../browse/filters';
import { buildFolderIndex, loadCollapsed, saveCollapsed } from '../browse/indexTree';
import { Resizer } from '../layout/Resizer';
import { RAIL_WIDTH } from '../layout/resizableWidths';
import { kindColor } from '../shared/Chip';
import { Icon } from '../shared/Icon';
import { buildCurrencyIndex, effectOf } from './decisionModel';
import { DecisionRowFull } from './DecisionRowFull';
import { governsParams, needsGoverns, parseScopeDirection, parseScopeTarget, scopeMatcher } from './decisionScope';

const COLLAPSE_FACET = 'decisions';

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
  /** 「どの対象か」（`tag:<id>` 等・01KYKS4Y56FAHRVCWKMQJK4RT6）。'' = 条件なし。 */
  on: string;
  /** 「どの向きか」（own / governing / subtree）。'' = 既定（subtree）。 */
  scope: string;
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
  /** 対象と向き（01KYKS4Y56FAHRVCWKMQJK4RT6）。他の5条件と AND で合成される。
      `decision:<ulid>` の形が旧単票の permalink を引き継ぐ。 */
  on: string;
  scope: string;
  onFiltersChange: (f: DecisionFilterState) => void;
  onOpenDecision: (id: string) => void;
}

const PERIOD_DAYS: Record<Exclude<PeriodFilter, 'all'>, number> = { '30d': 30, '90d': 90, '1y': 365 };

// 効力バッジは2値（01KYHW54B8ZXH0NEPH2J7N1X39 条項1）。記録の3値
// （supersede/amend/exception）は不変で、変えるのは画面の状態列だけ。
// かつてここは3値をそのまま出していて、amend を付けられただけの——**まだ
// 効いている**——decision が「改訂」として現行と別の状態に見え、履歴側だと
// 誤読された。付帯情報（後続に部分改訂・例外がある）は状態列ではなく行の
// 補助情報として出す（条項2）。バッジそのものは行（DecisionRowFull）が描く。

const splitTags = (v: string): string[] => (v ? v.split(',').filter(Boolean) : []);

export function DecisionsView({ searchQuery, targetKind, tagFilter, currency, period, on, scope, onFiltersChange, onOpenDecision }: Props) {
  const t = useT();
  // 記録日時・効力バッジ・要約の描画は行（DecisionRowFull）が持つ。ここが要るのは
  // 絞り込みの照合と、対象・条件の名乗りに使うラベルだけ。
  const { tagName, vocabLabel, transitionLabel } = useLookups();
  const { closeDrawer } = useDrawer();
  const [decisions, setDecisions] = useState<Decision[] | null>(null);
  const [tags, setTags] = useState<Tag[]>([]);
  const [vocab, setVocab] = useState<VocabEntry[]>([]);
  const [transitions, setTransitions] = useState<Transition[]>([]);
  const [facetsData, setFacetsData] = useState<FacetsResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [collapsedIds, setCollapsedIds] = useState<Set<string>>(() => loadCollapsed(COLLAPSE_FACET));

  const cardRefs = useRef<Map<string, HTMLElement>>(new Map());

  // Local filter state seeded from the URL. The list renders from these; the
  // URL is pushed (debounced) from the effect below.
  const [query, setQuery] = useState(() => searchQuery || '');
  const [kind, setKind] = useState<TargetKindFilter>(() => targetKind);
  const [cur, setCur] = useState<CurrencyFilter>(() => currency);
  const [per, setPer] = useState<PeriodFilter>(() => period);
  const [selectedTags, setSelectedTags] = useState<string[]>(() => splitTags(tagFilter));
  // 対象と向き。他の条件と同じく URL が正で、ここはその写し（リンクから来るので
  // 画面の widget では増えないが、チップの × で外せる＝条件を緩められる）。
  const [onRef, setOnRef] = useState<string>(() => on || '');
  const [scopeDir, setScopeDir] = useState<string>(() => scope || '');

  // Adopt state pushed in from *outside* our own typing/clicking (Back/Forward
  // → hashchange → new props). Runs on mount too, but the seeds already match
  // so it's a no-op there.
  useEffect(() => {
    setQuery(searchQuery || '');
    setKind(targetKind);
    setCur(currency);
    setPer(period);
    setSelectedTags(splitTags(tagFilter));
    setOnRef(on || '');
    setScopeDir(scope || '');
  }, [searchQuery, targetKind, currency, period, tagFilter, on, scope]);

  // Push local state back to the URL, but only when it genuinely diverges from
  // what the URL already encodes (echo/seed guard — the return leg of our own
  // push and the mount seed both no-op naturally, no dangling flag).
  useEffect(() => {
    const localTags = selectedTags.join(',');
    if (
      query === (searchQuery || '') &&
      kind === targetKind &&
      cur === currency &&
      per === period &&
      localTags === (tagFilter || '') &&
      onRef === (on || '') &&
      scopeDir === (scope || '')
    ) {
      return;
    }
    const id = setTimeout(
      () => onFiltersChange({ query, targetKind: kind, tagFilter: localTags, currency: cur, period: per, on: onRef, scope: scopeDir }),
      300,
    );
    return () => clearTimeout(id);
    // Deps are LOCAL state only (URL props read in-body) so an external nav
    // doesn't schedule a spurious push of stale local state.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, kind, cur, per, selectedTags, onRef, scopeDir]);

  // 「どの対象か」「どの向きか」の解釈は decisionScope が1箇所で持つ。
  const scopeTarget = useMemo(() => parseScopeTarget(onRef || undefined), [onRef]);
  const scopeDirection = useMemo(() => parseScopeDirection(scopeDir || undefined), [scopeDir]);

  // `governing` の判定は **CLI と同じ Go コア**（GET /api/governs＝
  // index.GovernsFor*）に委ねる。viewer 側に同じ選択規則をもう一実装置くと、
  // 「この記録を支配する規則は何か」に面ごとに違う答えが返る余地が復活する
  // （01KXYED61J6QBEX75H2XHVHW7Y の診断・追補 01KYJV3FYMDFRWQ939NBV2BPAC が
  // 名指しで警告した形）。静的書き出しでも同じ答えが返る（api.getGoverns）。
  const [governs, setGoverns] = useState<GovernsRef[] | null>(null);
  const governsWanted = needsGoverns(scopeTarget, scopeDirection);
  useEffect(() => {
    const params = governsParams(scopeTarget, scopeDirection);
    if (!params) {
      setGoverns(null);
      return;
    }
    let cancelled = false;
    setGoverns(null);
    api
      .getGoverns(params)
      .then((res) => {
        if (!cancelled) setGoverns(res.entries);
      })
      .catch(() => {
        // 取れなかったときは空集合として扱う——全件へ広げると「支配する規則」を
        // 名乗る一覧が無関係なものを並べる（名乗りと中身が食い違う）。
        if (!cancelled) setGoverns([]);
      });
    return () => {
      cancelled = true;
    };
  }, [scopeTarget?.type, scopeTarget?.id, scopeDirection]);

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
    // facets is the unified tag forest — the folder skeleton for the sidebar
    // index (req.comfortable-viewer.decision-browse amend: same
    // buildFolderIndex the tags/specs facets use). Each is a single bulk call
    // (no N+1); all five resolve in static mode too.
    Promise.all([api.getRules({}), api.getTags(), api.getVocab(), api.getTransitions({}), api.getFacets()])
      .then(([rules, tgs, vcb, tx, facets]) => {
        setDecisions(rules.decisions);
        setTags(tgs);
        setVocab(vcb);
        setTransitions(tx.transitions || []);
        setFacetsData(facets);
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
  //
  // 対象と向きもここに入れる（タグ AND やフリーワードと同じく、絞り込まれた
  // 集合が候補の母数でもある）。値の解釈は decisionScope が持つ。
  const matchesScope = useMemo(
    () => scopeMatcher({ target: scopeTarget, direction: scopeDirection, effTagsById, governs: governs ?? undefined }),
    [scopeTarget, scopeDirection, effTagsById, governs],
  );
  //
  // ⚠️ 対象が **1件の意思決定を名指ししている**ときは、他の条件を掛けない。
  // これは permalink（旧 #/decision/<id>）を引き継ぐ形で、「その1件を見せる」以外の
  // 意味を持たない URL である。既定の効力フィルタは「効いているものだけ」なので、
  // 掛けたままにすると**置き換え済みの意思決定を指す共有リンクが 0 件に着く**
  // ——実データで 158件中 1件が置き換え済み、かつ改訂チェーンを辿る導線
  // （置き換え/改訂チップ）は置き換え済みの相手を指すので、ここは日常的に踏まれる。
  const namesOneDecision = scopeTarget?.type === 'decision';
  const filterBase = useMemo(() => {
    if (!decisions) return [];
    if (namesOneDecision) return decisions.filter(matchesScope);
    return decisions.filter((d) => {
      if (kind !== 'all' && d.target.type !== kind) return false;
      // 効力は2値で判定する（条項1）。'all' は利用者が明示的に選んだときだけ。
      const e = effectOf(d.id, currencyIndex);
      if (cur === 'superseded' && e !== 'replaced') return false;
      if (cur === 'current' && e !== 'in-force') return false;
      if (per !== 'all') {
        const ageDays = (now - new Date(d.at).getTime()) / 86400000;
        if (!(ageDays <= PERIOD_DAYS[per])) return false;
      }
      if (!matchesScope(d)) return false;
      return true;
    });
  }, [decisions, currencyIndex, kind, cur, per, now, matchesScope, namesOneDecision]);

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
      // 1件を名指しした URL には、フリーワードもタグ AND も掛けない（上の
      // filterBase と同じ理由——その URL は「その1件を見せる」以外の意味を持たない）。
      if (namesOneDecision) return filterBase;
      const base = filterBase
        .filter((d) => !q || decisionTier(d) !== null)
        .filter(matchesTags)
        .slice()
        .reverse(); // newest-first (getRules is chronological asc)
      return q ? base.sort((a, b) => (decisionTier(a) ?? 4) - (decisionTier(b) ?? 4)) : base;
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [filterBase, q, effTagsById, selectedTags, namesOneDecision],
  );

  const byId = useMemo(() => new Map((decisions || []).map((d) => [d.id, d])), [decisions]);

  if (error) return <main class="decisions-view error">{error}</main>;
  if (!decisions || !facetsData) return <main class="decisions-view dim">{t.decisions.loading}</main>;
  // `governing` は Go コアへの問い合わせ待ちがある。取得前に「該当なし」と出すと
  // 一瞬「支配する規則は0件」と読める——待っていることを言う。
  if (governsWanted && governs === null) return <main class="decisions-view dim">{t.decisions.loading}</main>;

  const scrollToCard = (id: string) => {
    cardRefs.current.get(id)?.scrollIntoView({ block: 'start' });
    closeDrawer();
  };

  const toggleCollapse = (id: string) => {
    setCollapsedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      saveCollapsed(COLLAPSE_FACET, next);
      return next;
    });
  };

  // req.comfortable-viewer.decision-browse amend: the folder-index
  // classification for a decision is its target's OWN tags (not the
  // ancestor-closed/vocab-derived effTagsById above, which is the AND-filter
  // and search Tier's broader reach — buildFolderIndex handles ancestor
  // placement itself via the tree's own nesting).
  const decisionOwnTagIds = (d: Decision): string[] => {
    if (d.target.type === 'tag') return [d.target.id];
    if (d.target.type === 'vocab') return vocabById.get(d.target.id)?.tags || [];
    return txById.get(d.target.id)?.tags || [];
  };

  // 対象と向きが立っているときの名乗り。生 id はラベルに使わず名前で示す
  // （01KYCC2TF3NW3JRSSRK9ZHN078）——索引に無いものだけ、名乗るものが他に無いので
  // id へ落ちる。
  const scopeTargetName = (): string => {
    if (!scopeTarget) return '';
    if (scopeTarget.type === 'tag') return tagName(scopeTarget.id);
    if (scopeTarget.type === 'vocab') return vocabLabel(scopeTarget.id);
    if (scopeTarget.type === 'transition') return transitionLabel(scopeTarget.id).primary;
    const d = byId.get(scopeTarget.id);
    return d ? `${targetPrefix(d.target.type)} ${targetLabel(d)}` : t.decisions.scopeOneDecision;
  };
  const scopeChipLabel = (): string => {
    const name = scopeTargetName();
    if (scopeTarget?.type === 'decision') return t.decisions.scopeChipOne(name);
    if (scopeDirection === 'governing') return t.decisions.scopeChipGoverning(name);
    if (scopeDirection === 'own') return t.decisions.scopeChipOwn(name);
    return t.decisions.scopeChipSubtree(name);
  };
  const clearScope = () => {
    setOnRef('');
    setScopeDir('');
  };

  // AND condition chips — 対象/向きのチップを先頭に置き、タグ AND のチップが続く。
  // どれも × で外せる＝**戻るを押さずに条件を緩められる**。これが「1件に絞った
  // 一覧」が単票より良い理由そのものなので、外せない形にしない。
  const conditions: ConditionChip[] = [
    ...(scopeTarget ? [{ label: scopeChipLabel(), color: kindColor(scopeTarget.type === 'tag' ? tagById.get(scopeTarget.id)?.kind : undefined), onRemove: clearScope }] : []),
    ...selectedTags.map((id) => {
      const tg = tagById.get(id);
      return { label: tg?.name || id, color: kindColor(tg?.kind), onRemove: () => removeTag(id) };
    }),
  ];

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

  // req.comfortable-viewer.decision-browse amend: same タグ階層フォルダ index
  // the tags/specs/flow screens use (buildFolderIndex) — each decision files
  // into every tag folder its target's own tags reach, duplicated across
  // folders same as a multi-tagged spec. The founding "reach it without
  // knowing its tag" purpose stays intact: this is an added path alongside
  // the still-primary flat/chronological + free-text search list, not a
  // replacement for it.
  const indexItems: IndexItem[] = buildFolderIndex({
    roots: facetsData.roots,
    leaves: filtered.map((d) => ({
      id: d.id,
      label: targetLabel(d),
      color: kindColor(d.target.type === 'tag' ? tagById.get(d.target.id)?.kind : undefined),
      tags: decisionOwnTagIds(d),
    })),
    untaggedLabel: t.browse.uncategorized,
    folderColor: (tag) => kindColor(tag.kind),
    collapsedIds,
    onToggle: toggleCollapse,
    onSelect: scrollToCard,
  });

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
          <option value="current">{t.decisions.effectInForce}</option>
          <option value="superseded">{t.decisions.effectReplaced}</option>
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
        onClearConditions={() => {
          setSelectedTags([]);
          clearScope();
        }}
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
              {/* 結果が1件なら開いた状態で着地する（01KYKS4Y56FAHRVCWKMQJK4RT6）。
                  これは**初期既定**であって上書きではない——利用者が明示的に閉じた
                  保存値のほうが勝つ（01KYGYYN8HRNFQEDMBS3DZRRX7）。行の中身は
                  DecisionRowFull が1箇所で持つ（面ごとに書き分けない）。 */}
              {filtered.map((d) => (
                <li
                  key={d.id}
                  ref={(el) => {
                    if (el) cardRefs.current.set(d.id, el as HTMLElement);
                    else cardRefs.current.delete(d.id);
                  }}
                >
                  <DecisionRowFull d={d} defaultOpen={filtered.length === 1} onOpenDecision={onOpenDecision} byId={byId} />
                </li>
              ))}
            </ul>
          )}
        </div>
      </main>
    </div>
  );
}
