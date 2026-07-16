package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

const (
	searchTypeTag        = "tag"
	searchTypeTransition = "transition"
	searchTypeVocab      = "vocab"
	searchTypeDecision   = "decision"
)

// searchTypeOrder is both the valid --type values and the fixed display/sort
// order（spec req.evaluate-change.discovery: 「型別（tag/transition/vocab/decision）」）。
var searchTypeOrder = []string{searchTypeTag, searchTypeTransition, searchTypeVocab, searchTypeDecision}

// searchMatch is one hit: which record, which field matched, and a snippet
// around the match（T-cli-search: eff.log.search-matches「id と一致箇所付き」）。
type searchMatch struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Field   string `json:"field"`
	Snippet string `json:"snippet"`
}

type searchOutput struct {
	Matches []searchMatch `json:"matches"`
}

// newSearchCmd は `scholia search` を新設する。id を知らなくても keyword から
// scholia 記録（tag/transition/vocab/decision）を横断探索する read-only 派生
// コマンド（decision 01K.. on tag:req.evaluate-change.discovery / T-cli-search）。
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

			matches := searchSnapshot(&snap, keywords, types)

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

// searchSnapshot はスナップショット全体を、いずれかの keyword を含む
// フィールドについて大小無視部分一致でスキャンする（OR: keyword は複数
// 指定できる逆引きの入り口であり、どれか1つ拾えれば十分というのが
// 「探索」の趣旨のため）。transition は自身の id に加え、action/given/then
// が参照する vocab の id/label も対象にする（実質の内容一致・spec 実装者判断）。
func searchSnapshot(snap *store.Snapshot, keywords []string, types []string) []searchMatch {
	wanted := make(map[string]bool, len(searchTypeOrder))
	if len(types) == 0 {
		for _, t := range searchTypeOrder {
			wanted[t] = true
		}
	} else {
		for _, t := range types {
			wanted[t] = true
		}
	}

	vocabByID := make(map[string]model.VocabEntry, len(snap.Vocab))
	for _, v := range snap.Vocab {
		vocabByID[v.ID] = v
	}

	var matches []searchMatch
	seen := make(map[string]bool) // dedupe key: type|id|field

	add := func(typ, id, field, text string) {
		if text == "" {
			return
		}
		key := typ + "|" + id + "|" + field
		if seen[key] {
			return
		}
		for _, kw := range keywords {
			if containsFold(text, kw) {
				seen[key] = true
				matches = append(matches, searchMatch{Type: typ, ID: id, Field: field, Snippet: snippet(text, kw)})
				return
			}
		}
	}

	if wanted[searchTypeTag] {
		for _, t := range snap.Tags {
			add(searchTypeTag, t.ID, "id", t.ID)
			add(searchTypeTag, t.ID, "name", t.Name)
			add(searchTypeTag, t.ID, "description", t.Description)
		}
	}

	if wanted[searchTypeVocab] {
		for _, v := range snap.Vocab {
			add(searchTypeVocab, v.ID, "id", v.ID)
			add(searchTypeVocab, v.ID, "label", v.Label)
			add(searchTypeVocab, v.ID, "description", v.Description)
		}
	}

	if wanted[searchTypeTransition] {
		for _, tx := range snap.Transitions {
			add(searchTypeTransition, tx.ID, "id", tx.ID)
			addTransitionVocabRef(add, tx.ID, "action", vocabByID[tx.Action])
			for _, g := range tx.Given {
				addTransitionVocabRef(add, tx.ID, "given", vocabByID[g])
			}
			for _, e := range tx.Then {
				addTransitionVocabRef(add, tx.ID, "then", vocabByID[e])
			}
		}
	}

	if wanted[searchTypeDecision] {
		for _, d := range snap.Decisions {
			add(searchTypeDecision, d.ID, "why", d.Why)
			add(searchTypeDecision, d.ID, "changed", d.Changed)
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Type != matches[j].Type {
			return searchTypeRank(matches[i].Type) < searchTypeRank(matches[j].Type)
		}
		if matches[i].ID != matches[j].ID {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Field < matches[j].Field
	})
	return matches
}

// addTransitionVocabRef は transition の action/given/then スロットが参照する
// vocab の id/label を、そのスロット固有のフィールド名（例: "given:cond.x"）で
// 登録する。スロットごとに複数の vocab を参照しうるため、フィールド名に
// vocab id を含めて dedupe key の衝突（同スロット内の別ヒットの取りこぼし）を防ぐ。
func addTransitionVocabRef(add func(typ, id, field, text string), txID, slot string, v model.VocabEntry) {
	if v.ID == "" {
		return
	}
	text := v.ID
	if v.Label != "" && v.Label != v.ID {
		text = v.ID + " " + v.Label
	}
	add(searchTypeTransition, txID, slot+":"+v.ID, text)
}

func searchTypeRank(t string) int {
	for i, want := range searchTypeOrder {
		if t == want {
			return i
		}
	}
	return len(searchTypeOrder)
}

// containsFold は大小無視の部分一致判定。
func containsFold(text, kw string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(kw))
}

// snippet は一致箇所を中心に前後を切り出した 1 行スニペットを作る
// （ルーン単位で切り出し、日本語などマルチバイト文字を破壊しない）。
func snippet(text, kw string) string {
	oneline := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(oneline)
	byteIdx := strings.Index(lower, strings.ToLower(kw))
	if byteIdx < 0 {
		return truncateOneLine(oneline, 80)
	}
	runeIdx := len([]rune(lower[:byteIdx]))
	kwRuneLen := len([]rune(kw))
	runes := []rune(oneline)

	const context = 20
	start := runeIdx - context
	if start < 0 {
		start = 0
	}
	end := runeIdx + kwRuneLen + context
	if end > len(runes) {
		end = len(runes)
	}

	s := string(runes[start:end])
	if start > 0 {
		s = "…" + s
	}
	if end < len(runes) {
		s = s + "…"
	}
	return s
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
