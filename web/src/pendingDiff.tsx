import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useMemo, useState } from 'preact/hooks';
import { api, isStaticMode } from './api';
import type { DiffResult, TransitionChange } from './types';

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
  refresh: () => void;
}

const PendingDiffContext = createContext<PendingDiff | null>(null);

export function PendingDiffProvider({ children }: { children: ComponentChildren }) {
  const [result, setResult] = useState<DiffResult | null>(null);
  const [ready, setReady] = useState(false);
  const [unavailable, setUnavailable] = useState<'static' | 'error' | null>(isStaticMode ? 'static' : null);

  const load = () => {
    if (isStaticMode) {
      setUnavailable('static');
      setReady(true);
      return;
    }
    api
      .getDiff({ ref: BASE_REF })
      .then((r) => {
        setResult(r);
        setUnavailable(null);
        setReady(true);
      })
      .catch(() => {
        setUnavailable('error');
        setReady(true);
      });
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(load, []);

  const changedTransitionIds = useMemo(() => new Set((result?.transitions.changed || []).map((c) => c.id)), [result]);

  const getChange = (txId: string) => result?.transitions.changed?.find((c) => c.id === txId);

  const value: PendingDiff = { ready, unavailable, changedTransitionIds, getChange, refresh: load };
  return <PendingDiffContext.Provider value={value}>{children}</PendingDiffContext.Provider>;
}

export function usePendingDiff(): PendingDiff {
  const ctx = useContext(PendingDiffContext);
  if (!ctx) throw new Error('usePendingDiff() must be called within a PendingDiffProvider');
  return ctx;
}
