// Package store implements the 1-record-1-file persistence layer under .pmem/ (§3.1, §3.9).
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nkenji09/product-memory/internal/model"
)

const (
	DirName        = ".pmem"
	vocabDir       = "vocab"
	tagsDir        = "tags"
	transitionsDir = "transitions"
	decisionsDir   = "decisions"
	configFile     = "config.json"
)

// Store は .pmem ディレクトリ 1 個への参照。Dir は .pmem 自身の絶対パス。
type Store struct {
	Dir string
}

// Open は projectRoot/.pmem を既存ディレクトリとして開く。存在しなければエラー。
func Open(projectRoot string) (*Store, error) {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	pmemDir := filepath.Join(abs, DirName)
	info, err := os.Stat(pmemDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%s が見つかりません（%s は pmem init 済みですか？）", DirName, abs)
	}
	return &Store{Dir: pmemDir}, nil
}

// Discover は startDir から親方向に .pmem/ を探索する（git と同様の上方探索）。
func Discover(startDir string) (*Store, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	for {
		candidate := filepath.Join(dir, DirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return &Store{Dir: candidate}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("%s から上方探索しましたが %s が見つかりません（pmem init を実行してください）", startDir, DirName)
		}
		dir = parent
	}
}

// InitOptions は Init の挙動を制御するオプション（既定値 = 従来の Init 挙動）。
type InitOptions struct {
	// SkipGitignore が true のとき、.gitignore への追記をスキップする。
	SkipGitignore bool
}

// Init は projectRoot/.pmem 以下のディレクトリと config.json を作成する（冪等）。
// 既存 config は上書きしない。対象 repo の .gitignore に .pmem/index.db と
// .pmem/reviews/（AI コメント配送サイドカー・揮発層・§8.4）を追記する（§3.1）。
func Init(projectRoot string) (*Store, error) {
	return InitWithOptions(projectRoot, InitOptions{})
}

// InitWithOptions は Init と同じ処理を行うが、opts で .gitignore への追記有無を制御できる（§3.1）。
func InitWithOptions(projectRoot string, opts InitOptions) (*Store, error) {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	pmemDir := filepath.Join(abs, DirName)
	for _, sub := range []string{vocabDir, tagsDir, transitionsDir, decisionsDir} {
		if err := os.MkdirAll(filepath.Join(pmemDir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	s := &Store{Dir: pmemDir}

	cfgPath := filepath.Join(pmemDir, configFile)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := s.SaveConfig(model.DefaultConfig()); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	if !opts.SkipGitignore {
		if err := ensureGitignoreEntry(abs, DirName+"/index.db"); err != nil {
			return nil, err
		}
		if err := ensureGitignoreEntry(abs, DirName+"/reviews/"); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func ensureGitignoreEntry(projectRoot, entry string) error {
	path := filepath.Join(projectRoot, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(path, []byte(entry+"\n"), 0o644)
		}
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	content := string(data)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += entry + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

// --- atomic JSON write ---

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
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
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// --- per-category paths / existence ---

func (s *Store) vocabPath(id string) string { return filepath.Join(s.Dir, vocabDir, id+".json") }
func (s *Store) tagPath(id string) string   { return filepath.Join(s.Dir, tagsDir, id+".json") }
func (s *Store) transitionPath(id string) string {
	return filepath.Join(s.Dir, transitionsDir, id+".json")
}
func (s *Store) decisionPath(id string) string { return filepath.Join(s.Dir, decisionsDir, id+".json") }

func (s *Store) VocabExists(id string) bool {
	_, err := os.Stat(s.vocabPath(id))
	return err == nil
}

func (s *Store) TagExists(id string) bool {
	_, err := os.Stat(s.tagPath(id))
	return err == nil
}

func (s *Store) TransitionExists(id string) bool {
	_, err := os.Stat(s.transitionPath(id))
	return err == nil
}

// --- load / save ---

func (s *Store) LoadConfig() (model.Config, error) {
	var c model.Config
	err := readJSON(filepath.Join(s.Dir, configFile), &c)
	return c, err
}

func (s *Store) SaveConfig(c model.Config) error {
	return writeJSONAtomic(filepath.Join(s.Dir, configFile), c)
}

func (s *Store) LoadVocab(id string) (model.VocabEntry, error) {
	var v model.VocabEntry
	err := readJSON(s.vocabPath(id), &v)
	return v, err
}

func (s *Store) SaveVocab(v model.VocabEntry) error {
	return writeJSONAtomic(s.vocabPath(v.ID), v)
}

func (s *Store) LoadTag(id string) (model.Tag, error) {
	var t model.Tag
	err := readJSON(s.tagPath(id), &t)
	return t, err
}

func (s *Store) SaveTag(t model.Tag) error {
	return writeJSONAtomic(s.tagPath(t.ID), t)
}

func (s *Store) LoadTransition(id string) (model.Transition, error) {
	var t model.Transition
	err := readJSON(s.transitionPath(id), &t)
	return t, err
}

// SaveTransition は given を集合として書き込み時にソート＋重複排除する（§3.2）。then は順序保存。
func (s *Store) SaveTransition(t model.Transition) error {
	given := append([]string{}, t.Given...)
	sort.Strings(given)
	given = dedupeSorted(given)
	t.Given = given
	if t.Given == nil {
		t.Given = []string{}
	}
	if t.Then == nil {
		t.Then = []string{}
	}
	return writeJSONAtomic(s.transitionPath(t.ID), t)
}

func dedupeSorted(sorted []string) []string {
	if len(sorted) == 0 {
		return sorted
	}
	out := sorted[:1]
	for _, v := range sorted[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

func (s *Store) SaveDecision(d model.Decision) error {
	return writeJSONAtomic(s.decisionPath(d.ID), d)
}

func (s *Store) LoadDecision(id string) (model.Decision, error) {
	var d model.Decision
	err := readJSON(s.decisionPath(id), &d)
	return d, err
}

// --- snapshot (LoadAll) ---

// IDMismatch はファイル名と内部 id フィールドが一致しないレコードを表す（id-unique lint 用）。
type IDMismatch struct {
	Category string
	File     string
	RecordID string
}

// Snapshot は .pmem/ 全体の読み込みスナップショット。
type Snapshot struct {
	Config       model.Config
	Vocab        []model.VocabEntry
	Tags         []model.Tag
	Transitions  []model.Transition
	Decisions    []model.Decision
	IDMismatches []IDMismatch
}

type identifiable interface {
	GetID() string
}

func listRecords[T identifiable](dir, category string) ([]T, []IDMismatch, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var records []T
	var mismatches []IDMismatch
	for _, name := range names {
		var rec T
		if err := readJSON(filepath.Join(dir, name), &rec); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		fileID := strings.TrimSuffix(name, ".json")
		if rec.GetID() != fileID {
			mismatches = append(mismatches, IDMismatch{Category: category, File: name, RecordID: rec.GetID()})
		}
		records = append(records, rec)
	}
	return records, mismatches, nil
}

// LoadAll は .pmem/ 全体を読み込んだスナップショットを返す（派生インデックスの既定＝in-memory・§3.9）。
func (s *Store) LoadAll() (Snapshot, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return Snapshot{}, fmt.Errorf("config.json の読み込みに失敗: %w", err)
	}

	vocab, vocabMismatch, err := listRecords[model.VocabEntry](filepath.Join(s.Dir, vocabDir), "vocab")
	if err != nil {
		return Snapshot{}, err
	}
	tags, tagMismatch, err := listRecords[model.Tag](filepath.Join(s.Dir, tagsDir), "tag")
	if err != nil {
		return Snapshot{}, err
	}
	transitions, txMismatch, err := listRecords[model.Transition](filepath.Join(s.Dir, transitionsDir), "transition")
	if err != nil {
		return Snapshot{}, err
	}
	decisions, decMismatch, err := listRecords[model.Decision](filepath.Join(s.Dir, decisionsDir), "decision")
	if err != nil {
		return Snapshot{}, err
	}

	var mismatches []IDMismatch
	mismatches = append(mismatches, vocabMismatch...)
	mismatches = append(mismatches, tagMismatch...)
	mismatches = append(mismatches, txMismatch...)
	mismatches = append(mismatches, decMismatch...)

	return Snapshot{
		Config:       cfg,
		Vocab:        vocab,
		Tags:         tags,
		Transitions:  transitions,
		Decisions:    decisions,
		IDMismatches: mismatches,
	}, nil
}
