import { defineConfig } from 'vitest/config'
import preact from '@preact/preset-vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [preact()],
  test: {
    // Vitest blanks every `*.css` import by default (even with `?raw`) unless
    // explicitly opted in — see CSSEnablerPlugin in vitest's own source. Tests
    // that read real stylesheet text (e.g. decisionSummarySurfaces.test.ts,
    // which checks for CSS class-name collisions across view stylesheets)
    // need the raw source, not a blanked-out module.
    css: {
      include: [/\?raw(?:&|$)/],
    },
  },
  resolve: {
    alias: {
      react: 'preact/compat',
      'react-dom': 'preact/compat',
    },
  },
  build: {
    // Vite's dynamic-import preload helper would otherwise insert
    // <link rel="modulepreload"> (or a CSS preload) for a lazily-imported
    // chunk's own static deps before running it. `scholia export --html`
    // inlines every reachable chunk into one self-contained file with no
    // sibling assets at all (see internal/render/export.go) — those preload
    // fetches would only ever 404 there. Disabling this keeps every dynamic
    // import() (ours in Markdown.tsx, and mermaid's own internal
    // per-diagram-type ones) a plain pass-through call instead.
    modulePreload: false,
    // Some of mermaid's own async diagram-type chunks otherwise still carry
    // a hardcoded (non-empty even with modulePreload:false) dependency-
    // preload entry pointing at this app's own main CSS file, for reasons
    // internal to Rolldown's per-chunk CSS association — cssCodeSplit:false
    // removes any such association since there's nothing left to split (this
    // app already has exactly one global CSS file). That preload attempt
    // isn't just a wasted request offline: resolving its URL against
    // import.meta.url throws synchronously for a chunk loaded from a Blob
    // URL (see internal/render/export_bundle.go), which would otherwise
    // break that diagram type's lazy load entirely under `scholia export
    // --html`.
    cssCodeSplit: false,
  },
})
