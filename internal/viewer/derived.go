package viewer

import (
	"fmt"
	"net/http"

	"github.com/nkenji09/scholia/internal/diff"
	"github.com/nkenji09/scholia/internal/flow"
	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/render"
	"github.com/nkenji09/scholia/internal/store"
)

func registerDerivedRoutes(mux *http.ServeMux, s *store.Store, c *indexCache) {
	mux.HandleFunc("GET /api/spec", getSpecAllHandler(c))
	mux.HandleFunc("GET /api/spec/{tagId}", getSpecHandler(c))
	mux.HandleFunc("GET /api/lint", getLintHandler(c))
	mux.HandleFunc("GET /api/diff", getDiffHandler(s))
	mux.HandleFunc("GET /api/flow/{action}", getFlowHandler(c))
}

// getFlowHandler is the live handler for tx.viewer.action-flow-render — it
// shares flow.Analyze with `scholia flow`/`scholia gaps` (analysis logic is
// finalized by #39, not touched here). An unknown action id is not an error:
// flow.Analyze returns a Report with an empty matrix, so the frontend can
// render "この action を持つ遷移はありません" instead of the route crashing
// (§2 acceptance: 不明な action は穏当な空表示).
func getFlowHandler(c *indexCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actionID := r.PathValue("action")
		snap, ix, err := c.load()
		if err != nil {
			writeStoreError(w, err)
			return
		}
		report := flow.Analyze(&snap, ix, actionID)
		writeJSON(w, http.StatusOK, report)
	}
}

// specAllResponse は全タグの spec レポートを1本で返す（decision
// 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1）。key は tag id で、値は
// `GET /api/spec/{tagId}` が1件で返すものと同じ形——静的書き出しの
// `staticData.spec` と同一の形でもある（面間整合原則 D10b-2）。
//
// タグ1件につき1本投げていた `/api/spec/{tagId}` はそのまま残す（他の口から
// 使われうる・後方互換）。画面が使うのはこちらへ移る。
type specAllResponse struct {
	Reports map[string]render.SpecReport `json:"reports"`
}

func getSpecAllHandler(c *indexCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, ix, err := c.load()
		if err != nil {
			writeStoreError(w, err)
			return
		}
		reports, err := render.SpecAll(&snap, ix)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, specAllResponse{Reports: reports})
	}
}

func getSpecHandler(c *indexCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tagID := r.PathValue("tagId")
		snap, ix, err := c.load()
		if err != nil {
			writeStoreError(w, err)
			return
		}
		report, err := render.Spec(&snap, ix, tagID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}

type lintResponse struct {
	Findings   []lint.Finding `json:"findings"`
	ErrorCount int            `json:"errorCount"`
	WarnCount  int            `json:"warnCount"`
	InfoCount  int            `json:"infoCount"`
}

// buildLintResponse is shared by the live handler and the static export bake
// (§7 scholia export --html).
func buildLintResponse(snap store.Snapshot) lintResponse {
	findings := lint.Run(snap)
	if findings == nil {
		findings = []lint.Finding{}
	}
	var errorCount, warnCount, infoCount int
	for _, f := range findings {
		switch f.Severity {
		case lint.SeverityError:
			errorCount++
		case lint.SeverityWarn:
			warnCount++
		case lint.SeverityInfo:
			infoCount++
		}
	}
	return lintResponse{Findings: findings, ErrorCount: errorCount, WarnCount: warnCount, InfoCount: infoCount}
}

func getLintHandler(c *indexCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, _, err := c.load()
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, buildLintResponse(snap))
	}
}

// getDiffHandler は `?ref=<before>` で作業ツリー vs gitref（既定 HEAD・従来挙動）、
// `?ref=<before>&head=<after>` で gitref 対 gitref（`diff.DiffRefs`・§2 R-2 のタスク粒度=commit
// を可視化するコア経路。例: `?ref=<commit>^&head=<commit>` で1コミット分を再現）を返す。
// head 省略時の挙動・レスポンス形は既存と不変（後方互換）。
func getDiffHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("ref")
		if ref == "" {
			ref = "HEAD"
		}
		head := r.URL.Query().Get("head")
		if head != "" {
			result, err := diff.DiffRefs(s, ref, head)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("diff %q..%q に失敗しました: %v", ref, head, err))
				return
			}
			writeJSON(w, http.StatusOK, result)
			return
		}

		result, err := diff.Diff(s, ref)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("diff %q に失敗しました: %v", ref, err))
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}
