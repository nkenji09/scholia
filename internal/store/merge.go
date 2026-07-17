// merge.go — `scholia tx merge <dup> --into <survivor>`（#45 U4・決定⑩）。
//
// duplicate-atom lint が見つける「同一原子（action+given+then 一致）の複製」を
// 1 件に統合する正規操作。dup を削除し、dup 宛 decision の target を survivor へ
// 張替え、dup のタグを survivor へ union する。diff 層の merge ペア照合（旧
// transition 消滅＋現存 transition 宛・判断欄位不変）は、この操作を append-only
// 違反にしないための対（decision_fields.go）。
package store

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
)

// TxMergeResult summarizes `scholia tx merge`.
type TxMergeResult struct {
	DupID      string `json:"dupId"`
	SurvivorID string `json:"survivorId"`
	// UpdatedDecisions は target を survivor へ張替えた decision（追随）。
	UpdatedDecisions []string `json:"updatedDecisions,omitempty"`
	// AddedTags は union で survivor に増えたタグ。
	AddedTags []string `json:"addedTags,omitempty"`
	// BaselineRetargeted は .scholia/lint-baseline.json 内の target id を追随
	// 更新したか（baseline 不在・該当なしは false）。
	BaselineRetargeted bool `json:"baselineRetargeted,omitempty"`
}

// MergeTransitions merges duplicate transition dupID into survivorID.
// 同一原子（action が一致・given が集合として一致・then が順序リストとして
// 一致）のみ許可する——意味の異なる遷移の統合は「削除＋書き直し」であって
// merge ではない。適用は fileTxn で all-or-nothing（decision 張替えの途中で
// 失敗しても dangling target を残さない）。
func (s *Store) MergeTransitions(dupID, survivorID string) (TxMergeResult, error) {
	if survivorID == "" {
		return TxMergeResult{}, fmt.Errorf("--into <survivorId> は必須です")
	}
	if dupID == survivorID {
		return TxMergeResult{}, fmt.Errorf("dup %q と survivor が同一です", dupID)
	}
	if !s.TransitionExists(dupID) {
		return TxMergeResult{}, fmt.Errorf("transition %q が見つかりません", dupID)
	}
	if !s.TransitionExists(survivorID) {
		return TxMergeResult{}, fmt.Errorf("transition %q が見つかりません", survivorID)
	}

	dup, err := s.LoadTransition(dupID)
	if err != nil {
		return TxMergeResult{}, err
	}
	survivor, err := s.LoadTransition(survivorID)
	if err != nil {
		return TxMergeResult{}, err
	}
	if err := requireSameAtom(dup, survivor); err != nil {
		return TxMergeResult{}, err
	}

	snap, err := s.LoadAll()
	if err != nil {
		return TxMergeResult{}, err
	}

	result := TxMergeResult{DupID: dupID, SurvivorID: survivorID}

	// タグ union: survivor の並びを保持し、dup にしか無いタグを dup の並びで追加。
	have := make(map[string]bool, len(survivor.Tags))
	for _, tag := range survivor.Tags {
		have[tag] = true
	}
	for _, tag := range dup.Tags {
		if !have[tag] {
			have[tag] = true
			survivor.Tags = append(survivor.Tags, tag)
			result.AddedTags = append(result.AddedTags, tag)
		}
	}

	var updatedDec []model.Decision
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetTransition && d.Target.ID == dupID {
			d.Target.ID = survivorID
			updatedDec = append(updatedDec, d)
			result.UpdatedDecisions = append(result.UpdatedDecisions, d.ID)
		}
	}
	sort.Strings(result.UpdatedDecisions)

	tx := newFileTxn()
	apply := func() error {
		if len(result.AddedTags) > 0 {
			if err := tx.saveTransition(s, survivor); err != nil {
				return err
			}
		}
		for _, d := range updatedDec {
			if err := tx.saveDecision(s, d); err != nil {
				return err
			}
		}
		retargeted, err := tx.retargetBaseline(s, singleIDMap(dupID, survivorID))
		if err != nil {
			return err
		}
		result.BaselineRetargeted = retargeted
		dupPath := s.transitionPath(dupID)
		if err := tx.track(dupPath); err != nil {
			return err
		}
		return os.Remove(dupPath)
	}
	if err := apply(); err != nil {
		tx.rollback()
		return TxMergeResult{}, fmt.Errorf("tx merge の適用に失敗しました（変更はロールバックされました）: %w", err)
	}
	return result, nil
}

// requireSameAtom は dup と survivor が同一原子（action+given+then 一致）で
// あることを検証し、違いを具体的に示すエラーを返す。given は集合（§3.2）、
// then は順序リストとして比較する。
func requireSameAtom(dup, survivor model.Transition) error {
	var diffs []string
	if dup.Action != survivor.Action {
		diffs = append(diffs, fmt.Sprintf("action %q vs %q", dup.Action, survivor.Action))
	}
	dupGiven := append([]string{}, dup.Given...)
	survGiven := append([]string{}, survivor.Given...)
	sort.Strings(dupGiven)
	sort.Strings(survGiven)
	if !reflect.DeepEqual(dupGiven, survGiven) {
		diffs = append(diffs, fmt.Sprintf("given [%s] vs [%s]", strings.Join(dupGiven, ","), strings.Join(survGiven, ",")))
	}
	if !reflect.DeepEqual(dup.Then, survivor.Then) {
		diffs = append(diffs, fmt.Sprintf("then [%s] vs [%s]", strings.Join(dup.Then, ","), strings.Join(survivor.Then, ",")))
	}
	if len(diffs) > 0 {
		return fmt.Errorf("同一原子（action+given+then が一致する遷移）のみ merge できます: %s", strings.Join(diffs, " / "))
	}
	return nil
}
