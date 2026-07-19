package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
)

func newTxAddCmd() *cobra.Command {
	var action string
	var given, then, tags []string
	var priority int
	var asJSON bool
	var gate *gateFlags
	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "遷移を 1 件追加する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if action == "" {
				return fmt.Errorf("--action は必須です")
			}
			if len(then) == 0 {
				return fmt.Errorf("--then は必須です（empty-then）")
			}
			var priorityPtr *int
			if cmd.Flags().Changed("priority") {
				if priority < 1 {
					return fmt.Errorf("--priority は 1 以上の整数です（nil=未宣言・小さいほど先に評価・同一 action 内でのみ意味）")
				}
				p := priority
				priorityPtr = &p
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			if s.TransitionExists(id) {
				return fmt.Errorf("transition %q は既に存在します", id)
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
			for _, t := range snap.Tags {
				tagByID[t.ID] = t
			}

			if err := checkVocabSlot(vocabByID, "action", []string{action}, model.CategoryAction); err != nil {
				return err
			}
			if err := checkVocabSlot(vocabByID, "given", given, model.CategoryCondition); err != nil {
				return err
			}
			if err := checkVocabSlot(vocabByID, "then", then, model.CategoryEffect); err != nil {
				return err
			}
			for _, tagID := range tags {
				if _, ok := tagByID[tagID]; !ok {
					return fmt.Errorf("tags: %q が実在しません", tagID)
				}
			}

			t := model.Transition{ID: id, Action: action, Given: given, Then: then, Tags: tags, Priority: priorityPtr}
			// 書き込みゲート二層（#45 U3）: reject（exclusive-violation・
			// id-policy）なら保存せず exit 1。advisory は保存後に同一ターン表示。
			advisories, allowed, gateErr := runWriteGate(cmd, snap, lint.WriteOp{Transition: &t, IsNew: true}, gate)
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
			fmt.Fprintf(cmd.OutOrStdout(), "transition %s を作成しました\n", id)
			printWriteGateText(cmd, allowed, advisories)
			return nil
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "action の語彙 id（必須）")
	cmd.Flags().StringSliceVar(&given, "given", nil, "condition の語彙 id（カンマ区切り、複数指定可）")
	cmd.Flags().StringSliceVar(&then, "then", nil, "effect の語彙 id（カンマ区切り、順序保存、必須）")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "タグ id（カンマ区切り、複数指定可）")
	cmd.Flags().IntVar(&priority, "priority", 0, "同一 action 内の評価順（1 始まり・小さいほど先・未指定＝未宣言・#45 D8）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "作成したレコードを応答封筒 { record, advisories } の JSON で出力する")
	gate = addGateAllowFlags(cmd)
	return cmd
}

func checkVocabSlot(vocabByID map[string]model.VocabEntry, slot string, ids []string, wantCategory string) error {
	for _, id := range ids {
		v, ok := vocabByID[id]
		if !ok {
			return fmt.Errorf("%s: %q が実在する語彙を参照していません", slot, id)
		}
		if v.Category != wantCategory {
			return fmt.Errorf("%s: %q は %s カテゴリの語彙ではありません（実際は %s）", slot, id, wantCategory, v.Category)
		}
	}
	return nil
}
