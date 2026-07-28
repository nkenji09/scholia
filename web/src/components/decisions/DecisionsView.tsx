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
import { governsParams, needsGoverns, parseScopeDirection, parseScopeTarget } from './decisionScope';
import { namesOneDecision, sameConditions, selectBase, selectDecisions } from './decisionFilter';
import type { DecisionConditions, CurrencyFilter, PeriodFilter, TargetKindFilter } from './decisionFilter';

const COLLAPSE_FACET = 'decisions';

// 条件の型・URL との相互変換・照合は decisionFilter が1箇所で持つ（純関数）。
// ここは widget と描画だけを担う——判断を画面の中に散らすと、値として検査できる
// 形が壊れる（CLAUDE.md「配線ガードの書き方」1）。

interface Props {
  /** URL から起こした条件の全体。**1つの prop にまとめてある**——1つずつ渡す形は
      「この prop だけ握り潰す」変異の口をそのぶん増やす（差し戻し1回目で実際に
      `on` / `scope` を潰す変異が緑のまま素通りした）。 */
  conditions: DecisionConditions;
  onConditionsChange: (c: DecisionConditions) => void;
  onOpenDecision: (id: string) => void;
}

// 効力バッジは2値（01KYHW54B8ZXH0NEPH2J7N1X39 条項1）。記録の3値
// （supersede/amend/exception）は不変で、変えるのは画面の状態列だけ。
// かつてここは3値をそのまま出していて、amend を付けられただけの——**まだ
// 効いている**——decision が「改訂」として現行と別の状態に見え、履歴側だと
// 誤読された。付帯情報（後続に部分改訂・例外がある）は状態列ではなく行の
// 補助情報として出す（条項2）。バッジそのものは行（DecisionRowFull）が描く。

export function DecisionsView({ conditions, onConditionsChange, onOpenDecision }: Props) {
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

  // URL が正で、ここはその写し。widget の操作は即座に一覧へ効き、URL へは
  // debounce して書き戻す（combobox の「選ぶ→検索語を消す」の対が2回の navigate に
  // 割れて競合しないように）。
  const [local, setLocal] = useState<DecisionConditions>(() => conditions);
  const patch = (p: Partial<DecisionConditions>) => setLocal((prev) => ({ ...prev, ...p }));

  // 外（Back/Forward → hashchange → 新しい props）から来た状態を取り込む。
  // マウント時にも走るが、種が既に一致しているので no-op。
  useEffect(() => {
    setLocal(conditions);
  }, [conditions]);

  // 書き戻し。URL が既に表しているものと本当に食い違うときだけ走らせる
  // （自分が押した分の戻り足とマウントの種は、比較で自然に no-op になる）。
  useEffect(() => {
    if (sameConditions(local, conditions)) return;
    const id = setTimeout(() => onConditionsChange(local), 300);
    return () => clearTimeout(id);
    // 依存は**ローカル状態だけ**（URL 側は本文で読む）。外からの遷移で、古い
    // ローカル状態の書き戻しを予約してしまわないため。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [local]);

  // 「どの対象か」「どの向きか」の解釈は decisionScope が1箇所で持つ。
  const scopeTarget = useMemo(() => parseScopeTarget(local.on || undefined), [local.on]);
  const scopeDirection = useMemo(() => parseScopeDirection(local.scope || undefined), [local.scope]);

  // `governing` の判定は viewer 側に置かず、GET /api/governs（CLI `scholia rules`
  // と同じ Go パッケージ `internal/index` の GovernsFor*）へ委ねる。同じ選択規則を
  // 2箇所に書くと「この記録を支配する規則は何か」に面ごとに違う答えが返る余地が
  // 復活する（01KXYED61J6QBEX75H2XHVHW7Y の診断・追補 01KYJV3FYMDFRWQ939NBV2BPAC
  // が名指しで警告した形）。静的書き出しでも同じ答えが返る（api.getGoverns）。
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
    patch({ tags: local.tags.includes(id) ? local.tags : [...local.tags, id] });
    // Close the narrow-viewport drawer on select (same rule as BrowseView/
    // VocabView: picking a filter narrows the list, so the drawer's job is
    // done — adjusting the native selects / removing a chip doesn't close).
    closeDrawer();
  };
  const removeTag = (id: string) => patch({ tags: local.tags.filter((x) => x !== id) });

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

  const q = local.query.trim().toLowerCase();

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

  const now = Date.now();

  // 照合と並びは decisionFilter（純関数）が持つ。ここは索引と関連度の材料を渡すだけ
  // ——「呼び出しは残して適用の一行だけ削る」型の変異を、値のテストで落とせる形に
  // するため（CLAUDE.md「配線ガードの書き方」1・差し戻し1回目 R4）。
  const selectCtx = useMemo(
    () => ({
      effectOf: (id: string) => effectOf(id, currencyIndex),
      effTagsById,
      governs: governs ?? undefined,
      now,
      tierOf: (d: Decision) => decisionTier(d),
    }),
    // decisionTier は毎レンダー作り直される素の関数なので依存に置かない
    // （置くと毎レンダー ctx が変わり、下の useMemo が意味を失う）。
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [currencyIndex, effTagsById, governs, now, local.query],
  );

  // 候補（combobox）の母数。フリーワードとタグ AND を掛けない段階。
  const filterBase = useMemo(() => selectBase(decisions || [], local, selectCtx), [decisions, local, selectCtx]);
  const filtered = useMemo(() => selectDecisions(decisions || [], local, selectCtx), [decisions, local, selectCtx]);
  const isNamedOne = namesOneDecision(local);

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
  const clearScope = () => patch({ on: '', scope: '' });

  // AND condition chips — 対象/向きのチップを先頭に置き、タグ AND のチップが続く。
  // どれも × で外せる＝**戻るを押さずに条件を緩められる**。これが「1件に絞った
  // 一覧」が単票より良い理由そのものなので、外せない形にしない。
  //
  // ⚠️ **名指しの1件のあいだは、掛かっていない条件を名乗らない。** 名指し中は他の
  // 条件を適用しない（namesOneDecision）ので、タグ AND のチップを出したままにすると
  // 「絞っているのに結果が変わらない」チップが並ぶ。名乗りと中身を一致させるのが
  // この decision の主旨なので、その間は対象のチップだけを出す。
  const conditionChips: ConditionChip[] = isNamedOne
    ? scopeTarget
      ? [{ label: scopeChipLabel(), color: kindColor(undefined), onRemove: clearScope }]
      : []
    : [
        ...(scopeTarget
          ? [{ label: scopeChipLabel(), color: kindColor(scopeTarget.type === 'tag' ? tagById.get(scopeTarget.id)?.kind : undefined), onRemove: clearScope }]
          : []),
        ...local.tags.map((id) => {
          const tg = tagById.get(id);
          return { label: tg?.name || id, color: kindColor(tg?.kind), onRemove: () => removeTag(id) };
        }),
      ];

  // Combobox candidates: every tag that is an effective tag of some decision
  // still passing the other filters, minus the already-selected ones, minus
  // any that would leave zero results if added (same "AND-narrow, only offer
  // what helps" rule as BrowseView/VocabView).
  const selectedSet = new Set(local.tags);
  const corpusTagIds = new Set<string>();
  for (const d of filterBase) for (const id of effTagsById.get(d.id) || []) corpusTagIds.add(id);
  const wouldMatchAny = (candidate: string): boolean =>
    filterBase.some((d) => {
      const eff = effTagsById.get(d.id);
      return !!eff && eff.has(candidate) && local.tags.every((tg) => eff.has(tg));
    });
  const suggestions: SuggestionItem[] = (isNamedOne ? [] : Array.from(corpusTagIds))
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
  //
  // 名指しの1件のあいだは、これらの widget を出さない。効力の select が「現行」を
  // 表示したまま置き換え済みの行が出る、という**画面が嘘をつく**状態になるため
  // （名指し中は効力・種別・期間のいずれも適用していない）。代わりに、いま何が
  // 起きているかと抜け方（対象のチップを外す）を述べる。
  const extraControls = isNamedOne ? (
    <div class="decisions-rail-filters">
      <p class="decisions-named-note dim">{t.decisions.namedOneNote}</p>
    </div>
  ) : (
    <div class="decisions-rail-filters">
      <label class="decisions-filter">
        <span class="decisions-filter-label dim">{t.decisions.filterTargetKind}</span>
        <select value={local.targetKind} onChange={(e) => patch({ targetKind: (e.target as HTMLSelectElement).value as TargetKindFilter })}>
          <option value="all">{t.decisions.filterAll}</option>
          <option value="transition">{t.decisions.targetKindTransition}</option>
          <option value="tag">{t.decisions.targetKindTag}</option>
          <option value="vocab">{t.decisions.targetKindVocab}</option>
        </select>
      </label>
      <label class="decisions-filter">
        <span class="decisions-filter-label dim">{t.decisions.filterCurrency}</span>
        <select value={local.currency} onChange={(e) => patch({ currency: (e.target as HTMLSelectElement).value as CurrencyFilter })}>
          <option value="all">{t.decisions.filterAll}</option>
          <option value="current">{t.decisions.effectInForce}</option>
          <option value="superseded">{t.decisions.effectReplaced}</option>
        </select>
      </label>
      <label class="decisions-filter">
        <span class="decisions-filter-label dim">{t.decisions.filterPeriod}</span>
        <select value={local.period} onChange={(e) => patch({ period: (e.target as HTMLSelectElement).value as PeriodFilter })}>
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
        query={local.query}
        onQueryChange={(v) => patch({ query: v })}
        kindFacet="all"
        kindOptions={[]}
        onKindFacetChange={() => {}}
        conditions={conditionChips}
        onClearConditions={() => patch({ tags: [], on: '', scope: '' })}
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
            /* 名指しの1件が0件＝**その記録が無い**。ここで「条件に一致しません」と
               出すと、利用者は絞り込みを緩めれば出ると思って外しにいく（が出ない）
               ——旧単票が名指ししていた事実を落とさない。 */
            <p class="dim decisions-empty">{isNamedOne ? t.decisions.notFound : t.decisions.noMatch}</p>
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
