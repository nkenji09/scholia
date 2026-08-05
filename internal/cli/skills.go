package cli

import (
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	skills "github.com/nkenji09/scholia/agents/skills"
)

// newSkillsCmd は Claude Code 向けスキル（agents/skills/ を embed したもの）を
// 操作するコマンド群（名詞）。
func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "scholia の Claude Code 向けスキルを操作する",
	}
	cmd.AddCommand(newSkillsInstallCmd())
	cmd.AddCommand(newSkillsShowCmd())
	cmd.AddCommand(newSkillsLsCmd())
	return cmd
}

// skillsInstallOutput は --json 出力の形。
type skillsInstallOutput struct {
	Target  string   `json:"target"`
	Written []string `json:"written"`
	Skipped []string `json:"skipped"`
}

// newSkillsInstallCmd は embed 済みのスキルツリーを .claude/skills/ へ展開する。
// go install 済みの scholia バイナリだけで（cwd に agents/ が無い環境でも）
// 展開できることが目的（embed 由来。相対パス参照を持つスキル間の相対構造を保つ）。
func newSkillsInstallCmd() *cobra.Command {
	var userTarget bool
	var projectTarget bool
	var force bool
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "scholia の Claude Code スキルを .claude/skills/ へ展開する",
		Long: `scholia に同梱（embed）された Claude Code 向けスキル一式（scholia / scholia-change /
scholia-config-setup / _scholia-shared）を .claude/skills/ 配下へファイルとして展開する。

go install した scholia バイナリだけで、cwd に agents/ が存在しない環境でも展開できる
（スキル本体はバイナリに焼き込み済み）。

展開先は既定で --project（<cwd>/.claude/skills/）。--user 指定で ~/.claude/skills/ へ
展開する。--user と --project を同時指定するとエラー。

既存ファイルは既定では上書きしない（スキップする）。上書きするには --force を指定する。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if userTarget && projectTarget {
				return fmt.Errorf("--user と --project は同時に指定できません")
			}

			var targetRoot string
			if userTarget {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("ホームディレクトリの解決に失敗しました: %w", err)
				}
				targetRoot = filepath.Join(home, ".claude", "skills")
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				targetRoot = filepath.Join(cwd, ".claude", "skills")
			}

			out := skillsInstallOutput{
				Target:  targetRoot,
				Written: []string{},
				Skipped: []string{},
			}

			err := fs.WalkDir(skills.FS, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if path == "." {
					return nil
				}
				destPath := filepath.Join(targetRoot, filepath.FromSlash(path))
				if d.IsDir() {
					return os.MkdirAll(destPath, 0o755)
				}

				if !force {
					if _, statErr := os.Stat(destPath); statErr == nil {
						out.Skipped = append(out.Skipped, path)
						return nil
					} else if !os.IsNotExist(statErr) {
						return statErr
					}
				}

				if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
					return err
				}
				data, err := skills.FS.ReadFile(path)
				if err != nil {
					return err
				}
				if err := os.WriteFile(destPath, data, 0o644); err != nil {
					return err
				}
				out.Written = append(out.Written, path)
				return nil
			})
			if err != nil {
				return err
			}

			if asJSON {
				return emitJSON(cmd, out)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "展開先: %s\n", out.Target)
			fmt.Fprintf(cmd.OutOrStdout(), "書き込み: %d 件\n", len(out.Written))
			for _, p := range out.Written {
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", p)
			}
			if len(out.Skipped) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "警告: 既存のため %d 件をスキップしました（--force で上書き）\n", len(out.Skipped))
				for _, p := range out.Skipped {
					fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", p)
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "完了しました。")
			return nil
		},
	}

	cmd.Flags().BoolVar(&userTarget, "user", false, "~/.claude/skills/ へ展開する")
	cmd.Flags().BoolVar(&projectTarget, "project", false, "<cwd>/.claude/skills/ へ展開する（既定）")
	cmd.Flags().BoolVar(&force, "force", false, "既存ファイルを上書きする（既定はスキップ）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")

	return cmd
}

// --- show / ls: embed 済みスキル文書を端末へ渡す（decision 01KYRKD3SR80PKQJV3MZKZKG6M）---
//
// 別 repo に自分のスキルを置いている利用者が、共有リファレンス
// （_scholia-shared/references/）へ到達するための経路。install と違い何も
// ディスクに置かない（既決「両経路はファイルを複製せず symlink も張らない」）。
// パスではなくコマンドを配るのは、プラグイン実体が
// …/cache/scholia/scholia/<version>/skills/… とバージョンを含むパスに置かれ、
// install 先も --project/--user でパスが変わるため——どちらも他 repo に直書き
// させると腐る。

// skillDoc は embed ツリー内の 1 ファイル。Path は embed 相対パス（slash 区切り）。
type skillDoc struct {
	Path  string
	Short string // 拡張子なしの basename（短縮名）
}

// skillDocs は embed 済みスキルツリーのファイルを列挙する（ディレクトリは除く）。
func skillDocs() ([]skillDoc, error) {
	var docs []skillDoc
	err := fs.WalkDir(skills.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		docs = append(docs, skillDoc{
			Path:  path,
			Short: strings.TrimSuffix(pathpkg.Base(path), pathpkg.Ext(path)),
		})
		return nil
	})
	return docs, err
}

// resolveSkillDocs は name（短縮名・basename・embed 相対パスのいずれか）に
// 一致する文書を返す。短縮名は衝突しうる（SKILL.md が各スキルに 1 つある）ため
// 一致件数をそのまま返し、複数のときに片方を黙って選ばない判断は呼び出し側が持つ。
func resolveSkillDocs(docs []skillDoc, name string) []skillDoc {
	q := strings.Trim(strings.TrimSpace(filepath.ToSlash(name)), "/")
	var hits []skillDoc
	for _, d := range docs {
		if d.Path == q || pathpkg.Base(d.Path) == q || d.Short == q {
			hits = append(hits, d)
		}
	}
	return hits
}

// skillDocLines は候補一覧の表示行を組む。短縮名が他と衝突するものは
// 「パスで指定」と明示する（短縮名を打っても曖昧エラーになるため）。
func skillDocLines(docs []skillDoc, all []skillDoc) string {
	shortCount := map[string]int{}
	for _, d := range all {
		shortCount[d.Short]++
	}
	var b strings.Builder
	for _, d := range docs {
		if shortCount[d.Short] > 1 {
			fmt.Fprintf(&b, "  %s（短縮名 %q は複数に一致するのでパスで指定）\n", d.Path, d.Short)
			continue
		}
		fmt.Fprintf(&b, "  %s（短縮名: %s）\n", d.Path, d.Short)
	}
	return strings.TrimRight(b.String(), "\n")
}

// skillDocPaths は候補の embed 相対パスだけを並べる（曖昧一致の候補提示用。
// 短縮名では絞れないと分かっている場面なので短縮名は添えない）。
func skillDocPaths(docs []skillDoc) string {
	var b strings.Builder
	for _, d := range docs {
		fmt.Fprintf(&b, "  %s\n", d.Path)
	}
	return strings.TrimRight(b.String(), "\n")
}

// newSkillsShowCmd は embed 済みスキル文書の全文を stdout へ出す。節指定は
// 持たない——節番号や見出しを引数にすると見出しが他 repo との互換面になり、
// 正本の見出しを直すと他 repo の SKILL.md が腐る（decision の判断）。
func newSkillsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <名前>",
		Short: "scholia 同梱スキル文書の全文を stdout へ出す",
		Long: `scholia に embed された Claude Code スキル文書（SKILL.md・共有リファレンス）の
全文を stdout へ出す。ディスクには何も書かない。

<名前> は短縮名（拡張子なしの basename。例 modeling-principles）でも embed 相対パス
（例 _scholia-shared/references/modeling-principles.md）でもよい。短縮名が複数の
ファイルに一致するとき（各スキルにある SKILL.md 等）は、片方を選ばずエラーにする
ので embed 相対パスで指定する。

別 repo のスキルから共有リファレンスへ到達したいときは、パスを書かずにこのコマンドを
手順として置く（パスはプラグインのバージョンや install 先スコープで変わる）。

  scholia skills show modeling-principles
  scholia skills show scholia-change/SKILL.md`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, err := skillDocs()
			if err != nil {
				return err
			}
			hits := resolveSkillDocs(docs, args[0])
			switch len(hits) {
			case 1:
				data, err := skills.FS.ReadFile(hits[0].Path)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(data)
				return err
			case 0:
				return fmt.Errorf("%q はどのスキル文書にも一致しません。参照できる文書:\n%s",
					args[0], skillDocLines(docs, docs))
			default:
				return fmt.Errorf("%q は %d 件に一致します。embed 相対パスで指定してください:\n%s",
					args[0], len(hits), skillDocPaths(hits))
			}
		},
	}
}

// newSkillsLsCmd は show で指定できる名前を一覧する。名前が発見できなければ
// show は打たれないので、show と対で置く。
func newSkillsLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "scholia 同梱スキル文書の名前を一覧する",
		Long:  `scholia skills show で指定できる文書を、embed 相対パスと短縮名で一覧する。`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, err := skillDocs()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), skillDocLines(docs, docs))
			return nil
		},
	}
}
