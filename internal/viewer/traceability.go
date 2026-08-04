package viewer

import (
	"fmt"
	"net/http"

	"github.com/nkenji09/scholia/internal/index"
)

func registerTraceabilityRoute(mux *routeMux, c *indexCache) {
	mux.HandleFunc("GET /api/traceability", getTraceabilityHandler(c))
}

type traceabilityResponse struct {
	Kinds   []string                  `json:"kinds"`
	Entries []index.TraceabilityEntry `json:"entries"`
}

// getTraceabilityHandler serves §7's requirement traceability view: for each
// requested traceabilityKind (or all of config.traceabilityKinds if kind is
// omitted), every tag of that kind with the transitions that satisfy it and
// whether it's a gap (index.Traceability — same effective-tag semantics as
// lint's requirement-gap rule, §5).
func getTraceabilityHandler(c *indexCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, ix, err := c.load()
		if err != nil {
			writeStoreError(w, err)
			return
		}

		kind := r.URL.Query().Get("kind")
		kinds := snap.Config.TraceabilityKinds
		if kind != "" {
			if !containsStr(snap.Config.TraceabilityKinds, kind) {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("kind %q は config.traceabilityKinds に未宣言です", kind))
				return
			}
			kinds = []string{kind}
		}

		entries := index.Traceability(ix, kinds)
		if entries == nil {
			entries = []index.TraceabilityEntry{}
		}
		writeJSON(w, http.StatusOK, traceabilityResponse{Kinds: kinds, Entries: entries})
	}
}
