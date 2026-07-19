package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

// vocab owner-migrate（#45 D9・changed）: 既存 owner 自由文字列 → 当該プロジェクトの
// owner 一覧を提示し subject へのマッピングを支援する proposal ツール。
//
// これは移行の「足場」で、書き込みはしない（提案のみ・最小実装）。実適用は
// `vocab edit --owner <subject-id>` が担う（write-time で実在検証される）。
// ownerKind 宣言下では kind==ownerKind のタグ集合を候補として併記する。
type ownerMigrateEntry struct {
	Owner      string   `json:"owner"`
	Effects    []string `json:"effects"`
	Candidates []string `json:"candidates,omitempty"`
	// Resolved は owner が既に候補（subject タグ id）に一致するか。
	Resolved bool `json:"resolved"`
}

type ownerMigratePlan struct {
	OwnerKind  string              `json:"ownerKind,omitempty"`
	Candidates []string            `json:"candidates,omitempty"`
	Entries    []ownerMigrateEntry `json:"entries"`
}

func newVocabOwnerMigrateCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "owner-migrate",
		Short: "既存 owner 自由文字列 → subject タグへのマッピング案を提示する（書き込みはしない・#45 D9）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			// owner ごとに効果 id を集約。
			effectsByOwner := make(map[string][]string)
			for _, v := range snap.Vocab {
				if v.Category != model.CategoryEffect || v.Owner == "" {
					continue
				}
				effectsByOwner[v.Owner] = append(effectsByOwner[v.Owner], v.ID)
			}

			// ownerKind 宣言下の候補（kind==ownerKind のタグ id）。
			var candidates []string
			candSet := make(map[string]bool)
			if snap.Config.OwnerKind != "" {
				for _, tag := range snap.Tags {
					if tag.Kind == snap.Config.OwnerKind {
						candidates = append(candidates, tag.ID)
						candSet[tag.ID] = true
					}
				}
				sort.Strings(candidates)
			}

			owners := make([]string, 0, len(effectsByOwner))
			for o := range effectsByOwner {
				owners = append(owners, o)
			}
			sort.Strings(owners)

			plan := ownerMigratePlan{OwnerKind: snap.Config.OwnerKind, Candidates: candidates}
			for _, o := range owners {
				effs := effectsByOwner[o]
				sort.Strings(effs)
				plan.Entries = append(plan.Entries, ownerMigrateEntry{
					Owner:      o,
					Effects:    effs,
					Candidates: candidates,
					Resolved:   candSet[o],
				})
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(plan)
			}

			w := cmd.OutOrStdout()
			if snap.Config.OwnerKind == "" {
				fmt.Fprintln(w, "ownerKind は未宣言です（owner は自由文字列のまま）。宣言後に候補が提示されます。")
			} else {
				fmt.Fprintf(w, "ownerKind=%s の候補タグ: %v\n", snap.Config.OwnerKind, candidates)
			}
			if len(plan.Entries) == 0 {
				fmt.Fprintln(w, "owner を持つ effect はありません。")
				return nil
			}
			fmt.Fprintln(w, "既存 owner とその効果（vocab edit --owner <subject-id> で適用してください・書き込みはしません）:")
			for _, e := range plan.Entries {
				mark := ""
				if e.Resolved {
					mark = "（既に候補に一致）"
				}
				fmt.Fprintf(w, "  owner=%q%s → 効果 %d 件: %v\n", e.Owner, mark, len(e.Effects), e.Effects)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "移行案を JSON で出力する")
	return cmd
}
