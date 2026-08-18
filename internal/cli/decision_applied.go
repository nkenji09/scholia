package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

// newDecisionAppliedCmd は「この decision を引いた結果、それが結論を決めた」
// 出来事のうち**矛盾・却下**を1件記録する口（01M09FHEQH7PVZ2BTKGXY5YMNN 変更4）。
//
// # なぜ decision を作る経路に紐づけないのか
//
// 却下の印を `review reject` に付ける設計は採らない。**その経路が実測でほとんど
// 使われていない**（本 repo の decision 187 件のうち、`review reject` の機構を
// 通った記録は0件）。使われていない経路にしか印が付かない設計は、
// **印を足したのに数が出ない状態を再現する。**
//
// だからこの口は、既存の decision を指して applied[] に1件足すだけである。
// その decision が `decide` で作られたか `review adopt` か viewer かを見ない。
//
// # ⚠️ この口には「通らないと先へ進めない」性質が無い
//
// `add-commit` は是正の着地手順に既に入っているので、種別を必須にすれば打たずに
// 進めない。**この口は呼ばなくても作業が完了する。** 呼ばせるのは配布スキルの
// 記述だけで、それは明文化であって歯止めではない（01KXS68HCNQ0H9QKNYFQ869J19 の型に
// 正面から当たる）。それでも置くのは、置かなければゼロのままだからである。
// **取れるかどうかは、後で数が出るかどうかで分かる。**
func newDecisionAppliedCmd() *cobra.Command {
	var kind, landed string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "applied <decisionId> --kind conflict|rejection [--landed <decisionId>]",
		Short: "引かれた decision に「記録が結論を決めた」印を1件足す（矛盾・却下・追記専用・判断欄位は不変）",
		Long: "引かれた decision に「記録が結論を決めた」印を1件足す（矛盾・却下）。\n\n" +
			"conflict は記録と衝突したので止めた場合、rejection は記録が既に決めていたので\n" +
			"採らなかった場合。--landed には着地した decision の id を渡す（rejection は必須。\n" +
			"conflict は「指摘のほうが誤りで記録が正しかった」なら何も着地しないので任意）。\n\n" +
			"是正（correction）はこの口では受けない——結ぶ commit があるので\n" +
			"`scholia decision add-commit <id> <hash> --kind correction` に相乗りする。",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := checkAppliedKindFlag(kind); err != nil {
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

			mark := model.AppliedMark{
				Kind:     kind,
				At:       time.Now().UTC().Format(time.RFC3339),
				Decision: landed,
			}
			// 形の検査（種別と指し先が噛み合っているか）は model に1つだけ置いて
			// ある。保存の口でも同じ関数が当たるが、ここで先に呼ぶのは、
			// 実在照合（下）より前に「何を直せばよいか」を出すためである。
			if err := model.ValidateAppliedMark(mark, id); err != nil {
				return err
			}

			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			// 指し先の decision の実在照合（decide --acknowledges・
			// --supersedes と同型）。
			if err := model.ValidateAppliedTargets(snap.Decisions, []model.AppliedMark{mark}); err != nil {
				return err
			}

			added := model.AppendAppliedMarks(d.Applied, []model.AppliedMark{mark})
			if len(added) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "decision %s: この印は既に付いています（冪等・変更なし）\n", id)
				return nil
			}
			d.Applied = append(append([]model.AppliedMark(nil), d.Applied...), added...)

			if err := s.UpdateDecision(d); err != nil {
				return err
			}
			saved, err := s.LoadDecision(id)
			if err != nil {
				return err
			}

			if asJSON {
				return emitWriteJSON(cmd, saved, nil, nil, false)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s に %s の印を足しました（applied=%d 件）\n",
				id, kind, len(saved.Applied))
			if landed != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  → 着地した decision: %s\n", landed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "印の種別（必須）。"+appliedKindHelp())
	cmd.Flags().StringVar(&landed, "landed", "", "着地した decision の id（rejection は必須・conflict は任意）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを JSON で出力する")
	return cmd
}

// appliedPortKinds はこの口が受ける種別（3値から是正を除いたもの）。
// **書き写さない**——model 側の3値から導く。種別が増えたらここも一緒に増える。
func appliedPortKinds() []string {
	var out []string
	for _, k := range model.AppliedKinds() {
		if k == model.AppliedCorrection {
			continue
		}
		out = append(out, k)
	}
	return out
}

func appliedKindHelp() string {
	return model.AppliedConflict + "＝記録と衝突したので止めた / " +
		model.AppliedRejection + "＝記録が既に決めていたので採らなかった"
}

// checkAppliedKindFlag は --kind がこの口の受ける値かを見る純関数（CLAUDE.md 1）。
// 是正は「この口には無い値」ではなく「別の口に在る値」として案内する。
func checkAppliedKindFlag(kind string) error {
	for _, k := range appliedPortKinds() {
		if k == kind {
			return nil
		}
	}
	if kind == model.AppliedCorrection {
		return fmt.Errorf("--kind %s はこの口では受けません。是正は結ぶ commit があるので "+
			"`scholia decision add-commit <decisionId> <hash> --kind %s` を使ってください",
			model.AppliedCorrection, model.AppliedCorrection)
	}
	if kind == "" {
		return fmt.Errorf("--kind は必須です（%s）", strings.Join(appliedPortKinds(), "|"))
	}
	return fmt.Errorf("--kind %q は %s のいずれかである必要があります", kind, strings.Join(appliedPortKinds(), "|"))
}
