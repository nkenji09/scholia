import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';
import { api } from './api';
import { useT } from './i18n';
import type { Config, Decision, Tag, Transition, VocabEntry } from './types';
import { kindDeclObject } from './types';
import { resolveRoleKinds, roleLabel } from './roleKinds';
import type { SheetRole } from './roleKinds';
import { buildCurrencyIndex, formatDecisionAt as formatDecisionAtWithZone } from './components/decisions/decisionModel';
import type { CurrencyIndex } from './components/decisions/decisionModel';

const EMPTY_TAG_KIND_LABELS: Record<string, string> = {};

// Built-in fallbacks for the additive Display config (2026-07-11 tweaks5
// §1/§2) — used when config.display is unset/predates the field, or when
// a specific field within it is empty. Kept here (not scattered across
// Header.tsx/HomeView.tsx) so there is exactly one place that decides what
// "unset" means.
const DEFAULT_PRODUCT_NAME = 'scholia';
const DEFAULT_SUBTITLE = 'scholia';

// viewer-overview-browser: 概要ビュー（仕様シート）が依存する4つの「役割」の解決は
// ./roleKinds へ切り出した（画面を起こさずに入力→出力で検査できるようにするため）。
export type { SheetRole } from './roleKinds';

// Internal record ids (T-mfa-verify, tag/vocab ids) are the join keys the
// UI navigates by, but v2 feedback was explicit: people reading the viewer
// should see names/labels, not ids (調整3). This module fetches vocab/tags/
// transitions once at app startup and exposes id → human-readable-label
// lookups so every view can resolve a label without re-fetching or
// re-deriving the id → label mapping itself.
interface Lookups {
  ready: boolean;
  /** 全 decision から一度だけ組む現行性索引（01KYHW54B8ZXH0NEPH2J7N1X39）。
      効力の判定は「他の decision が supersede でこれを置き換えたか」＝**全体を
      見ないと決まらない**ので、カードごとに引き直さず app 起動時に1つ作って
      共有する。カードごとに部分集合から判定すると、その集合の外にいる
      置き換え側を見落として「置き換え済みのものを効いていると表示する」。 */
  currencyIndex: CurrencyIndex;
  vocabById: Map<string, VocabEntry>;
  tagById: Map<string, Tag>;
  transitionById: Map<string, Transition>;
  /** VocabEntry.Label, falling back to the id when unknown/unresolved (見せる情報がラベルしかない場合のみ id を出す)。 */
  vocabLabel: (id: string) => string;
  /** Tag.Name, falling back to the id when unknown/unresolved. */
  tagName: (id: string) => string;
  /** A transition's human-readable headline: its action's label, plus a short "→ then…" summary. */
  transitionLabel: (txId: string) => { primary: string; secondary?: string };
  /** Turns a raw `GET /api/search` matchedOn entry ("tag:x" / "vocab:x" / "kind:x" / "id") into localized prose instead of a bare id. */
  describeMatch: (matchedOn: string) => string;
  /** config.tagKindLabels[kind], falling back to the bare kind id when
      unset — the single place tagKind display labels get resolved
      (2026-07-11 tweaks3 §2). Every kind badge/facet-label in the UI must
      route through this rather than reading Config.tagKindLabels
      directly, so a future design change to the fallback rule only
      touches one function. */
  tagKindLabel: (kind: string | undefined) => string;
  /** #45 D9: config.tagKinds の object 宣言 description（kind バッジ tooltip 用）。
      未宣言 / string 宣言 / description 空のときは undefined。 */
  tagKindDescription: (kind: string | undefined) => string | undefined;
  /** #45 D9: config.ownerKind（オプトイン）。非空のとき owner は subject タグ id
      参照＝正準ルートを持つ。未宣言（""）なら owner は自由文字列のまま。 */
  ownerKind: string;
  /** viewer-overview-browser: 概要ビューの4役割（component/part/constraint/
      group）→ 実 kind id の前計算マップ。config.tagKinds の behaviors 宣言で
      上書き、無ければリテラル id（constraint→property）へフォールバック。役割で
      比較したい箇所は kind リテラルを直書きせずこれを引く（1箇所で解決）。 */
  roleKinds: Record<SheetRole, string>;
  /** その役割が behaviors 宣言で解決されたか（false＝フォールバックに落ちた＝
      プロジェクトがまだ役割を宣言していない）。「タグがまだ無い」と「役割が
      宣言されていない」は利用者のやることが違うので、文言を分けるために要る。 */
  roleDeclared: Record<SheetRole, boolean>;
  /** 役割 component を担う kind の**表示上の呼び名**。宣言があればその kind の
      ラベル（tagKindLabels 経由）、宣言が無ければ空文字。
      ⚠️ **画面に「コンポーネント」等の役割名を書くときは、必ずここを通す。**
      01KYCC2THS5RX3HB27SQGFWSA5 が「役割はリテラル kind id 固定でなく宣言で
      解決する」と定めた以上、その役割の呼び名も config が決める。空文字のときは
      呼び名が存在しない＝役割名を含まない別の文言で語る（強引に id を出さない・
      01KYCC2TF3NW3JRSSRK9ZHN078）。 */
  componentRoleLabel: string;
  /** Header's product name: config.display.productName, falling back to
      "scholia" (2026-07-11 tweaks5 §2). */
  productName: string;
  /** Header's subtitle: the live config.branch (current git branch),
      falling back to "scholia" when the project isn't a git repo,
      HEAD is detached, or git failed (2026-07-11 tweaks5 §2). */
  headerSubtitle: string;
  /** HOME's tagline: config.display.tagline, falling back to the built-in
      copy (2026-07-11 tweaks5 §1). */
  tagline: string;
  /** HOME's intro paragraph, same resolution rule as tagline. */
  intro: string;
  /** decision.at (ISO 8601 UTC) → "YYYY-MM-DD HH:MM" in config.timezone,
      falling back to UTC when unset (req.comfortable-viewer.config-editing
      amend). Bound here (not a raw config.timezone read) so every screen
      renders a decision's time the same way without threading config
      through itself. */
  formatDecisionAt: (at: string) => string;
}

const LookupsContext = createContext<Lookups | null>(null);

function composeTransitionLabel(t: Transition | undefined, txId: string, vocabLabel: (id: string) => string) {
  if (!t) return { primary: txId };
  const primary = vocabLabel(t.action);
  const secondary = t.then.length > 0 ? `→ ${t.then.map(vocabLabel).join('、')}` : undefined;
  return { primary, secondary };
}

export function LookupsProvider({ children }: { children: ComponentChildren }) {
  const t = useT();
  const [vocabById, setVocabById] = useState<Map<string, VocabEntry>>(new Map());
  const [tagById, setTagById] = useState<Map<string, Tag>>(new Map());
  const [transitionById, setTransitionById] = useState<Map<string, Transition>>(new Map());
  const [tagKindLabels, setTagKindLabels] = useState<Record<string, string>>(EMPTY_TAG_KIND_LABELS);
  const [config, setConfig] = useState<Config | null>(null);
  const [decisions, setDecisions] = useState<Decision[]>([]);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    Promise.all([api.getVocab(), api.getTags(), api.getTransitions({}), api.getConfig(), api.getRules({})])
      .then(([vocab, tags, tx, config, rules]) => {
        setVocabById(new Map(vocab.map((v) => [v.id, v])));
        setTagById(new Map(tags.map((t) => [t.id, t])));
        setTransitionById(new Map((tx.transitions || []).map((t) => [t.id, t])));
        setTagKindLabels(config.tagKindLabels || EMPTY_TAG_KIND_LABELS);
        setConfig(config);
        setDecisions(rules.decisions || []);
        setReady(true);
      })
      .catch(() => {
        // Views that need this data surface their own fetch errors already
        // (they call the same api.* functions); lookups degrade to
        // id-fallback labels rather than blocking the whole app on a second
        // failure of the same request.
        setReady(true);
      });
  }, []);

  const vocabLabel = (id: string) => vocabById.get(id)?.label || id;
  const tagName = (id: string) => tagById.get(id)?.name || id;
  const transitionLabel = (txId: string) => composeTransitionLabel(transitionById.get(txId), txId, vocabLabel);
  const tagKindLabel = (kind: string | undefined) => (kind && tagKindLabels[kind]) || kind || '';

  // #45 D9: kind バッジ tooltip 用の description 索引（tagKinds の object 宣言から）。
  const tagKindDescriptions: Record<string, string> = {};
  for (const decl of config?.tagKinds || []) {
    const o = kindDeclObject(decl);
    if (o.description) tagKindDescriptions[o.id] = o.description;
  }
  const tagKindDescription = (kind: string | undefined) => (kind ? tagKindDescriptions[kind] : undefined);
  const ownerKind = config?.ownerKind || '';

  // viewer-overview-browser: 役割 → 実 kind id と、その呼び名（判定は ./roleKinds）。
  const resolvedRoles = resolveRoleKinds(config?.tagKinds);
  const { kinds: roleKinds, declared: roleDeclared } = resolvedRoles;
  const componentRoleLabel = roleLabel(resolvedRoles, 'component', tagKindLabels);

  const describeMatch = (matchedOn: string) => {
    if (matchedOn === 'id') return t.lookups.searchById;
    const [prefix, ...rest] = matchedOn.split(':');
    const id = rest.join(':');
    if (prefix === 'tag') return `${t.lookups.tagPrefix}${tagName(id)}`;
    if (prefix === 'vocab') return `${t.lookups.vocabPrefix}${vocabLabel(id)}`;
    if (prefix === 'kind') return `${t.lookups.kindPrefix}${id}`;
    return matchedOn;
  };

  const productName = config?.display?.productName || DEFAULT_PRODUCT_NAME;
  const headerSubtitle = config?.branch || DEFAULT_SUBTITLE;
  const tagline = config?.display?.tagline || t.home.tagline;
  const intro = config?.display?.intro || t.home.intro;
  // effectiveTimezone (not the raw project `timezone`) already folds in
  // this machine's config.local.json override, if any (req.comfortable-
  // viewer.config-editing amend) — reading `timezone` directly here would
  // silently ignore it.
  const formatDecisionAt = (at: string) => formatDecisionAtWithZone(at, config?.effectiveTimezone);

  const currencyIndex = buildCurrencyIndex(decisions);

  const value: Lookups = {
    ready,
    currencyIndex,
    vocabById,
    tagById,
    transitionById,
    vocabLabel,
    tagName,
    transitionLabel,
    describeMatch,
    tagKindLabel,
    tagKindDescription,
    ownerKind,
    roleKinds,
    roleDeclared,
    componentRoleLabel,
    productName,
    headerSubtitle,
    tagline,
    intro,
    formatDecisionAt,
  };
  return <LookupsContext.Provider value={value}>{children}</LookupsContext.Provider>;
}

export function useLookups(): Lookups {
  const ctx = useContext(LookupsContext);
  if (!ctx) throw new Error('useLookups() must be called within a LookupsProvider');
  return ctx;
}
