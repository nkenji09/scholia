import { useEffect, useState } from 'preact/hooks';

// Hash-based routing so Back/Forward work in both `pmem view` (served over
// HTTP) and a `pmem export --html` file opened via file:// or a plain static
// file server. History.pushState is unreliable on file:// in some browsers;
// assigning `location.hash` is not — it always both updates the visible URL
// and pushes a browser history entry, with no server round-trip, which is
// exactly the "static export with working Back/Forward" behavior this needs.

// 'traceability'/'compare' removed (2026-07-11, user request): not covered
// by the Claude Design mock and dropped from the nav for now — trivially
// restorable from git history (internal/viewer's /api/traceability and
// /api/diff endpoints are untouched; only this frontend surface is gone).
export type ViewName = 'home' | 'browse' | 'vocab' | 'spec' | 'tags' | 'config';

export interface Route {
  view: ViewName;
  tagId?: string;
  txId?: string;
}

const VIEWS: ViewName[] = ['home', 'browse', 'vocab', 'spec', 'tags', 'config'];
// HOME is the new landing page (.concierge/decision.md G-2, resolved:
// default route moves from 'browse' to 'home'). An empty/unknown hash still
// falls back to DEFAULT_ROUTE below, so bookmarks of `#` or the bare page
// URL land on HOME now instead of Browse — every other existing route
// (#/browse/..., #/spec/..., etc.) is unaffected since parseRoute only
// consults DEFAULT_ROUTE when the hash's view segment is absent or unknown.
const DEFAULT_ROUTE: Route = { view: 'home' };

function isViewName(s: string): s is ViewName {
  return (VIEWS as string[]).includes(s);
}

export function parseRoute(hash: string): Route {
  const raw = hash.replace(/^#\/?/, '');
  if (!raw) return DEFAULT_ROUTE;
  const parts = raw.split('/').filter((p) => p.length > 0).map(decodeURIComponent);
  const view = parts[0];
  if (!isViewName(view)) return DEFAULT_ROUTE;

  const route: Route = { view };
  switch (view) {
    case 'browse':
      for (let i = 1; i < parts.length - 1; i += 2) {
        if (parts[i] === 'tag') route.tagId = parts[i + 1];
        else if (parts[i] === 'tx') route.txId = parts[i + 1];
      }
      break;
    case 'spec':
      if (parts[1]) route.tagId = parts[1];
      break;
  }
  return route;
}

export function routeHash(route: Route): string {
  const seg: string[] = [route.view];
  switch (route.view) {
    case 'browse':
      if (route.tagId) seg.push('tag', encodeURIComponent(route.tagId));
      if (route.txId) seg.push('tx', encodeURIComponent(route.txId));
      break;
    case 'spec':
      if (route.tagId) seg.push(encodeURIComponent(route.tagId));
      break;
  }
  return `#/${seg.join('/')}`;
}

function currentRoute(): Route {
  return parseRoute(window.location.hash);
}

export function useHashRoute(): [Route, (route: Route) => void] {
  const [route, setRoute] = useState<Route>(currentRoute);

  useEffect(() => {
    const onHashChange = () => setRoute(currentRoute());
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  const navigate = (next: Route) => {
    const hash = routeHash(next);
    if (window.location.hash === hash) {
      setRoute(next);
      return;
    }
    // Triggers the 'hashchange' listener above, which updates `route`; a
    // new browser history entry is pushed as a side effect of the
    // assignment itself (see module comment).
    window.location.hash = hash;
  };

  return [route, navigate];
}
