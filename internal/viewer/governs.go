package viewer

import (
	"net/http"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/store"
)

func registerGovernsRoutes(mux *http.ServeMux, c *indexCache) {
	mux.HandleFunc("GET /api/governs", getGovernsHandler(c))
}

// governsResponse is the per-record governs payload (#45 D10b-1): the set of
// decisions governing a tag / transition / vocab record, each tagged with
// provenance (own / effective-tag / parent). Exactly one of tag/tx/vocab is
// given. The entries come from index.GovernsForTag/Transition/Vocab — the same
// functions the static export bakes and the same query core `scholia rules`
// selects over — so CLI and viewer never diverge on which decisions govern a
// record (面間整合原則 D10b-2).
//
// 返すのは**参照だけ**（decision id ＋ 出自）で本文は持たない（追補
// 01KYJP68V2GR4QJ8HNW6HEP00T 条項2）。この形は静的書き出しの焼き込みと同一で、
// 消費側（継承規則の開示）は live でも静的でも同じ1つのコードパスを通る——
// 形が違うと2つの経路ができ、開示の答えが割れうる（同 条項3）。
// 本文が要る面はもう無い（governs 欄は廃止済み・意思決定欄は own を別途持つ）。
type governsResponse struct {
	Entries []index.GovernsRef `json:"entries"`
}

// governsAllResponse は `?all=1` の応答（decision 01KZ5N5CJ2VFMZAGSFPSCZAMTZ
// 条項1）。カード1枚につき1本投げていたのを1本に畳むための口で、key は
// "tag:<id>" / "transition:<id>" / "vocab:<id>" ——静的書き出しの
// `staticData.governs` と同一の綴りである（面間整合原則 D10b-2: live でも
// 静的でも消費側は同じ1つのコードパスを通る）。
//
// ⚠️ `all` が無いときの挙動（tag/tx/vocab のいずれか1つ必須・違反は 400）は
// 従来のまま。既存の呼び手は壊れない。
type governsAllResponse struct {
	ByRef map[string][]index.GovernsRef `json:"byRef"`
}

func getGovernsHandler(c *indexCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tagID, txID, vocabID := q.Get("tag"), q.Get("tx"), q.Get("vocab")

		if q.Get("all") != "" {
			if tagID != "" || txID != "" || vocabID != "" {
				writeError(w, http.StatusBadRequest, "all と tag / tx / vocab は同時に指定できません")
				return
			}
			snap, ix, err := c.load()
			if err != nil {
				writeStoreError(w, err)
				return
			}
			byRef, err := index.AllGoverns(&snap, ix)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, governsAllResponse{ByRef: byRef})
			return
		}

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

		snap, _, err := c.load()
		if err != nil {
			writeStoreError(w, err)
			return
		}

		entries, err := buildGoverns(&snap, tagID, txID, vocabID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		// RefsOf は make(...) で必ず非 nil を返すので、空でも JSON は [] になる。
		writeJSON(w, http.StatusOK, governsResponse{Entries: index.RefsOf(entries)})
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
