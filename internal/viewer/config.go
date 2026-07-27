package viewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/nkenji09/scholia/internal/diff"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func registerConfigRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/config", getConfigHandler(s))
	mux.HandleFunc("PUT /api/config", putConfigHandler(s))
	mux.HandleFunc("PUT /api/config/local", putLocalConfigHandler(s))
}

// configResponse embeds model.Config (its fields flatten into the top level
// of the JSON object, unchanged from before) and adds the machine-local
// override state (req.comfortable-viewer.config-editing amend): LocalOverride
// is config.local.json's raw content (for the settings screen's "this
// machine only" section — editing it must never accidentally round-trip an
// override value back into the shared project PUT /api/config below), and
// EffectiveTimezone is the already-resolved value display code should
// render decision.at with (local override wins, DESIGN model.Config.
// EffectiveTimezone).
type configResponse struct {
	model.Config
	LocalOverride     model.LocalConfigOverride `json:"localOverride"`
	EffectiveTimezone string                    `json:"effectiveTimezone"`
}

func getConfigHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := s.LoadConfig()
		if err != nil {
			writeStoreError(w, err)
			return
		}
		// Branch is live/derived, not persisted (model.Config's doc
		// comment) — computed fresh on every GET rather than cached, so it
		// stays correct across `git checkout` while the server keeps running.
		cfg.Branch = diff.CurrentBranch(filepath.Dir(s.Dir))

		local, err := s.LoadLocalConfigOverride()
		if err != nil {
			writeStoreError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, configResponse{
			Config:            cfg,
			LocalOverride:     local,
			EffectiveTimezone: cfg.EffectiveTimezone(local),
		})
	}
}

// putLocalConfigHandler serves PUT /api/config/local — the machine-local
// override write path, independent of PUT /api/config (req.comfortable-
// viewer.config-editing amend). Full-replace semantics, same as the project
// config PUT (§7.2 note in putConfigHandler's doc): the settings screen's
// "this machine only" form round-trips the whole local-override draft it
// loaded, so a save that doesn't touch one field still resends it.
func putLocalConfigHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var patch model.LocalConfigOverride
		if err := dec.Decode(&patch); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("config body が不正です: %v", err))
			return
		}
		if patch.Timezone != "" {
			if err := model.ValidateTimezone(patch.Timezone); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		if err := s.SaveLocalConfigOverride(patch); err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, patch)
	}
}

// configPatch is the editable subset of model.Config the viewer may write
// (§7: "ビューアで書けるのは config だけ"). It mirrors the same key set
// `scholia config set` accepts (internal/cli/config.go configKey* constants),
// plus TagKindLabels (2026-07-11 tweaks3 §2), Display (2026-07-11
// tweaks5 §1/§2, additive — see model.Config's doc comment), and Timezone
// (req.comfortable-viewer.config-editing amend, additive); schemaVersion/
// kinds/idPrefix/Branch are excluded the same way (Branch is derived, not a
// stored preference — never settable via PUT). Unlike `config set` (one key
// per call), PUT replaces the whole editable object at once to match a
// single edit-form submission (implementation decision, result.md) — so a
// PUT body that omits tagKindLabels/display/timezone clears them, same as
// any other field here; ConfigView.tsx always round-trips the full draft it
// loaded, so a normal save never does this by accident.
type configPatch struct {
	// TagKinds は #45 D9 で []model.KindDecl（union 型）に移行。object 宣言
	// （label/description/behaviors）を round-trip で保全し、port だけ変えた PUT で
	// behaviors が黙って消えないようにする。応答は縮退 Marshal（string 形に戻る）。
	TagKinds          []model.KindDecl    `json:"tagKinds"`
	FacetKinds        []string            `json:"facetKinds"`
	TraceabilityKinds []string            `json:"traceabilityKinds"`
	Roots             []string            `json:"roots"`
	Viewer            viewerPortPatch     `json:"viewer"`
	TagKindLabels     map[string]string   `json:"tagKindLabels"`
	Display           model.DisplayConfig `json:"display"`
	Timezone          string              `json:"timezone"`
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

		if patch.Timezone != "" {
			if err := model.ValidateTimezone(patch.Timezone); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		cfg, err := s.LoadConfig()
		if err != nil {
			writeStoreError(w, err)
			return
		}

		// tagKinds の除去は使用中の tag があれば拒否する（scholia config set と同等・
		// DESIGN §6・config_set.go）。id 比較のまま（#45 D9・object 宣言でも id で判定）。
		if removed := diffStrings(kindDeclIDs(cfg.TagKinds), kindDeclIDs(patch.TagKinds)); len(removed) > 0 {
			snap, err := s.LoadAll()
			if err != nil {
				writeStoreError(w, err)
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
		cfg.Timezone = patch.Timezone

		if err := s.SaveConfig(cfg); err != nil {
			writeStoreError(w, err)
			return
		}
		cfg.Branch = diff.CurrentBranch(filepath.Dir(s.Dir))
		writeJSON(w, http.StatusOK, cfg)
	}
}

// kindDeclIDs は KindDecl スライスから id のみを取り出す（#45 D9・使用中検査は
// object 宣言でも id 比較で行う）。
func kindDeclIDs(decls []model.KindDecl) []string {
	out := make([]string, 0, len(decls))
	for _, d := range decls {
		out = append(out, d.ID)
	}
	return out
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
