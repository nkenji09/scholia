package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

func newDecideCmd() *cobra.Command {
	var on, why, changed, ref string
	var commits []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "decide",
		Short: "意思決定を 1 件記録する（transition か tag に付く・append-only・§3.5）",
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
			if err := s.SaveDecision(d); err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(d)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s を記録しました（%s:%s）\n", d.ID, targetType, targetID)
			return nil
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "対象。transition:<id> または tag:<id>（必須）")
	cmd.Flags().StringVar(&why, "why", "", "なぜそうしたか（必須）")
	cmd.Flags().StringVar(&changed, "changed", "", "何を変更したか（任意）")
	cmd.Flags().StringVar(&ref, "ref", "", "参照。URL・commit hash 推奨（file:line は lint ref-freshness で警告）")
	cmd.Flags().StringArrayVar(&commits, "commit", nil, "実装した commit hash（複数指定可・繰り返し可。着地後に結ぶ場合は `scholia decision add-commit` を使う）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "作成したレコードを JSON で出力する")
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
