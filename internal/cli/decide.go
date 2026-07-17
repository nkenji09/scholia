package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
)

func newDecideCmd() *cobra.Command {
	var on, why, changed, ref string
	var commits []string
	var asJSON, dryRun bool
	cmd := &cobra.Command{
		Use:   "decide",
		Short: "意思決定を 1 件記録する（transition か tag に付く・append-only・§3.5）",
		Long: "意思決定を 1 件記録する（transition か tag に付く・append-only・§3.5）。\n\n" +
			"decision は append-only で保存後の why/changed は直せない。--dry-run で保存せず\n" +
			"advisory（腐る file:line・消えた文書参照など書き方規律の警告）だけを先にプレビューし、\n" +
			"ゼロにしてから本番の decide を打つ（#45 U3 の推奨手順）。",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetType, targetID, err := parseDecisionOn(on)
			if err != nil {
				return err
			}
			if why == "" {
				return fmt.Errorf("--why は必須です")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			switch targetType {
			case model.DecisionTargetTransition:
				if !s.TransitionExists(targetID) {
					return fmt.Errorf("transition %q が実在しません", targetID)
				}
			case model.DecisionTargetTag:
				if !s.TagExists(targetID) {
					return fmt.Errorf("tag %q が実在しません", targetID)
				}
			}

			id, err := model.NewULID()
			if err != nil {
				return err
			}
			d := model.Decision{
				ID:      id,
				Target:  model.DecisionTarget{Type: targetType, ID: targetID},
				Why:     why,
				Changed: changed,
				Ref:     ref,
				At:      time.Now().UTC().Format(time.RFC3339),
				Commits: dedupeAppend(nil, commits),
			}

			// 書き込みゲート二層（#45 U3）: decision に reject 規則は無いが、
			// why/changed/ref への advisory を保存前に検査できる。append-only
			// のため「保存後に直す」が効かない——--dry-run はここで止まる。
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			advisories, allowed, gateErr := runWriteGate(cmd, snap, lint.WriteOp{Decision: &d, IsNew: true}, nil)
			if gateErr != nil {
				return gateErr
			}

			if dryRun {
				if asJSON {
					return emitWriteJSON(cmd, d, advisories, allowed, true)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: decision は保存していません（%s:%s への advisory プレビュー）\n", targetType, targetID)
				if len(advisories) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "advisory: なし（このまま decide してよい）")
				}
				printWriteGateText(cmd, allowed, advisories)
				return nil
			}

			if err := s.SaveDecision(d); err != nil {
				return err
			}

			if asJSON {
				return emitWriteJSON(cmd, d, advisories, allowed, false)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s を記録しました（%s:%s）\n", d.ID, targetType, targetID)
			printWriteGateText(cmd, allowed, advisories)
			return nil
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "対象。transition:<id> または tag:<id>（必須）")
	cmd.Flags().StringVar(&why, "why", "", "なぜそうしたか（必須）")
	cmd.Flags().StringVar(&changed, "changed", "", "何を変更したか（任意）")
	cmd.Flags().StringVar(&ref, "ref", "", "参照。URL・commit hash 推奨（file:line は lint ref-freshness で警告）")
	cmd.Flags().StringArrayVar(&commits, "commit", nil, "実装した commit hash（複数指定可・繰り返し可。着地後に結ぶ場合は `scholia decision add-commit` を使う）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "作成したレコードを応答封筒 { record, advisories } の JSON で出力する")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "保存せず advisory だけプレビューする（decision は append-only・保存後の why は直せない。decide の前に必ず打つ）")
	return cmd
}

// parseDecisionOn は --on の "transition:<id>" / "tag:<id>" を分解する。
func parseDecisionOn(on string) (targetType, targetID string, err error) {
	if on == "" {
		return "", "", fmt.Errorf("--on は必須です（transition:<id> または tag:<id>）")
	}
	parts := strings.SplitN(on, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("--on の形式が不正です（transition:<id> または tag:<id> である必要があります）: %q", on)
	}
	switch parts[0] {
	case model.DecisionTargetTransition, model.DecisionTargetTag:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("--on の対象種別は transition|tag のいずれかである必要があります（実際は %q）", parts[0])
	}
}
