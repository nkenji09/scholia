package cli

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/gitio"
	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/store"
)

// decision_edit.go — 取り込み先に載っていない decision を直す／消す口
// （01M1N5YPTRSYAJWH3W7GK4FP64）。
//
// # なぜこの口が要るのか — 凍るのは「載ってから」だった
//
// append-only が守るのは「**誰かが読んだかもしれない過去の判断が、消えたり書き換わったり
// しないこと**」である。**取り込み先に載っていない decision は、まだ誰の目にも触れていない。**
// 守るべき過去がまだ無い。
//
// 歯止め（`diff --check <base>`）は最初からそう動いていた——**比べる相手が base** なので、
// ブランチの中だけに在る decision は「増えた 1 件」でしかなく、中身が途中で変わっても
// 増えた 1 件であることは変わらない（**実測**: ブランチ内の書き換えは exit 0、
// 載ったあとの書き換えは exit 1）。**直せなかったのは口が無かったからで、禁じられていた
// からではない。**
//
// # 拒否は fail closed
//
// 🔴 **取り込み先が決められないときは拒否する。** 分からないときに許すと、誤りの向きが
// 「凍結を破る側」になる。base は `--base` → `origin/HEAD` → `origin/main` の順で解決し、
// **どれも解決しなければ落ちる。** 使った base は出力に書く（黙って推測した base で
// 判定しない）。
//
// # 落ちない範囲（射程・正直に名乗る）
//
//   - 🔴 **共有ブランチに push したあとは、他の人が既に読んでいるかもしれない。**
//     見ているのは「base に在るか」だけなので、push 済みの feature ブランチでの書き換えは
//     止められない。
//   - 🔴 **そのブランチから枝分かれした人は古い版を持つ。** base としか比べないので
//     その食い違いは検出されない。
//   - **この口の拒否を迂回する手段（ファイルを直に編集する）は残る。** 最後の歯止めは
//     CI の `diff --check` である。
//   - **`target` と `at` は編集できない**——対象が変われば、それは別の判断である。

// resolveBaseRef は「凍結の境界となる ref」を決める。決められなければ落ちる。
func resolveBaseRef(gitRoot, explicit string) (string, error) {
	tryRefs := []string{"origin/HEAD", "origin/main"}
	if explicit != "" {
		tryRefs = []string{explicit}
	}
	for _, ref := range tryRefs {
		if _, err := gitio.Run(gitRoot, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			return ref, nil
		}
	}
	if explicit != "" {
		return "", fmt.Errorf("--base に渡した %q が解決しません", explicit)
	}
	return "", fmt.Errorf("取り込み先（base）を決められません（origin/HEAD・origin/main のどちらも解決しません）。" +
		"--base <ref> で明示してください——**決められないまま編集を許すと、凍結を破る側へ倒れます**")
}

// decisionLandedAt は decision のファイルが base に既に在るかを返す。
func decisionLandedAt(s *store.Store, id, baseRef string) (bool, string, error) {
	projectRoot := s.Dir
	gitRoot, relPrefix, err := gitio.ResolveContext(projectRoot)
	if err != nil {
		return false, "", fmt.Errorf("git 管理下ではありません（この口は取り込み先と比べて判定します）: %w", err)
	}
	rel := path.Join(relPrefix, "decisions", id+".json")
	if _, err := gitio.Run(gitRoot, "cat-file", "-e", baseRef+":"+rel); err == nil {
		return true, rel, nil
	}
	return false, rel, nil
}

// requireNotLanded は「取り込み先に載っていないこと」を確かめる共通の関門。
func requireNotLanded(s *store.Store, id, explicitBase string) (baseRef string, err error) {
	gitRoot, _, err := gitio.ResolveContext(s.Dir)
	if err != nil {
		return "", fmt.Errorf("git 管理下ではありません（この口は取り込み先と比べて判定します）: %w", err)
	}
	baseRef, err = resolveBaseRef(gitRoot, explicitBase)
	if err != nil {
		return "", err
	}
	landed, _, err := decisionLandedAt(s, id, baseRef)
	if err != nil {
		return "", err
	}
	if landed {
		return "", fmt.Errorf("decision %s は既に %s に載っています——載ったあとの判断は直せません（append-only）。"+
			"訂正は新しい decision を足してください（scholia decide --supersedes %s）", id, baseRef, id)
	}
	return baseRef, nil
}

func newDecisionEditCmd() *cobra.Command {
	var why, changed, ref, base string
	cmd := &cobra.Command{
		Use:   "edit <decisionId>",
		Short: "取り込み先に載っていない decision の判断を直す（載ったあとは拒否・01M1N5YPTRSYAJWH3W7GK4FP64）",
		Long: "取り込み先（base）に載っていない decision の判断欄位を直す。\n\n" +
			"append-only が凍らせるのは「base に載ってから」であって、作った瞬間からではない。\n" +
			"base に既に在る decision は拒否する（訂正は scholia decide --supersedes で新しく足す）。\n" +
			"target と at は直せない——対象が変われば、それは別の判断である。",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if why == "" && changed == "" && ref == "" {
				return fmt.Errorf("--why / --changed / --ref のいずれかは必要です")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			d, err := s.LoadDecision(id)
			if err != nil {
				return fmt.Errorf("decision %q を読み込めません: %w", id, err)
			}
			baseRef, err := requireNotLanded(s, id, base)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("why") {
				d.Why = why
			}
			if cmd.Flags().Changed("changed") {
				d.Changed = changed
			}
			if cmd.Flags().Changed("ref") {
				d.Ref = ref
			}

			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			// ⚠️ **見出しの拒否規則を新しい本文にも当てる**（IsNew=true）。
			// 直した why が、decide で通らない形になれるのはおかしい。
			advisories, allowed, gateErr := runWriteGate(cmd, snap, lint.WriteOp{Decision: &d, IsNew: true}, nil)
			if gateErr != nil {
				return gateErr
			}
			saved, err := s.UpdateDecision(d)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s を直しました（%s にはまだ載っていません）\n", saved.ID, baseRef)
			printWriteGateText(cmd, allowed, advisories)
			return nil
		},
	}
	cmd.Flags().StringVar(&why, "why", "", "なぜそうしたか（見出し＋本文）")
	cmd.Flags().StringVar(&changed, "changed", "", "何を変更したか")
	cmd.Flags().StringVar(&ref, "ref", "", "参照")
	cmd.Flags().StringVar(&base, "base", "", "凍結の境界となる ref（既定: origin/HEAD → origin/main の順に解決。どれも解決しなければ拒否）")
	return cmd
}

func newDecisionRmCmd() *cobra.Command {
	var base string
	cmd := &cobra.Command{
		Use:   "rm <decisionId>",
		Short: "取り込み先に載っていない decision を消す（載ったあとは拒否・01M1N5YPTRSYAJWH3W7GK4FP64）",
		Long: "取り込み先（base）に載っていない decision を消す。\n\n" +
			"base に既に在る decision は拒否する——誰かが読んだかもしれない判断は消さない。",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			if _, err := s.LoadDecision(id); err != nil {
				return fmt.Errorf("decision %q を読み込めません: %w", id, err)
			}
			baseRef, err := requireNotLanded(s, id, base)
			if err != nil {
				return err
			}
			p := path.Join(s.Dir, "decisions", id+".json")
			if err := os.Remove(strings.TrimSpace(p)); err != nil {
				return fmt.Errorf("decision %s を消せません: %w", id, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s を消しました（%s にはまだ載っていませんでした）\n", id, baseRef)
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "凍結の境界となる ref（既定: origin/HEAD → origin/main の順に解決。どれも解決しなければ拒否）")
	return cmd
}
