import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';
import { useBodyScrollLock } from './scrollLock';

// Off-canvas rail / sticky-rail responsive state (design reference
// pmem-viewer.dc.html: componentDidMount's `this.mql`, ~line 640, and the
// railStyle/showBackdrop/showFilterToggle wiring around lines 866/972-980).
// A single context because the breakpoint crossing and the drawer's open
// state need to be seen by both Header (the 絞り込み toggle button) and
// BrowseRail/BrowseView/VocabView (the drawer itself + every action that
// closes it) — plain props would mean threading this through app.tsx into
// every view.

// Design's own breakpoint: `window.matchMedia('(max-width: 860px)')`.
const BREAKPOINT = 860;

interface Drawer {
  isNarrow: boolean;
  drawerOpen: boolean;
  openDrawer: () => void;
  closeDrawer: () => void;
  toggleDrawer: () => void;
}

const DrawerContext = createContext<Drawer | null>(null);

export function DrawerProvider({ children }: { children: ComponentChildren }) {
  const [isNarrow, setIsNarrow] = useState(() => (typeof window !== 'undefined' ? window.matchMedia(`(max-width: ${BREAKPOINT}px)`).matches : false));
  const [drawerOpen, setDrawerOpen] = useState(false);

  useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${BREAKPOINT}px)`);
    // Design closes the drawer whenever the narrow/wide boundary is
    // crossed (its _onMql sets both isNarrow and drawerOpen:false together)
    // — avoids a stuck-open drawer if the viewport crosses the breakpoint
    // (window resize, device rotation) while it happened to be open.
    const onChange = () => {
      setIsNarrow(mql.matches);
      setDrawerOpen(false);
    };
    setIsNarrow(mql.matches);
    if (mql.addEventListener) mql.addEventListener('change', onChange);
    else mql.addListener(onChange);
    return () => {
      if (mql.removeEventListener) mql.removeEventListener('change', onChange);
      else mql.removeListener(onChange);
    };
  }, []);

  // Lock background scroll while the drawer is open on narrow viewports
  // (2026-07-11 tweaks5 §6; extracted into a shared hook in #20 so
  // CommentPanel can lock the same way — see scrollLock.ts's doc comment
  // for why this doesn't also kill the drawer's own internal scroll).
  useBodyScrollLock(isNarrow && drawerOpen);

  const value: Drawer = {
    isNarrow,
    drawerOpen,
    openDrawer: () => setDrawerOpen(true),
    closeDrawer: () => setDrawerOpen(false),
    toggleDrawer: () => setDrawerOpen((o) => !o),
  };
  return <DrawerContext.Provider value={value}>{children}</DrawerContext.Provider>;
}

export function useDrawer(): Drawer {
  const ctx = useContext(DrawerContext);
  if (!ctx) throw new Error('useDrawer() must be called within a DrawerProvider');
  return ctx;
}
