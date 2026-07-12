import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';
import { api } from './api';
import { useT } from './i18n';
import type { Config, Tag, Transition, VocabEntry } from './types';

const EMPTY_TAG_KIND_LABELS: Record<string, string> = {};

// Built-in fallbacks for the additive Display config (2026-07-11 tweaks5
// §1/§2) — used when config.display is unset/predates the field, or when
// a specific field within it is empty. Kept here (not scattered across
// Header.tsx/HomeView.tsx) so there is exactly one place that decides what
// "unset" means.
const DEFAULT_PRODUCT_NAME = 'pmem';
const DEFAULT_SUBTITLE = 'product-memory';

// Internal record ids (T-mfa-verify, tag/vocab ids) are the join keys the
// UI navigates by, but v2 feedback was explicit: people reading the viewer
// should see names/labels, not ids (調整3). This module fetches vocab/tags/
// transitions once at app startup and exposes id → human-readable-label
// lookups so every view can resolve a label without re-fetching or
// re-deriving the id → label mapping itself.
interface Lookups {
  ready: boolean;
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
  /** Header's product name: config.display.productName, falling back to
      "pmem" (2026-07-11 tweaks5 §2). */
  productName: string;
  /** Header's subtitle: the live config.branch (current git branch),
      falling back to "product-memory" when the project isn't a git repo,
      HEAD is detached, or git failed (2026-07-11 tweaks5 §2). */
  headerSubtitle: string;
  /** HOME's tagline: config.display.tagline, falling back to the built-in
      copy (2026-07-11 tweaks5 §1). */
  tagline: string;
  /** HOME's intro paragraph, same resolution rule as tagline. */
  intro: string;
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
  const [ready, setReady] = useState(false);

  useEffect(() => {
    Promise.all([api.getVocab(), api.getTags(), api.getTransitions({}), api.getConfig()])
      .then(([vocab, tags, tx, config]) => {
        setVocabById(new Map(vocab.map((v) => [v.id, v])));
        setTagById(new Map(tags.map((t) => [t.id, t])));
        setTransitionById(new Map((tx.transitions || []).map((t) => [t.id, t])));
        setTagKindLabels(config.tagKindLabels || EMPTY_TAG_KIND_LABELS);
        setConfig(config);
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

  const value: Lookups = {
    ready,
    vocabById,
    tagById,
    transitionById,
    vocabLabel,
    tagName,
    transitionLabel,
    describeMatch,
    tagKindLabel,
    productName,
    headerSubtitle,
    tagline,
    intro,
  };
  return <LookupsContext.Provider value={value}>{children}</LookupsContext.Provider>;
}

export function useLookups(): Lookups {
  const ctx = useContext(LookupsContext);
  if (!ctx) throw new Error('useLookups() must be called within a LookupsProvider');
  return ctx;
}
