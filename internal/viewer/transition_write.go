package viewer

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerTransitionWriteRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("POST /api/transition", postTransitionHandler(s))
}

// transitionPostBody mirrors change-cockpit-design-v3.md §1's POST body
// (Option A / P3). Every field is a vocab-id (or tag-id) slot — there is no
// label/description field in this type, so a free-text edit is structurally
// impossible to express, not merely rejected by a runtime check (§1's
// "構造ガード"). The body carries the full desired state of the transition
// (not a partial patch): the vocab picker always starts from the working
// tree's current record (`GET /api/transitions/{id}` / `/api/diff`'s
// `after`) and sends back the edited whole, mirroring `pmem tx add`'s full
// record rather than `pmem tx edit`'s flags-changed partial update.
type transitionPostBody struct {
	ID     string   `json:"id"`
	Action string   `json:"action"`
	Given  []string `json:"given"`
	Then   []string `json:"then"`
	Tags   []string `json:"tags"`
}

// postTransitionHandler serves POST /api/transition: the proposal-rework
// write (§1 (Wp)/§8.8 P3). This is G-1′ — the broadest loosening of §7's
// "viewer only writes config" rule, because it lets viewer write an atom
// (Transition) itself rather than a derived record (decision). It is
// contained by three things: (1) the structural guard above (vocab-id
// slots only, enforced by the type + DisallowUnknownFields), (2) the same
// existence/category validation `pmem tx add`/`pmem tx edit` run before
// `store.SaveTransition` (internal/cli/tx_add.go's checkVocabSlot,
// duplicated below — see parseDecisionOn in decision.go for why this
// package can't import internal/cli), and (3) git remains human-only: this
// handler only ever touches the working tree's `.pmem/transitions/<id>.json`
// (uncommitted), never `git`.
//
// Scoped to editing an existing transition only (id must already resolve) —
// creating new transitions is P5 territory (§8.8), out of scope here.
func postTransitionHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body transitionPostBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("transition body が不正です: %v", err))
			return
		}

		if body.ID == "" {
			writeError(w, http.StatusBadRequest, "id は必須です")
			return
		}
		if !s.TransitionExists(body.ID) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("transition %q が実在しません", body.ID))
			return
		}
		if body.Action == "" {
			writeError(w, http.StatusBadRequest, "action は必須です")
			return
		}
		if len(body.Then) == 0 {
			writeError(w, http.StatusBadRequest, "then を空にはできません（empty-then）")
			return
		}

		snap, err := s.LoadAll()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		vocabByID := make(map[string]model.VocabEntry, len(snap.Vocab))
		for _, v := range snap.Vocab {
			vocabByID[v.ID] = v
		}
		tagByID := make(map[string]model.Tag, len(snap.Tags))
		for _, tg := range snap.Tags {
			tagByID[tg.ID] = tg
		}

		if err := checkVocabSlotWrite(vocabByID, "action", []string{body.Action}, model.CategoryAction); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := checkVocabSlotWrite(vocabByID, "given", body.Given, model.CategoryCondition); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := checkVocabSlotWrite(vocabByID, "then", body.Then, model.CategoryEffect); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		for _, tagID := range body.Tags {
			if _, ok := tagByID[tagID]; !ok {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("tags: %q が実在しません", tagID))
				return
			}
		}

		t := model.Transition{ID: body.ID, Action: body.Action, Given: body.Given, Then: body.Then, Tags: body.Tags}
		if err := s.SaveTransition(t); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		saved, err := s.LoadTransition(body.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, saved)
	}
}

// checkVocabSlotWrite validates that every id in a slot resolves to a vocab
// entry of the expected category — a duplicate of
// internal/cli/tx_add.go's checkVocabSlot (same import-cycle reason as
// decision.go's parseDecisionOn/dedupeAppend: internal/cli already imports
// internal/viewer, so the reverse import would cycle). Keep in sync if the
// vocab-slot validation rule ever changes.
func checkVocabSlotWrite(vocabByID map[string]model.VocabEntry, slot string, ids []string, wantCategory string) error {
	for _, id := range ids {
		v, ok := vocabByID[id]
		if !ok {
			return fmt.Errorf("%s: %q が実在する語彙を参照していません", slot, id)
		}
		if v.Category != wantCategory {
			return fmt.Errorf("%s: %q は %s カテゴリの語彙ではありません（実際は %s）", slot, id, wantCategory, v.Category)
		}
	}
	return nil
}
