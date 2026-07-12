package viewer

import (
	"fmt"
	"net/http"

	"github.com/nkenji09/product-memory/internal/diff"
	"github.com/nkenji09/product-memory/internal/lint"
	"github.com/nkenji09/product-memory/internal/render"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerDerivedRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/spec/{tagId}", getSpecHandler(s))
	mux.HandleFunc("GET /api/lint", getLintHandler(s))
	mux.HandleFunc("GET /api/diff", getDiffHandler(s))
}

func getSpecHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tagID := r.PathValue("tagId")
		snap, ix, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
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
// (§7 pmem export --html).
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

func getLintHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, _, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
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
