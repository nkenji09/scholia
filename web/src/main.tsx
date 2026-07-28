import { render } from 'preact'
import './index.css'
import { AppRoot } from './root'

// We own scroll restoration across view round-trips and reloads (per-view
// sessionStorage, see scrollRestore.ts). Turn off the browser's built-in
// restoration so it doesn't race our restore and yank a reloaded view back to
// the top after we've positioned it (view-state-continuity).
if ('scrollRestoration' in history) history.scrollRestoration = 'manual';

// provider の入れ子は root.tsx（AppRoot）が1箇所で持つ。ここが持っていると
// 描画を起こすテストから読み込めない（このモジュールは import しただけで
// render する副作用モジュールなので）——理由は root.tsx の冒頭に書いてある。
render(<AppRoot />, document.getElementById('app')!)
