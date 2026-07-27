package viewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// withoutDecision は snapshot から id の decision を除いたコピーを返す（保存直後の
// advisory 判定で「自分自身を既存 decision として数える」ことを避ける）。
func withoutDecision(snap store.Snapshot, id string) store.Snapshot {
	out := make([]model.Decision, 0, len(snap.Decisions))
	for _, d := range snap.Decisions {
		if d.ID != id {
			out = append(out, d)
		}
	}
	snap.Decisions = out
	return snap
}

func registerDecisionRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("POST /api/decision", postDecisionHandler(s))
}

// decisionPostBody mirrors change-cockpit-design-v3.md §1's POST body
// (Option A). Commits is always empty at adoption time (§8.5: the decision
// is created with `commits[] 空`, filled in later by `scholia decision
// add-commit` once a human commits) — accepted here rather than rejected so
// the frontend can just always send `commits: []` per the design's body
// example without a special case.
type decisionPostBody struct {
	On      string   `json:"on"`
	Why     string   `json:"why"`
	Changed string   `json:"changed,omitempty"`
	Ref     string   `json:"ref,omitempty"`
	Commits []string `json:"commits,omitempty"`
	// Supersedes は提案が宣言した現行性リンク（adopt が結線まで束ねる要件・
	// 01KYHE08WNA8H1Q1DM2H45Y4TK）。デコーダが DisallowUnknownFields を立てて
	// いるので、型に持たない限りフロントが送ると 400 になる——ドロワーの Adopt
	// が結線できるかどうかは、この1フィールドの有無で決まる。
	Supersedes []model.SupersedeLink `json:"supersedes,omitempty"`
}

// postDecisionHandler serves POST /api/decision: the adopt flow's one write
// (§8.5/§8.8 P4). It reuses `scholia decide`'s own path (internal/cli/decide.go)
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
		case model.DecisionTargetVocab:
			if !s.VocabExists(targetID) {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("vocab %q が実在しません", targetID))
				return
			}
		}

		id, err := model.NewULID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 現行性リンクの検証は CLI（decide --supersedes / decision link /
		// review adopt）と同一の model 側関数を通す——面ごとに書き分けると
		// 「CLI では弾かれるのに viewer では通る」宙吊りリンクが生まれる。
		links, err := validateSupersedeBody(s, id, body.Supersedes)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		d := model.Decision{
			ID:         id,
			Target:     model.DecisionTarget{Type: targetType, ID: targetID},
			Why:        body.Why,
			Changed:    body.Changed,
			Ref:        body.Ref,
			At:         time.Now().UTC().Format(time.RFC3339),
			Commits:    dedupeAppend(body.Commits),
			Supersedes: links,
		}
		if err := s.SaveDecision(d); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 未宣言はブロックせず advisory（adopt が結線まで束ねる要件・
		// 01KYHE08WNA8H1Q1DM2H45Y4TK）。CLI review adopt と同じ非ブロック扱いで、
		// 同じ検査コアを通す。埋め込みなので JSON は従来の Decision がフラットに
		// 出たまま advisories が増えるだけ——既存の読み手は壊れない。
		var advisories []lint.Finding
		if snap, err := s.LoadAll(); err == nil {
			advisories = lint.TargetUnlinkedSupersede(withoutDecision(snap, d.ID), d.Target, d.Supersedes)
		}
		writeJSON(w, http.StatusCreated, struct {
			model.Decision
			Advisories []lint.Finding `json:"advisories,omitempty"`
		}{Decision: d, Advisories: advisories})
	}
}

// validateSupersedeBody は POST body の supersedes[] を検証し、保存する形へ
// 正規化する。mode 3値検証・自己参照禁止・重複指定・旧 decision の実在照合・
// 閉路検査を、CLI と同じ model 側関数（と同じ順序）で通す。
func validateSupersedeBody(s *store.Store, newID string, links []model.SupersedeLink) ([]model.SupersedeLink, error) {
	if len(links) == 0 {
		return nil, nil
	}
	// mode の3値検証と自己参照/重複は CLI の parse 段に相当する部分。viewer は
	// 構造化 JSON を受け取るので文字列解析は要らないが、同じ不変条件を課す。
	seen := make(map[string]bool, len(links))
	out := make([]model.SupersedeLink, 0, len(links))
	for _, l := range links {
		if l.ID == "" {
			return nil, fmt.Errorf("supersedes: id が空です")
		}
		if !model.ValidSupersedeMode(l.Mode) {
			return nil, fmt.Errorf("supersedes: mode %q は supersede|amend|exception のいずれかである必要があります", l.Mode)
		}
		if l.ID == newID {
			return nil, fmt.Errorf("supersedes: decision は自分自身（%s）を supersede できません", newID)
		}
		if seen[l.ID] {
			return nil, fmt.Errorf("supersedes: 旧 decision %q が重複指定されています", l.ID)
		}
		seen[l.ID] = true
		out = append(out, l)
	}
	snap, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	if err := model.ValidateSupersedeTargets(snap.Decisions, out); err != nil {
		return nil, err
	}
	if model.SupersedeCreatesCycle(snap.Decisions, newID, out) {
		return nil, fmt.Errorf("supersedes: この結線は decision の supersede グラフに循環を作ります（新→旧の有向グラフに閉路）")
	}
	return out, nil
}

// parseDecisionOn parses "transition:<id>" / "tag:<id>" / "vocab:<id>" — a
// duplicate of internal/cli/decide.go's unexported parseDecisionOn. Not
// imported from there: internal/cli already imports internal/viewer (view.go,
// for `scholia view`/`scholia export`), so the reverse import would cycle. The
// two copies must be kept in sync if the --on/`on` grammar ever changes.
func parseDecisionOn(on string) (targetType, targetID string, err error) {
	if on == "" {
		return "", "", fmt.Errorf("on は必須です（transition:<id>・tag:<id>・vocab:<id>）")
	}
	parts := strings.SplitN(on, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("on の形式が不正です（transition:<id>・tag:<id>・vocab:<id> である必要があります）: %q", on)
	}
	switch parts[0] {
	case model.DecisionTargetTransition, model.DecisionTargetTag, model.DecisionTargetVocab:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("on の対象種別は transition|tag|vocab のいずれかである必要があります（実際は %q）", parts[0])
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
