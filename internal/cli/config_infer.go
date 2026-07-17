package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// newConfigInferIDPolicyCmd は `scholia config infer-id-policy`（#45 U2/P4）。
// 既存 id の prefix 分布から config.idPolicy の宣言案を提案する。read-only——
// config.json への実宣言は各 store の運用判断として人が行う（このコマンドは
// 一切書き込まない）。混在 store では「57/60 が T-」型の分布開示で確信度が
// 読めるよう、宣言案と分布を併記する。
func newConfigInferIDPolicyCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "infer-id-policy",
		Short: "既存 id の分布から config.idPolicy の宣言案を出す（read-only・config.json には書き込まない）",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			result := inferIDPolicy(snap)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			printInferIDPolicyText(cmd, result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（idPolicy 宣言案＋prefix 分布）")
	return cmd
}

// prefixDist は 1 種別の prefix 分布。prefix を持たない id（区切り無し）は
// キー "(none)" で開示する。
type prefixDist struct {
	Total    int            `json:"total"`
	Prefixes map[string]int `json:"prefixes,omitempty"`
}

type inferIDPolicyResult struct {
	IDPolicy      model.IDPolicy `json:"idPolicy"`
	Distributions struct {
		Transition prefixDist            `json:"transition"`
		Vocab      map[string]prefixDist `json:"vocab"`
		TagByKind  map[string]prefixDist `json:"tagByKind"`
	} `json:"distributions"`
}

func inferIDPolicy(snap store.Snapshot) inferIDPolicyResult {
	var result inferIDPolicyResult

	var txIDs []string
	for _, t := range snap.Transitions {
		txIDs = append(txIDs, t.ID)
	}
	result.Distributions.Transition = prefixDistOf(txIDs)
	if best, _ := bestPrefix(result.Distributions.Transition); best != "" {
		result.IDPolicy.Transition = best
	}

	result.Distributions.Vocab = make(map[string]prefixDist)
	for _, category := range []string{model.CategoryCondition, model.CategoryAction, model.CategoryEffect} {
		var ids []string
		for _, v := range snap.Vocab {
			if v.Category == category {
				ids = append(ids, v.ID)
			}
		}
		if len(ids) == 0 {
			continue
		}
		dist := prefixDistOf(ids)
		result.Distributions.Vocab[category] = dist
		if best, _ := bestPrefix(dist); best != "" {
			if result.IDPolicy.Vocab == nil {
				result.IDPolicy.Vocab = make(map[string]string)
			}
			result.IDPolicy.Vocab[category] = best
		}
	}

	result.Distributions.TagByKind = make(map[string]prefixDist)
	idsByKind := make(map[string][]string)
	for _, t := range snap.Tags {
		if t.Kind == "" {
			continue // kind 未設定タグは kind 別宣言の対象外
		}
		idsByKind[t.Kind] = append(idsByKind[t.Kind], t.ID)
	}
	for kind, ids := range idsByKind {
		dist := prefixDistOf(ids)
		result.Distributions.TagByKind[kind] = dist
		if best, _ := bestPrefix(dist); best != "" {
			if result.IDPolicy.TagByKind == nil {
				result.IDPolicy.TagByKind = make(map[string]string)
			}
			result.IDPolicy.TagByKind[kind] = best
		}
	}
	return result
}

// prefixDistOf は id 群の「最初の区切り（. / -）まで」の prefix 分布を返す。
func prefixDistOf(ids []string) prefixDist {
	dist := prefixDist{Total: len(ids), Prefixes: make(map[string]int)}
	for _, id := range ids {
		prefix := "(none)"
		for k := 0; k < len(id); k++ {
			if id[k] == '.' || id[k] == '-' {
				prefix = id[:k+1]
				break
			}
		}
		dist.Prefixes[prefix]++
	}
	return dist
}

// bestPrefix は最多の prefix（"(none)" 除く・同数なら辞書順先頭）を返す。
func bestPrefix(d prefixDist) (string, int) {
	var keys []string
	for p := range d.Prefixes {
		if p != "(none)" {
			keys = append(keys, p)
		}
	}
	sort.Strings(keys)
	best, bestCount := "", 0
	for _, p := range keys {
		if d.Prefixes[p] > bestCount {
			best, bestCount = p, d.Prefixes[p]
		}
	}
	return best, bestCount
}

func printInferIDPolicyText(cmd *cobra.Command, result inferIDPolicyResult) {
	out := cmd.OutOrStdout()

	line := func(label string, dist prefixDist) {
		best, count := bestPrefix(dist)
		if best == "" {
			fmt.Fprintf(out, "%s: prefix を推定できません（%d 件）\n", label, dist.Total)
			return
		}
		fmt.Fprintf(out, "%s: %s %d/%d", label, best, count, dist.Total)
		if count < dist.Total {
			var parts []string
			var keys []string
			for p := range dist.Prefixes {
				keys = append(keys, p)
			}
			sort.Slice(keys, func(i, j int) bool {
				if dist.Prefixes[keys[i]] != dist.Prefixes[keys[j]] {
					return dist.Prefixes[keys[i]] > dist.Prefixes[keys[j]]
				}
				return keys[i] < keys[j]
			})
			for _, p := range keys {
				parts = append(parts, fmt.Sprintf("%s %d", p, dist.Prefixes[p]))
			}
			fmt.Fprintf(out, "（内訳: %s）", joinComma(parts))
		}
		fmt.Fprintln(out)
	}

	if result.Distributions.Transition.Total > 0 {
		line("transition", result.Distributions.Transition)
	}
	for _, category := range []string{model.CategoryCondition, model.CategoryAction, model.CategoryEffect} {
		if dist, ok := result.Distributions.Vocab[category]; ok {
			line("vocab "+category, dist)
		}
	}
	var kinds []string
	for kind := range result.Distributions.TagByKind {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		line("tag kind "+kind, result.Distributions.TagByKind[kind])
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "宣言案（config.json の idPolicy に手で追記する——このコマンドは書き込まない）:")
	data, err := json.MarshalIndent(struct {
		IDPolicy model.IDPolicy `json:"idPolicy"`
	}{result.IDPolicy}, "", "  ")
	if err == nil {
		fmt.Fprintln(out, string(data))
	}
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
