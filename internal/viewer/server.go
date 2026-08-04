package viewer

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/nkenji09/scholia/internal/store"
	webdist "github.com/nkenji09/scholia/web"
)

// NewHandler builds the HTTP handler for `scholia view`: the JSON API under
// /api/ plus the embedded SPA (with client-side-routing fallback to
// index.html) for everything else.
func NewHandler(s *store.Store) (http.Handler, error) {
	distFS, err := fs.Sub(webdist.FS, "dist")
	if err != nil {
		return nil, err
	}

	// The API routes live on their own sub-mux with no catch-all pattern, so
	// Go's ServeMux applies its built-in 404 (unmatched path) / 405
	// (matched path, wrong method) behavior instead of falling through to
	// the SPA's "/" handler — jsonAPIHandler then re-emits that outcome as
	// JSON instead of stdlib's plain-text body (§7: /api/ is a JSON
	// contract, not part of the SPA's route space).
	apiMux, _ := buildAPIMux(s)

	root := http.NewServeMux()
	root.Handle("/api/", jsonAPIHandler{mux: apiMux})
	root.Handle("/", spaHandler{fs: distFS})
	return root, nil
}

// routeMux は登録された pattern を控えながら ServeMux へ配る薄い包み。
//
// 控えるのは**歯止めのため**である。共有スナップショットを同時に読む口が
// 増えたとき、その口が同時要求ガードを通っているかは「ガードの表に足したか」
// でしか決まらず、**足し忘れは誰にも止められなかった**——実際、この単位が
// 新設した3つの一括の口は、置いたばかりの同時要求ガードを1つも通っていなかった
// （`CLAUDE.md` 5「新しく作った面には、ガードを置き忘れる」）。
//
// ここで pattern を控えておくと、ガードは**登録された口そのもの**を回れる。
// 表に無い pattern が現れたらガードが落ちるので、口を足した人はそこで気づく。
type routeMux struct {
	mux      *http.ServeMux
	patterns []string
}

func (m *routeMux) HandleFunc(pattern string, h http.HandlerFunc) {
	m.patterns = append(m.patterns, pattern)
	m.mux.HandleFunc(pattern, h)
}

// buildAPIMux は /api/ 以下の口をすべて登録し、登録した pattern も返す。
// NewHandler と歯止め（index_cache_test.go）が**同じ1つの登録**を通る
// ——写しを作ると、製品が口を足したのに歯止めだけ古いままになる。
func buildAPIMux(s *store.Store) (*http.ServeMux, []string) {
	// 派生 index は**起動時に1回**建て、`.scholia/` が変わったときだけ建て直す
	// （decision 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項2・語彙 cond.index-built）。
	// 読み取り系のハンドラは全部これを共有する——1本ずつが毎回ストア全体を
	// 読み直していたのが、画面あたり O(N²) の片方の因子だった。
	c := newIndexCache(s)
	c.prime()

	// The API routes live on their own sub-mux with no catch-all pattern, so
	// Go's ServeMux applies its built-in 404 / 405 behavior (see
	// jsonAPIHandler below).
	m := &routeMux{mux: http.NewServeMux()}
	registerConfigRoutes(m, s)
	registerFacetRoutes(m, c)
	registerTransitionRoutes(m, s, c)
	registerTransitionWriteRoutes(m, s)
	registerRulesRoute(m, c)
	registerGovernsRoutes(m, c)
	registerDerivedRoutes(m, s, c)
	registerTraceabilityRoute(m, c)
	registerSearchRoute(m, c)
	registerReviewsRoute(m, s, c)
	registerDecisionRoutes(m, s)
	return m.mux, m.patterns
}

// jsonAPIHandler wraps an API-only ServeMux (no catch-all pattern) so that
// unmatched paths and method mismatches — which stdlib's ServeMux already
// detects correctly and would respond to with a 404/405 plain-text body —
// are instead reported as JSON.
type jsonAPIHandler struct {
	mux *http.ServeMux
}

func (h jsonAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, pattern := h.mux.Handler(r)
	if pattern != "" {
		// Handler() alone doesn't populate {wildcard} path values into the
		// request context — only ServeMux.ServeHTTP does that internally —
		// so the matched case must go back through the mux itself rather
		// than invoking the handler Handler() returned directly.
		h.mux.ServeHTTP(w, r)
		return
	}

	// pattern == "" covers both cases stdlib's default handler already gets
	// right: no pattern matches the path (404), or a pattern matches the
	// path but not the method (405, with an Allow header). Capture that
	// outcome without letting it write its plain-text body to the real
	// response, then re-emit as JSON.
	cap := &statusCapture{header: make(http.Header)}
	handler.ServeHTTP(cap, r)
	if allow := cap.header.Get("Allow"); allow != "" {
		w.Header().Set("Allow", allow)
	}
	msg := "not found"
	if cap.status == http.StatusMethodNotAllowed {
		msg = "method not allowed"
	}
	writeError(w, cap.status, msg)
}

// statusCapture is a minimal http.ResponseWriter that records the status
// code and headers a handler would have written, discarding the body.
type statusCapture struct {
	header http.Header
	status int
}

func (c *statusCapture) Header() http.Header         { return c.header }
func (c *statusCapture) Write(b []byte) (int, error) { return len(b), nil }
func (c *statusCapture) WriteHeader(status int)      { c.status = status }

type spaHandler struct {
	fs fs.FS
}

// ServeHTTP reads the matched file (or index.html as SPA fallback) directly
// rather than delegating to http.FileServerFS, which redirects any request
// resolving to "index.html" back to "/" — an infinite-loop-avoiding
// redirect that would turn our own index.html rewrite into a 301 with no
// body instead of serving the page.
func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	data, err := fs.ReadFile(h.fs, name)
	if err != nil {
		name = "index.html"
		data, err = fs.ReadFile(h.fs, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
