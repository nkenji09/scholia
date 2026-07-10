package viewer

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/nkenji09/product-memory/internal/store"
	webdist "github.com/nkenji09/product-memory/web"
)

// NewHandler builds the HTTP handler for `pmem view`: the JSON API under
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
	apiMux := http.NewServeMux()
	registerConfigRoutes(apiMux, s)
	registerFacetRoutes(apiMux, s)
	registerTransitionRoutes(apiMux, s)
	registerRulesRoute(apiMux, s)
	registerDerivedRoutes(apiMux, s)
	registerTraceabilityRoute(apiMux, s)
	registerSearchRoute(apiMux, s)

	root := http.NewServeMux()
	root.Handle("/api/", jsonAPIHandler{mux: apiMux})
	root.Handle("/", spaHandler{fs: distFS})
	return root, nil
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
