package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
)

func newTxEditCmd() *cobra.Command {
	var action string
	var given, then, tags []string
	var asJSON bool
	var gate *gateFlags
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "遷移の指定フィールドのみ更新する（tx add と同一の検証を通す）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			s, err := openStore()
			if err != nil {
				return err
			}
			t, err := s.LoadTransition(id)
			if err != nil {
				return fmt.Errorf("transition %q を読み込めません: %w", id, err)
			}

			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			vocabByID := make(map[string]model.VocabEntry, len(snap.Vocab))
			for _, v := range snap.Vocab {
				vocabByID[v.ID] = v
			}
			tagByID := make(map[string]model.Tag, len(snap.Tags))
			for _, tg := range snap.Tags {
				tagByID[tg.ID] = tg
			}

			if cmd.Flags().Changed("action") {
				if err := checkVocabSlot(vocabByID, "action", []string{action}, model.CategoryAction); err != nil {
					return err
				}
				t.Action = action
			}
			if cmd.Flags().Changed("given") {
				if err := checkVocabSlot(vocabByID, "given", given, model.CategoryCondition); err != nil {
					return err
				}
				t.Given = given
			}
			if cmd.Flags().Changed("then") {
				if len(then) == 0 {
					return fmt.Errorf("--then を空にはできません（empty-then）")
				}
				if err := checkVocabSlot(vocabByID, "then", then, model.CategoryEffect); err != nil {
					return err
				}
				t.Then = then
			}
			if cmd.Flags().Changed("tags") {
				for _, tagID := range tags {
					if _, ok := tagByID[tagID]; !ok {
						return fmt.Errorf("tags: %q が実在しません", tagID)
					}
				}
				t.Tags = tags
			}

			// 書き込みゲート二層（#45 U3）: 既存 id の edit のため id-policy は
			// 対象外だが、exclusive-violation は編集後の given に対して検査する。
			advisories, allowed, gateErr := runWriteGate(cmd, snap, lint.WriteOp{Transition: &t, IsNew: false}, gate)
			if gateErr != nil {
				return gateErr
			}
			if err := s.SaveTransition(t); err != nil {
				return err
			}
			saved, err := s.LoadTransition(id)
			if err != nil {
				return err
			}

			if asJSON {
				return emitWriteJSON(cmd, saved, advisories, allowed, false)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "transition %s を更新しました\n", id)
			printWriteGateText(cmd, allowed, advisories)
			return nil
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "action の語彙 id")
	cmd.Flags().StringSliceVar(&given, "given", nil, "condition の語彙 id（カンマ区切り・完全置換）")
	cmd.Flags().StringSliceVar(&then, "then", nil, "effect の語彙 id（カンマ区切り・順序保存・完全置換）")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "タグ id（カンマ区切り・完全置換）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを応答封筒 { record, advisories } の JSON で出力する")
	gate = addGateAllowFlags(cmd)
	return cmd
}
