import { render } from 'preact'
import './index.css'
import { App } from './app.tsx'
import { LookupsProvider } from './lookups'
import { PendingDiffProvider } from './pendingDiff'
import { CommentsProvider } from './components/comments/useComments'
import { DrawerProvider } from './drawer'
import { LangProvider } from './i18n'

render(
  <LangProvider>
    <LookupsProvider>
      <PendingDiffProvider>
        <CommentsProvider>
          <DrawerProvider>
            <App />
          </DrawerProvider>
        </CommentsProvider>
      </PendingDiffProvider>
    </LookupsProvider>
  </LangProvider>,
  document.getElementById('app')!,
)
