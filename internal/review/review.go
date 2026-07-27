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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
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
	// Supersedes は「この提案が採用されたら、昇格先 decision が置き換える／改訂
	// する／例外化する旧 decision」への宣言（additive/omitempty）。adopt が
	// decision の supersedes[] へそのまま持ち上げるので、「adopt の後に手で
	// scholia decision link する」手作業が要らなくなる。
	//
	// 結線先を本文（Body）の prose から推測しないための構造化フィールド:
	// decision の link は追記専用で unlink が無く、誤結線を取り消せない
	// （model.AppendSupersedeLinks は既存 link の mode 改変も拒否する）。
	// 取り消せない操作を推測で行わないため、宣言由来のみを持ち上げる。
	Supersedes []model.SupersedeLink `json:"supersedes,omitempty"`
}

func path(scholiaDir, id string) string {
	return filepath.Join(scholiaDir, dirName, id+".json")
}

// NotFoundError は cond.review-exists が満たされないこと（指定 id の提案コメントが
// 無い）。文字列ではなく型で返すのは、面ごとに見せ方が違うため——CLI は「どの id を
// 指したか」を出す必要があるので Error() に id を含める。viewer は
// 01KYCC2TF3NW3JRSSRK9ZHN078（viewer は生レコード id を表示しない・id は
// deep-link の href としてのみ用いる）に従い、id を含まない文言を組み直す。
// review の id は ULID なので、この文言をそのまま body に載せると漏れる。
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("review %q が実在しません", e.ID)
}

// FileError は1件の提案コメントファイルが読めない／JSON として壊れていること。
// store.RecordFileError と同じ意図——ファイル名は `<ULID>.json` なので、CLI には
// 「どのファイルを直すか」として出し、viewer には種別と壊れ方だけを渡す。
// Parse=true は JSON 構文エラー（原因文字列にパスを含まない）。
type FileError struct {
	Name  string // "<id>.json"
	Parse bool
	Err   error
}

func (e *FileError) Error() string { return fmt.Sprintf("%s: %v", e.Name, e.Err) }
func (e *FileError) Unwrap() error { return e.Err }

func newFileError(name string, err error) *FileError {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	return &FileError{Name: name, Parse: errors.As(err, &syntaxErr) || errors.As(err, &typeErr), Err: err}
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
			return Review{}, &NotFoundError{ID: id}
		}
		return Review{}, err
	}
	var r Review
	if err := json.Unmarshal(data, &r); err != nil {
		return Review{}, newFileError(id+".json", err)
	}
	return r, nil
}

// Delete removes scholiaDir/reviews/<id>.json. It errors if the review doesn't
// exist (cond.review-exists) — adopt/reject call this only after the
// decision it's being folded into has already been saved (§8.4/#35
// tx.review.adopt/-reject: append-decision then delete-review, in that
// order, so a proposal's why is never lost); rm calls it directly as the
// escape hatch (tx.cli.review-rm: delete with no decision left behind).
func Delete(scholiaDir, id string) error {
	if err := os.Remove(path(scholiaDir, id)); err != nil {
		if os.IsNotExist(err) {
			return &NotFoundError{ID: id}
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
			return nil, newFileError(name, err)
		}
		var r Review
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, newFileError(name, err)
		}
		out = append(out, r)
	}
	return out, nil
}
