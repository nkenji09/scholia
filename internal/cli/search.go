package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
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

// searchMatchOut is one match enriched with its owning subjects（案 B・発見性）:
// the subject-kind tags the record belongs to, i.e. the scope candidates a user
// can pass to `--tag` to narrow to this record. Embeds index.RecordMatch so the
// JSON keeps type/id/field/snippet unchanged and only adds "subjects"（additive・
// 後方互換）。
type searchMatchOut struct {
	searchMatch
	Subjects []string `json:"subjects"`
}

type searchOutput struct {
	Matches []searchMatchOut `json:"matches"`
}

// newSearchCmd は `scholia search` を新設する。id を知らなくても keyword から
// scholia 記録（tag/transition/vocab/decision）を横断探索する read-only 派生
// コマンド（decision 01K.. on tag:req.evaluate-change.discovery / tx.cli.search）。
// rules/list/diff 同様に in-memory snapshot 上の query であり、何も保存しない。
func newSearchCmd() *cobra.Command {
	var types []string
	var tags []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "search <keyword> [keyword...]",
		Short: "keyword から tag/transition/vocab/decision を横断探索する（id を知らなくても記録に辿り着く逆引き・read-only）",
		Long: `keyword から scholia 記録（tag/transition/vocab/decision）を横断探索する逆引き（read-only）。

複数 keyword は OR（ヒットが広がる）。記録が増えると概念語だけではノイズが増えるため、
--tag <tagId> で「その tag のサブツリー（実効タグ包含・list --tag と同義）に属する record」へ
絞り込める（--type と AND 合成）。vocab はそのサブツリーの遷移が参照する語彙（コンポの語彙）も含む。

各ヒットには属する subject（コンポ）を注記し、末尾に matched subjects を要約する。tagId を
知らなくても、まず広く search して結果から scope 候補（subject）を見つけ、そのまま --tag に
渡して絞り込める（発見性）。

例:
  scholia search swap                                  # "swap" を全型から逆引き（広い）
  scholia search swap --tag ui.date-range-picker       # そのコンポのサブツリーに絞る
  scholia search swap --tag ui.date-range-picker --type transition
  scholia search swap --tag a --tag b                  # tag は繰り返し可＝OR`,
		Args: cobra.MinimumNArgs(1),
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

			scopeTags := make([]string, 0, len(tags))
			for _, t := range tags {
				if t = strings.TrimSpace(t); t != "" {
					scopeTags = append(scopeTags, t)
				}
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			// --tag の存在検証は list --tag / rules --tag と揃える（実在しない tag は
			// 静かな 0 件ではなくエラー＝スコープ引数の typo を拾う）。
			for _, t := range scopeTags {
				if !s.TagExists(t) {
					return fmt.Errorf("--tag %q が実在しません", t)
				}
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
			// --tag: 概念ヒットを指定 tag のサブツリー（実効タグ包含・list --tag と
			// 同義）に属する record へ絞る（keyword=OR・--type=AND と AND 合成・#1）。
			matches = index.FilterMatchesByTags(ix, &snap, matches, scopeTags)

			// 案 B（発見性）: 各ヒットに owning subject（属する subject タグ＝そのまま
			// --tag に渡せるスコープ候補）を付ける。--tag と同じ帰属判定を流用する
			// （index.OwningSubjects）。ownerKind 未宣言なら subjects は空で無害。
			ownerKind := snap.Config.OwnerKind
			enriched := make([]searchMatchOut, 0, len(matches))
			for _, m := range matches {
				enriched = append(enriched, searchMatchOut{
					searchMatch: m,
					Subjects:    index.OwningSubjects(ix, &snap, ownerKind, m.Type, m.ID),
				})
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(searchOutput{Matches: enriched})
			}
			printSearchMatches(cmd, enriched)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&types, "type", nil, "型で絞り込む（tag|transition|vocab|decision・繰り返し可・既定は全型）")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "そのタグのサブツリー（実効タグ包含・list --tag と同義）に属する record へ絞り込む（繰り返し可＝OR・--type と AND 合成）")
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

func printSearchMatches(cmd *cobra.Command, matches []searchMatchOut) {
	out := cmd.OutOrStdout()
	if len(matches) == 0 {
		fmt.Fprintln(out, "search: 該当なし")
		return
	}
	fmt.Fprintf(out, "search: %d 件\n", len(matches))

	grouped := make(map[string][]searchMatchOut, len(searchTypeOrder))
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
			// 案 B: 各行に owning subject を注記（無ければ従来どおり注記なし）。
			suffix := ""
			if len(m.Subjects) > 0 {
				suffix = "  · subject: " + strings.Join(m.Subjects, ", ")
			}
			fmt.Fprintf(out, "  %s [%s] %s%s\n", m.ID, m.Field, m.Snippet, suffix)
		}
	}

	// 末尾に matched subjects の要約（そのまま --tag に渡せるスコープ候補）。
	printMatchedSubjects(out, matches)
}

// printMatchedSubjects prints a trailing summary of the distinct subjects the
// hits belong to, each with its hit count — the concrete `--tag` values a user
// can try to narrow (案 B の発見性導線). Silent when no hit carries a subject
// (e.g. ownerKind 未宣言・subject に属さない記録のみ).
func printMatchedSubjects(out io.Writer, matches []searchMatchOut) {
	counts := make(map[string]int)
	for _, m := range matches {
		for _, s := range m.Subjects {
			counts[s]++
		}
	}
	if len(counts) == 0 {
		return
	}
	subjects := make([]string, 0, len(counts))
	for s := range counts {
		subjects = append(subjects, s)
	}
	// 件数降順 → id 昇順で安定ソート。
	sort.Slice(subjects, func(i, j int) bool {
		if counts[subjects[i]] != counts[subjects[j]] {
			return counts[subjects[i]] > counts[subjects[j]]
		}
		return subjects[i] < subjects[j]
	})
	parts := make([]string, 0, len(subjects))
	for _, s := range subjects {
		parts = append(parts, fmt.Sprintf("%s (%d)", s, counts[s]))
	}
	fmt.Fprintf(out, "matched subjects（--tag <id> で絞り込めます）: %s\n", strings.Join(parts, ", "))
}
