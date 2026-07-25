import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import type { JSX } from 'preact';
import { api } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { useDrawer } from '../../drawer';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';
import { Markdown } from '../Markdown';
import { CommentButton } from '../comments/CommentButton';
import { HashLink } from '../shared/HashLink';
import { routeHash } from '../../router';
import { buildCurrencyIndex, currencyOf } from '../decisions/decisionModel';
import type { Config, Decision, Tag, TraceabilityResponse } from '../../types';

// 概要ビュー（viewer-overview-browser）: 左=構造ツリー、右=コンポーネント仕様
// シート。design（scholia-viewer.dc.html の treeVals()/sheetVals()）の構造・
// 状態を Preact へ翻訳しつつ、データは design のダミー（window.__SCHOLIA__）では
// なく実 api から取り、design が前提にした便宜フィールド（KIND メタ・tx.part・
// reqScope・decision.summary/current）を実データから導出して汎用化している
// （どの .scholia でも config.tagKinds と実データだけで破綻なく描ける）。

interface Props {
  /** タグ（コンポーネント/要件/制約）を ブラウザの詳細で開く（#/spec/<id>）。 */
  onOpenTag: (tagId: string) => void;
  /** 遷移（仕様）を ブラウザの詳細で開く（#/browse/tx/<id>）。 */
  onOpenTx: (txId: string) => void;
}

// kind → トークン色。design の KIND() の color を kind 名でマップ（tokens.css の
// --k-*）。未知 kind は --lm-text-dim にフォールバック（ハードコード列挙に頼り
// 切らない・§汎用性）。
const KIND_COLOR: Record<string, string> = {
  group: 'var(--k-grp)',
  component: 'var(--k-cmp)',
  part: 'var(--k-prt)',
  requirement: 'var(--k-req)',
  property: 'var(--k-prop)',
  concept: 'var(--k-con)',
  axis: 'var(--k-axis)',
};
const KIND_ICON: Record<string, IconName> = {
  group: 'folder-tree',
  component: 'component',
  part: 'puzzle',
  requirement: 'target',
  property: 'ban',
  concept: 'lightbulb',
  axis: 'git-fork',
};

function kindColorVar(kind: string | undefined): string {
  return (kind && KIND_COLOR[kind]) || 'var(--lm-text-dim)';
}
function kindIconName(kind: string | undefined): IconName {
  return (kind && KIND_ICON[kind]) || 'tags';
}

function cssEscape(s: string): string {
  const w = window as unknown as { CSS?: { escape?: (v: string) => string } };
  return w.CSS && w.CSS.escape ? w.CSS.escape(s) : s.replace(/["\\]/g, '\\$&');
}

// decision に summary フィールドは無い（実 decision は why のみ）。design の
// 1行 summary は why の1行目（最初の改行/句点まで）で代替し、全文は展開で見せる。
function summaryOf(text: string): string {
  const s = (text || '').trim();
  if (!s) return '';
  const nl = s.search(/\n/);
  const line = nl >= 0 ? s.slice(0, nl) : s;
  const p = line.search(/[。．.](\s|$)/);
  return (p >= 0 ? line.slice(0, p + 1) : line).trim();
}

// component の description（markdown）を lead（1行目）と body（責務本文）に割る。
// 1行しか無ければ lead は出さず、全文を責務として markdown 描画する。
function splitLead(desc: string | undefined): { lead: string; body: string } {
  const d = (desc || '').trim();
  if (!d) return { lead: '', body: '' };
  const nl = d.indexOf('\n');
  if (nl < 0) return { lead: '', body: d };
  return { lead: d.slice(0, nl).trim(), body: d.slice(nl + 1).trim() };
}

// 制約タグ判定。役割 kind（constraint・既定は property）に一致するか、または
// Tag.fulfillment==='property'（遷移で充足されない性質型要件）を併せて拾う。
// fulfillment は Tag のスキーマ値なので kind 一般化後もリテラル比較のまま。
function isConstraintTag(tag: Tag, constraintKind: string): boolean {
  return tag.kind === constraintKind || tag.fulfillment === 'property';
}

// 1 件の decision と、それがこの文脈に効く経路の via ラベル（空なら直接 target）。
type RuleEntry = { d: Decision; via: string };

interface TreeRow {
  key: string;
  tagId: string;
  depth: number;
  name: string;
  kind?: string;
  hasKids: boolean;
  open: boolean;
  isSelected: boolean;
  isGap: boolean;
  count: number | null;
  onToggle?: () => void;
  onClick: () => void;
}

export function OverviewView({ onOpenTag, onOpenTx }: Props) {
  const t = useT();
  const lookups = useLookups();
  const { vocabById, tagById, transitionById, tagName, vocabLabel, tagKindLabel, formatDecisionAt, roleKinds } = lookups;
  const { isNarrow, drawerOpen, closeDrawer } = useDrawer();
  // viewer-overview-browser: 概要ビューの役割 → 実 kind id（config.tagKinds の
  // behaviors 宣言で解決済み・無ければリテラル id フォールバック）。以降の
  // 「component/part/property/group」比較はすべてこの解決済み id を使う。
  const componentKind = roleKinds.component;
  const partKind = roleKinds.part;
  const constraintKind = roleKinds.constraint;
  const groupKind = roleKinds.group;

  const [config, setConfig] = useState<Config | null>(null);
  const [traceability, setTraceability] = useState<TraceabilityResponse | null>(null);
  const [decisions, setDecisions] = useState<Decision[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [sel, setSel] = useState<string | null>(null);
  const [treeOpen, setTreeOpen] = useState<Record<string, boolean>>({});
  const [openWhy, setOpenWhy] = useState<Record<string, boolean>>({});
  // 各コンテキスト（tx / part / constraint / component）ごとの「規則 (N)」展開状態。
  // 初期は全て畳む（キー未登録＝false）。part セクションの開閉も openParts で管理し、
  // 初期は全 part 畳む（④）。決定はレコード文脈にインラインで on-demand 展開する（⑤）。
  const [openRules, setOpenRules] = useState<Record<string, boolean>>({});
  const [openParts, setOpenParts] = useState<Record<string, boolean>>({});
  const scrollToPartRef = useRef<string | null>(null);
  const mainRef = useRef<HTMLElement>(null);

  useEffect(() => {
    // roots/traceabilityKinds/tagKinds は config、gap は traceability、decision
    // 群は rules から。tags/vocab/transitions は useLookups が既に一度ロード済み
    // なので再取得しない（同じ api.* を二重に叩かない）。
    Promise.all([api.getConfig(), api.getTraceability(), api.getRules({})])
      .then(([cfg, trace, rules]) => {
        setConfig(cfg);
        setTraceability(trace);
        setDecisions(rules.decisions);
      })
      .catch((e) => setError(String(e)));
  }, []);

  // design の prep() 相当。実データから effByTx（tx.tags ∪ vocab の tags ∪
  // 祖先展開）・satByTag・gapSet・decByTarget・currencyIndex を組む。
  const index = useMemo(() => {
    if (!config || !traceability || !decisions || !lookups.ready) return null;
    const tags = Array.from(tagById.values());

    const childrenByParent = new Map<string, Tag[]>();
    for (const tag of tags) {
      for (const p of tag.parentIds || []) {
        const arr = childrenByParent.get(p) || [];
        arr.push(tag);
        childrenByParent.set(p, arr);
      }
    }

    const ancCache = new Map<string, Set<string>>();
    const ancestorsOf = (id: string): Set<string> => {
      const cached = ancCache.get(id);
      if (cached) return cached;
      const out = new Set<string>();
      const visit = (cur: string) => {
        const tag = tagById.get(cur);
        if (!tag) return;
        for (const p of tag.parentIds || []) {
          if (!out.has(p)) {
            out.add(p);
            visit(p);
          }
        }
      };
      visit(id);
      ancCache.set(id, out);
      return out;
    };

    const effByTx = new Map<string, Set<string>>();
    for (const tx of transitionById.values()) {
      const raw = new Set<string>(tx.tags || []);
      for (const vid of [tx.action, ...(tx.given || []), ...(tx.then || [])]) {
        const v = vocabById.get(vid);
        if (v && v.tags) for (const g of v.tags) raw.add(g);
      }
      const eff = new Set(raw);
      for (const g of raw) for (const a of ancestorsOf(g)) eff.add(a);
      effByTx.set(tx.id, eff);
    }

    const satByTag = new Map<string, string[]>();
    for (const [txId, eff] of effByTx) {
      for (const g of eff) {
        const arr = satByTag.get(g) || [];
        arr.push(txId);
        satByTag.set(g, arr);
      }
    }

    // gap は traceability（canonical）から。satByTag での再導出には依らない。
    const gapSet = new Set<string>();
    for (const e of traceability.entries) if (e.gap) gapSet.add(e.tag.id);

    const decByTarget = new Map<string, Decision[]>();
    for (const d of decisions) {
      const arr = decByTarget.get(d.target.id) || [];
      arr.push(d);
      decByTarget.set(d.target.id, arr);
    }

    const currencyIndex = buildCurrencyIndex(decisions);
    // traceabilityKinds（requirement＋property 等）が「要件系＝葉」。config 由来
    // なので、旧スキーマ（requirement のみ 等）でも空にならず汎用に効く。
    const leafKinds = new Set(config.traceabilityKinds || []);

    return { tags, childrenByParent, ancestorsOf, effByTx, satByTag, gapSet, decByTarget, currencyIndex, leafKinds };
  }, [config, traceability, decisions, lookups.ready, tagById, transitionById, vocabById]);

  const hasComponents = !!index && index.tags.some((tg) => tg.kind === componentKind);

  // 既定選択（最初の component タグ）＋ ツリー初期展開（group 全て＋選択の祖先）。
  useEffect(() => {
    if (!index) return;
    if (sel === null) {
      const firstComp = index.tags.find((tg) => tg.kind === componentKind);
      if (firstComp) setSel(firstComp.id);
      setTreeOpen((prev) => {
        if (Object.keys(prev).length) return prev;
        const open: Record<string, boolean> = {};
        for (const tg of index.tags) if (tg.kind === groupKind) open[tg.id] = true;
        if (firstComp) {
          open[firstComp.id] = true;
          for (const a of index.ancestorsOf(firstComp.id)) open[a] = true;
        }
        return open;
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [index]);

  const toggleNode = (id: string) => setTreeOpen((p) => ({ ...p, [id]: !p[id] }));
  const toggleWhy = (id: string) => setOpenWhy((p) => ({ ...p, [id]: !p[id] }));
  const toggleRules = (key: string) => setOpenRules((p) => ({ ...p, [key]: !p[key] }));
  const togglePart = (id: string) => setOpenParts((p) => ({ ...p, [id]: !p[id] }));

  const selectComponent = (id: string, scrollToPart?: string) => {
    scrollToPartRef.current = scrollToPart || null;
    setSel(id);
    setTreeOpen((p) => ({ ...p, [id]: true }));
    if (isNarrow) closeDrawer();
  };

  const onTreeClick = (tag: Tag) => {
    if (!index) return;
    if (tag.kind === componentKind) selectComponent(tag.id);
    else if (tag.kind === partKind) {
      const parent = (tag.parentIds || []).find((p) => tagById.get(p)?.kind === componentKind) || (tag.parentIds || [])[0];
      if (parent) {
        // part 行クリック（①）: 親コンポーネントを選択し、その part セクションを開いて
        // ページ内アンカーへスクロール（別ページ遷移にしない）。
        setOpenParts((p) => ({ ...p, [tag.id]: true }));
        selectComponent(parent, tag.id);
      } else onOpenTag(tag.id);
    } else if ((index.childrenByParent.get(tag.id) || []).length) toggleNode(tag.id);
    else onOpenTag(tag.id);
  };

  // part クリックで親コンポーネントを選び、その part セクションを開いてからカードへ
  // スクロール。openParts も依存に入れる（対象 part が畳まれた状態から開かれた直後の
  // commit で要素が出現するため）。ref が立っていない通常のトグルでは no-op。
  useEffect(() => {
    const partId = scrollToPartRef.current;
    if (!partId || !mainRef.current) return;
    const el = mainRef.current.querySelector<HTMLElement>(`[data-part="${cssEscape(partId)}"]`);
    if (el) {
      scrollToPartRef.current = null;
      el.scrollIntoView({ block: 'start', behavior: 'smooth' });
    }
  }, [sel, openParts]);

  // ---- 構造ツリー行（treeVals 相当） ----
  const treeRows: TreeRow[] = [];
  if (index) {
    const seen = new Set<string>();
    const walk = (id: string, depth: number) => {
      const tag = tagById.get(id);
      if (!tag || seen.has(id)) return;
      seen.add(id);
      const kids = index.childrenByParent.get(id) || [];
      // ①: part をナビの最深にする。要件葉（leafKinds＝requirement/property 系）は
      // ツリーに出さない — 要件はシート内の各振る舞いカードで見えるので不要。
      // 木は構造 kind（group/component/part 等の非葉）だけで組む。
      const structuralChildren = kids.filter((c) => !index.leafKinds.has(c.kind || '')).map((c) => c.id);
      const isPart = tag.kind === partKind;
      const hasKids = structuralChildren.length > 0;
      const open = !!treeOpen[id];
      const isSel = tag.kind === componentKind && sel === id;
      const count =
        tag.kind === componentKind
          ? kids.filter((c) => c.kind === partKind).length
          : isPart
            ? (index.satByTag.get(id) || []).length
            : structuralChildren.length;
      treeRows.push({
        key: id,
        tagId: id,
        depth,
        name: tag.name || id,
        kind: tag.kind,
        hasKids,
        open,
        isSelected: isSel,
        isGap: index.gapSet.has(id),
        count: count > 0 ? count : null,
        onToggle: hasKids ? () => toggleNode(id) : undefined,
        onClick: () => onTreeClick(tag),
      });
      if (!open) return;
      for (const cid of structuralChildren) walk(cid, depth + 1);
    };
    // design は config.roots を起点にするが、roots 未設定のプロジェクト（この
    // repo 自身の旧スキーマ等）でツリーが空になるのを避け、parentIds を持たない
    // トップレベルタグをフォールバック起点にする（汎用性・空表示回避）。
    const rootIds = config && config.roots.length ? config.roots : index.tags.filter((tg) => !(tg.parentIds && tg.parentIds.length)).map((tg) => tg.id);
    for (const r of rootIds) walk(r, 0);
  }

  // ---- コンポーネント仕様シート（sheetVals 相当） ----
  const selTag = sel ? tagById.get(sel) : undefined;
  const sheet = useMemo(() => {
    if (!index || !selTag || selTag.kind !== componentKind) return null;
    const c = selTag;

    const crumbs: Tag[] = [];
    {
      let up = (c.parentIds || [])[0];
      const guard = new Set<string>();
      while (up && tagById.get(up) && !guard.has(up)) {
        guard.add(up);
        crumbs.unshift(tagById.get(up)!);
        up = (tagById.get(up)!.parentIds || [])[0];
      }
    }

    const childTags = index.childrenByParent.get(c.id) || [];
    const parts = childTags.filter((tg) => tg.kind === partKind);
    const propsList = childTags.filter((tg) => isConstraintTag(tg, constraintKind));

    const cmpTxIds = new Set(index.satByTag.get(c.id) || []);

    // 全子孫（parentIds 展開）— gap は「このコンポーネント配下の要件系タグで未充足」。
    const descendants = new Set<string>();
    {
      const stack = [...childTags];
      while (stack.length) {
        const d = stack.pop()!;
        if (descendants.has(d.id)) continue;
        descendants.add(d.id);
        for (const cc of index.childrenByParent.get(d.id) || []) stack.push(cc);
      }
    }

    // coverage: このコンポーネントの transition が実効化する要件系タグ（＝充足）と、
    // 配下の要件系タグのうち gap のもの（＝未充足）。scope フィールドは実データに
    // 無いので、この2集合の和を「対象要件」とする（TraceabilityResponse 由来）。
    const satisfiedReqs = new Set<string>();
    for (const txId of cmpTxIds) {
      const eff = index.effByTx.get(txId);
      if (!eff) continue;
      for (const g of eff) {
        const gt = tagById.get(g);
        if (gt && index.leafKinds.has(gt.kind || '') && !index.gapSet.has(g)) satisfiedReqs.add(g);
      }
    }
    const gapReqs: string[] = [];
    for (const d of descendants) {
      const dt = tagById.get(d);
      if (dt && index.leafKinds.has(dt.kind || '') && index.gapSet.has(d)) gapReqs.push(d);
    }
    const covSat = satisfiedReqs.size;
    const covTotal = covSat + gapReqs.length;
    const covPct = covTotal > 0 ? Math.round((covSat / covTotal) * 100) : 0;

    // ⑤: 平坦な「現行ルール」リストは廃止。decision を「それが関連するレコードの
    // 文脈」に割り当て、必要な時に開いて見られるようインライン化する。1 つの
    // decision は target を 1 つだけ持つ（decByTarget のキー）ので、各 decision は
    // ちょうど 1 コンテキストに入り、コンテキスト間で重複しない。カバーする target は
    // component-tag／transition／part／constraint(property)／充足・未充足の要件で、
    // 旧 flat リストが集めていた集合と同一（＝取りこぼしなし）。
    const notSuperseded = (d: Decision) => currencyOf(d.id, index.currencyIndex) !== 'superseded';
    // current/amended を先、superseded を後ろに並べる（inline 展開内の並び順）。
    const orderRules = (arr: RuleEntry[]): RuleEntry[] =>
      [...arr].sort((a, b) => (notSuperseded(a.d) ? 0 : 1) - (notSuperseded(b.d) ? 0 : 1));
    const rulesFor = (targetId: string, via = ''): RuleEntry[] =>
      orderRules((index.decByTarget.get(targetId) || []).map((d) => ({ d, via })));
    const countCurrent = (arr: RuleEntry[]) => arr.reduce((n, e) => n + (notSuperseded(e.d) ? 1 : 0), 0);

    // part ごとの振る舞い（transition の WHEN/GIVEN/THEN・vocab ラベル解決）と、
    // 各 transition／part を target とする decision（規則）。
    const partBlocks = parts.map((p) => {
      const txIds = index.satByTag.get(p.id) || [];
      const behaviors = txIds
        .map((txId) => {
          const tx = transitionById.get(txId);
          if (!tx) return null;
          const eff = index.effByTx.get(txId);
          const reqs: Tag[] = [];
          if (eff) {
            for (const g of eff) {
              const gt = tagById.get(g);
              if (gt && index.leafKinds.has(gt.kind || '') && !isConstraintTag(gt, constraintKind)) reqs.push(gt);
            }
          }
          return {
            id: txId,
            action: vocabLabel(tx.action),
            given: (tx.given || []).map((g) => vocabLabel(g)),
            then: (tx.then || []).map((g) => vocabLabel(g)),
            reqs: reqs.slice(0, 6),
            rules: rulesFor(txId), // ⑤: この振る舞いカードに紐づく規則
          };
        })
        .filter((b): b is NonNullable<typeof b> => b !== null);
      return { id: p.id, name: p.name || p.id, txCount: txIds.length, behaviors, rules: rulesFor(p.id) };
    });

    // 「〜しない」制約（property 子タグ）＋ 各制約を target とする decision。
    const propBlocks = propsList.map((p) => ({
      id: p.id,
      name: p.name || p.id,
      description: p.description,
      rules: rulesFor(p.id), // ⑤: この制約カードに紐づく規則
    }));

    // 取りこぼし防止（⑤）: このコンポーネントを充足する transition（cmpTxIds）のうち、
    // どの part 配下の振る舞いカードにも現れないもの（part タグを持たず component 直下）
    // は、behaviors に居場所がない。その decision は component 本体の規則へ回収する
    // （旧 flat リストが cmpTxIds 全体を viaSpec で集めていた到達性を保つ）。
    const renderedTxIds = new Set<string>();
    for (const p of partBlocks) for (const b of p.behaviors) renderedTxIds.add(b.id);
    const orphanTxIds = [...cmpTxIds].filter((id) => !renderedTxIds.has(id));

    // コンポーネント本体に紐づく規則（⑤）: component タグ自身が target ＋ その
    // コンポーネントが充足／未充足する要件タグ target ＋ part 外の直属 transition
    // （via ラベル付き）。
    const componentRules = orderRules([
      ...(index.decByTarget.get(c.id) || []).map((d) => ({ d, via: t.overview.viaComponent })),
      ...[...satisfiedReqs, ...gapReqs].flatMap((r) =>
        (index.decByTarget.get(r) || []).map((d) => ({ d, via: t.overview.viaTag(tagName(r)) })),
      ),
      ...orphanTxIds.flatMap((id) => (index.decByTarget.get(id) || []).map((d) => ({ d, via: t.overview.viaSpec }))),
    ]);

    // ヘッダのサマリ用「現行ルール N」= 全コンテキストの現行（非失効）decision の総数。
    // 各 decision は 1 コンテキストにのみ入るので二重計上しない（旧 flat の現行数と一致）。
    let ruleCount = countCurrent(componentRules);
    for (const p of partBlocks) {
      ruleCount += countCurrent(p.rules);
      for (const b of p.behaviors) ruleCount += countCurrent(b.rules);
    }
    for (const p of propBlocks) ruleCount += countCurrent(p.rules);

    const { lead, body } = splitLead(c.description);

    return {
      c,
      crumbs,
      lead,
      body,
      partBlocks,
      propBlocks,
      componentRules,
      covSat,
      covTotal,
      covPct,
      partCount: parts.length,
      ruleCount,
      gapReqs,
    };
  }, [index, selTag, t, tagById, transitionById, vocabLabel, tagName, componentKind, partKind, constraintKind]);

  // ---- render ----
  if (error) {
    return (
      <div class="overview-view">
        <main class="overview-main">
          <div class="overview-empty">{error}</div>
        </main>
      </div>
    );
  }
  if (!index) {
    return (
      <div class="overview-view">
        <main class="overview-main">
          <div class="overview-empty dim">{t.overview.loading}</div>
        </main>
      </div>
    );
  }

  const railClass = 'overview-rail' + (isNarrow ? ' overview-rail-narrow' : '') + (isNarrow && drawerOpen ? ' overview-rail-open' : '');

  // ⑤: decision 行（現行/失効バッジ・要約＋全文トグル・via・target への <a> deep-link）。
  // 平坦リストの行を文脈内インライン展開に流用する共通レンダラ。
  const ruleTargetLink = (d: Decision) => {
    if (d.target.type === 'transition') {
      return (
        <HashLink href={routeHash({ view: 'browse', txId: d.target.id })} onNavigate={() => onOpenTx(d.target.id)} class="overview-rule-link" title={t.overview.openInBrowser} ariaLabel={t.overview.openInBrowser}>
          <Icon name="arrow-up-right" size={13} />
        </HashLink>
      );
    }
    if (d.target.type === 'tag') {
      return (
        <HashLink href={routeHash({ view: 'spec', tagId: d.target.id })} onNavigate={() => onOpenTag(d.target.id)} class="overview-rule-link" title={t.overview.openInBrowser} ariaLabel={t.overview.openInBrowser}>
          <Icon name="arrow-up-right" size={13} />
        </HashLink>
      );
    }
    return null; // vocab を target とする decision はこの画面の文脈には現れない
  };
  const renderRuleRow = ({ d, via }: RuleEntry) => {
    const cur = currencyOf(d.id, index.currencyIndex);
    const stale = cur === 'superseded';
    const open = !!openWhy[d.id];
    return (
      <div key={d.id} class={'overview-rule' + (stale ? ' stale' : '')}>
        <div class="overview-rule-top">
          <span class={'overview-rule-summary' + (stale ? ' struck' : '')}>{summaryOf(d.why)}</span>
          <span class={'overview-rule-badge' + (cur === 'amended' ? ' amended' : '') + (stale ? ' stale' : '')}>
            <Icon name={stale ? 'circle-slash' : 'circle-check'} size={11} />
            {stale ? t.decisions.currencySuperseded : cur === 'amended' ? t.decisions.currencyAmended : t.decisions.currencyCurrent}
          </span>
        </div>
        {open && <p class="overview-rule-why">{d.why}</p>}
        <div class="overview-rule-meta">
          <button type="button" class="overview-rule-toggle" onClick={() => toggleWhy(d.id)}>
            <Icon name={open ? 'chevron-up' : 'chevron-down'} size={13} />
            {open ? t.overview.backToSummary : t.overview.readFull}
          </button>
          {via && <span class="overview-rule-via">{via}</span>}
          {ruleTargetLink(d)}
          <span class="overview-rule-spacer" />
          <span class="overview-rule-at">
            {formatDecisionAt(d.at)}
          </span>
        </div>
      </div>
    );
  };
  // ⑤: 「規則 (N)」展開トグル（初期折りたたみ）。N は当該レコードを target とする
  // decision 総数（現行＋失効）。中身は現行を先・失効を後ろに並べる（orderRules 済み）。
  const renderRules = (key: string, entries: RuleEntry[], label: string) => {
    if (!entries.length) return null;
    const open = !!openRules[key];
    return (
      <div class="overview-rules-inline">
        <button type="button" class="overview-rules-toggle" onClick={() => toggleRules(key)} aria-expanded={open}>
          <Icon name={open ? 'chevron-down' : 'chevron-right'} size={13} />
          <Icon name="gavel" size={13} />
          {label}
        </button>
        {open && <div class="overview-rules-list">{entries.map(renderRuleRow)}</div>}
      </div>
    );
  };

  return (
    <div class="overview-view">
      {isNarrow && drawerOpen && <div class="overview-backdrop" onClick={closeDrawer} />}

      {(!isNarrow || drawerOpen) && (
        <aside class={railClass}>
          <div class="overview-rail-head">
            <span class="overview-rail-head-title">
              <Icon name="list-tree" size={14} />
              {t.overview.treeHeading}
            </span>
            {/* ③: 非機能な「たたむ」ボタンは撤去（PC で畳む要件なし）。モバイルの
                ドロワー開閉は backdrop クリック／コンポーネント選択で従来どおり維持。 */}
          </div>
          <div class="overview-tree">
            {treeRows.map((row) => (
              <div
                key={row.key}
                class={'overview-tree-row' + (row.isSelected ? ' selected' : '')}
                style={{ paddingLeft: `calc(${row.depth} * 13px + 2px)` }}
              >
                {row.hasKids ? (
                  <button type="button" class="overview-tree-toggle" aria-label="開閉" onClick={row.onToggle}>
                    <Icon name={row.open ? 'chevron-down' : 'chevron-right'} size={13} />
                  </button>
                ) : (
                  <span class="overview-tree-spacer" />
                )}
                <button
                  type="button"
                  class={'overview-tree-label kind-' + (row.kind || 'none')}
                  style={{ '--kc': kindColorVar(row.kind) } as JSX.CSSProperties}
                  onClick={row.onClick}
                >
                  <span class="overview-tree-dot" />
                  <span class="overview-tree-name">{row.name}</span>
                  {row.isGap && <Icon name="triangle-alert" size={12} class="overview-tree-gap" />}
                  {row.count != null && <span class="overview-tree-count">{row.count}</span>}
                </button>
              </div>
            ))}
          </div>
        </aside>
      )}

      <main class="overview-main" ref={mainRef}>
        {sheet ? (
          <div class="overview-sheet">
            {/* パンくず（祖先 group）— 生 id は出さない（ユーザーに id は見せない方針） */}
            <section class="overview-head">
              <div class="overview-crumbs">
                {sheet.crumbs.map((cr, i) => (
                  <span key={cr.id} class="overview-crumb-wrap">
                    {i > 0 && <Icon name="chevron-right" size={12} />}
                    <span class="overview-crumb" style={{ '--kc': kindColorVar(cr.kind) } as JSX.CSSProperties}>
                      {cr.name || cr.id}
                    </span>
                  </span>
                ))}
              </div>

              <div class="overview-title-row">
                <h1 class="overview-title">{sheet.c.name || sheet.c.id}</h1>
                <span class="overview-kind-badge" style={{ '--kc': kindColorVar(sheet.c.kind) } as JSX.CSSProperties}>
                  <Icon name={kindIconName(sheet.c.kind)} size={13} />
                  {tagKindLabel(sheet.c.kind) || sheet.c.kind}
                </span>
                <span class="overview-title-spacer" />
                {/* ②: 実 <a href="#/spec/<id>">。平打ち=SPA 遷移／⌘・Ctrl・中クリック=別タブ。 */}
                <HashLink href={routeHash({ view: 'spec', tagId: sheet.c.id })} onNavigate={() => onOpenTag(sheet.c.id)} class="overview-open-browser" title={t.overview.openInBrowser}>
                  <Icon name="search" size={14} />
                  {t.overview.openInBrowser}
                </HashLink>
              </div>

              {sheet.lead && <p class="overview-lead">{sheet.lead}</p>}

              {/* カバレッジ統計 */}
              <div class="overview-cov">
                <span class="overview-cov-label">
                  <Icon name="radar" size={14} />
                  {t.overview.coverageHeading}
                </span>
                {sheet.covTotal > 0 ? (
                  <>
                    <span class="overview-cov-ratio">
                      <span class="overview-cov-num">
                        {sheet.covSat} / {sheet.covTotal}
                      </span>
                      <span class="overview-cov-suffix">{t.overview.coverageSuffix}</span>
                    </span>
                    <span class="overview-cov-bar">
                      <span class="overview-cov-bar-fill" style={{ width: `${sheet.covPct}%` }} />
                    </span>
                  </>
                ) : (
                  <span class="overview-cov-none dim">{t.overview.coverageNone}</span>
                )}
                <span class="overview-cov-meta">
                  <Icon name="puzzle" size={13} /> {t.overview.partCount(sheet.partCount)}
                  <span class="overview-cov-dot">·</span>
                  <Icon name="gavel" size={13} /> {t.overview.ruleCount(sheet.ruleCount)}
                </span>
              </div>

              {sheet.gapReqs.length > 0 && (
                <div class="overview-gap">
                  <span class="overview-gap-head">
                    <Icon name="triangle-alert" size={13} />
                    {t.overview.gapCount(sheet.gapReqs.length)}
                  </span>
                  <div class="overview-gap-chips">
                    {sheet.gapReqs.map((rid) => (
                      <HashLink key={rid} href={routeHash({ view: 'spec', tagId: rid })} onNavigate={() => onOpenTag(rid)} class="overview-gap-chip">
                        {tagName(rid)}
                      </HashLink>
                    ))}
                  </div>
                </div>
              )}
            </section>

            {/* 責務 ＋ ⑤ このコンポーネントの規則（component タグ自身＋充足要件 target） */}
            {(sheet.body || sheet.lead || sheet.componentRules.length > 0) && (
              <section class="overview-section">
                <div class="overview-section-head">
                  <Icon name="crosshair" size={16} class="overview-section-icon" />
                  <span class="overview-section-title">{t.overview.responsibilityHeading}</span>
                  <CommentButton recordType="tag" recordId={sheet.c.id} recordTitle={sheet.c.name || sheet.c.id} anchor="responsibility" anchorLabel={t.overview.responsibilityHeading} />
                </div>
                {sheet.body ? <Markdown text={sheet.body} class="overview-responsibility" /> : sheet.lead ? <p class="overview-responsibility">{sheet.lead}</p> : null}
                {renderRules('component:' + sheet.c.id, sheet.componentRules, t.overview.componentRulesToggle(sheet.componentRules.length))}
              </section>
            )}

            {/* 構成要素ごとの振る舞い */}
            {sheet.partBlocks.length > 0 && (
              <section class="overview-section">
                <div class="overview-section-head">
                  <Icon name="puzzle" size={16} class="overview-section-icon" />
                  <span class="overview-section-title">{t.overview.behaviorsHeading}</span>
                  <span class="overview-section-hint">{t.overview.behaviorsHint}</span>
                  <CommentButton recordType="tag" recordId={sheet.c.id} recordTitle={sheet.c.name || sheet.c.id} anchor="behaviors" anchorLabel={t.overview.behaviorsHeading} />
                </div>
                {sheet.partBlocks.map((p) => {
                  // ④: part 見出しは「ピル」でなく「見出し」。kind 色はドットのアクセントに
                  // 留め、タイポは見出し。ヘッダクリックで開閉（chevron 付き）・初期は畳む。
                  const partOpen = !!openParts[p.id];
                  return (
                  <div key={p.id} class="overview-part" data-part={p.id}>
                    <button type="button" class="overview-part-head" onClick={() => togglePart(p.id)} aria-expanded={partOpen}>
                      <Icon name={partOpen ? 'chevron-down' : 'chevron-right'} size={15} class="overview-part-chevron" />
                      <span class="overview-part-dot" style={{ '--kc': 'var(--k-prt)' } as JSX.CSSProperties} />
                      <span class="overview-part-title">{p.name}</span>
                      <span class="overview-part-spacer" />
                      <span class="overview-part-count">{t.overview.txCount(p.txCount)}</span>
                    </button>
                    {partOpen && (
                    <div class="overview-part-body">
                    {/* ⑤: この part を target とする規則 */}
                    {renderRules('part:' + p.id, p.rules, t.overview.rulesToggle(p.rules.length))}
                    {p.behaviors.map((b) => (
                      <div key={b.id} class="overview-behavior">
                        <div class="overview-when">
                          <span class="overview-when-label">
                            <Icon name="circle-play" size={12} />
                            {t.flow.trigger}
                          </span>
                          <span class="overview-when-text">{b.action}</span>
                          {/* ②: 実 <a href="#/browse/tx/<id>">。⌘/Ctrl/中クリックで別タブ。 */}
                          <HashLink href={routeHash({ view: 'browse', txId: b.id })} onNavigate={() => onOpenTx(b.id)} class="overview-open-tx" title={t.overview.openInBrowser} ariaLabel={t.overview.openInBrowser}>
                            <Icon name="arrow-up-right" size={15} />
                          </HashLink>
                        </div>
                        <div class="overview-gt">
                          <span class="overview-slot-label given">
                            <Icon name="funnel" size={11} />
                            {t.flow.given}
                          </span>
                          <div class="overview-slot-list">
                            {b.given.length === 0 ? (
                              <span class="overview-slot-empty">{t.overview.unconditional}</span>
                            ) : (
                              b.given.map((g, i) => (
                                <span key={i} class="overview-slot-item">
                                  <span class="overview-slot-dot given" />
                                  <span>{g}</span>
                                </span>
                              ))
                            )}
                          </div>
                          <span class="overview-slot-label then">
                            <Icon name="arrow-right-to-line" size={11} />
                            {t.flow.result}
                          </span>
                          <div class="overview-slot-list">
                            {b.then.map((th, i) => (
                              <span key={i} class="overview-slot-item">
                                <span class="overview-then-n">{i + 1}</span>
                                <span>{th}</span>
                              </span>
                            ))}
                          </div>
                        </div>
                        {b.reqs.length > 0 && (
                          <div class="overview-reqs">
                            <span class="overview-reqs-label">{t.overview.satisfiesReqs}</span>
                            {b.reqs.map((r) => (
                              <HashLink key={r.id} href={routeHash({ view: 'spec', tagId: r.id })} onNavigate={() => onOpenTag(r.id)} class="overview-req-chip" style={{ '--kc': kindColorVar(r.kind) } as JSX.CSSProperties}>
                                <Icon name={kindIconName(r.kind)} size={11} />
                                {r.name || r.id}
                              </HashLink>
                            ))}
                          </div>
                        )}
                        {/* ⑤: この振る舞い（transition）を target とする規則 */}
                        {renderRules('tx:' + b.id, b.rules, t.overview.rulesToggle(b.rules.length))}
                      </div>
                    ))}
                    </div>
                    )}
                  </div>
                  );
                })}
              </section>
            )}

            {/* 「〜しない」制約（property 子タグ） */}
            {sheet.propBlocks.length > 0 && (
              <section class="overview-section">
                <div class="overview-section-head">
                  <Icon name="ban" size={16} class="overview-section-icon" />
                  <span class="overview-section-title">{t.overview.constraintsHeading}</span>
                  <span class="overview-section-hint">{t.overview.constraintsHint}</span>
                  <CommentButton recordType="tag" recordId={sheet.c.id} recordTitle={sheet.c.name || sheet.c.id} anchor="constraints" anchorLabel={t.overview.constraintsHeading} />
                </div>
                <div class="overview-props-grid">
                  {sheet.propBlocks.map((p) => (
                    <div key={p.id} class="overview-prop-card">
                      {/* ②: 制約名は実 <a href="#/spec/<id>">（⌘/Ctrl+クリックで別タブ）。 */}
                      <HashLink href={routeHash({ view: 'spec', tagId: p.id })} onNavigate={() => onOpenTag(p.id)} class="overview-prop-name">
                        <Icon name="ban" size={14} class="overview-prop-icon" />
                        {p.name}
                      </HashLink>
                      {p.description && <span class="overview-prop-why">{p.description}</span>}
                      {/* ⑤: この制約を target とする規則 */}
                      {renderRules('prop:' + p.id, p.rules, t.overview.rulesToggle(p.rules.length))}
                    </div>
                  ))}
                </div>
              </section>
            )}
          </div>
        ) : (
          <div class="overview-empty dim">{hasComponents ? t.overview.selectPrompt : t.overview.noComponents}</div>
        )}
      </main>
    </div>
  );
}
