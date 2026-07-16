// Package review implements the AI-comment delivery sidecar under
// .scholia/reviews/ (§8.4): a read-only overlay that lets AI/CLI attach a
// proposal comment to a record without writing to browser localStorage.
//
// Reviews are deliberately not records: store.LoadAll only opens the four
// fixed subdirectories (vocab/tags/transitions/decisions), so this package
// reads/writes .scholia/reviews/ through its own path, invisible to LoadAll and
// scholia lint (§8.4 grounding).
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const dirName = "reviews"

// Record types a review's RecordRef may point at.
const (
	RecordTypeTransition = "transition"
	RecordTypeVocab      = "vocab"
	RecordTypeTag        = "tag"
)

// SourceAI is the default --source for `scholia review add` (§8.4: "AI は提案時に必ずコメントを付ける").
const SourceAI = "ai"

// RecordRef is the record a review comments on.
type RecordRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Review is one proposal comment written to .scholia/reviews/<id>.json.
type Review struct {
	ID        string    `json:"id"`
	RecordRef RecordRef `json:"recordRef"`
	Body      string    `json:"body"`
	Source    string    `json:"source"`
	CreatedAt string    `json:"createdAt"` // RFC3339
}

func path(scholiaDir, id string) string {
	return filepath.Join(scholiaDir, dirName, id+".json")
}

// Add atomically writes r to scholiaDir/reviews/<r.ID>.json (tmp-file-then-rename,
// mirroring store.writeJSONAtomic).
func Add(scholiaDir string, r Review) error {
	dir := filepath.Join(scholiaDir, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path(scholiaDir, r.ID)); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// Get reads a single review by id. It errors if the review doesn't exist
// (cond.review-exists — adopt/reject/rm all check this before acting on an
// id, so a clear "does not exist" error is what callers surface, not a raw
// os.ErrNotExist).
func Get(scholiaDir, id string) (Review, error) {
	data, err := os.ReadFile(path(scholiaDir, id))
	if err != nil {
		if os.IsNotExist(err) {
			return Review{}, fmt.Errorf("review %q が実在しません", id)
		}
		return Review{}, err
	}
	var r Review
	if err := json.Unmarshal(data, &r); err != nil {
		return Review{}, fmt.Errorf("%s: %w", id, err)
	}
	return r, nil
}

// Delete removes scholiaDir/reviews/<id>.json. It errors if the review doesn't
// exist (cond.review-exists) — adopt/reject call this only after the
// decision it's being folded into has already been saved (§8.4/#35
// T-review-adopt/-reject: append-decision then delete-review, in that
// order, so a proposal's why is never lost); rm calls it directly as the
// escape hatch (T-cli-review-rm: delete with no decision left behind).
func Delete(scholiaDir, id string) error {
	if err := os.Remove(path(scholiaDir, id)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("review %q が実在しません", id)
		}
		return err
	}
	return nil
}

// List reads every review under scholiaDir/reviews/, sorted by id (which sorts
// chronologically for ULIDs). A missing reviews/ directory is not an error —
// it just means no review has been written yet.
func List(scholiaDir string) ([]Review, error) {
	dir := filepath.Join(scholiaDir, dirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var out []Review
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		var r Review
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, r)
	}
	return out, nil
}
