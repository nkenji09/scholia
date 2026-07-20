package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/index"
)

const (
	searchTypeTag        = index.RecordTag
	searchTypeTransition = index.RecordTransition
	searchTypeVocab      = index.RecordVocab
	searchTypeDecision   = index.RecordDecision
)

// searchTypeOrder is both the valid --type values and the fixed display/sort
// order（spec req.evaluate-change.discovery: 「型別（tag/transition/vocab/decision）」）。
var searchTypeOrder = []string{searchTypeTag, searchTypeTransition, searchTypeVocab, searchTypeDecision}

// searchMatch is one hit: which record, which field matched, and a snippet
// around the match（tx.cli.search: eff.log.search-matches「id と一致箇所付き」）。
// index.RecordMatch と同一構造（CLI 出力書式の安定のため別型で保持）。
type searchMatch = index.RecordMatch

type searchOutput struct {
	Matches []searchMatch `json:"matches"`
}

// newSearchCmd は `scholia search` を新設する。id を知らなくても keyword から
// scholia 記録（tag/transition/vocab/decision）を横断探索する read-only 派生
// コマンド（decision 01K.. on tag:req.evaluate-change.discovery / tx.cli.search）。
// rules/list/diff 同様に in-memory snapshot 上の query であり、何も保存しない。
func newSearchCmd() *cobra.Command {
	var types []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "search <keyword> [keyword...]",
		Short: "keyword から tag/transition/vocab/decision を横断探索する（id を知らなくても記録に辿り着く逆引き・read-only）",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, t := range types {
				if !isValidSearchType(t) {
					return fmt.Errorf("--type は %s のいずれか（%q は不正）", strings.Join(searchTypeOrder, "|"), t)
				}
			}

			keywords := make([]string, 0, len(args))
			for _, a := range args {
				if a = strings.TrimSpace(a); a != "" {
					keywords = append(keywords, a)
				}
			}
			if len(keywords) == 0 {
				return fmt.Errorf("search キーワードを1つ以上指定してください")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			// 検索コア一本化（#45 D10b-3）: CLI と viewer が同一の
			// index.SearchRecords に委譲する。CLI はここで index.Build して
			// snapshot を渡す（transition の実効タグ・action kind もヒットに含む）。
			ix := index.Build(&snap)
			matches := index.SearchRecords(ix, keywords, types)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(searchOutput{Matches: matches})
			}
			printSearchMatches(cmd, matches)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&types, "type", nil, "型で絞り込む（tag|transition|vocab|decision・繰り返し可・既定は全型）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}

func isValidSearchType(t string) bool {
	for _, want := range searchTypeOrder {
		if t == want {
			return true
		}
	}
	return false
}

func printSearchMatches(cmd *cobra.Command, matches []searchMatch) {
	out := cmd.OutOrStdout()
	if len(matches) == 0 {
		fmt.Fprintln(out, "search: 該当なし")
		return
	}
	fmt.Fprintf(out, "search: %d 件\n", len(matches))

	grouped := make(map[string][]searchMatch, len(searchTypeOrder))
	for _, m := range matches {
		grouped[m.Type] = append(grouped[m.Type], m)
	}
	for _, typ := range searchTypeOrder {
		ms := grouped[typ]
		if len(ms) == 0 {
			continue
		}
		fmt.Fprintf(out, "%s (%d):\n", typ, len(ms))
		for _, m := range ms {
			fmt.Fprintf(out, "  %s [%s] %s\n", m.ID, m.Field, m.Snippet)
		}
	}
}
