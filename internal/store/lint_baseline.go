// lint_baseline.go — `scholia lint --ci` の warn 台帳（ratchet・#45 U4／P2）。
//
// .scholia/lint-baseline.json は「既存 warn を即座に赤くする移行断絶を作らず、
// 以後の増加だけを止める」ための歯止め台帳。キーは (rule, target) の組で
// message を含めない——warn 文言の改善で baseline が無効化されてはならない。
// severity も含めない（baseline は warn 専用: error は常に fail・info/advisory は
// ratchet の対象外）。更新は `scholia lint baseline update` 経由のみ（更新自体が
// PR diff に現れてレビュー対象になる）。ファイル不在＝ratchet 非活性（opt-in）。
// baseline を .scholia/ 内に置くのは rename／merge の原子的参照張替えの射程に
// 入れるため（RetargetLintBaseline）。
package store

import (
	"os"
	"path/filepath"
	"sort"
)

const lintBaselineFile = "lint-baseline.json"

// BaselineEntry は baseline の 1 エントリ（rule+target キー・message 非含有）。
type BaselineEntry struct {
	Rule   string `json:"rule"`
	Target string `json:"target"`
}

// LintBaseline は .scholia/lint-baseline.json 全体。
type LintBaseline struct {
	SchemaVersion int             `json:"schemaVersion"`
	Findings      []BaselineEntry `json:"findings"`
}

// LintBaselinePath は baseline ファイルの絶対パス。
func (s *Store) LintBaselinePath() string {
	return filepath.Join(s.Dir, lintBaselineFile)
}

// LoadLintBaseline は baseline を読む。ファイル不在は (nil, nil)＝ratchet 非活性。
func (s *Store) LoadLintBaseline() (*LintBaseline, error) {
	var b LintBaseline
	if err := readJSON(s.LintBaselinePath(), &b); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

// SaveLintBaseline は baseline を正規形（rule, target 順でソート・重複排除・
// schemaVersion=1）で書き出す。呼び出し元は `scholia lint baseline update` と
// rename／merge の追随（RetargetLintBaseline）のみ——それ以外の経路で baseline を
// 書かない。
func (s *Store) SaveLintBaseline(b LintBaseline) error {
	if b.SchemaVersion == 0 {
		b.SchemaVersion = 1
	}
	b.Findings = normalizeBaselineEntries(b.Findings)
	return writeJSONAtomic(s.LintBaselinePath(), b)
}

func normalizeBaselineEntries(entries []BaselineEntry) []BaselineEntry {
	seen := make(map[BaselineEntry]bool, len(entries))
	out := make([]BaselineEntry, 0, len(entries))
	for _, e := range entries {
		if seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return out[i].Target < out[j].Target
	})
	return out
}

// retargetedBaseline は mapID を適用した baseline を返す。baseline 不在または
// 変化なしは (nil, nil)。
func (s *Store) retargetedBaseline(mapID func(string) string) (*LintBaseline, error) {
	b, err := s.LoadLintBaseline()
	if err != nil || b == nil {
		return nil, err
	}
	changed := false
	for i, e := range b.Findings {
		if n := mapID(e.Target); n != e.Target {
			b.Findings[i].Target = n
			changed = true
		}
	}
	if !changed {
		return nil, nil
	}
	return b, nil
}

// RetargetLintBaseline は rename／merge に追随して baseline 内の target id を
// 張替える（#45 U4——baseline が旧 id を指したまま「新規 warn」を誤発火する経路を
// 塞ぐ）。baseline 不在は no-op。張替えの結果重複した entry は正規化で畳まれる。
func (s *Store) RetargetLintBaseline(mapID func(string) string) (changed bool, err error) {
	b, err := s.retargetedBaseline(mapID)
	if err != nil || b == nil {
		return false, err
	}
	return true, s.SaveLintBaseline(*b)
}

// retargetBaseline は fileTxn 版（RenameTag／MergeTransitions のロールバック
// 射程に baseline を含める）。
func (tx *fileTxn) retargetBaseline(s *Store, mapID func(string) string) (bool, error) {
	b, err := s.retargetedBaseline(mapID)
	if err != nil || b == nil {
		return false, err
	}
	if err := tx.track(s.LintBaselinePath()); err != nil {
		return false, err
	}
	return true, s.SaveLintBaseline(*b)
}

// singleIDMap は {old→new} 1 対の mapID。
func singleIDMap(oldID, newID string) func(string) string {
	return func(id string) string {
		if id == oldID {
			return newID
		}
		return id
	}
}
