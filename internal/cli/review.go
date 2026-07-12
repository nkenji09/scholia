package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/review"
)

// newReviewCmd は AI/人の提案コメント（レビュー）を .pmem/reviews/ に書く経路
// （read-only オーバーレイ・§8.4）。「AI は提案時に必ずコメントを付ける」を
// viewer 上で成立させるための CLI 入口 — viewer 自身はコメントを書かない
// （G-3 は反転しない・read-only endpoint は GET /api/reviews のみ）。
func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "提案コメント（レビュー）を操作する（.pmem/reviews/・read-only オーバーレイ・§8.4）",
	}
	cmd.AddCommand(newReviewAddCmd())
	cmd.AddCommand(newReviewListCmd())
	return cmd
}

func newReviewAddCmd() *cobra.Command {
	var on, body, source string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "提案コメントを1件記録する（transition/vocab/tag に付く・§8.4）",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetType, targetID, err := parseReviewOn(on)
			if err != nil {
				return err
			}
			if body == "" {
				return fmt.Errorf("--body は必須です")
			}
			if source == "" {
				source = review.SourceAI
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			switch targetType {
			case review.RecordTypeTransition:
				if !s.TransitionExists(targetID) {
					return fmt.Errorf("transition %q が実在しません", targetID)
				}
			case review.RecordTypeVocab:
				if !s.VocabExists(targetID) {
					return fmt.Errorf("vocab %q が実在しません", targetID)
				}
			case review.RecordTypeTag:
				if !s.TagExists(targetID) {
					return fmt.Errorf("tag %q が実在しません", targetID)
				}
			}

			id, err := model.NewULID()
			if err != nil {
				return err
			}
			r := review.Review{
				ID:        id,
				RecordRef: review.RecordRef{Type: targetType, ID: targetID},
				Body:      body,
				Source:    source,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			}
			if err := review.Add(s.Dir, r); err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "review %s を記録しました（%s:%s）\n", r.ID, targetType, targetID)
			return nil
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "対象。transition:<id> / vocab:<id> / tag:<id>（必須）")
	cmd.Flags().StringVar(&body, "body", "", "提案コメント本文＝why（必須）")
	cmd.Flags().StringVar(&source, "source", review.SourceAI, "書き手。既定は ai")
	cmd.Flags().BoolVar(&asJSON, "json", false, "作成したレコードを JSON で出力する")
	return cmd
}

func newReviewListCmd() *cobra.Command {
	var on string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "提案コメント（レビュー）を一覧表示する（.pmem/reviews/・§8.4）",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			reviews, err := review.List(s.Dir)
			if err != nil {
				return err
			}

			if on != "" {
				targetType, targetID, err := parseReviewOn(on)
				if err != nil {
					return err
				}
				filtered := make([]review.Review, 0, len(reviews))
				for _, r := range reviews {
					if r.RecordRef.Type == targetType && r.RecordRef.ID == targetID {
						filtered = append(filtered, r)
					}
				}
				reviews = filtered
			}

			if asJSON {
				if reviews == nil {
					reviews = []review.Review{}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(reviews)
			}
			if len(reviews) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "レビューはありません")
				return nil
			}
			for _, r := range reviews {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s:%s\t%s\t%s\n", r.ID, r.RecordRef.Type, r.RecordRef.ID, r.Source, r.Body)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "対象で絞り込む。transition:<id> / vocab:<id> / tag:<id>")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}

// parseReviewOn は --on の "transition:<id>" / "vocab:<id>" / "tag:<id>" を分解する（decide.go の parseDecisionOn に倣う）。
func parseReviewOn(on string) (targetType, targetID string, err error) {
	if on == "" {
		return "", "", fmt.Errorf("--on は必須です（transition:<id> / vocab:<id> / tag:<id>）")
	}
	parts := strings.SplitN(on, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("--on の形式が不正です（transition:<id> / vocab:<id> / tag:<id> である必要があります）: %q", on)
	}
	switch parts[0] {
	case review.RecordTypeTransition, review.RecordTypeVocab, review.RecordTypeTag:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("--on の対象種別は transition|vocab|tag のいずれかである必要があります（実際は %q）", parts[0])
	}
}
