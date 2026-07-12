package viewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
// write (§1 (Wp)/§8.8 P3) plus new-transition creation (§8.8 P5・M-5「追加」).
// This is G-1′ — the broadest loosening of §7's "viewer only writes config"
// rule, because it lets viewer write an atom (Transition) itself rather
// than a derived record (decision). It is contained by three things: (1)
// the structural guard above (vocab-id slots only, enforced by the type +
// DisallowUnknownFields), (2) the same existence/category validation
// `pmem tx add`/`pmem tx edit` run before `store.SaveTransition`
// (internal/cli/tx_add.go's checkVocabSlot, duplicated below — see
// parseDecisionOn in decision.go for why this package can't import
// internal/cli), and (3) git remains human-only: this handler only ever
// touches the working tree's `.pmem/transitions/<id>.json` (uncommitted),
// never `git`.
//
// Create vs. edit is decided purely by whether body.ID already resolves
// (checked once, before the write, so the response status matches what
// actually happened): unknown id → create (201), existing id → edit (200,
// P3's original behavior). Both paths share every validation below — a
// newly-created transition is just as vocab-only/structurally-guarded as an
// edited one.
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
		// P3 nit: store.SaveTransition は given のみソート＋重複排除する
		// （§3.2）。then/tags は UI（VocabPicker）が重複追加を防いでいるが、
		// API を生で叩けば重複 id を保存できてしまう保険的な二重ガード —
		// CLI 経路（internal/cli/tx_add.go・tx_edit.go）の挙動を変えない
		// ため store でなく viewer 側でのみ dedupe する。then は順序を保つ。
		body.Then = dedupePreserveOrder(body.Then)
		body.Tags = dedupePreserveOrder(body.Tags)
		// id は SaveTransition/RemoveTransitionUnlinked 経由でそのまま
		// ファイル名になる（store.transitionPath）。P5 前は既存 id への
		// 上書きに限られていた（TransitionExists の事前検証が必須だっ
		// た）ため実質無害だったが、新規 id 作成を許すここからは
		// path-traversal（"../" 等）で `.pmem/transitions/` の外へ書ける
		// 攻撃面になる — 作成/編集どちらの経路でも弾く。
		if !validTransitionID(body.ID) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("id %q は不正です（'/' '\\' や '.'/'..' は使えません）", body.ID))
			return
		}
		existed := s.TransitionExists(body.ID)
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
		status := http.StatusOK
		if !existed {
			status = http.StatusCreated
		}
		writeJSON(w, status, saved)
	}
}

// dedupePreserveOrder removes duplicate ids while keeping the first
// occurrence's position (unlike store.dedupeSorted, which sorts).
func dedupePreserveOrder(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// validTransitionID rejects ids that could escape .pmem/transitions/ once
// interpolated into a filename (store.transitionPath = <id>+".json") — no
// path separators and no bare "." / ".." segment.
func validTransitionID(id string) bool {
	if id == "." || id == ".." {
		return false
	}
	return !strings.ContainsAny(id, "/\\")
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
