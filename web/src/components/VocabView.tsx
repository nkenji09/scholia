import { useEffect, useRef, useState } from 'preact/hooks';
import { api } from '../api';
import { useLookups } from '../lookups';
import { useDrawer } from '../drawer';
import { useT } from '../i18n';
import type { Config, Transition, VocabEntry } from '../types';
import { BrowseRail } from './browse/BrowseRail';
import type { ConditionChip, KindOption, SuggestionItem } from './browse/BrowseRail';
import type { FilterCondition } from './browse/filters';
import { buildCategoryKindIndex, loadCollapsed, saveCollapsed } from './browse/indexTree';
import { VocabCard } from './browse/VocabCard';
import { CommentButton } from './comments/CommentButton';
import { kindColor, OWNER_COLOR } from './shared/Chip';

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
  const t = useT();
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

  const [query, setQuery] = useState('');
  const [categoryFacet, setCategoryFacet] = useState('all');
  // コンポ別モード（vocab-view-p2）: '' = グローバル（全語彙・Phase 1）、subject
  // タグ id = その subject に属す遷移が参照する導出語彙。導出は Go 側
  // （GET /api/vocab?subject=…）に委ね、ここは差し替えた集合を Phase 1 と同じ
  // buildCategoryKindIndex で描くだけ。null は導出結果の読み込み中。
  const [subject, setSubject] = useState('');
  const [subjectVocab, setSubjectVocab] = useState<VocabEntry[] | null>(null);
  // Generalized from a bare tag-id array (vocab-owner-tag) so owner can join
  // tag as a second, AND-composed condition kind — same FilterCondition
  // shape BrowseView.tsx uses for its own facets, but this page still keeps
  // its own local filter state/matching (see the class comment below for
  // why Vocab doesn't just become a third BrowseView facet).
  const [filters, setFilters] = useState<FilterCondition[]>([]);

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

  if (error) return <div class="browse-view error">{error}</div>;
  if (!vocab) return <div class="browse-view dim">{t.vocab.loading}</div>;

  // 表示対象の語彙集合: コンポ別モードでは導出語彙、グローバルでは全語彙。以降の
  // 一覧/索引/フィルタ候補/件数はすべてこの activeVocab を基準に組む（Phase 1 の
  // 描画機構はそのまま・集合だけ差し替える）。導出結果を待つ間は loading。
  const activeVocab = subject ? subjectVocab : vocab;
  if (!activeVocab) return <div class="browse-view dim">{t.vocab.loading}</div>;

  // コンポ別モードの selector 候補は「コンポ軸」のタグ。導出（VocabBySubject）は
  // 任意タグで効くが、意味のある軸は config.facetKinds（＝Browse の facet 軸・
  // 既定 pmem では subject を含む／このプロジェクトでは requirement・concept が
  // コンポを表す）。kind をハードコードせず facetKinds に委ねると、subject kind が
  // 無いプロジェクトでも component 相当タグを選べる。facetKinds の宣言順に
  // optgroup へまとめ、各群内は名前順。
  const facetKinds = config?.facetKinds ?? [];
  const subjectGroups = facetKinds
    .map((kind) => ({
      label: tagKindLabel(kind),
      options: Array.from(tagById.values())
        .filter((tag) => tag.kind === kind)
        .sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id))
        .map((tag) => ({ id: tag.id, label: tag.name || tag.id })),
    }))
    .filter((g) => g.options.length > 0);
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
  const indexItems = buildCategoryKindIndex({
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

  const conditions: ConditionChip[] = filters.map((f, i) => {
    if (f.type === 'tag') {
      const tag = tagById.get(f.id);
      return { label: tag?.name || f.id, color: kindColor(tag?.kind), onRemove: () => removeFilter(i) };
    }
    return { label: f.id, color: OWNER_COLOR, onRemove: () => removeFilter(i) };
  });

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
  const suggestions: SuggestionItem[] = [
    ...Array.from(tagById.values())
      .filter((tag) => !filters.some((f) => f.type === 'tag' && f.id === tag.id) && wouldMatchAny({ type: 'tag', id: tag.id }))
      .map((tag) => ({ id: tag.id, label: tag.name || tag.id, color: kindColor(tag.kind), kindLabel: t.nav.tags, onSelect: () => addFilter({ type: 'tag', id: tag.id }) })),
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
        subjectSelect={
          subjectGroups.length > 0
            ? {
                label: t.vocab.subjectLabel,
                allLabel: t.vocab.subjectAll,
                value: subject,
                groups: subjectGroups,
                // モード切替は kindFacet と同類（レール内で見る対象を変えるだけ）
                // なのでドロワーは閉じない。切替時にスクロール目標は持ち越さない。
                onChange: setSubject,
              }
            : undefined
        }
      />
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
