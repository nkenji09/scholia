import type { ComponentChildren } from 'preact';

// A modified click (Cmd/Ctrl/Shift or middle-button) is the browser's own
// "open this link elsewhere" gesture — new tab, new window, copy. When one of
// these is held we must NOT intercept: let the anchor's default behavior run so
// the href opens as a real link (deep-linking restores focus/scroll there).
// Plain left clicks are the in-app SPA nav case, handled by HashLink below.
export function isModifiedClick(e: MouseEvent): boolean {
  return e.metaKey || e.ctrlKey || e.shiftKey || e.button === 1;
}

interface Props {
  /** In-app hash route (e.g. `#/browse/tx/<id>`), built via router.ts's
      routeHash so it round-trips through parseRoute on the other side. */
  href: string;
  /** Runs on a plain left click only — the existing SPA navigation (keeps the
      view's focus/scroll behavior). Modified/middle clicks skip this and fall
      through to the browser (open in new tab/window, copy link). */
  onNavigate: () => void;
  class?: string;
  title?: string;
  children: ComponentChildren;
}

// Renders in-page navigation as a genuine <a href> rather than a button, so the
// same targets that plain-click navigate within the SPA also support Cmd/Ctrl+
// click (new tab), middle-click, and right-click "copy link" — the deep-linking
// contract (any focus is a shareable URL) extended to the links themselves.
export function HashLink({ href, onNavigate, class: className, title, children }: Props) {
  const onClick = (e: MouseEvent) => {
    // Modified/middle click → leave the browser's default link handling alone.
    if (isModifiedClick(e)) return;
    // Plain left click → in-app SPA nav (preserve existing focus/scroll).
    e.preventDefault();
    onNavigate();
  };
  return (
    <a href={href} class={className} title={title} onClick={onClick}>
      {children}
    </a>
  );
}
