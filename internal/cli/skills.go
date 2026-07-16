package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
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
