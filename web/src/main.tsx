import { render } from 'preact'
import './index.css'
import { App } from './app.tsx'
import { LookupsProvider } from './lookups'
import { PendingDiffProvider } from './pendingDiff'
import { ReviewsProvider } from './reviews'
import { CommentsProvider } from './components/comments/useComments'
import { DrawerProvider } from './drawer'
import { LangProvider } from './i18n'

// We own scroll restoration across view round-trips and reloads (per-view
// sessionStorage, see scrollRestore.ts). Turn off the browser's built-in
// restoration so it doesn't race our restore and yank a reloaded view back to
// the top after we've positioned it (view-state-continuity).
if ('scrollRestoration' in history) history.scrollRestoration = 'manual';

render(
  <LangProvider>
    <LookupsProvider>
      <PendingDiffProvider>
        <ReviewsProvider>
          <CommentsProvider>
            <DrawerProvider>
              <App />
            </DrawerProvider>
          </CommentsProvider>
        </ReviewsProvider>
      </PendingDiffProvider>
    </LookupsProvider>
  </LangProvider>,
  document.getElementById('app')!,
)
