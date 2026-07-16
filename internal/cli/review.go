package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/review"
)

// newReviewCmd は AI/人の提案コメント（レビュー）を .scholia/reviews/ に書く経路
// （read-only オーバーレイ・§8.4）。「AI は提案時に必ずコメントを付ける」を
// viewer 上で成立させるための CLI 入口 — viewer 自身はレビューを書かない
// （G-3 は反転しない）。adopt/reject/rm は削除のみ扱う書込（§35: decision
// 昇格＋昇格元コメント掃除・T-review-adopt/-reject/T-cli-review-rm）。
func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "提案コメント（レビュー）を操作する（.scholia/reviews/・read-only オーバーレイ・§8.4）",
	}
	cmd.AddCommand(newReviewAddCmd())
	cmd.AddCommand(newReviewListCmd())
	cmd.AddCommand(newReviewAdoptCmd())
	cmd.AddCommand(newReviewRejectCmd())
	cmd.AddCommand(newReviewRmCmd())
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
		Short: "提案コメント（レビュー）を一覧表示する（.scholia/reviews/・§8.4）",
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

// newReviewAdoptCmd は `scholia review adopt <id>`（T-review-adopt）。review の
// 内容を「採用」decision に昇格し（review 本文を why の素材に）、その後に
// review を削除する — 順序固定（先に昇格＝why を失わない、後で削除＝掃除）。
func newReviewAdoptCmd() *cobra.Command {
	return newReviewDecideCmd(reviewDecideAdopt)
}

// newReviewRejectCmd は `scholia review reject <id>`（T-review-reject）。
// 昇格経路と掃除は adopt と同一 — decision の why（不採用・理由）だけが異なる。
func newReviewRejectCmd() *cobra.Command {
	return newReviewDecideCmd(reviewDecideReject)
}

type reviewDecideKind int

const (
	reviewDecideAdopt reviewDecideKind = iota
	reviewDecideReject
)

// newReviewDecideCmd builds `review adopt`/`review reject` — identical
// shape (given=cond.review-exists, then=[append-decision, delete-review]),
// differing only in verb/short text and the default why when --why is
// omitted (adopt: review 本文そのまま／reject: 却下である旨を前置き).
func newReviewDecideCmd(kind reviewDecideKind) *cobra.Command {
	verb, shortDesc := "adopt", "AI 提案コメント(review)を採用し、decision に昇格した上で review を削除する（T-review-adopt）"
	if kind == reviewDecideReject {
		verb, shortDesc = "reject", "AI 提案コメント(review)を却下し、decision に昇格した上で review を削除する（T-review-reject）"
	}

	var why, changed, ref string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   verb + " <id>",
		Short: shortDesc,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			s, err := openStore()
			if err != nil {
				return err
			}

			// cond.review-exists: Get も対象の RecordRef/Body（decision の
			// 材料）を読むために必要なので、存在確認と読み取りを兼ねる。
			r, err := review.Get(s.Dir, id)
			if err != nil {
				return err
			}

			targetType, err := decisionTargetType(r.RecordRef.Type)
			if err != nil {
				return err
			}

			w := why
			if w == "" {
				if kind == reviewDecideReject {
					w = fmt.Sprintf("却下: %s", r.Body)
				} else {
					w = r.Body
				}
			}

			decID, err := model.NewULID()
			if err != nil {
				return err
			}
			d := model.Decision{
				ID:      decID,
				Target:  model.DecisionTarget{Type: targetType, ID: r.RecordRef.ID},
				Why:     w,
				Changed: changed,
				Ref:     ref,
				At:      time.Now().UTC().Format(time.RFC3339),
			}
			// eff.storage.append-decision — 先に昇格。ここで失敗したら review
			// はまだ在るので why を失わない（下の delete-review へ進まない）。
			if err := s.SaveDecision(d); err != nil {
				return err
			}

			// eff.storage.delete-review — 昇格後の掃除。
			if err := review.Delete(s.Dir, id); err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(d)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "review %s を decision %s に昇格し、review を削除しました（%s:%s）\n", id, d.ID, targetType, r.RecordRef.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&why, "why", "", "確定する why（省略時は review 本文を使う。reject は「却下: 」を前置き）")
	cmd.Flags().StringVar(&changed, "changed", "", "何を変更したか（任意）")
	cmd.Flags().StringVar(&ref, "ref", "", "参照。URL・commit hash 推奨")
	cmd.Flags().BoolVar(&asJSON, "json", false, "作成した decision を JSON で出力する")
	return cmd
}

// newReviewRmCmd は `scholia review rm <id>`（T-cli-review-rm・escape hatch）。
// decision を残さず review だけを削除する — review.Delete 自体が
// cond.review-exists（存在しなければエラー）を満たす。
func newReviewRmCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "review を decision を残さず削除する（escape hatch・T-cli-review-rm）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			if err := review.Delete(s.Dir, id); err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{"id": id})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "review %s を削除しました（decision は作成していません）\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}

// decisionTargetType maps a review's RecordRef.Type (transition/vocab/tag)
// to a decision's Target.Type (transition/tag only — model.DecisionTarget
// has no vocab arm, mirroring parseDecisionOn/postDecisionHandler). A vocab
// review can't be adopted/rejected into a decision this way; the CLI errors
// rather than silently dropping the vocab id.
func decisionTargetType(reviewRecordType string) (string, error) {
	switch reviewRecordType {
	case review.RecordTypeTransition:
		return model.DecisionTargetTransition, nil
	case review.RecordTypeTag:
		return model.DecisionTargetTag, nil
	default:
		return "", fmt.Errorf("review の対象種別 %q は decision 化できません（transition/tag のみ）", reviewRecordType)
	}
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
