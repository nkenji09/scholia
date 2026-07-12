package viewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerDecisionRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("POST /api/decision", postDecisionHandler(s))
}

// decisionPostBody mirrors change-cockpit-design-v3.md §1's POST body
// (Option A). Commits is always empty at adoption time (§8.5: the decision
// is created with `commits[] 空`, filled in later by `pmem decision
// add-commit` once a human commits) — accepted here rather than rejected so
// the frontend can just always send `commits: []` per the design's body
// example without a special case.
type decisionPostBody struct {
	On      string   `json:"on"`
	Why     string   `json:"why"`
	Changed string   `json:"changed,omitempty"`
	Ref     string   `json:"ref,omitempty"`
	Commits []string `json:"commits,omitempty"`
}

// postDecisionHandler serves POST /api/decision: the adopt flow's one write
// (§8.5/§8.8 P4). It reuses `pmem decide`'s own path (internal/cli/decide.go)
// — target validation, model.NewULID, store.SaveDecision — rather than a
// second implementation of decision creation logic. append-only is enforced
// by construction, not by a check here: every call mints a fresh ULID via
// model.NewULID, so this handler can only ever add a new decision file, never
// touch an existing one (§8.7/P-1: commit済み decision の凍結は git 側の
// 責務・このハンドラは常に新規作成のみ).
func postDecisionHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body decisionPostBody
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("decision body が不正です: %v", err))
			return
		}

		targetType, targetID, err := parseDecisionOn(body.On)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.Why == "" {
			writeError(w, http.StatusBadRequest, "why は必須です")
			return
		}

		switch targetType {
		case model.DecisionTargetTransition:
			if !s.TransitionExists(targetID) {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("transition %q が実在しません", targetID))
				return
			}
		case model.DecisionTargetTag:
			if !s.TagExists(targetID) {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("tag %q が実在しません", targetID))
				return
			}
		}

		id, err := model.NewULID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		d := model.Decision{
			ID:      id,
			Target:  model.DecisionTarget{Type: targetType, ID: targetID},
			Why:     body.Why,
			Changed: body.Changed,
			Ref:     body.Ref,
			At:      time.Now().UTC().Format(time.RFC3339),
			Commits: dedupeAppend(body.Commits),
		}
		if err := s.SaveDecision(d); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, d)
	}
}

// parseDecisionOn parses "transition:<id>" / "tag:<id>" — a duplicate of
// internal/cli/decide.go's unexported parseDecisionOn. Not imported from
// there: internal/cli already imports internal/viewer (view.go, for `pmem
// view`/`pmem export`), so the reverse import would cycle. The two copies
// must be kept in sync if the --on/`on` grammar ever changes.
func parseDecisionOn(on string) (targetType, targetID string, err error) {
	if on == "" {
		return "", "", fmt.Errorf("on は必須です（transition:<id> または tag:<id>）")
	}
	parts := strings.SplitN(on, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("on の形式が不正です（transition:<id> または tag:<id> である必要があります）: %q", on)
	}
	switch parts[0] {
	case model.DecisionTargetTransition, model.DecisionTargetTag:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("on の対象種別は transition|tag のいずれかである必要があります（実際は %q）", parts[0])
	}
}

// dedupeAppend drops duplicate entries from additions, keeping first-seen
// order — a duplicate of internal/cli/decision.go's dedupeAppend (existing
// param dropped: a freshly created decision never has prior commits). Same
// import-cycle reason as parseDecisionOn above.
func dedupeAppend(additions []string) []string {
	seen := make(map[string]bool, len(additions))
	out := make([]string, 0, len(additions))
	for _, v := range additions {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
