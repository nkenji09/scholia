import { useEffect, useRef, useState } from 'preact/hooks';
import { api } from '../api';
import { useLookups } from '../lookups';
import { useDrawer } from '../drawer';
import { useT } from '../i18n';
import type { Config, Transition, VocabEntry } from '../types';
import { BrowseRail } from './browse/BrowseRail';
import type { ConditionChip, KindOption, SuggestionItem } from './browse/BrowseRail';
import { Resizer } from './layout/Resizer';
import { RAIL_WIDTH } from './layout/resizableWidths';
import type { FilterCondition } from './browse/filters';
import { encodeFilters, decodeFilters } from './browse/filters';
import type { SearchStateChange } from './browse/BrowseView';
import {
  buildCategoryKindIndex,
  buildTransitionVocabIndex,
  loadCollapsed,
  loadIndexMode,
  saveCollapsed,
  saveIndexMode,
  type VocabIndexMode,
} from './browse/indexTree';
import { VocabCard } from './browse/VocabCard';
import { CommentButton } from './comments/CommentButton';
import { kindColor, OWNER_COLOR } from './shared/Chip';
import { useScrollRestore } from '../scrollRestore';

interface Props {
  /** Per-view sessionStorage key for scroll continuity (always 'vocab'). */
  scrollKey: string;
  onSelectTx: (id: string) => void;
  /** Vocab entry to scroll to on mount (router's #/vocab/<id>) — used by the
      comment panel's "位置へ移動" on vocab comments. */
  initialFocusId?: string;
  /** Vocab's browse state as reflected in the URL (view-state-continuity), so
      往復/reload/バック restore it the same way BrowseView's tags/specs do.
      categoryFacet rides the shared `k` param, subject the vocab-only `s`,
      filters the shared `f` codec (tag/owner conditions). */
  searchQuery: string;
  searchCategoryFacet: string;
  /** undefined = no `f` param (filters default to []); a string decodes to the
      active tag/owner conditions. */
  searchFiltersEncoded: string | undefined;
  searchSubject: string;
  /** Fired (debounced) on local query/facet/filters/subject change so the
      caller mirrors it into the URL (shared with BrowseView via App). */
  onSearchChange: (state: SearchStateChange) => void;
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
export function VocabView({
  scrollKey,
  onSelectTx,
  initialFocusId,
  searchQuery,
  searchCategoryFacet,
  searchFiltersEncoded,
  searchSubject,
  onSearchChange,
}: Props) {
  const t = useT();
  // tagKindLabel: facet チップ/サジェストの種別ラベルを選択タグの kind から解決
  // （②・combobox の facet kind を反映。単一の「コンポーネント」ハードコードを廃す）。
  const { tagById, tagKindLabel } = useLookups();
  const { closeDrawer } = useDrawer();
  const [vocab, setVocab] = useState<VocabEntry[] | null>(null);
  const [transitions, setTransitions] = useState<Transition[]>([]);
  // category→kind ツリーの kind 順（依頼H・vocab-view-p1）は config.kinds に
  // 拠るので、facets（タグフォレスト）の代わりに config を取得する。
  const [config, setConfig] = useState<Config | null>(null);
  const [error, setError] = useState<string | null>(null);
  // 見出しフォルダ（category→kind ツリー）の折りたたみ状態 — vocab 専用の
  // per-facet localStorage キーで復元（tags/specs とは独立）。
  const [collapsedIds, setCollapsedIds] = useState<Set<string>>(() => loadCollapsed('vocab'));
  // 索引の表示モード（vocab-tree-mode）: 'category-kind'=モードA（既存・維持）、
  // 'transition'=モードB（消費 transition 文脈）。選択は localStorage 永続で
  // 再訪時に保つ（既存の collapse 永続と同系）。
  const [indexMode, setIndexMode] = useState<VocabIndexMode>(() => loadIndexMode());

  // Seeded from the URL (view-state-continuity) so a reload / deep link lands
  // with this view's search already applied — see the sync effects below.
  const [query, setQuery] = useState(() => searchQuery || '');
  const [categoryFacet, setCategoryFacet] = useState(() => searchCategoryFacet || 'all');
  // コンポ別モード（vocab-view-p2）: '' = グローバル（全語彙・Phase 1）、subject
  // タグ id = その subject に属す遷移が参照する導出語彙。導出は Go 側
  // （GET /api/vocab?subject=…）に委ね、ここは差し替えた集合を Phase 1 と同じ
  // buildCategoryKindIndex で描くだけ。null は導出結果の読み込み中。
  const [subject, setSubject] = useState(() => searchSubject || '');
  const [subjectVocab, setSubjectVocab] = useState<VocabEntry[] | null>(null);
  // モードB（vocab-tree-mode）の scope transitions: subject が選ばれたら、その
  // subject の実効タグを持つ遷移（VocabBySubject と同じ per-component 導出の
  // transition 側・server が実効タグ＝祖先ロールアップで判定）。'' はグローバルで
  // 全遷移（下の transitions）を使うので null。モードB のツリー構築のみが使う。
  const [subjectTransitions, setSubjectTransitions] = useState<Transition[] | null>(null);
  // Generalized from a bare tag-id array (vocab-owner-tag) so owner can join
  // tag as a second, AND-composed condition kind — same FilterCondition
  // shape BrowseView.tsx uses for its own facets, but this page still keeps
  // its own local filter state/matching (see the class comment below for
  // why Vocab doesn't just become a third BrowseView facet).
  const [filters, setFilters] = useState<FilterCondition[]>(() =>
    searchFiltersEncoded !== undefined ? decodeFilters(searchFiltersEncoded) : [],
  );

  const cardRefs = useRef<Map<string, HTMLElement>>(new Map());
  const scrollTarget = useRef<string | null>(initialFocusId || null);

  // Content-aware setFilters: bail out (keep the same array reference) when the
  // decoded value already matches, so the adopt effect below doesn't hand React
  // a fresh-but-equal array every run (same guard BrowseView uses).
  const setFiltersIfChanged = (next: FilterCondition[]) =>
    setFilters((prev) => (encodeFilters(prev) === encodeFilters(next) ? prev : next));

  // Adopt search pushed in from *outside* this component (Back/Forward →
  // hashchange → new props). Runs on mount too, but the useState seeds above
  // already match, so it's a no-op there.
  useEffect(() => {
    setQuery(searchQuery || '');
    setCategoryFacet(searchCategoryFacet || 'all');
    setSubject(searchSubject || '');
    setFiltersIfChanged(searchFiltersEncoded !== undefined ? decodeFilters(searchFiltersEncoded) : []);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchQuery, searchCategoryFacet, searchSubject, searchFiltersEncoded]);

  // Push local query/facet/filters/subject back out to the URL, but only when
  // local state genuinely diverges from what the URL already encodes (echo /
  // seed guard) — mirrors BrowseView's url-sync push. Deps are LOCAL state
  // only; the URL props are read in-body so an external navigation doesn't
  // schedule a spurious push of stale local state.
  useEffect(() => {
    const urlFiltersEncoded = searchFiltersEncoded !== undefined ? searchFiltersEncoded : '';
    const localFiltersEncoded = encodeFilters(filters);
    if (
      query === (searchQuery || '') &&
      categoryFacet === (searchCategoryFacet || 'all') &&
      subject === (searchSubject || '') &&
      localFiltersEncoded === urlFiltersEncoded
    ) {
      return;
    }
    // Empty filters carry no `f` param (clean URL); vocab has no focus-tag
    // default to preserve, so '' always means "no filters".
    const nextFiltersEncoded = localFiltersEncoded === '' ? undefined : localFiltersEncoded;
    const id = setTimeout(() => {
      onSearchChange({ query, kindFacet: categoryFacet, filtersEncoded: nextFiltersEncoded, subject });
    }, 350);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, categoryFacet, filters, subject]);

  // Re-arm the scroll target if a comment's "位置へ移動" jumps here while
  // VocabView is already mounted (same pattern as BrowseView's per-facet
  // reset effect for initialFocusTagId/initialFocusTxId).
  useEffect(() => {
    scrollTarget.current = initialFocusId || null;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialFocusId]);

  useEffect(() => {
    Promise.all([api.getVocab(), api.getTransitions({}), api.getConfig()])
      .then(([v, tx, c]) => {
        setVocab(v);
        setTransitions(tx.transitions || []);
        setConfig(c);
      })
      .catch((err) => setError(String(err)));
  }, []);

  // コンポ別モード: subject が選ばれたらその導出語彙を取得する。'' に戻したら
  // 破棄してグローバルへ。競合する応答は cancelled ガードで捨てる。
  useEffect(() => {
    if (!subject) {
      setSubjectVocab(null);
      return;
    }
    let cancelled = false;
    setSubjectVocab(null);
    api
      .getVocab({ subject })
      .then((v) => {
        if (!cancelled) setSubjectVocab(v);
      })
      .catch((err) => {
        if (!cancelled) setError(String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [subject]);

  // モードB の scope transitions を subject に追従して取得（vocab-tree-mode）。
  // subjectVocab と別 effect にして既存の導出経路に触れない（追加レンズ）。'' は
  // グローバル（全 transitions を使う）なので破棄。競合応答は cancelled で捨てる。
  useEffect(() => {
    if (!subject) {
      setSubjectTransitions(null);
      return;
    }
    let cancelled = false;
    setSubjectTransitions(null);
    api
      .getTransitions({ tag: subject })
      .then((res) => {
        if (!cancelled) setSubjectTransitions(res.transitions || []);
      })
      .catch((err) => {
        if (!cancelled) setError(String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [subject]);

  useEffect(() => {
    const id = scrollTarget.current;
    if (!id || !vocab) return;
    const el = cardRefs.current.get(id);
    if (el) {
      el.scrollIntoView({ block: 'start' });
      scrollTarget.current = null;
    }
  });

  // Restore this view's saved scroll once its cards are loaded — unless a
  // focused entry (#/vocab/<id> jump) will scrollIntoView instead
  // (view-state-continuity). Ready = the active list resolved (subject mode
  // waits for its derived vocab; global mode just needs the base list).
  const contentReady = subject ? !!subjectVocab : !!vocab;
  useScrollRestore(scrollKey, contentReady, !!initialFocusId);

  // Same close-on-select rule as BrowseView.tsx: addFilter/scroll-to close
  // the narrow-viewport drawer, kindFacet/removeFilter/query don't.
  const addFilter = (f: FilterCondition) => {
    setFilters((prev) => (prev.some((p) => p.type === f.type && p.id === f.id) ? prev : [...prev, f]));
    closeDrawer();
  };
  const removeFilter = (i: number) => setFilters((prev) => prev.filter((_, idx) => idx !== i));
  // 見出しフォルダの開閉（依頼C-1）— ドロワーは閉じない（レールから畳むので
    // 閉じると自己矛盾。removeFilter/kindFacet と同じ理由）。
  const toggleCollapse = (key: string) => {
    setCollapsedIds((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      saveCollapsed('vocab', next);
      return next;
    });
  };
  // 索引モード切替（vocab-tree-mode）— 選択を localStorage 永続。ドロワーは
  // 閉じない（索引の見せ方を変えるだけで絞り込み選択ではない・toggleCollapse と同じ）。
  const changeIndexMode = (mode: string) => {
    const m: VocabIndexMode = mode === 'transition' ? 'transition' : 'category-kind';
    setIndexMode(m);
    saveIndexMode(m);
  };

  if (error) return <div class="browse-view error">{error}</div>;
  if (!vocab) return <div class="browse-view dim">{t.vocab.loading}</div>;

  // 表示対象の語彙集合: コンポ別モードでは導出語彙、グローバルでは全語彙。以降の
  // 一覧/索引/フィルタ候補/件数はすべてこの activeVocab を基準に組む（Phase 1 の
  // 描画機構はそのまま・集合だけ差し替える）。導出結果を待つ間は loading。
  const activeVocab = subject ? subjectVocab : vocab;
  if (!activeVocab) return <div class="browse-view dim">{t.vocab.loading}</div>;

  // コンポ別モード（combobox-unify）の軸は config.facetKinds のタグ。導出
  // （VocabBySubject）は任意タグで効くが、意味のある「コンポーネント」は Browse の
  // facet 軸に一致する（このプロジェクトでは subject/requirement/concern）。kind を
  // ハードコードせず facetKinds に委ねると、subject kind が無いプロジェクトでも
  // コンポ相当タグを選べる。この集合はコンボボックスのコンポーネント候補と、
  // タグ候補からの除外（二重表示防止）の両方に使う。
  const facetKinds = config?.facetKinds ?? [];
  const facetKindSet = new Set(facetKinds);
  const isFacetTag = (kind: string | undefined): boolean => !!kind && facetKindSet.has(kind);
  const subjectName = subject ? tagById.get(subject)?.name || subject : '';

  // Single predicate for "does this vocab entry satisfy this one condition"
  // — shared by `visible` (the actual list filter) and `wouldMatchAny`
  // below (the combobox's "would this candidate leave ≥1 result" check).
  // Deliberately the ONLY place this membership rule is written: a second,
  // simplified copy for candidate-narrowing was the root cause of a prior
  // bug (vocab-owner-tag rework) — "shows as a candidate" and "actually
  // narrows something" must always agree.
  const matchesFilter = (v: VocabEntry, f: FilterCondition): boolean =>
    f.type === 'tag' ? (v.tags || []).includes(f.id) : v.owner === f.id;

  const q = query.trim().toLowerCase();
  const kindOptions: KindOption[] = CATEGORIES.map((c) => ({
    key: c,
    label: t.vocab.categoryLabel(c),
    count: activeVocab.filter((v) => v.category === c).length,
  }));

  const visible = activeVocab
    .filter((v) => categoryFacet === 'all' || v.category === categoryFacet)
    .filter((v) => !q || (v.id + ' ' + v.label + ' ' + (v.description || '')).toLowerCase().includes(q))
    .filter((v) => filters.every((f) => matchesFilter(v, f)))
    .sort((a, b) => a.id.localeCompare(b.id));

  const scrollToCard = (id: string) => {
    scrollTarget.current = id;
    cardRefs.current.get(id)?.scrollIntoView({ block: 'start' });
    closeDrawer();
  };
  // 索引を category→kind ツリーに（依頼H・vocab-view-p1）: vocab が必ず持つ
  // intrinsic な軸で分類するので、タグ未付与でも未分類にフラットに落ちない。
  // タグは二次的な横断フィルタとして残る（filters/matchesFilter は無改修）。
  const kindOrder = (category: string): string[] => (config?.kinds as Record<string, string[]> | undefined)?.[category] || [];
  // モードB（vocab-tree-mode）の scope transitions: グローバルは全 transitions、
  // subject 選択時はその subject 導出の transitions。subject 選択直後で導出が
  // まだ届いていない間（subjectTransitions===null）は「全 vocab が未使用」に見える
  // 誤表示を避けるため index を空にする（vocab 一覧側は subjectVocab で既に表示）。
  const scopeTransitions = subject ? subjectTransitions : transitions;
  const modeBReady = !subject || subjectTransitions !== null;
  // モードB のノード名＝その transition の action vocab の label（WHEN 句）。全
  // vocab（scope に依らず読み込み済の base list）から引くので subject モードでも
  // 解決できる。未解決（稀）は undefined → buildTransitionVocabIndex が id へ落とす。
  const vocabLabelById = new Map(vocab.map((v) => [v.id, v.label]));
  const indexItems =
    indexMode === 'transition'
      ? buildTransitionVocabIndex({
          transitions:
            modeBReady && scopeTransitions
              ? // ノード=action(きっかけ) なので leaf の refs からは action を落とし、
                // 前提(given)＋結果(then) のみを leaf 化する（①）。
                scopeTransitions.map((tx) => ({ id: tx.id, label: vocabLabelById.get(tx.action), refs: [...tx.given, ...tx.then] }))
              : [],
          // 母集合＝可視 vocab（mode A の leaves と同源）。検索/カテゴリ/タグの
          // 絞り込みがそのまま索引に効く。leaf の色は役割（きっかけ/前提/結果）＝
          // vocab.category の色。
          vocabById: modeBReady
            ? new Map(visible.map((v) => [v.id, { id: v.id, label: v.label, color: kindColor(v.category) }]))
            : new Map(),
          unusedLabel: t.vocab.unusedBucket,
          transitionColor: 'var(--lm-text-muted)',
          unusedColor: 'var(--lm-text-dim)',
          collapsedIds,
          onToggle: toggleCollapse,
          onSelect: scrollToCard,
        })
      : buildCategoryKindIndex({
          leaves: visible.map((v) => ({
            id: v.id,
            label: v.label,
            color: kindColor(v.category),
            category: v.category,
            kind: v.kind,
          })),
          categories: CATEGORIES,
          categoryLabel: t.vocab.categoryLabel,
          kindOrder,
          otherKindLabel: t.vocab.otherKind,
          folderColor: (category) => kindColor(category),
          collapsedIds,
          onToggle: toggleCollapse,
          onSelect: scrollToCard,
        });

  // アクティブなコンポーネント（subject モード）を、除去可能チップとして conditions
  // 行の先頭に置く（combobox-unify）。廃止した <select> の「全体」に代わる復帰導線＝
  // その × で setSubject('') によりグローバルへ戻る。`prefix` で tag/owner の AND
  // フィルタチップと視覚的に区別する。onClearConditions（全消し）は filters だけを
  // クリアし subject は残す（コンポーネントは別軸なので個別に外す設計）。
  const conditions: ConditionChip[] = [
    ...(subject
      ? [
          {
            label: subjectName,
            // 接頭ラベル＝選択タグの kind（tagKindLabels）。facetKinds は
            // subject/requirement/concern なので、要件/関心 facet なら「要件」「関心」と
            // 出す（②・単一「コンポーネント」ハードコードを廃す）。未設定 kind は素の id。
            prefix: tagKindLabel(tagById.get(subject)?.kind),
            color: kindColor(tagById.get(subject)?.kind),
            onRemove: () => setSubject(''),
          },
        ]
      : []),
    ...filters.map((f, i) => {
      if (f.type === 'tag') {
        const tag = tagById.get(f.id);
        return { label: tag?.name || f.id, color: kindColor(tag?.kind), onRemove: () => removeFilter(i) };
      }
      return { label: f.id, color: OWNER_COLOR, onRemove: () => removeFilter(i) };
    }),
  ];

  // Combobox candidates (2026-07-11 tweaks3 §3): every known tag, plus
  // (vocab-owner-tag) every distinct owner value, narrowed to whichever
  // would actually leave ≥1 result if added — same "AND-narrow, only offer
  // what helps" UX as BrowseView.tsx's facet combobox (tags/specs).
  //
  // History (vocab-owner-tag → rework): this box originally (tweaks4,
  // 2026-07-11) narrowed candidates with a *shortcut* predicate —
  // `(v.tags || []).includes(tagId)` computed only over vocab.tags — which
  // is almost always empty in practice (tagging normally happens at the
  // transition/spec level, not per vocab word), so nearly every real tag
  // failed the check and the box looked broken. The first fix over-
  // corrected by dropping narrowing entirely. The actual defect wasn't
  // "narrowing is wrong" — it was "the predicate used for narrowing didn't
  // match the predicate used for filtering". `wouldMatchAny` below reuses
  // `matchesFilter` (the exact same rule `visible` above filters with), so
  // "shown as a candidate" and "actually narrows the list" can never
  // diverge again.
  const wouldMatchAny = (candidate: FilterCondition): boolean => {
    const testFilters = [...filters, candidate];
    return activeVocab.some((v) => testFilters.every((f) => matchesFilter(v, f)));
  };
  const ownerValues = Array.from(new Set(activeVocab.map((v) => v.owner).filter((o): o is string => !!o)));
  // コンボボックスのサジェストを3系統に統合（combobox-unify）。廃止した <select> の
  // コンポ別モード切替を、この1入力のコンポーネント候補に吸収する。並びは
  // コンポーネント→タグ→owner（打った文字は BrowseRail が (id+' '+label) で部分一致）。
  const suggestions: SuggestionItem[] = [
    // (1) コンポーネント（subject 軸）: facetKinds のタグ。wouldMatchAny gate は
    // 通さない — コンポーネントタグは transition 側に付き vocab.tags には無いのが
    // 正常で、gate すると全部消える。選択は AND フィルタではなく setSubject の
    // モード切替。現在 active な subject は除外し、他コンポは切替候補として残す。
    ...Array.from(tagById.values())
      .filter((tag) => isFacetTag(tag.kind) && tag.id !== subject)
      .sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id))
      // 種別バッジ＝その facet タグの kind ラベル（tagKindLabels）。requirement/
      // concern の facet タグにも一律「コンポーネント」と出さない（②）。
      .map((tag) => ({ id: tag.id, label: tag.name || tag.id, color: kindColor(tag.kind), kindLabel: tagKindLabel(tag.kind), onSelect: () => setSubject(tag.id) })),
    // (2) タグ（二次フィルタ）: facetKinds 以外のタグで wouldMatchAny を通るもの。
    // facetKinds 分は (1) に回すので排他になり二重表示しない。選択は AND フィルタ。
    ...Array.from(tagById.values())
      .filter((tag) => !isFacetTag(tag.kind) && !filters.some((f) => f.type === 'tag' && f.id === tag.id) && wouldMatchAny({ type: 'tag', id: tag.id }))
      .map((tag) => ({ id: tag.id, label: tag.name || tag.id, color: kindColor(tag.kind), kindLabel: t.nav.tags, onSelect: () => addFilter({ type: 'tag', id: tag.id }) })),
    // (3) owner: 現状どおり（wouldMatchAny gate・AND フィルタ）。
    ...ownerValues
      .filter((o) => !filters.some((f) => f.type === 'owner' && f.id === o) && wouldMatchAny({ type: 'owner', id: o }))
      .map((o) => ({ id: o, label: o, color: OWNER_COLOR, kindLabel: t.vocab.owner, onSelect: () => addFilter({ type: 'owner', id: o }) })),
  ];

  return (
    <div class="browse-view">
      <BrowseRail
        query={query}
        onQueryChange={setQuery}
        kindFacet={categoryFacet}
        kindOptions={kindOptions}
        onKindFacetChange={setCategoryFacet}
        conditions={conditions}
        onClearConditions={() => setFilters([])}
        indexItems={indexItems}
        suggestions={suggestions}
        indexModes={[
          { key: 'category-kind', label: t.vocab.treeModeCategory },
          { key: 'transition', label: t.vocab.treeModeTransition },
        ]}
        indexMode={indexMode}
        onIndexModeChange={changeIndexMode}
      />
      {/* drawer-resize: 左レールの横幅リサイズ。tag/spec の BrowseView と同じ
          Resizer/RAIL_WIDTH（CSS var・localStorage・clamp・narrow 非表示）を
          共有し、語彙ページだけ配線漏れだったのを是正して一様に効かせる。 */}
      <Resizer config={RAIL_WIDTH} direction="rail" className="pmem-resizer--rail" />
      <main class="browse-main">
        <div class="browse-main-head">
          <h1>
            {t.vocab.heading}
            <CommentButton recordType="page" recordId="vocab" recordTitle={t.vocab.heading} anchor="page" anchorLabel={t.comments.pageAnchorLabel} />
          </h1>
          <span class="dim">{t.vocab.intro}</span>
        </div>
        <div class="browse-card-list">
          {visible.length === 0 ? (
            <div class="card-empty">{subject ? t.vocab.subjectEmpty(subjectName) : t.vocab.empty}</div>
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
                onFilterTag={(id) => addFilter({ type: 'tag', id })}
                onFilterOwner={(owner) => addFilter({ type: 'owner', id: owner })}
                onSelectTx={onSelectTx}
              />
            ))
          )}
        </div>
      </main>
    </div>
  );
}
