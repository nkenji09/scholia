package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
)

// newDecisionAddCommitCmd は既存 decision の commits[] に追記専用で足す
// （§3.5 append-only の精緻化）。target/why/changed/ref/at ら判断フィールドは
// 一切書き換えない — 実装ミス直し等で decision を無駄に増やさないための経路。
//
// # 種別（--kind）は必須（01M09FHEQH7PVZ2BTKGXY5YMNN 変更3）
//
// 値は2つ。**実装**（この decision の判断を実装した commit・初回／続き／後付け
// 結線を含む）と、**是正**（この decision に書いてあるとおりに実装されていな
// かったのを直した commit）。是正のときは引かれた側——つまりこの decision——の
// applied[] に印を1件足す。
//
// ⚠️ **既定値を置かない。** 打ち忘れが機械で捕まえられない以上、既定値は
// 「打ち忘れたぶんだけ是正が少なく見える」という一方向に偏った誤差になる。
// 観測を足す目的は、少なく見える数字を作ることではない。
//
// ⚠️ **この口が是正の打ち忘れを止められるのは、`add-commit` が是正の着地手順に
// 既に入っているからである。** 矛盾・却下の口（`decision applied`）には同じ
// 性質が無い——呼ばなくても作業が完了する。
func newDecisionAddCommitCmd() *cobra.Command {
	var asJSON bool
	var kind string
	cmd := &cobra.Command{
		Use:   "add-commit <decisionId> <hash> [<hash>...] --kind implementation|correction",
		Short: "decision に実装コミットを追記する（追加専用・判断フィールドは不変・§3.5）",
		Long: "decision に実装コミットを追記する（追加専用・判断フィールドは不変・§3.5）。\n\n" +
			"--kind は必須。implementation はこの decision の判断を実装した commit、\n" +
			"correction はこの decision に書いてあるとおりに実装されていなかったのを\n" +
			"直した commit で、correction のときは applied[] に是正の印を1件足す。",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			hashes := args[1:]

			correction, err := parseAddCommitKind(kind)
			if err != nil {
				return err
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			d, err := s.LoadDecision(id)
			if err != nil {
				return fmt.Errorf("decision %q を読み込めません: %w", id, err)
			}

			d.Commits = dedupeAppend(d.Commits, hashes)
			var addedMarks []model.AppliedMark
			if correction {
				now := time.Now().UTC().Format(time.RFC3339)
				candidates := make([]model.AppliedMark, 0, len(hashes))
				for _, h := range hashes {
					candidates = append(candidates, model.AppliedMark{
						Kind: model.AppliedCorrection, At: now, Commit: h,
					})
				}
				addedMarks = model.AppendAppliedMarks(d.Applied, candidates)
				d.Applied = append(append([]model.AppliedMark(nil), d.Applied...), addedMarks...)
			}

			// 書き込みゲート二層（#45 U3）: add-commit に reject 規則は無い
			//（commits 追記のみ・判断欄位は不変）。既存 why/changed への
			// advisory は acknowledge-only として同一ターンに表示される。
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			advisories, allowed, gateErr := runWriteGate(cmd, snap, lint.WriteOp{Decision: &d, IsNew: false}, nil)
			if gateErr != nil {
				return gateErr
			}
			// desc 現在形ゲート三点配線の第3点（#45 D7）: 実装結線（add-commit）と
			// 同一ターンに、対象 desc の鮮度（stale-tense）を advisory で気づかせる。
			advisories = append(advisories, lint.TargetDescStaleTense(snap, d.Target)...)
			// 結ぶ commit の実在照合は保存の口（store）で当たる。ここでは
			// 「照合できたかどうか」だけを先に確かめて、後で名乗るために持つ。
			repo := s.CommitRepo()
			if err := s.UpdateDecision(d); err != nil {
				return err
			}
			saved, err := s.LoadDecision(id)
			if err != nil {
				return err
			}

			if asJSON {
				return emitWriteJSON(cmd, saved, advisories, allowed, false)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s に commits を追加しました（commits=%d 件）\n", id, len(saved.Commits))
			if len(addedMarks) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  是正の印を %d 件足しました（applied=%d 件）\n", len(addedMarks), len(saved.Applied))
			}
			writeCommitVerifyNotice(cmd, repo, len(hashes))
			printWriteGateText(cmd, allowed, advisories)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを応答封筒 { record, advisories } の JSON で出力する")
	cmd.Flags().StringVar(&kind, "kind", "", "commit の種別（必須）。"+addCommitKindHelp())
	return cmd
}

// addCommitKinds は `add-commit --kind` が受ける2値。
//
// ⚠️ **applied[] の3値（model.AppliedKinds）とは別の集合である。** 矛盾・却下は
// 結ぶ commit を持たないので、この口には乗らない（乗せると hash の無い呼び出しが
// できてしまい、実在照合が効かない印の作り方が1つ増える）。それらは
// `scholia decision applied` が受ける。
const (
	addCommitKindImplementation = "implementation"
	addCommitKindCorrection     = model.AppliedCorrection
)

func addCommitKindHelp() string {
	return addCommitKindImplementation + "＝この decision の判断を実装した commit / " +
		addCommitKindCorrection + "＝書いてあるとおりに実装されていなかったのを直した commit（applied[] に是正の印が1件付く）"
}

// parseAddCommitKind は --kind を解釈する純関数（CLAUDE.md 1）。
// 戻り値は「是正か」。既定値は無く、空は error。
func parseAddCommitKind(kind string) (correction bool, err error) {
	switch kind {
	case addCommitKindImplementation:
		return false, nil
	case addCommitKindCorrection:
		return true, nil
	case "":
		return false, fmt.Errorf("--kind は必須です（%s）。"+
			"既定値は置いていません——省略できると、打ち忘れたぶんだけ是正が少なく見える一方向の誤差になるためです",
			strings.Join([]string{addCommitKindImplementation, addCommitKindCorrection}, "|"))
	}
	return false, fmt.Errorf("--kind %q は %s のいずれかである必要があります", kind,
		strings.Join([]string{addCommitKindImplementation, addCommitKindCorrection}, "|"))
}
