import { useEffect } from 'preact/hooks';

// Shared background-scroll lock for full-screen overlays (the off-canvas
// BrowseRail drawer and the CommentPanel slide-over both need it — #20
// drawerscroll fix, 2026-07-11). `overflow:hidden` alone doesn't stop iOS
// Safari's rubber-band scroll from reaching content behind a fixed overlay
// ("scroll-through"), so <body> itself is pinned with position:fixed at its
// current scroll offset while `active` is true, and restored (scrolled back
// to the same position) once it goes false.
//
// This only ever touches <body> — the overlay's own scroll container keeps
// its normal overflow-y:auto (plus -webkit-overflow-scrolling:touch/
// overscroll-behavior:contain declared in its CSS) so it can still scroll
// internally; pinning body does not, by itself, disable touch-scrolling
// inside a position:fixed descendant.
export function useBodyScrollLock(active: boolean) {
  useEffect(() => {
    if (!active) return;
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
  }, [active]);
}
