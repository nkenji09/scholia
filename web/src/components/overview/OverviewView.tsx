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
import { useScrollRestore, useElementScrollRestore } from '../../scrollRestore';
import { loadRegionShape, saveRegionShape } from '../../regionShape';
import { loadCardSectionOpen, saveCardSectionOpen } from '../../collapseState';
import { summaryOf } from '../../decisionSummary';
import { buildCurrencyIndex, effectOf, relatedDecisions, replacedBy } from '../decisions/decisionModel';
import { formatScopeTarget } from '../decisions/decisionScope';
import type { Config, Decision, Tag, TraceabilityResponse } from '../../types';

// 概要ビュー（viewer-overview-browser）: 左=構造ツリー、右=コンポーネント仕様
// シート。design（scholia-viewer.dc.html の treeVals()/sheetVals()）の構造・
// 状態を Preact へ翻訳しつつ、データは design のダミー（window.__SCHOLIA__）では
// なく実 api から取り、design が前提にした便宜フィールド（KIND メタ・tx.part・
// reqScope・decision.summary/current）を実データから導出して汎用化している
// （どの .scholia でも config.tagKinds と実データだけで破綻なく描ける）。

interface Props {
  /** URL 上の現在地＝いま開いているコンポーネント（deep-linking の適用面拡張・
      01KYGYYMZSS1Y0BFEJ69Q1JC40）。未指定なら既定（最初のコンポーネント）を選ぶ。
      既定は URL に書き戻さない——初期表示で履歴を1件消費させないため。 */
  componentId?: string;
  /** URL 上のアンカー＝そのコンポーネント内の構成要素。指定されるとその位置まで寄せる。 */
  partId?: string;
  /** 現在地を移す。app 側の navigate を通るので、そのつど履歴に1件残る
      （粒度の作り分けはしない＝01KYGYYMZSS… 条項(3)）。 */
  onSelectComponent: (componentId: string, partId?: string) => void;
  /** タグ（コンポーネント/要件/制約）を ブラウザの詳細で開く（#/spec/<id>）。 */
  onOpenTag: (tagId: string) => void;
  /** 遷移（仕様）を ブラウザの詳細で開く（#/browse/tx/<id>）。 */
  onOpenTx: (txId: string) => void;
}

// 概要シートの折りたたみは「保存値 > 初期折りたたみ既定」で解決する
// （collapsible-section の amend・01KYGYYN8HRNFQEDMBS3DZRRX7）。既定は
// progressive disclosure に従って畳んだ状態（01KYCC2TK3…）だが、それが効くのは
// 保存値が無いときだけ——一度開いた利用者の意思は再訪でも尊重される。
//
// 保存先・寿命はカード側の折りたたみと同じ（collapseState）。ここで独自の保存先を
// 作らないのは、寿命を既決から動かさないため（01KXDFD2SRHJJ0E551V240JMKT(1)）。
// localStorage の読み出しはキーごとに1度だけキャッシュし、描画のたびには叩かない。
function useSectionOpen() {
  const cache = useRef(new Map<string, boolean>());
  const [, bump] = useState(0);
  // 区切りは id/section のどちらにも現れない文字をエスケープ表記で置く。生の制御文字を
  // ソースへ直接埋めると git がファイルをバイナリ扱いし、diff も grep も効かなくなる。
  const cacheKey = (recordId: string, section: string) => `${recordId}\u0000${section}`;

  const isOpen = (recordId: string, section: string, fallback: boolean): boolean => {
    const k = cacheKey(recordId, section);
    const hit = cache.current.get(k);
    if (hit !== undefined) return hit;
    const resolved = loadCardSectionOpen(recordId, section) ?? fallback;
    cache.current.set(k, resolved);
    return resolved;
  };

  const toggle = (recordId: string, section: string, fallback: boolean) => {
    const next = !isOpen(recordId, section, fallback);
    cache.current.set(cacheKey(recordId, section), next);
    saveCardSectionOpen(recordId, section, next);
    bump((n) => n + 1);
  };

  /** アンカー等の「開けというシグナル」。保存はしない——利用者自身の操作ではないので
      永続値を書き換えず、この場の表示だけ開く（CollapsibleSection の focusOpen と同じ
      一方向の扱い・01KXDFD2SRHJJ0E551V240JMKT(2)）。 */
  const forceOpen = (recordId: string, section: string) => {
    const k = cacheKey(recordId, section);
    if (cache.current.get(k) === true) return;
    cache.current.set(k, true);
    bump((n) => n + 1);
  };

  return { isOpen, toggle, forceOpen };
}

// 折りたたみのセクション名（collapseState のキー空間。カード側と混ざらないよう
// overview- を冠する）。
const SEC_PART = 'overview-part';
const SEC_RULES = 'overview-rules';
const SEC_WHY = 'overview-why';
const SEC_HISTORY = 'overview-rules-history';

// スクロール保持のキー。本体と独立スクロール領域で別空間を使い、互いを壊さない
// （01KYGYYN44… / 01KYH0ESVG…）。#/home も同じ画面なので同じキーを共有する。
const SCROLL_KEY = 'overview';
// 構造ツリー＝この面の独立スクロール領域。位置と形は同じ識別子で対にする
// （01KYH8GX987GQX08C56G58JP2N）。
const TREE_REGION = 'overview:tree';

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

// decision に summary フィールドは無い（実 decision は why のみ）。要約の切り出しは
// decisionSummary.summaryOf に一本化してある——ここに自前の実装を戻さないこと。
// 面ごとに書くと面ごとに違う長さの「要約」が出るし、実際この面が持っていた自前版は
// 日本語の句点で切れず第1段落をまるごと返していた（01KYHW54B8ZXH0NEPH2J7N1X39 条項6）。

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
  /** 別レコードへ移動する行の指し先。null なら「その場で開閉するだけ」の構造ノードで、
      遷移ではないのでアンカーにしない（01KXFK3Q1NY9J8Q7FX14T31N7K の filter 除外と同じ趣旨）。 */
  href: string | null;
  onClick: () => void;
}

export function OverviewView({ componentId, partId, onSelectComponent, onOpenTag, onOpenTx }: Props) {
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

  // ツリーの展開状態＝この独立スクロール領域の「器の形」。位置と対で覚える
  // （01KYH8GX987GQX08C56G58JP2N）——離脱前に開いていた枝が復帰時に畳まれると領域が
  // 短くなり、覚えていた位置がそこに存在しなくなる。最初の描画から復元済みにするため
  // effect ではなく初期化子で読む（ツリーが描かれた時点で高さが正しく、位置の復元がそこへ
  // 着地できる）。
  const [treeOpen, setTreeOpen] = useState<Record<string, boolean>>(() => loadRegionShape<Record<string, boolean>>(TREE_REGION) ?? {});
  // 各コンテキスト（tx / part / constraint / component）ごとの「規則 (N)」展開、part
  // セクションの開閉、decision 全文の開閉。既定は畳んだ状態（progressive disclosure・
  // ④⑤）だが、利用者が明示的に開閉した保存値があればそちらが勝つ（01KYGYYN8H…）。
  const sections = useSectionOpen();
  const mainRef = useRef<HTMLElement>(null);
  const treeRef = useRef<HTMLDivElement>(null);

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

  // 現在地は URL が持つ（01KYGYYMZSS…）。URL が指すコンポーネントを優先し、無効な id
  // や未指定なら既定（最初の component タグ）へ落とす。既定は URL へ書き戻さない
  // ——素の #/overview が「既定のコンポーネント」を意味する（router.ts）。
  const defaultComponentId = index ? (index.tags.find((tg) => tg.kind === componentKind)?.id ?? null) : null;
  const selFromUrl = componentId && tagById.get(componentId)?.kind === componentKind ? componentId : null;
  const sel = selFromUrl ?? defaultComponentId;

  // ツリーの展開: 覚えている形があればそれを使い、無ければ group を既定で開く。どちらの
  // 場合も現在地までの経路は開く——URL 直打ち・ブラウザバックで飛び込んできたときに、
  // 選ばれている行が畳まれた枝の中に隠れないため。
  useEffect(() => {
    if (!index) return;
    setTreeOpen((prev) => {
      const next = { ...prev };
      let changed = false;
      // 既定は「覚えている形が無いとき」だけ効く。覚えている形があるなら、そこに畳まれた
      // 枝があること自体が利用者の状態なので、既定で開き直さない。
      if (!Object.keys(prev).length) {
        for (const tg of index.tags) {
          if (tg.kind === groupKind && !next[tg.id]) {
            next[tg.id] = true;
            changed = true;
          }
        }
      }
      if (sel) {
        if (!next[sel]) {
          next[sel] = true;
          changed = true;
        }
        for (const a of index.ancestorsOf(sel)) {
          if (!next[a]) {
            next[a] = true;
            changed = true;
          }
        }
      }
      return changed ? next : prev;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [index, sel]);

  // 形を覚える。ツリーが実在してから（index が揃ってから）書く——それ以前に書くと、
  // 復元した形をまだ組み立てていない空の状態で上書きしてしまう。位置側の「復元が済むまで
  // 保存しない」と同じ考え方（01KXFEJ8HZWGTE6D7FHA5W9PS0）。
  useEffect(() => {
    if (!index) return;
    saveRegionShape(TREE_REGION, treeOpen);
  }, [index, treeOpen]);

  const toggleNode = (id: string) => setTreeOpen((p) => ({ ...p, [id]: !p[id] }));

  // 構成要素の行が押されたことを数える。URL の変化とは別に「寄せてくれ」という要求を
  // 立てるための口で、下のアンカー effect の依存に入る。
  //
  // なぜ要るか: すでに同じ現在地にいるとき、平打ちしても URL は変わらない（navigate は
  // 同一 hash なら何もしないのが正しい——BrowseView の URL 同期はその参照安定性に依って
  // いる）。URL が変わらなければ再描画も起きないので、URL の変化だけを見ている限り
  // 「押しても何も起きない」になる。平打ちが no-op になる欠陥は
  // 01KXG8QRCXMG70PBW32R9ETCA7 が「クリックしたら必ず対象へ辿り着く」へ是正した既決で、
  // main も同じ形（クリックのたびに寄せ先を立て直す）で満たしていた。URL 経由の到達
  // （直打ち・reload・バック）は従来どおり partId の変化で走る。
  const [anchorRequest, setAnchorRequest] = useState(0);

  // 現在地の移動は必ず URL 経由（＝そのつど履歴に1件残る・条項(3)）。狭い画面で
  // ドロワーから選んだときに閉じる従来の挙動はここで維持する（view が変わらないので
  // app 側の view 監視では閉じない）。
  const goTo = (id: string, part?: string) => {
    if (isNarrow) closeDrawer();
    if (part) setAnchorRequest((n) => n + 1);
    onSelectComponent(id, part);
  };

  /** part タグの親コンポーネント。無ければ null＝ページ内アンカーにできないので、
      その part はブラウザの詳細へ送る（コンポーネントでない親を選んでも仕様シートは
      描けず、行き先の無い URL になるため）。 */
  const componentParentOf = (tag: Tag): string | null => (tag.parentIds || []).find((p) => tagById.get(p)?.kind === componentKind) || null;

  // 行の指し先。別レコードへ移動する行は実アンカーにする（ページ内の遷移リンクは
  // 本物の <a href> という一般則 01KXFK3Q1NY9J8Q7FX14T31N7K を概要にも及ぼす・条項(2)）。
  // 子を持つ構造ノードだけは「その場で開閉するだけ」で遷移ではないので、従来どおり
  // ボタンのまま＝一般則の対象外。
  const treeLinkFor = (tag: Tag): { href: string; navigate: () => void } | null => {
    if (!index) return null;
    if (tag.kind === componentKind) {
      return { href: routeHash({ view: 'overview', componentId: tag.id }), navigate: () => goTo(tag.id) };
    }
    if (tag.kind === partKind) {
      const parent = componentParentOf(tag);
      if (parent) return { href: routeHash({ view: 'overview', componentId: parent, partId: tag.id }), navigate: () => goTo(parent, tag.id) };
      return { href: routeHash({ view: 'spec', tagId: tag.id }), navigate: () => onOpenTag(tag.id) };
    }
    if ((index.childrenByParent.get(tag.id) || []).length) return null;
    return { href: routeHash({ view: 'spec', tagId: tag.id }), navigate: () => onOpenTag(tag.id) };
  };

  // URL がアンカーする構成要素は開いて見せる。保存はしない（利用者の操作ではないので
  // 永続値を書き換えない）——保存済みの「閉じ」があるときにマウント時点でそれを
  // 上書きしないのは 01KXDFD2SRHJJ0E551V240JMKT(2) が守っている当のもの。
  useEffect(() => {
    if (partId) sections.forceOpen(partId, SEC_PART);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [partId]);

  // アンカーされた構成要素の位置まで寄せる。
  //
  // 走る条件は4つ:
  // - URL のアンカーが変わったとき（別の構成要素へ移る）
  // - 現在地のコンポーネントが変わったとき
  // - **シートが描かれたとき**（index が揃う）。アンカー付きの URL へバック／reload で
  //   入ってくる経路では、この面が組み直される時点で partId も sel も最初から確定して
  //   いる（タグ表は app 全体で共有されており読み込み済み）。つまり「変わった」瞬間が
  //   無く、寄せ先の要素が現れるのはデータが揃った後——ここを依存に入れないと、
  //   バックで戻ってきたときだけ寄らない。
  // - 行が押されたとき（anchorRequest）。同じ現在地にいる状態での再クリックでは URL が
  //   変わらず再描画も起きないため——URL の変化だけを見ていると「押しても何も起きない」に
  //   なり、それは 01KXG8QRCXMG70PBW32R9ETCA7 が是正した欠陥そのものに戻る。
  //
  // 対象セクションは直前の effect の forceOpen で一拍遅れて展開されるため、最初の寄せの
  // 時点では document がまだ低く、ブラウザ側で位置が clamp される。これは「読み込み中の
  // レイアウト由来の位置は利用者の位置ではない」という既決（01KXFEJ8HZWGTE6D7FHA5W9PS0）
  // が本体スクロール復元で受けているのと同型の罠なので、対処も同型にする——本体側
  // （scrollRestore.ts の reinforce）と同じく、伸びたあとに一度だけ寄せ直す。新経路のために
  // 別機構を作らない。
  useEffect(() => {
    if (!partId) return;
    const root = mainRef.current;
    if (!root) return;
    const find = () => root.querySelector<HTMLElement>(`[data-part="${cssEscape(partId)}"]`);
    const el = find();
    if (!el) return;
    el.scrollIntoView({ block: 'start', behavior: 'smooth' });
    // rAF ではなく setTimeout なのも本体側と同じ理由（タブが背面でも動かすため）。
    const reinforce = setTimeout(() => find()?.scrollIntoView({ block: 'start', behavior: 'smooth' }), 120);
    return () => clearTimeout(reinforce);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [partId, sel, index, anchorRequest]);

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
      const link = treeLinkFor(tag);
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
        href: link ? link.href : null,
        onClick: link ? link.navigate : () => toggleNode(id),
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
    // 効いている／置き換え済みの振り分けは renderRules 側が行う（条項4: 置き換え
    // 済みは既定で畳む）。並び順で効力を表さないので、ここでの並べ替えはしない。
    const inForce = (d: Decision) => effectOf(d.id, index.currencyIndex) === 'in-force';
    const rulesFor = (targetId: string, via = ''): RuleEntry[] =>
      (index.decByTarget.get(targetId) || []).map((d) => ({ d, via }));
    const countCurrent = (arr: RuleEntry[]) => arr.reduce((n, e) => n + (inForce(e.d) ? 1 : 0), 0);

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


    // ヘッダのサマリ用「現行ルール N」= シート内で実際に読める、効いている
    // decision の総数（01KYHW54B8ZXH0NEPH2J7N1X39 条項5: 見出しの件数と、開いて
    // 見える行数が一致すること）。コンポーネント本体の欄を廃止した以上、そこに
    // 集めていた分はシートから読めないので数にも含めない——含めると「N と言って
    // いるのに N 件見つからない」になる。
    let ruleCount = 0;
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
      covSat,
      covTotal,
      covPct,
      partCount: parts.length,
      ruleCount,
      gapReqs,
    };
  }, [index, selTag, t, tagById, transitionById, vocabLabel, tagName, componentKind, partKind, constraintKind]);

  // 本体のスクロール位置を面ごとに保持・復元する（01KYGYYN44…）。ready は「データが
  // 揃って仕様シートが並んだ」時点。アンカー付きで来たときは寄せる先が別にあるので
  // 復元しない（BrowseView が focus 時に譲るのと同じ扱い）。
  useScrollRestore(SCROLL_KEY, !!index, !!partId);
  // 構造ツリーは本体とは別に独立してスクロールする領域。同じ規律で保持・復元する
  // （01KYH0ESVG…）。狭い画面でドロワーが閉じている間は要素そのものが無いので ready を落とす。
  useElementScrollRestore(TREE_REGION, treeRef, !!index && (!isNarrow || drawerOpen));

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
  // 効力の表示は2値（01KYHW54B8ZXH0NEPH2J7N1X39 条項1）。記録の3値をそのまま
  // 状態の3値として写すと「現行 ⇔ 改訂」という存在しない対立に読まれる——
  // amend / exception を付けられた decision は**効いている**。判定と語は
  // decisionModel / strings と共有し、ここで別の答えを作らない（面間整合）。
  const renderRuleRow = ({ d, via }: RuleEntry) => {
    const replaced = effectOf(d.id, index.currencyIndex) === 'replaced';
    const related = replaced ? [] : relatedDecisions(d.id, index.currencyIndex);
    const replacer = replaced ? replacedBy(d.id, index.currencyIndex) : undefined;
    const open = sections.isOpen(d.id, SEC_WHY, false);
    return (
      <div key={d.id} class={'overview-rule' + (replaced ? ' stale' : '')}>
        <div class="overview-rule-top">
          <span class={'overview-rule-summary' + (replaced ? ' struck' : '')}>{summaryOf(d.why)}</span>
          <span class={'overview-rule-badge' + (replaced ? ' stale' : '')}>
            <Icon name={replaced ? 'circle-slash' : 'circle-check'} size={11} />
            {replaced ? t.decisions.effectReplaced : t.decisions.effectInForce}
          </span>
        </div>
        {open && <Markdown text={d.why} class="overview-rule-why" />}
        <div class="overview-rule-meta">
          <button type="button" class="overview-rule-toggle" onClick={() => sections.toggle(d.id, SEC_WHY, false)}>
            <Icon name={open ? 'chevron-up' : 'chevron-down'} size={13} />
            {open ? t.overview.backToSummary : t.overview.readFull}
          </button>
          {via && <span class="overview-rule-via">{via}</span>}
          {/* 条項2: 「後続に部分改訂・例外が付いている」は状態ではなく付帯情報。
              条項7: 関係は並び順ではなく明示的な導線で辿る。 */}
          {related.length > 0 && (
            <HashLink
              href={routeHash({ view: 'decision', decisionId: related[0].id })}
              onNavigate={() => {
                window.location.hash = routeHash({ view: 'decision', decisionId: related[0].id });
              }}
              class="overview-rule-related"
            >
              {t.decisions.readTogether(related.length)}
            </HashLink>
          )}
          {replacer && (
            <HashLink
              href={routeHash({ view: 'decision', decisionId: replacer.id })}
              onNavigate={() => {
                window.location.hash = routeHash({ view: 'decision', decisionId: replacer.id });
              }}
              class="overview-rule-related"
            >
              {t.decisions.openReplacement}
            </HashLink>
          )}
          {ruleTargetLink(d)}
          <span class="overview-rule-spacer" />
          <span class="overview-rule-at">
            {formatDecisionAt(d.at)}
          </span>
        </div>
      </div>
    );
  };
  // 「規則 (N)」展開トグル。N は**効いている**規則の数（条項5: 見出しの数と
  // 開いて見える行数が一致する）。置き換え済みは既定で畳み、0件なら口を出さない
  // （条項4）。
  // 各文脈の規則を「同じ条件で一覧としても開ける」ようにする
  // （01KYKS4Y56FAHRVCWKMQJK4RT6・概要から自然に絞り込まれた一覧へ踏む経路）。
  //
  // scopeRef はこの文脈が指す対象（part / 制約はタグ、振る舞いは遷移）。向きは
  // **own** ——インライン展開が出しているのが rulesFor（decByTarget＝その対象
  // ちょうど）なので、リンク先が同じ集合になるように揃える。ここを governing に
  // すると「展開して見える件数」と「踏んだ先の件数」が食い違う（条項5 と同じ趣旨）。
  //
  // ⚠️ このプロジェクトでは概要タブが空（役割 kind を持つタグが0件）なので、
  // **この経路は本 repo の画面では確かめられない**。設定に宣言が入るまで実機確認は
  // できないことを result.md に開示してある。
  const rulesListHref = (scopeRef: string) => routeHash({ view: 'decisions', decisionOn: scopeRef, decisionScope: 'own' });

  const renderRules = (key: string, entries: RuleEntry[], label: (n: number) => string, scopeRef: string) => {
    if (!entries.length) return null;
    const inForce = entries.filter((e) => effectOf(e.d.id, index.currencyIndex) === 'in-force');
    const replaced = entries.filter((e) => effectOf(e.d.id, index.currencyIndex) === 'replaced');
    const open = sections.isOpen(key, SEC_RULES, false);
    const histOpen = sections.isOpen(key, SEC_HISTORY, false);
    const listHref = rulesListHref(scopeRef);
    return (
      <div class="overview-rules-inline">
        <button type="button" class="overview-rules-toggle" onClick={() => sections.toggle(key, SEC_RULES, false)} aria-expanded={open}>
          <Icon name={open ? 'chevron-down' : 'chevron-right'} size={13} />
          <Icon name="gavel" size={13} />
          {label(inForce.length)}
        </button>
        <HashLink
          href={listHref}
          onNavigate={() => {
            window.location.hash = listHref;
          }}
          class="overview-rules-list-link"
          title={t.overview.openRulesListTitle}
        >
          <Icon name="scroll-text" size={12} />
          {t.overview.openRulesList}
        </HashLink>
        {open && (
          <div class="overview-rules-list">
            {inForce.map(renderRuleRow)}
            {replaced.length > 0 && (
              <div class="overview-rules-history">
                <button
                  type="button"
                  class="overview-rules-history-toggle"
                  onClick={() => sections.toggle(key, SEC_HISTORY, false)}
                  aria-expanded={histOpen}
                >
                  <Icon name={histOpen ? 'chevron-down' : 'chevron-right'} size={13} />
                  <Icon name="scroll-text" size={13} />
                  {t.decisions.replacedHeading(replaced.length)}
                </button>
                {histOpen && <div class="overview-rules-list">{replaced.map(renderRuleRow)}</div>}
              </div>
            )}
          </div>
        )}
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
          <div class="overview-tree" ref={treeRef}>
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
                {row.href ? (
                  <HashLink
                    href={row.href}
                    onNavigate={row.onClick}
                    class={'overview-tree-label kind-' + (row.kind || 'none')}
                    style={{ '--kc': kindColorVar(row.kind) } as JSX.CSSProperties}
                  >
                    <span class="overview-tree-dot" />
                    <span class="overview-tree-name">{row.name}</span>
                    {row.isGap && <Icon name="triangle-alert" size={12} class="overview-tree-gap" />}
                    {row.count != null && <span class="overview-tree-count">{row.count}</span>}
                  </HashLink>
                ) : (
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
                )}
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

            {/* 責務。かつてここに「このコンポーネントの規則」欄があったが廃止した
                （01KYHW4NBNVN9BFXYZMBX8MPF8 条項1）。あの欄はコンポーネントタグ
                自身への決定・要件タグへの決定・どの構成要素にも属さない振る舞いへの
                決定を1つに寄せた混成集合で、文脈が1つに定まらなかった——
                01KYCC2TGFSH3JCCTXSKD7JDR4 が排したはずの平坦リストの縮小再現。
                コンポーネント本体に紐づく決定はカードの意思決定欄で読む
                （右上「ブラウザで開く」）。構成要素・振る舞い・制約の文脈インライン
                展開は残す（そこはカードが文脈そのものなので宛先が自明）。 */}
            {(sheet.body || sheet.lead) && (
              <section class="overview-section">
                <div class="overview-section-head">
                  <Icon name="crosshair" size={16} class="overview-section-icon" />
                  <span class="overview-section-title">{t.overview.responsibilityHeading}</span>
                  <CommentButton recordType="tag" recordId={sheet.c.id} recordTitle={sheet.c.name || sheet.c.id} anchor="responsibility" anchorLabel={t.overview.responsibilityHeading} />
                </div>
                {sheet.body ? <Markdown text={sheet.body} class="overview-responsibility" /> : sheet.lead ? <p class="overview-responsibility">{sheet.lead}</p> : null}
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
                  const partOpen = sections.isOpen(p.id, SEC_PART, false);
                  return (
                  <div key={p.id} class="overview-part" data-part={p.id}>
                    <button type="button" class="overview-part-head" onClick={() => sections.toggle(p.id, SEC_PART, false)} aria-expanded={partOpen}>
                      <Icon name={partOpen ? 'chevron-down' : 'chevron-right'} size={15} class="overview-part-chevron" />
                      <span class="overview-part-dot" style={{ '--kc': 'var(--k-prt)' } as JSX.CSSProperties} />
                      <span class="overview-part-title">{p.name}</span>
                      <span class="overview-part-spacer" />
                      <span class="overview-part-count">{t.overview.txCount(p.txCount)}</span>
                    </button>
                    {partOpen && (
                    <div class="overview-part-body">
                    {/* ⑤: この part を target とする規則 */}
                    {renderRules('part:' + p.id, p.rules, t.overview.rulesToggle, formatScopeTarget({ type: 'tag', id: p.id }))}
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
                        {renderRules('tx:' + b.id, b.rules, t.overview.rulesToggle, formatScopeTarget({ type: 'transition', id: b.id }))}
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
                      {renderRules('prop:' + p.id, p.rules, t.overview.rulesToggle, formatScopeTarget({ type: 'tag', id: p.id }))}
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
