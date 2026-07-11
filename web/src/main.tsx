import { render } from 'preact'
import './index.css'
import { App } from './app.tsx'
import { LookupsProvider } from './lookups'
import { CommentsProvider } from './components/comments/useComments'
import { DrawerProvider } from './drawer'

render(
  <LookupsProvider>
    <CommentsProvider>
      <DrawerProvider>
        <App />
      </DrawerProvider>
    </CommentsProvider>
  </LookupsProvider>,
  document.getElementById('app')!,
)
