import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useMemo, useState } from 'preact/hooks';
import { api, isStaticMode } from './api';
import type { DiffChange, DiffResult, Tag, Transition, TransitionChange, VocabEntry } from './types';

// Pending diff (現状 vs 提案) — change-cockpit-design-v3.md §2/§5 P2. base=
// 'main' (never HEAD — §2 is explicit that the evaluation question is "what
// does this proposal change relative to base", not relative to the last
// commit). Shared Context (same shape as lookups.tsx) rather than a
// drawer-local fetch: SpecCard's clean-flag needs to know which transitions
// have a pending change *before* any drawer is opened, and ProposalCard
// needs the same data once a drawer opens — one fetch, two consumers.
const BASE_REF = 'main';

interface PendingDiff {
  ready: boolean;
  /** Why the diff isn't available, or null when it is. 'static' = pmem
      export --html (no server, no other ref to compare against — same
      constraint as every other api.ts getter). 'error' = server mode but
      the fetch failed (e.g. `main` doesn't resolve). Either way this must
      never block comments/task functionality, which don't depend on this
      module at all. */
  unavailable: 'static' | 'error' | null;
  changedTransitionIds: Set<string>;
  getChange: (txId: string) => TransitionChange | undefined;
  /** §8.8 P5・M-5「追加」: ids of transitions present in the working tree
      but not at base — the subject spec list decorates these as a green
      "新規 Transition（提案）" card (§3's 3-種別表). */
  addedTransitionIds: Set<string>;
  /** #32 A是正: `addedTransitionIds` に属する transition の全内容（after
      のみ・before は存在しない）。ProposalCard の after-only（全追加）描画
      が working-tree の現在値を取るのに使う — useLookups().transitionById
      は起動時 1 回のキャッシュで新規追加を拾えないことがあるため、必ず
      pending diff（refresh() で都度取り直す）側から引く。 */
  getAddedTransition: (txId: string) => Transition | undefined;
  /** §8.8 P5・M-5「削除」: full records (base-side) of transitions present
      at base but removed from the working tree — these no longer resolve
      via GET /api/transitions/{id}, so the tombstone card renders straight
      from this diff-supplied Before snapshot rather than a TransitionDetail. */
  removedTransitions: Transition[];
  /** §8.8 P5・M-5 一般化: vocab/tag の変更版 changedTransitionIds/getChange
      （`recordType` に応じて transition/vocab/tag のいずれかを見る
      isProposalComment/clean-flag の derive 元。§8.2/§8.3 を全種別へ）。 */
  changedVocabIds: Set<string>;
  getVocabChange: (id: string) => DiffChange<VocabEntry> | undefined;
  /** §8.8 P5・M-5「追加/削除」の vocab 版（addedTransitionIds/removedTransitions と対称）。 */
  addedVocabIds: Set<string>;
  /** #32 A是正: getAddedTransition の vocab 版。 */
  getAddedVocab: (id: string) => VocabEntry | undefined;
  removedVocab: VocabEntry[];
  changedTagIds: Set<string>;
  getTagChange: (id: string) => DiffChange<Tag> | undefined;
  addedTagIds: Set<string>;
  /** #32 A是正: getAddedTransition の tag 版。 */
  getAddedTag: (id: string) => Tag | undefined;
  removedTags: Tag[];
  /** Bumped on every refresh() — a plain re-render trigger other views
      (BrowseView's specs-facet transition list) key their own refetch off
      of, so a create/delete elsewhere doesn't require prop-drilling a
      dedicated callback down to wherever the mutation happened (comment
      drawer, subject list "+ 新規" entry, …). */
  version: number;
  refresh: () => void;
}

const PendingDiffContext = createContext<PendingDiff | null>(null);

export function PendingDiffProvider({ children }: { children: ComponentChildren }) {
  const [result, setResult] = useState<DiffResult | null>(null);
  const [ready, setReady] = useState(false);
  const [unavailable, setUnavailable] = useState<'static' | 'error' | null>(isStaticMode ? 'static' : null);
  const [version, setVersion] = useState(0);

  const load = () => {
    if (isStaticMode) {
      setUnavailable('static');
      setReady(true);
      setVersion((v) => v + 1);
      return;
    }
    api
      .getDiff({ ref: BASE_REF })
      .then((r) => {
        setResult(r);
        setUnavailable(null);
        setReady(true);
        setVersion((v) => v + 1);
      })
      .catch(() => {
        setUnavailable('error');
        setReady(true);
        setVersion((v) => v + 1);
      });
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(load, []);

  const changedTransitionIds = useMemo(() => new Set((result?.transitions.changed || []).map((c) => c.id)), [result]);
  const addedTransitionIds = useMemo(() => new Set((result?.transitions.added || []).map((tx) => tx.id)), [result]);
  const removedTransitions = useMemo(() => result?.transitions.removed || [], [result]);

  const changedVocabIds = useMemo(() => new Set((result?.vocab.changed || []).map((c) => c.id)), [result]);
  const addedVocabIds = useMemo(() => new Set((result?.vocab.added || []).map((v) => v.id)), [result]);
  const removedVocab = useMemo(() => result?.vocab.removed || [], [result]);

  const changedTagIds = useMemo(() => new Set((result?.tags.changed || []).map((c) => c.id)), [result]);
  const addedTagIds = useMemo(() => new Set((result?.tags.added || []).map((tg) => tg.id)), [result]);
  const removedTags = useMemo(() => result?.tags.removed || [], [result]);

  const getChange = (txId: string) => result?.transitions.changed?.find((c) => c.id === txId);
  const getVocabChange = (id: string) => result?.vocab.changed?.find((c) => c.id === id);
  const getTagChange = (id: string) => result?.tags.changed?.find((c) => c.id === id);
  const getAddedTransition = (txId: string) => result?.transitions.added?.find((tx) => tx.id === txId);
  const getAddedVocab = (id: string) => result?.vocab.added?.find((v) => v.id === id);
  const getAddedTag = (id: string) => result?.tags.added?.find((tg) => tg.id === id);

  const value: PendingDiff = {
    ready,
    unavailable,
    changedTransitionIds,
    getChange,
    addedTransitionIds,
    getAddedTransition,
    removedTransitions,
    changedVocabIds,
    getVocabChange,
    addedVocabIds,
    getAddedVocab,
    removedVocab,
    changedTagIds,
    getTagChange,
    addedTagIds,
    getAddedTag,
    removedTags,
    version,
    refresh: load,
  };
  return <PendingDiffContext.Provider value={value}>{children}</PendingDiffContext.Provider>;
}

export function usePendingDiff(): PendingDiff {
  const ctx = useContext(PendingDiffContext);
  if (!ctx) throw new Error('usePendingDiff() must be called within a PendingDiffProvider');
  return ctx;
}
