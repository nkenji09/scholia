import { render } from 'preact'
import './index.css'
import { App } from './app.tsx'
import { LookupsProvider } from './lookups'

render(
  <LookupsProvider>
    <App />
  </LookupsProvider>,
  document.getElementById('app')!,
)
