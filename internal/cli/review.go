package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/review"
	"github.com/nkenji09/scholia/internal/store"
)

// newReviewCmd は AI/人の提案コメント（レビュー）を .scholia/reviews/ に書く経路
// （read-only オーバーレイ・§8.4）。「AI は提案時に必ずコメントを付ける」を
// viewer 上で成立させるための CLI 入口 — viewer 自身はレビューを書かない
// （G-3 は反転しない）。adopt/reject/rm は削除のみ扱う書込（§35: decision
// 昇格＋昇格元コメント掃除・tx.review.adopt/-reject/tx.cli.review-rm）。
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
	var supersedes []string
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

			// --supersedes（結線の宣言）: 提案時点では昇格先 decision の id が
			// まだ無いので自己参照は起こりえない（selfID は空）。実在照合は
			// 下の openStore 後に decide/link と同じ経路で行う。
			links, err := parseSupersedeLinks(supersedes, "")
			if err != nil {
				return err
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			if len(links) > 0 {
				snap, err := s.LoadAll()
				if err != nil {
					return err
				}
				if err := model.ValidateSupersedeTargets(snap.Decisions, links); err != nil {
					return err
				}
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
				ID:         id,
				RecordRef:  review.RecordRef{Type: targetType, ID: targetID},
				Body:       body,
				Source:     source,
				CreatedAt:  time.Now().UTC().Format(time.RFC3339),
				Supersedes: links,
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
			for _, l := range r.Supersedes {
				fmt.Fprintf(cmd.OutOrStdout(), "  置き換え宣言 → %s (%s)\n", l.ID, l.SupersedeMode())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "対象。transition:<id> / vocab:<id> / tag:<id>（必須）")
	cmd.Flags().StringVar(&body, "body", "", "提案コメント本文＝why（必須）")
	cmd.Flags().StringVar(&source, "source", review.SourceAI, "書き手。既定は ai")
	cmd.Flags().StringArrayVar(&supersedes, "supersedes", nil, "採用されたら置き換える旧 decision <ulid>[:<mode>]（mode=supersede|amend|exception・既定 amend・繰り返し可）。adopt がそのまま decision へ持ち上げる")
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

// newReviewAdoptCmd は `scholia review adopt <id>`（tx.review.adopt）。review の
// 内容を「採用」decision に昇格し（review 本文を why の素材に）、その後に
// review を削除する — 順序固定（先に昇格＝why を失わない、後で削除＝掃除）。
func newReviewAdoptCmd() *cobra.Command {
	return newReviewDecideCmd(reviewDecideAdopt)
}

// newReviewRejectCmd は `scholia review reject <id>`（tx.review.reject）。
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
	verb, shortDesc := "adopt", "AI 提案コメント(review)を採用し、decision に昇格した上で review を削除する（tx.review.adopt）"
	if kind == reviewDecideReject {
		verb, shortDesc = "reject", "AI 提案コメント(review)を却下し、decision に昇格した上で review を削除する（tx.review.reject）"
	}

	var why, changed, ref string
	var supersedes []string
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

			// 現行性リンク（supersedes）: 提案時の宣言（r.Supersedes）を昇格先
			// decision へ持ち上げ、--supersedes の指定を追記する。これが
			// 「adopt の後に手で decision link する」手作業を消す本体。
			// reject は旧 decision を改訂も失効もさせないので載せない。
			var links []model.SupersedeLink
			if kind == reviewDecideAdopt {
				links, err = adoptSupersedeLinks(s, r, decID, supersedes)
				if err != nil {
					return err
				}
			}

			d := model.Decision{
				ID:         decID,
				Target:     model.DecisionTarget{Type: targetType, ID: r.RecordRef.ID},
				Why:        w,
				Changed:    changed,
				Ref:        ref,
				At:         time.Now().UTC().Format(time.RFC3339),
				Supersedes: links,
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

			// desc 現在形ゲート三点配線の第2点（#45 D7）: adopt 応答に対象 desc の
			// stale-tense advisory を添える（採用した判断の対象 record が古びていないか
			// を同一ターンに気づかせる）。adopt では「対象に既存 decision があるのに
			// 結線の宣言が無い」advisory も同じ経路・同じ非ブロック扱いで添える。
			var descAdvisories []lint.Finding
			if snap, err := s.LoadAll(); err == nil {
				descAdvisories = lint.TargetDescStaleTense(snap, d.Target)
				if kind == reviewDecideAdopt {
					// snap は保存後なので d 自身を数に含めない（自分を「既存」と
					// 数えると常に advisory が出る）。
					descAdvisories = append(descAdvisories, lint.TargetUnlinkedSupersede(withoutDecision(snap, d.ID), d.Target, d.Supersedes)...)
				}
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(struct {
					model.Decision
					Advisories []lint.Finding `json:"advisories,omitempty"`
				}{Decision: d, Advisories: descAdvisories})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "review %s を decision %s に昇格し、review を削除しました（%s:%s）\n", id, d.ID, targetType, r.RecordRef.ID)
			for _, l := range d.Supersedes {
				fmt.Fprintf(cmd.OutOrStdout(), "  結線 → %s (%s)\n", l.ID, l.SupersedeMode())
			}
			for _, f := range descAdvisories {
				fmt.Fprintf(cmd.OutOrStdout(), "advisory(%s): %s\n", f.Rule, f.Message)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&why, "why", "", "確定する why（省略時は review 本文を使う。reject は「却下: 」を前置き）")
	cmd.Flags().StringVar(&changed, "changed", "", "何を変更したか（任意）")
	cmd.Flags().StringVar(&ref, "ref", "", "参照。URL・commit hash 推奨")
	if kind == reviewDecideAdopt {
		cmd.Flags().StringArrayVar(&supersedes, "supersedes", nil, "置き換える旧 decision <ulid>[:<mode>]（mode=supersede|amend|exception・既定 amend・繰り返し可）。review の宣言に追記される")
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "作成した decision を JSON で出力する")
	return cmd
}

// adoptSupersedeLinks は review の結線宣言（r.Supersedes）に --supersedes の指定を
// 追記し、decide/decision link と同一の検証（実在照合・自己参照禁止・重複/mode 改変
// 拒否・閉路検査）を通した最終リンク集合を返す。
//
// 検証を model 側の共有関数で行うのは、面ごとに書き分けると「CLI では弾かれるのに
// viewer では通る」宙吊りリンクが生まれるため（viewer の POST /api/decision も同じ
// 関数を呼ぶ）。
func adoptSupersedeLinks(s *store.Store, r review.Review, decID string, flagSpecs []string) ([]model.SupersedeLink, error) {
	flagLinks, err := parseSupersedeLinks(flagSpecs, decID)
	if err != nil {
		return nil, err
	}
	// 宣言に無い分だけを足す（同一 {id,mode} は冪等 skip・同一 id で mode 違いは
	// error＝提案時の宣言を採用時に黙って書き換えない）。
	added, err := model.AppendSupersedeLinks(r.Supersedes, flagLinks)
	if err != nil {
		return nil, err
	}
	links := append(append([]model.SupersedeLink(nil), r.Supersedes...), added...)
	if len(links) == 0 {
		return nil, nil
	}
	// 宣言は review 作成時に検証済みだが、その後に対象 decision が消えている
	// 場合があるので昇格時にもう一度照合する。
	snap, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	if err := model.ValidateSupersedeTargets(snap.Decisions, links); err != nil {
		return nil, err
	}
	// decID は未保存の新規 id なので閉路は構造的に起きない。それでも同じ経路を
	// 通しておく（将来 adopt が既存 decision を編集する形に変わっても穴が開かない）。
	if model.SupersedeCreatesCycle(snap.Decisions, decID, links) {
		return nil, fmt.Errorf("supersedes: この結線は decision の supersede グラフに循環を作ります（新→旧の有向グラフに閉路）")
	}
	return links, nil
}

// withoutDecision は snapshot から id の decision を除いたコピーを返す（他の
// フィールドは共有）。保存直後の advisory 判定で「自分自身を既存 decision として
// 数える」ことを避けるために使う。
func withoutDecision(snap store.Snapshot, id string) store.Snapshot {
	out := make([]model.Decision, 0, len(snap.Decisions))
	for _, d := range snap.Decisions {
		if d.ID != id {
			out = append(out, d)
		}
	}
	snap.Decisions = out
	return snap
}

// newReviewRmCmd は `scholia review rm <id>`（tx.cli.review-rm・escape hatch）。
// decision を残さず review だけを削除する — review.Delete 自体が
// cond.review-exists（存在しなければエラー）を満たす。
func newReviewRmCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "review を decision を残さず削除する（escape hatch・tx.cli.review-rm）",
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
