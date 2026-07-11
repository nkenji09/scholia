package viewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/nkenji09/product-memory/internal/diff"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerConfigRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/config", getConfigHandler(s))
	mux.HandleFunc("PUT /api/config", putConfigHandler(s))
}

func getConfigHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := s.LoadConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Branch is live/derived, not persisted (model.Config's doc
		// comment) — computed fresh on every GET rather than cached, so it
		// stays correct across `git checkout` while the server keeps running.
		cfg.Branch = diff.CurrentBranch(filepath.Dir(s.Dir))
		writeJSON(w, http.StatusOK, cfg)
	}
}

// configPatch is the editable subset of model.Config the viewer may write
// (§7: "ビューアで書けるのは config だけ"). It mirrors the same key set
// `pmem config set` accepts (internal/cli/config.go configKey* constants),
// plus TagKindLabels (2026-07-11 tweaks3 §2) and Display (2026-07-11
// tweaks5 §1/§2, additive — see model.Config's doc comment);
// pmemVersion/kinds/idPrefix/Branch are excluded the same way (Branch is
// derived, not a stored preference — never settable via PUT). Unlike
// `config set` (one key per call), PUT replaces the whole editable object
// at once to match a single edit-form submission (implementation decision,
// result.md) — so a PUT body that omits tagKindLabels/display clears them,
// same as any other field here; ConfigView.tsx always round-trips the full
// draft it loaded, so a normal save never does this by accident.
type configPatch struct {
	TagKinds          []string            `json:"tagKinds"`
	FacetKinds        []string            `json:"facetKinds"`
	TraceabilityKinds []string            `json:"traceabilityKinds"`
	Roots             []string            `json:"roots"`
	Viewer            viewerPortPatch     `json:"viewer"`
	TagKindLabels     map[string]string   `json:"tagKindLabels"`
	Display           model.DisplayConfig `json:"display"`
}

type viewerPortPatch struct {
	Port int `json:"port"`
}

func putConfigHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var patch configPatch
		if err := dec.Decode(&patch); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("config body が不正です: %v", err))
			return
		}

		cfg, err := s.LoadConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// tagKinds の除去は使用中の tag があれば拒否する（pmem config set と同等・DESIGN §6・config_set.go）。
		if removed := diffStrings(cfg.TagKinds, patch.TagKinds); len(removed) > 0 {
			snap, err := s.LoadAll()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if inUse := tagsUsingKinds(snap.Tags, removed); len(inUse) > 0 {
				writeError(w, http.StatusBadRequest, fmt.Sprintf(
					"kind %v は %d 件の tag で使用中のため tagKinds から外せません: %v",
					removed, len(inUse), inUse))
				return
			}
		}

		cfg.TagKinds = patch.TagKinds
		cfg.FacetKinds = patch.FacetKinds
		cfg.TraceabilityKinds = patch.TraceabilityKinds
		cfg.Roots = patch.Roots
		cfg.Viewer.Port = patch.Viewer.Port
		cfg.TagKindLabels = patch.TagKindLabels
		cfg.Display = patch.Display

		if err := s.SaveConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		cfg.Branch = diff.CurrentBranch(filepath.Dir(s.Dir))
		writeJSON(w, http.StatusOK, cfg)
	}
}

func diffStrings(before, after []string) []string {
	afterSet := make(map[string]bool, len(after))
	for _, v := range after {
		afterSet[v] = true
	}
	var out []string
	for _, v := range before {
		if !afterSet[v] {
			out = append(out, v)
		}
	}
	return out
}

func tagsUsingKinds(tags []model.Tag, kinds []string) []string {
	want := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	var out []string
	for _, t := range tags {
		if want[t.Kind] {
			out = append(out, t.ID)
		}
	}
	sort.Strings(out)
	return out
}
