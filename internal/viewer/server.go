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

	mux := http.NewServeMux()
	registerConfigRoutes(mux, s)
	registerFacetRoutes(mux, s)
	registerTransitionRoutes(mux, s)
	registerRulesRoute(mux, s)
	registerDerivedRoutes(mux, s)
	registerTraceabilityRoute(mux, s)
	registerSearchRoute(mux, s)
	mux.Handle("/", spaHandler{fs: distFS})
	return mux, nil
}

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
