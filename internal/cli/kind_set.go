package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

func newKindSetCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "set <condition|action|effect> <k1,k2,...>",
		Short: "config.kinds[category] の宣言を更新する",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			category, raw := args[0], args[1]
			if !isValidCategory(category) {
				return fmt.Errorf("category は condition|action|effect のいずれかである必要があります（実際は %q）", category)
			}
			kinds := splitNonEmpty(raw)

			s, err := openStore()
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			removed := diffStrings(cfg.KindsFor(category), kinds)
			if len(removed) > 0 {
				snap, err := s.LoadAll()
				if err != nil {
					return err
				}
				if inUse := vocabUsingKinds(snap.Vocab, category, removed); len(inUse) > 0 {
					return fmt.Errorf(
						"kind %s は %d 件の vocab で使用中のため宣言から外せません: %s",
						strings.Join(removed, ","), len(inUse), strings.Join(inUse, ", "))
				}
			}

			setKindsFor(&cfg, category, kinds)
			if err := s.SaveConfig(cfg); err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(cfg.Kinds)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config.kinds.%s を更新しました: %s\n", category, strings.Join(kinds, ", "))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後の config.kinds を JSON で出力する")
	return cmd
}

func setKindsFor(cfg *model.Config, category string, kinds []string) {
	switch category {
	case model.CategoryCondition:
		// #45 D9: condition は []KindDecl（union）。CSV を id 集合として解釈し、
		// 既存 object 宣言（label/description）を id が残る限り保持する（description
		// 付き condition kind を string CSV set で消さない）。
		cfg.Kinds.Condition = mergeCondKindIDs(cfg.Kinds.Condition, kinds)
	case model.CategoryAction:
		cfg.Kinds.Action = kinds
	case model.CategoryEffect:
		cfg.Kinds.Effect = kinds
	}
}

// mergeCondKindIDs は id 集合を既存 condition KindDecl 群に反映する（object
// メタデータを id が残る限り保持・新規は string 宣言で追加）。
func mergeCondKindIDs(existing []model.KindDecl, ids []string) []model.KindDecl {
	byID := make(map[string]model.KindDecl, len(existing))
	for _, d := range existing {
		byID[d.ID] = d
	}
	out := make([]model.KindDecl, 0, len(ids))
	for _, id := range ids {
		if d, ok := byID[id]; ok {
			out = append(out, d)
		} else {
			out = append(out, model.KindDecl{ID: id})
		}
	}
	return out
}

// splitNonEmpty splits a comma-separated flag value, trimming whitespace and
// dropping empty entries (so "a, ,b" and "a,b" behave the same).
func splitNonEmpty(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// diffStrings returns elements of before that are absent from after (§6
// kind/config set 系の「使用中 kind を宣言から外そうとしたらエラー」判定に使う).
func diffStrings(before, after []string) []string {
	keep := make(map[string]bool, len(after))
	for _, v := range after {
		keep[v] = true
	}
	var removed []string
	for _, v := range before {
		if !keep[v] {
			removed = append(removed, v)
		}
	}
	return removed
}

func vocabUsingKinds(vocab []model.VocabEntry, category string, kinds []string) []string {
	want := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	var out []string
	for _, v := range vocab {
		if v.Category == category && want[v.Kind] {
			out = append(out, v.ID)
		}
	}
	sort.Strings(out)
	return out
}
