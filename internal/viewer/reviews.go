package viewer

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/review"
	"github.com/nkenji09/scholia/internal/store"
)

func registerReviewsRoute(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/reviews", getReviewsHandler(s))
	mux.HandleFunc("DELETE /api/reviews/{id}", deleteReviewHandler(s))
}

// getReviewsHandler serves GET /api/reviews: the AI-comment delivery sidecar
// (§8.4). Reviews are written by `scholia review add` under .scholia/reviews/ —
// not records, so they go through internal/review's own reader instead of
// store.LoadAll (§8.4 grounding: LoadAll only opens the four fixed
// subdirectories and never sees reviews/). This is a read-only route; the
// viewer never writes (creates) a review — G-3 is not reversed. DELETE below
// is the one write this file has: it only ever removes an overlay comment
// the frontend has already folded into a decision (§35 tx.review.adopt/
// -reject cleanup step), never adds or edits one.
func getReviewsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviews, err := review.List(s.Dir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := make([]reviewResponse, 0, len(reviews))
		if len(reviews) > 0 {
			snap, err := s.LoadAll()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, rv := range reviews {
				out = append(out, reviewResponse{
					Review:             rv,
					SupersedesDetail:   resolveSupersedeTargets(snap, rv.Supersedes),
					PriorDecisionCount: countDecisionsOnRecord(snap, rv.RecordRef),
				})
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// reviewResponse は Review に「宣言された結線先の解決結果」を derive で足した
// 応答形（保存しない・superseded-by の逆リンクと同じ流儀）。ドロワーが Adopt の
// 前に置き換え対象を人に見せるために要る——旧 decision は ULID しか持たないので、
// 読める素性（どのレコードへの決定か・いつ・why の要約）はサーバ側で解決する。
type reviewResponse struct {
	review.Review
	SupersedesDetail []supersedeTargetDetail `json:"supersedesDetail,omitempty"`
	// PriorDecisionCount は昇格先レコードに既にある decision の件数（derive）。
	// 「既存 decision があるのに結線の宣言が無い」をドロワーが Adopt を押す前に
	// 出すための材料——CLI adopt の supersede-unlinked advisory と同じ条件を、
	// 人が判断できる時点で見せる（黙認もブロックもしない）。
	PriorDecisionCount int `json:"priorDecisionCount,omitempty"`
}

// supersedeTargetDetail は結線先1件の読める素性。ID は deep-link（#/decision/
// <ulid>）の href としてのみ使う値で、表示は targetName/targetId・at・
// whySummary で行う（viewer は生 ULID を読ませない・01KYCC2TF3NW3JRSSRK9ZHN078）。
type supersedeTargetDetail struct {
	ID         string `json:"id"`
	Mode       string `json:"mode"`
	TargetType string `json:"targetType,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	TargetName string `json:"targetName,omitempty"`
	At         string `json:"at,omitempty"`
	WhySummary string `json:"whySummary,omitempty"`
	// Missing は「宣言された旧 decision が store に無い」——review を書いた後に
	// 対象が消えた場合に立つ。adopt は実在照合で弾くので、押す前に見せる。
	Missing bool `json:"missing,omitempty"`
}

func resolveSupersedeTargets(snap store.Snapshot, links []model.SupersedeLink) []supersedeTargetDetail {
	if len(links) == 0 {
		return nil
	}
	byID := make(map[string]model.Decision, len(snap.Decisions))
	for _, d := range snap.Decisions {
		byID[d.ID] = d
	}
	out := make([]supersedeTargetDetail, 0, len(links))
	for _, l := range links {
		detail := supersedeTargetDetail{ID: l.ID, Mode: l.SupersedeMode()}
		d, ok := byID[l.ID]
		if !ok {
			detail.Missing = true
			out = append(out, detail)
			continue
		}
		detail.TargetType = d.Target.Type
		detail.TargetID = d.Target.ID
		detail.TargetName = decisionTargetName(snap, d.Target)
		detail.At = d.At
		detail.WhySummary = summarizeWhy(d.Why)
		out = append(out, detail)
	}
	return out
}

// countDecisionsOnRecord は review の対象レコードちょうどに付いた decision の件数を
// 返す（祖先タグへの決定は数えない＝`decision list --on` と同じ完全一致）。
func countDecisionsOnRecord(snap store.Snapshot, ref review.RecordRef) int {
	var n int
	for _, d := range snap.Decisions {
		if d.Target.Type == ref.Type && d.Target.ID == ref.ID {
			n++
		}
	}
	return n
}

// decisionTargetName は decision の対象レコードの読める名前を返す（tag は name・
// vocab は label・transition は action 語彙の label）。解決できなければ空。
func decisionTargetName(snap store.Snapshot, target model.DecisionTarget) string {
	switch target.Type {
	case model.DecisionTargetTag:
		for _, t := range snap.Tags {
			if t.ID == target.ID {
				return t.Name
			}
		}
	case model.DecisionTargetVocab:
		for _, v := range snap.Vocab {
			if v.ID == target.ID {
				return v.Label
			}
		}
	case model.DecisionTargetTransition:
		for _, tx := range snap.Transitions {
			if tx.ID == target.ID {
				for _, v := range snap.Vocab {
					if v.ID == tx.Action {
						return v.Label
					}
				}
			}
		}
	}
	return ""
}

// summarizeWhy は why の1行目を上限付きで返す（ドロワーのカードに載る要約。
// 全文は deep-link 先の decision 詳細が持つ）。
func summarizeWhy(why string) string {
	line := why
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimSpace(line)
	const max = 120
	r := []rune(line)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return line
}

type deleteReviewResponse struct {
	ID string `json:"id"`
}

// deleteReviewHandler serves DELETE /api/reviews/{id}: the cleanup half of
// adopt/reject (§35 tx.review.adopt/tx.review.reject — eff.storage.
// delete-review, called by the frontend only after its POST /api/decision
// has already succeeded, so a proposal's why is never lost). Server-mode
// only, like every other viewer write (§7 narrow rule) — a static export
// has no write API at all, so this route simply doesn't exist there.
func deleteReviewHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validTransitionID(id) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("id %q は不正です（'/' '\\' や '.'/'..' は使えません）", id))
			return
		}
		if err := review.Delete(s.Dir, id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, deleteReviewResponse{ID: id})
	}
}
