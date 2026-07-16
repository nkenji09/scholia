package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/refs"
	"github.com/nkenji09/scholia/internal/store"
)

// renameRefsFlags are the --rewrite-refs/--no-refs flags shared by
// tag/vocab/tx rename (handoff "コマンド / フラグ"). Default (both false) is
// dry-run: scan and report, never touch source.
type renameRefsFlags struct {
	rewrite bool
	noRefs  bool
}

func (f *renameRefsFlags) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.rewrite, "rewrite-refs", false,
		"ソース中の旧 id 参照を境界安全にその場で書き換える（既定は走査のみ・ソース不変）")
	cmd.Flags().BoolVar(&f.noRefs, "no-refs", false,
		"ソース走査自体を省略する（rename のみ）")
}

// projectRoot returns the project root (parent of .scholia/) a Store was
// opened against. refs scanning/rewriting operates on source files outside
// .scholia/, so it needs the root, not the store dir.
func projectRoot(s *store.Store) string {
	return filepath.Dir(s.Dir)
}

// refsOptions loads config.sourceRefs (if set) and turns it into the
// refs.Options that scope file discovery — the wiring that makes
// config.json's sourceRefs.scan/exclude actually affect rename's implicit
// scan and `scholia refs scan|rewrite` (there is no `scholia config set` key for
// it yet; setting it today means editing config.json directly or via the
// store's Go API). A nil SourceRefs (the common case: unset) yields the
// zero-value Options, which narrows nothing — identical to behavior
// before this field existed.
func refsOptions(s *store.Store) (refs.Options, error) {
	cfg, err := s.LoadConfig()
	if err != nil {
		return refs.Options{}, err
	}
	if cfg.SourceRefs == nil {
		return refs.Options{}, nil
	}
	return refs.Options{Scan: cfg.SourceRefs.Scan, Exclude: cfg.SourceRefs.Exclude}, nil
}

// renameOutput is what tag/vocab/tx rename print in --json mode: the
// store's rename summary alongside the refs report (nil when --no-refs was
// given), in one JSON document.
type renameOutput[T any] struct {
	Rename T            `json:"rename"`
	Refs   *refs.Report `json:"refs,omitempty"`
}

// applyRenameRefs runs the shared rename→refs behavior (handoff "挙動テーブ
// ル") after a `.scholia` rename has already committed: with --no-refs, do
// nothing. Otherwise scan pairs' OldIDs in source; with --rewrite-refs,
// apply the boundary-safe rewrite; otherwise only collect the dry-run
// preview. Returns nil when refs scanning was skipped.
//
// The `.scholia` rename this runs after is already committed — fileTxn's
// scope ends there. Refs application is best-effort and never unwinds it;
// a per-file write failure is reported (and the caller should exit
// non-zero) but the rename stays in effect.
func applyRenameRefs(s *store.Store, pairs []refs.Pair, flags renameRefsFlags) (*refs.Report, error) {
	if flags.noRefs || len(pairs) == 0 {
		return nil, nil
	}
	opts, err := refsOptions(s)
	if err != nil {
		return nil, fmt.Errorf("config の読み込みに失敗しました（rename 自体は確定済みです）: %w", err)
	}
	report, err := refs.Execute(projectRoot(s), pairs, flags.rewrite, opts)
	if err != nil {
		return nil, fmt.Errorf("ソース走査に失敗しました（rename 自体は確定済みです）: %w", err)
	}
	return &report, nil
}

// printRenameRefsReport renders a refs report in the rename commands'
// human-readable (non-JSON) output.
func printRenameRefsReport(cmd *cobra.Command, report *refs.Report, rewrite bool) {
	if report == nil {
		return
	}
	out := cmd.OutOrStdout()
	for _, sk := range report.Skipped {
		fmt.Fprintf(out, "  skip (%s): %s\n", sk.Reason, sk.Path)
	}
	if len(report.Matches) == 0 {
		fmt.Fprintln(out, "ソース中に残存する旧 id の参照は見つかりませんでした。")
		return
	}
	if rewrite {
		fmt.Fprintf(out, "ソース参照を書き換えました（%d 箇所・%d ファイル）\n", len(report.Matches), len(report.RewrittenFiles))
		for _, f := range report.Failed {
			fmt.Fprintf(out, "  失敗: %s: %s（rename 自体は確定済み・scholia refs rewrite で再実行可）\n", f.Path, f.Err)
		}
		return
	}
	fmt.Fprintf(out, "ソース中に旧 id の参照が %d 箇所残っています（ソースは変更していません）:\n", len(report.Matches))
	for _, m := range report.Matches {
		fmt.Fprintf(out, "  %s:%d: %s\n", m.Path, m.Line, m.Text)
	}
	for _, p := range uniqueRewriteSuggestions(report.Matches) {
		fmt.Fprintf(out, "`scholia refs rewrite %s %s --apply` で置換できます\n", p.OldID, p.NewID)
	}
}

// uniqueRewriteSuggestions collects the distinct rewrite pairs worth
// suggesting to the user. Pairs where OldID == NewID are dropped — that
// shape only occurs from `scholia refs scan`'s inventory read (ScanIDs uses
// NewID=OldID as a placeholder Execute never uses in dry-run mode), where
// there is no rename in progress, and printing `scholia refs rewrite X X
// --apply` would be a no-op suggestion nobody should ever run.
func uniqueRewriteSuggestions(matches []refs.Match) []refs.Pair {
	seen := map[refs.Pair]bool{}
	var out []refs.Pair
	for _, m := range matches {
		if m.Old == m.New {
			continue
		}
		p := refs.Pair{OldID: m.Old, NewID: m.New}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OldID < out[j].OldID })
	return out
}

// refsFailedErr builds the non-zero-exit error for a rename command when
// --rewrite-refs applied but left one or more files unwritten. The `.scholia`
// rename itself is unaffected (already committed).
func refsFailedErr(report *refs.Report) error {
	if report == nil || len(report.Failed) == 0 {
		return nil
	}
	return fmt.Errorf("ソース書換に失敗したファイルがあります（%d 件。rename 自体は確定済み・scholia refs rewrite --apply で再実行可）", len(report.Failed))
}

func encodeRenameJSON[T any](cmd *cobra.Command, rename T, report *refs.Report) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(renameOutput[T]{Rename: rename, Refs: report})
}
