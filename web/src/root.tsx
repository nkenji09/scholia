import { App } from './app.tsx'
import { LookupsProvider } from './lookups'
import { PendingDiffProvider } from './pendingDiff'
import { ReviewsProvider } from './reviews'
import { CommentsProvider } from './components/comments/useComments'
import { DrawerProvider } from './drawer'
import { LangProvider } from './i18n'

// アプリの合成ルート（provider の入れ子 ＋ App）。
//
// **なぜ main.tsx から切り出すか。** main.tsx は import しただけで
// `render(..., document.getElementById('app'))` を実行する副作用モジュールなので、
// テストから読み込めない。ここが分かれていないと、描画を起こす harness
// （renderWiring.test.tsx）は provider の入れ子を**自分で書き写す**しかなく、
// 製品の合成ルートと harness の合成ルートが別物になる——「harness は緑だが
// 製品は壊れている（あるいはその逆）」が起こりうる形で、まさに
// 01KYH2533234PGSN4MDQ6ZXJHA が名指しした「共通配線を迂回して面の中で組み直す」
// の、テスト側での再演になる。
//
// 入れ子の順序そのものには意味がある（LookupsProvider は useT を使うので
// LangProvider の内側でなければならない、等）。写しを作らず、製品と harness が
// **同じ1つの木**を起こす。
export function AppRoot() {
  return (
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
    </LangProvider>
  )
}
