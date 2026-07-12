import { render } from 'preact'
import './index.css'
import { App } from './app.tsx'
import { LookupsProvider } from './lookups'
import { PendingDiffProvider } from './pendingDiff'
import { ReviewsProvider } from './reviews'
import { CommentsProvider } from './components/comments/useComments'
import { DrawerProvider } from './drawer'
import { LangProvider } from './i18n'

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
