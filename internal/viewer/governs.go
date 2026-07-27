package viewer

import (
	"net/http"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/store"
)

func registerGovernsRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/governs", getGovernsHandler(s))
}

// governsResponse is the per-record governs payload (#45 D10b-1): the set of
// decisions governing a tag / transition / vocab record, each tagged with
// provenance (own / effective-tag / parent). Exactly one of tag/tx/vocab is
// given. The entries come from index.GovernsForTag/Transition/Vocab — the same
// functions the static export bakes and the same query core `scholia rules`
// selects over — so CLI and viewer never diverge on which decisions govern a
// record (面間整合原則 D10b-2).
type governsResponse struct {
	Entries []index.GovernsEntry `json:"entries"`
}

func getGovernsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tagID, txID, vocabID := q.Get("tag"), q.Get("tx"), q.Get("vocab")

		selected := 0
		for _, v := range []string{tagID, txID, vocabID} {
			if v != "" {
				selected++
			}
		}
		if selected != 1 {
			writeError(w, http.StatusBadRequest, "tag / tx / vocab のいずれか1つを指定してください")
			return
		}

		snap, _, err := loadIndexed(s)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		entries, err := buildGoverns(&snap, tagID, txID, vocabID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if entries == nil {
			entries = []index.GovernsEntry{}
		}
		writeJSON(w, http.StatusOK, governsResponse{Entries: entries})
	}
}

// buildGoverns dispatches to the right index.GovernsFor* function; shared by
// the live handler and the static export bake so both compute governs
// identically (§9 single source of truth).
func buildGoverns(snap *store.Snapshot, tagID, txID, vocabID string) ([]index.GovernsEntry, error) {
	switch {
	case tagID != "":
		return index.GovernsForTag(snap, tagID)
	case txID != "":
		return index.GovernsForTransition(snap, txID)
	case vocabID != "":
		return index.GovernsForVocab(snap, vocabID)
	default:
		return []index.GovernsEntry{}, nil
	}
}
