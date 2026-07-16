package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
)

// refSnapshot is the .scholia/ records read from a git ref via `git ls-tree` /
// `git show` — the same shape as store.Snapshot minus the parts (Config,
// IDMismatches) that diff doesn't compare (§4 only covers vocab/tag/
// transition/decision).
type refSnapshot struct {
	Vocab       []model.VocabEntry
	Tags        []model.Tag
	Transitions []model.Transition
	Decisions   []model.Decision
}

const (
	vocabDir       = "vocab"
	tagsDir        = "tags"
	transitionsDir = "transitions"
	decisionsDir   = "decisions"
)

// baselineMissingError は「ref 自体は妥当な操作対象だが、ベースライン（HEAD の
// コミット or ref 上の .scholia）が単に存在しない」ことを表す。既定 ref（ユーザーが
// gitref を明示指定していない）の場合、Diff はこれを空ベースラインへフォール
// バックする（初回ユーザーが git init 直後 / .scholia 未コミットで詰まらないため）。
// ユーザーが gitref を明示指定した場合は従来どおりエラーとして扱う（typo・実在
// しない ref の握り潰しを避ける）。
type baselineMissingError struct {
	msg string
}

func (e *baselineMissingError) Error() string { return e.msg }

func requireGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git コマンドが見つかりません（PATH を確認してください）: %w", err)
	}
	return nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// gitRepoRoot は dir から git リポジトリのルートを解決する。git 未インストール／
// dir が git リポジトリでない場合に分かりやすいエラーを返す。
func gitRepoRoot(dir string) (string, error) {
	if err := requireGit(); err != nil {
		return "", err
	}
	out, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%s は git リポジトリではありません: %w", dir, err)
	}
	return strings.TrimSpace(out), nil
}

func verifyRef(repoRoot, ref string) error {
	if _, err := runGit(repoRoot, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
		return &baselineMissingError{msg: fmt.Sprintf("gitref %q が解決できません（存在するコミット／ブランチ／タグですか？）: %v", ref, err)}
	}
	return nil
}

// loadRefSnapshot は `git ls-tree -r <ref> -- <relDir>` で relDir 以下のファイル一覧を取り、
// 各ファイルを `git show <ref>:<path>` で読んで unmarshal する。relDir は repoRoot からの
// 相対パス（"/" 区切り）で、通常は ".scholia"。
func loadRefSnapshot(repoRoot, relDir, ref string) (refSnapshot, error) {
	if err := verifyRef(repoRoot, ref); err != nil {
		return refSnapshot{}, err
	}

	out, err := runGit(repoRoot, "ls-tree", "-r", "--name-only", ref, "--", relDir)
	if err != nil {
		return refSnapshot{}, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	if len(paths) == 0 {
		return refSnapshot{}, &baselineMissingError{msg: fmt.Sprintf("%s に %s が見つかりません（gitref・パスを確認してください）", ref, relDir)}
	}

	var snap refSnapshot
	prefix := relDir + "/"
	for _, path := range paths {
		if !strings.HasSuffix(path, ".json") {
			continue
		}
		rest := strings.TrimPrefix(path, prefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 {
			continue // config.json など category 直下でないものは diff の対象外（§4）
		}
		category := parts[0]

		content, err := runGit(repoRoot, "show", ref+":"+path)
		if err != nil {
			return refSnapshot{}, err
		}

		switch category {
		case vocabDir:
			var v model.VocabEntry
			if err := json.Unmarshal([]byte(content), &v); err != nil {
				return refSnapshot{}, fmt.Errorf("%s:%s: %w", ref, path, err)
			}
			snap.Vocab = append(snap.Vocab, v)
		case tagsDir:
			var t model.Tag
			if err := json.Unmarshal([]byte(content), &t); err != nil {
				return refSnapshot{}, fmt.Errorf("%s:%s: %w", ref, path, err)
			}
			snap.Tags = append(snap.Tags, t)
		case transitionsDir:
			var t model.Transition
			if err := json.Unmarshal([]byte(content), &t); err != nil {
				return refSnapshot{}, fmt.Errorf("%s:%s: %w", ref, path, err)
			}
			snap.Transitions = append(snap.Transitions, t)
		case decisionsDir:
			var d model.Decision
			if err := json.Unmarshal([]byte(content), &d); err != nil {
				return refSnapshot{}, fmt.Errorf("%s:%s: %w", ref, path, err)
			}
			snap.Decisions = append(snap.Decisions, d)
		}
	}
	return snap, nil
}

// relToRepoRoot は absDir（.scholia の絶対パス）を repoRoot からの "/" 区切り相対パスにする。
func relToRepoRoot(repoRoot, absDir string) (string, error) {
	rel, err := filepath.Rel(repoRoot, absDir)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// resolveSymlinks は EvalSymlinks が失敗した場合（未作成パス等）に元のパスへフォールバックする。
func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
