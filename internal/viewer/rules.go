package viewer

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerRulesRoute(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/rules", getRulesHandler(s))
}

type rulesResponse struct {
	Decisions []model.Decision `json:"decisions"`
}

func getRulesHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tagID, txID, facet := q.Get("tag"), q.Get("tx"), q.Get("facet")

		selected := 0
		for _, v := range []string{tagID, txID, facet} {
			if v != "" {
				selected++
			}
		}
		if selected > 1 {
			writeError(w, http.StatusBadRequest, "tag / tx / facet は同時に指定できません")
			return
		}

		snap, _, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		decisions, err := selectRulesDecisions(&snap, tagID, txID, facet)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		sort.SliceStable(decisions, func(i, j int) bool { return decisions[i].At < decisions[j].At })
		writeJSON(w, http.StatusOK, rulesResponse{Decisions: decisions})
	}
}

// selectRulesDecisions mirrors internal/cli/rules.go's selector semantics
// exactly (--tag/--tx/--facet). Reimplemented here because that function is
// unexported in package cli; it composes only the exported index primitives
// (EffectiveTags/TagAncestors), not a second copy of derived-view logic (§7).
func selectRulesDecisions(snap *store.Snapshot, tagID, txID, facet string) ([]model.Decision, error) {
	switch {
	case txID != "":
		tx, ok := findTransitionByID(snap.Transitions, txID)
		if !ok {
			return nil, fmt.Errorf("transition %q が実在しません", txID)
		}
		targetTags := make(map[string]bool)
		for _, id := range index.EffectiveTags(snap, &tx) {
			targetTags[id] = true
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			if d.Target.Type == model.DecisionTargetTransition {
				return d.Target.ID == txID
			}
			return d.Target.Type == model.DecisionTargetTag && targetTags[d.Target.ID]
		}), nil

	case tagID != "":
		if !tagIDExists(snap.Tags, tagID) {
			return nil, fmt.Errorf("tag %q が実在しません", tagID)
		}
		ancestors := make(map[string]bool)
		for _, id := range index.TagAncestors(snap, tagID) {
			ancestors[id] = true
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			return d.Target.Type == model.DecisionTargetTag && ancestors[d.Target.ID]
		}), nil

	case facet != "":
		facetTags := make(map[string]bool)
		for _, t := range snap.Tags {
			if t.Kind == facet {
				facetTags[t.ID] = true
			}
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			return d.Target.Type == model.DecisionTargetTag && facetTags[d.Target.ID]
		}), nil

	default:
		return append([]model.Decision{}, snap.Decisions...), nil
	}
}

func findTransitionByID(transitions []model.Transition, id string) (model.Transition, bool) {
	for _, t := range transitions {
		if t.ID == id {
			return t, true
		}
	}
	return model.Transition{}, false
}

func tagIDExists(tags []model.Tag, id string) bool {
	for _, t := range tags {
		if t.ID == id {
			return true
		}
	}
	return false
}

func filterDecisions(decisions []model.Decision, keep func(model.Decision) bool) []model.Decision {
	out := make([]model.Decision, 0, len(decisions))
	for _, d := range decisions {
		if keep(d) {
			out = append(out, d)
		}
	}
	return out
}

// rulesForTransition applies the --tx selector inline for the transition
// detail endpoint, given effective tags already computed by the caller.
func rulesForTransition(snap *store.Snapshot, txID string, effectiveTags []string) []model.Decision {
	targetTags := make(map[string]bool, len(effectiveTags))
	for _, id := range effectiveTags {
		targetTags[id] = true
	}
	decisions := filterDecisions(snap.Decisions, func(d model.Decision) bool {
		if d.Target.Type == model.DecisionTargetTransition {
			return d.Target.ID == txID
		}
		return d.Target.Type == model.DecisionTargetTag && targetTags[d.Target.ID]
	})
	sort.SliceStable(decisions, func(i, j int) bool { return decisions[i].At < decisions[j].At })
	return decisions
}
