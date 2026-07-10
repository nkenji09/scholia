# web

ビューア SPA のソース（Preact + Vite）。`npm run build` の出力 `dist/` を `internal/viewer` が `go:embed` で焼き込む。

```
npm install
npm run dev     # ローカル開発（vite dev server。API は pmem view 側で別途起動）
npm run build   # dist/ を生成（コミット対象・DESIGN §9/§10）
```
