package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/store"
)

func newLintCmd() *cobra.Command {
	var asJSON, verbose, ci bool
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "記録の自己矛盾を検査する（§5）",
		Long: "記録の自己矛盾を検査する（§5）。error があれば exit 1・warn/info は exit 0。\n\n" +
			"--ci（#45 U4・歯止め＝ratchet）: error は常に exit 1。warn は\n" +
			".scholia/lint-baseline.json（rule+target キーの台帳）に無い新規分のみ exit 1。\n" +
			"baseline 不在なら ratchet は非活性（warn は fail しない・opt-in）。info と\n" +
			"advisory（authoring 規律）は ratchet の対象外。baseline の更新は\n" +
			"`scholia lint baseline update` 経由のみ（更新自体が PR diff に現れてレビュー\n" +
			"対象になる）。rename／tx merge は baseline 内の target id を追随更新する。",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			findings := lint.Run(snap)
			if findings == nil {
				findings = []lint.Finding{}
			}

			var ciEval *ciResult
			if ci {
				ciEval, err = evaluateCI(s, findings)
				if err != nil {
					return err
				}
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				errorCount, warnCount, infoCount := countBySeverity(findings)
				out := struct {
					Findings   []lint.Finding `json:"findings"`
					ErrorCount int            `json:"errorCount"`
					WarnCount  int            `json:"warnCount"`
					InfoCount  int            `json:"infoCount"`
					CI         *ciResult      `json:"ci,omitempty"`
				}{Findings: findings, ErrorCount: errorCount, WarnCount: warnCount, InfoCount: infoCount, CI: ciEval}
				if err := enc.Encode(out); err != nil {
					return err
				}
			} else {
				printLintText(cmd, findings, verbose)
				if ciEval != nil {
					printLintCIText(cmd, findings, ciEval)
				}
			}

			if lint.HasError(findings) {
				return fmt.Errorf("lint failed: %d error(s)", countErrors(findings))
			}
			if ciEval != nil && len(ciEval.NewWarns) > 0 {
				return fmt.Errorf("lint --ci failed: baseline に無い新規 warn %d 件", len(ciEval.NewWarns))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（decision-coverage は direct/via-tag/none 全件＋coverage 付き）")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "decision-coverage via-tag の内訳（どのタグ経由か）を表示する")
	cmd.Flags().BoolVar(&ci, "ci", false,
		"CI モード（歯止め）: error 常時 exit 1・baseline に無い新規 warn のみ exit 1（baseline 不在は非活性）・info/advisory 不問")
	cmd.AddCommand(newLintBaselineCmd())
	return cmd
}

// lintSection は lint テキスト出力の区分。findings はこの4系統のいずれか
// ちょうど1つに落ちる（decision-coverage の direct/via-tag は lintSectionCoverage）。
type lintSection string

const (
	// lintSectionDisplayed は是正対象（error/warn/info）。lint の本体。
	lintSectionDisplayed lintSection = "displayed"
	// lintSectionTypedAck は typed 容認済み（#45 D6）。
	lintSectionTypedAck lintSection = "typed-ack"
	// lintSectionAckOnly は acknowledge-only（decision 判断欄位由来・
	// append-only により是正不能・#45 U2）。
	lintSectionAckOnly lintSection = "ack-only"
	// lintSectionCoverage は decision-coverage の3段集計。
	lintSectionCoverage lintSection = "coverage"
)

// lintDetail は区分の明細をどこまで出すか。件数（サマリ行）は粒度に依らず常に出る。
type lintDetail string

const (
	lintDetailNone lintDetail = "none" // 件数だけ出す
	lintDetailAll  lintDetail = "all"  // 明細を全件出す
)

// lintTextPlan は「どの finding がどの区分に落ち、どの区分の明細を出すか」。
// テキスト出力はこの値だけを見て描く（renderLintText）。
type lintTextPlan struct {
	Displayed []lint.Finding
	TypedAck  []lint.Finding
	AckOnly   []lint.Finding
	ViaTag    []lint.Finding // decision-coverage via-tag の出自内訳

	CoverageDirect int
	CoverageViaTag int
	CoverageNone   int

	// Detail は区分ごとの明細粒度。描画はこの表を通る——表に無い経路で
	// 明細を出さない（区分×モードの検査が素通りしないようにするため）。
	Detail map[lintSection]lintDetail
}

// planLintText はテキスト出力の方針を findings と verbose だけから決める純関数。
// 画面（io.Writer）から切り離してあるのは、表示方針を値で検査できるようにするため。
//
// 粒度:
//   - displayed: 常に全件明細。exit code の根拠（error）と --ci の ratchet 対象
//     （warn）を含むので畳まない。
//   - typedAck / ackOnly / coverage: 既定は件数のみ・--verbose で明細。
//
// ackOnly を既定で畳むのは、この区分が append-only により是正不能で exit code
// にも --ci の ratchet にも関与せず（evaluateCI は warn だけを見る）、同じ棚卸しを
// `scholia retrofit` が修正候補つきで出すため——用途は失われない。区分を新設した
// decision 自身が、別掲の目的を「恒常ノイズを新設しないこと」と述べている。
func planLintText(findings []lint.Finding, verbose bool) lintTextPlan {
	p := lintTextPlan{Detail: make(map[lintSection]lintDetail, 4)}
	for _, f := range findings {
		switch {
		case f.Coverage != "" && f.Coverage != lint.CoverageNone:
			// direct/via-tag は3段集計に畳む（毎回列挙する info ノイズを出さない・U1）。
			if f.Coverage == lint.CoverageViaTag {
				p.ViaTag = append(p.ViaTag, f)
			}
		case f.AcknowledgedBy != "":
			// typed 容認は是正対象の displayed に混ぜない（信じられる緑を返すため）。
			p.TypedAck = append(p.TypedAck, f)
		case f.AcknowledgeOnly:
			p.AckOnly = append(p.AckOnly, f)
		default:
			p.Displayed = append(p.Displayed, f)
		}
	}
	p.CoverageDirect, p.CoverageViaTag, p.CoverageNone = lint.CoverageCounts(findings)

	folded := lintDetailNone
	if verbose {
		folded = lintDetailAll
	}
	p.Detail[lintSectionDisplayed] = lintDetailAll
	p.Detail[lintSectionTypedAck] = folded
	p.Detail[lintSectionAckOnly] = folded
	p.Detail[lintSectionCoverage] = folded
	return p
}

// printLintText は既定のテキスト出力。
func printLintText(cmd *cobra.Command, findings []lint.Finding, verbose bool) {
	renderLintText(cmd.OutOrStdout(), planLintText(findings, verbose))
}

// renderLintText は plan を描く。粒度の判断はここでは行わない（plan.Detail に従う）。
func renderLintText(out io.Writer, p lintTextPlan) {
	if len(p.Displayed) == 0 && len(p.AckOnly) == 0 && len(p.TypedAck) == 0 {
		fmt.Fprintln(out, "lint: 問題は見つかりませんでした")
	}
	if p.Detail[lintSectionDisplayed] == lintDetailAll {
		for _, f := range p.Displayed {
			fmt.Fprintf(out, "[%s] %s: %s\n", f.Severity, f.Rule, f.Message)
		}
	}
	if len(p.TypedAck) > 0 {
		fmt.Fprintf(out, "typed 容認済み（decision で意図的に残す gap・#45 D6）: %d 件\n", len(p.TypedAck))
		if p.Detail[lintSectionTypedAck] == lintDetailAll {
			for _, f := range p.TypedAck {
				fmt.Fprintf(out, "  [%s] %s: %s → 容認 decision %s\n", f.Severity, f.Rule, f.Target, f.AcknowledgedBy)
			}
		}
	}
	if len(p.AckOnly) > 0 {
		fmt.Fprintf(out,
			"acknowledge-only（decision 判断欄位・append-only により是正不能・容認で畳む対象）: %d 件（明細は --verbose / 修正候補つきの棚卸しは scholia retrofit）\n",
			len(p.AckOnly))
		if p.Detail[lintSectionAckOnly] == lintDetailAll {
			for _, f := range p.AckOnly {
				fmt.Fprintf(out, "  [%s] %s: %s\n", f.Severity, f.Rule, f.Message)
			}
		}
	}

	if p.CoverageDirect+p.CoverageViaTag+p.CoverageNone == 0 {
		return
	}
	if p.Detail[lintSectionCoverage] == lintDetailAll {
		fmt.Fprintf(out, "decision-coverage: direct %d / via-tag %d / none %d\n", p.CoverageDirect, p.CoverageViaTag, p.CoverageNone)
		if p.CoverageViaTag > 0 {
			fmt.Fprintln(out, "decision-coverage via-tag の内訳:")
			for _, f := range p.ViaTag {
				fmt.Fprintf(out, "  %s: %s\n", f.Target, f.Detail)
			}
		}
	} else {
		fmt.Fprintf(out, "decision-coverage: direct %d / via-tag %d / none %d（via-tag の内訳は --verbose）\n", p.CoverageDirect, p.CoverageViaTag, p.CoverageNone)
	}
}

func countErrors(findings []lint.Finding) int {
	n, _, _ := countBySeverity(findings)
	return n
}

func countBySeverity(findings []lint.Finding) (errorCount, warnCount, infoCount int) {
	for _, f := range findings {
		switch f.Severity {
		case lint.SeverityError:
			errorCount++
		case lint.SeverityWarn:
			warnCount++
		case lint.SeverityInfo:
			infoCount++
		}
	}
	return
}

// ciResult は --ci の判定結果（--json の additive フィールド ci にもなる）。
type ciResult struct {
	// BaselinePresent が false のとき ratchet は非活性（warn は fail しない）。
	BaselinePresent bool `json:"baselinePresent"`
	BaselineCount   int  `json:"baselineCount"`
	// NewWarns は baseline に無い warn（1 件でもあれば exit 1）。
	NewWarns []lint.Finding `json:"newWarns,omitempty"`
	// Stale は baseline に載っているが今回の warn に出なかった entry
	// （info 報告のみ・次の baseline update で自然消滅）。
	Stale []store.BaselineEntry `json:"stale,omitempty"`
}

func evaluateCI(s *store.Store, findings []lint.Finding) (*ciResult, error) {
	baseline, err := s.LoadLintBaseline()
	if err != nil {
		return nil, err
	}
	res := &ciResult{BaselinePresent: baseline != nil}
	if baseline == nil {
		return res, nil
	}
	res.BaselineCount = len(baseline.Findings)

	inBaseline := make(map[store.BaselineEntry]bool, len(baseline.Findings))
	for _, e := range baseline.Findings {
		inBaseline[e] = true
	}
	currentWarns := make(map[store.BaselineEntry]bool)
	for _, f := range findings {
		if f.Severity != lint.SeverityWarn {
			continue
		}
		// typed 容認（#45 D6）で畳んだ warn は「意図して残す gap」＝新規 warn に
		// 数えない（baseline ratchet の対象外）。currentWarns にも入れない——
		// 容認済みは stale 判定にも関与させない（baseline に載っていた entry が
		// 容認へ移行しても stale 扱いにせず、次の baseline update で自然整理する）。
		if f.AcknowledgedBy != "" {
			continue
		}
		key := store.BaselineEntry{Rule: f.Rule, Target: f.Target}
		currentWarns[key] = true
		if !inBaseline[key] {
			res.NewWarns = append(res.NewWarns, f)
		}
	}
	for _, e := range baseline.Findings {
		if !currentWarns[e] {
			res.Stale = append(res.Stale, e)
		}
	}
	return res, nil
}

func printLintCIText(cmd *cobra.Command, findings []lint.Finding, ci *ciResult) {
	out := cmd.OutOrStdout()
	if !ci.BaselinePresent {
		fmt.Fprintln(out, "lint --ci: baseline 不在（.scholia/lint-baseline.json）＝warn の歯止め（ratchet）は非活性です（error のみ exit 1）")
		return
	}
	errorCount, _, _ := countBySeverity(findings)
	fmt.Fprintf(out, "lint --ci: error %d / 新規 warn %d（baseline %d 件・stale %d 件）\n",
		errorCount, len(ci.NewWarns), ci.BaselineCount, len(ci.Stale))
	for _, e := range ci.Stale {
		fmt.Fprintf(out, "  [info] stale baseline entry: %s %s（現在は出ていません。次の `scholia lint baseline update` で自然消滅）\n", e.Rule, e.Target)
	}
	if len(ci.NewWarns) > 0 {
		fmt.Fprintf(out, "lint --ci: 新規 warn %d 件が baseline にありません:\n", len(ci.NewWarns))
		for _, f := range ci.NewWarns {
			fmt.Fprintf(out, "  %s: %s\n", f.Rule, f.Target)
		}
		fmt.Fprintln(out, "容認する場合は `scholia lint baseline update` を実行し、baseline の diff を PR でレビューしてください")
	}
}

func newLintBaselineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "lint --ci の warn 台帳（.scholia/lint-baseline.json）を操作する",
	}
	cmd.AddCommand(newLintBaselineUpdateCmd())
	return cmd
}

func newLintBaselineUpdateCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "baseline を現在の warn 集合（rule+target）で全置換する",
		Long: "baseline（.scholia/lint-baseline.json）を現在の warn 集合（rule+target キー・\n" +
			"message 非含有）で全置換する。拡大も縮小も 1 回の実行で同時に反映され、両方が\n" +
			"PR diff に現れてレビュー対象になる（棚上げの可視化）。baseline はこのコマンド\n" +
			"以外で書かない（rename／tx merge の target id 追随を除く）。",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			findings := lint.Run(snap)

			var entries []store.BaselineEntry
			for _, f := range findings {
				if f.Severity != lint.SeverityWarn {
					continue
				}
				// typed 容認（#45 D6）で畳んだ warn は baseline に載せない——
				// evaluateCI（lint --ci）が AcknowledgedBy を除外するのと揃える。
				// 揃えないと acknowledges で解消した gap が baseline に居座り、
				// stale として報告されるのに update で消えない不整合が起きる。
				if f.AcknowledgedBy != "" {
					continue
				}
				entries = append(entries, store.BaselineEntry{Rule: f.Rule, Target: f.Target})
			}

			old, err := s.LoadLintBaseline()
			if err != nil {
				return err
			}
			oldSet := make(map[store.BaselineEntry]bool)
			if old != nil {
				for _, e := range old.Findings {
					oldSet[e] = true
				}
			}
			newSet := make(map[store.BaselineEntry]bool, len(entries))
			for _, e := range entries {
				newSet[e] = true
			}
			added, removed := 0, 0
			for e := range newSet {
				if !oldSet[e] {
					added++
				}
			}
			for e := range oldSet {
				if !newSet[e] {
					removed++
				}
			}

			if err := s.SaveLintBaseline(store.LintBaseline{Findings: entries}); err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(struct {
					Path    string `json:"path"`
					Count   int    `json:"count"`
					Added   int    `json:"added"`
					Removed int    `json:"removed"`
				}{Path: s.LintBaselinePath(), Count: len(newSet), Added: added, Removed: removed})
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"lint-baseline.json を warn %d 件（rule+target）で全置換しました（追加 %d・削除 %d）\n",
				len(newSet), added, removed)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新サマリを JSON で出力する")
	return cmd
}
