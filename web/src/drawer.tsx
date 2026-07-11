import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';

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
  // (2026-07-11 tweaks5 §6). `overflow:hidden` alone doesn't stop iOS
  // Safari's rubber-band scroll from reaching content behind a fixed
  // overlay ("scroll-through") — pinning <body> itself with
  // position:fixed at its current scroll offset is the standard
  // workaround, so it's restored (scrolled back to the same position)
  // when the drawer closes instead of jumping to the top.
  useEffect(() => {
    if (!(isNarrow && drawerOpen)) return;
    const scrollY = window.scrollY;
    const body = document.body.style;
    const prev = { position: body.position, top: body.top, left: body.left, right: body.right, width: body.width, overflow: body.overflow };
    body.position = 'fixed';
    body.top = `-${scrollY}px`;
    body.left = '0';
    body.right = '0';
    body.width = '100%';
    body.overflow = 'hidden';
    return () => {
      body.position = prev.position;
      body.top = prev.top;
      body.left = prev.left;
      body.right = prev.right;
      body.width = prev.width;
      body.overflow = prev.overflow;
      window.scrollTo(0, scrollY);
    };
  }, [isNarrow, drawerOpen]);

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
