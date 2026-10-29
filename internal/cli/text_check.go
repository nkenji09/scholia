package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/diff"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
	"github.com/nkenji09/scholia/internal/textcheck"
)

// 問いの出どころ。人向けの行と --json の textCheck.question に出る。
const (
	questionSourceDefault = "既定"
	questionSourceConfig  = "config"
)

// textCheckJSON は、--json の textCheck の欄（decide --dry-run と text-check だけが持つ）。
type textCheckJSON struct {
	// Checked は、読み手が答えを返して確かめが終わったことを表す。
	Checked  bool `json:"checked"`
	NoReader bool `json:"noReader,omitempty"`
	Accepted bool `json:"accepted,omitempty"`
	// Reason は、読ませる文が無く読み手を呼ばなかったときの理由。
	Reason     string                 `json:"reason,omitempty"`
	Question   string                 `json:"question,omitempty"`
	Candidates []textcheck.Candidate  `json:"candidates"`
	NotChecked []textcheck.NotChecked `json:"notChecked"`
}

// textCheckOutcome は、読ませた結果と、問いの出どころ。Ran は読み手を実際に起動したことを表す。
type textCheckOutcome struct {
	Result textcheck.Result
	Source string
	Ran    bool
	// Accepted は、--accept-text-candidates で読み手を呼ばずに通したことを表す。
	Accepted bool
	// NoTexts は、読ませる文が 1 つも無く、読み手を呼ばなかったことを表す。
	NoTexts bool
}

const textCheckAcceptedLine = "確かめ: 候補を受け入れて読み手を呼ばなかった"
const textCheckAcceptFlag = "accept-text-candidates"

func (o textCheckOutcome) json() *textCheckJSON {
	j := &textCheckJSON{
		Checked:    o.Result.Checked() && !o.Accepted && !o.NoTexts,
		Accepted:   o.Accepted,
		Reason:     o.noTextsReason(),
		NoReader:   o.Result.NoReader,
		Candidates: o.Result.Candidates,
		NotChecked: o.Result.NotChecked,
	}
	if o.Ran {
		j.Question = o.Source
	}
	if j.Candidates == nil {
		j.Candidates = []textcheck.Candidate{}
	}
	if j.NotChecked == nil {
		j.NotChecked = []textcheck.NotChecked{}
	}
	return j
}

// jsonIfAny は、読み手の指定が無かったときは nil（textCheck の欄を出さない）、あれば textCheck の欄を返す。
func (o textCheckOutcome) jsonIfAny() *textCheckJSON {
	if o.Result.NoReader {
		return nil
	}
	return o.json()
}

// lines は、人向けの行。読み手が走ったときは、最初に問いの出どころの行を足す
// （config の名前の打ち間違いで、上書きが効いていないことに気づけるようにするため）。
const noTextsLine = "確かめ: 読ませる文が無い（読み手は呼んでいない）"

func (o textCheckOutcome) noTextsReason() string {
	if o.NoTexts {
		return "読ませる文が無い"
	}
	return ""
}

func (o textCheckOutcome) lines() []string {
	if o.NoTexts {
		return []string{noTextsLine}
	}
	if o.Accepted {
		return []string{textCheckAcceptedLine}
	}
	lines := textcheck.Lines(o.Result)
	if o.Ran {
		lines = append([]string{"確かめ: 問いは " + o.Source}, lines...)
	}
	return lines
}

// runTextCheck は、記録の文を読み手に 1 回だけ読ませる。decide --dry-run と text-check が共有する。
// 読み手の指定が無ければ、config も読まずに指定が無いことを返す。読ませる文が無ければ、読み手を起動しない。
func runTextCheck(cmd *cobra.Command, s *store.Store, records []textcheck.Record) (textCheckOutcome, error) {
	reader := textcheck.Reader{Stderr: cmd.ErrOrStderr()}
	if reader.Named() == "" {
		return textCheckOutcome{Result: textcheck.Result{NoReader: true}}, nil
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		return textCheckOutcome{}, err
	}
	var withTexts []textcheck.Record
	for _, r := range records {
		if len(r.Texts) > 0 {
			withTexts = append(withTexts, r)
		}
	}
	// 読み手を起動できない指定は、読ませる文の有無より先に見る。
	if reason := reader.Precheck(); reason != "" {
		return textCheckOutcome{Result: textcheck.NotCheckedAll(withTexts, reason)}, nil
	}
	if len(withTexts) == 0 {
		return textCheckOutcome{NoTexts: true}, nil
	}
	// 読み込んだ値が空白だけ（手で書かれた config）なら、読み手を呼ばず、確かめが行われなかったとして扱う。
	if err := model.ValidateReaderQuestion(cfg.ReaderQuestion); err != nil {
		return textCheckOutcome{Result: textcheck.NotCheckedAll(withTexts, "readerQuestion が空白だけ")}, nil
	}
	question, source := textcheck.DefaultQuestion, questionSourceDefault
	if cfg.ReaderQuestion != "" {
		question, source = cfg.ReaderQuestion, questionSourceConfig
	}
	return textCheckOutcome{Result: reader.Read(question, withTexts), Source: source, Ran: true}, nil
}

func newTextCheckCmd() *cobra.Command {
	var base string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "text-check <kind>:<id>... | --base <ref>",
		Short: "記録の文を読み手の AI に読ませ、読む人によって意味が分かれうる言葉の候補を出す",
		Long: "指定した記録（decision:<id>・tx:<id>・tag:<id>・vocab:<id>）、または --base <ref> と比べて\n" +
			"文が足された・変わった記録の文の欄を、まとめて 1 回だけ読み手に読ませる。\n" +
			"読み手は、環境変数 " + textcheck.Variable + " に書いた実行ファイル 1 つ（名前か絶対パス・引数なし）。\n" +
			"リポジトリの設定からは読まない。transition には文の欄が無いので、実在の確認だけで読ませない。\n\n" +
			"終了コード: 確かめ終えたら候補の有無によらず 0。確かめが行われなかった（読み手の指定が無い・\n" +
			"時間切れ・起動できない・答えの形の誤り）なら 1。名指しした記録が無ければ、読み手を呼ばずに 1。",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && base == "" {
				return fmt.Errorf("記録（<kind>:<id>）か --base <ref> のどちらかが必要です")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			targets, missing, err := namedTextTargets(snap, args)
			if err != nil {
				return err
			}
			if len(missing) > 0 {
				return fmt.Errorf("次の記録が実在しません（読み手は呼んでいません）: %s", strings.Join(missing, ", "))
			}
			if base != "" {
				res, err := diff.Diff(s, base)
				if err != nil {
					return err
				}
				targets = append(targets, changedTextTargets(res)...)
			}
			outcome, err := runTextCheck(cmd, s, textRecords(targets))
			if err != nil {
				return err
			}
			if asJSON {
				if err := emitJSON(cmd, struct {
					TextCheck *textCheckJSON `json:"textCheck"`
				}{outcome.json()}); err != nil {
					return err
				}
			} else if outcome.Result.NoReader {
				fmt.Fprintln(cmd.OutOrStdout(), "確かめ: 読み手が指定されていない（環境変数 "+textcheck.Variable+" に実行ファイルを書く）")
			} else {
				for _, l := range outcome.lines() {
					fmt.Fprintln(cmd.OutOrStdout(), l)
				}
			}
			if !outcome.Result.Checked() {
				return fmt.Errorf("確かめが行われませんでした")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "この ref と比べて文が足された・変わった記録を対象にする（scholia diff と同じ比べ方）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "結果を { textCheck } の JSON で出力する")
	return cmd
}

// textTarget は、読ませる記録 1 件。kind と id の組で同じ記録かを決める。
type textTarget struct {
	kind, id string
	texts    []textcheck.Text
}

func textField(field, text string) []textcheck.Text {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return []textcheck.Text{{Field: field, Text: text}}
}

func decisionTexts(d model.Decision) []textcheck.Text {
	return append(textField("why", d.Why), textField("changed", d.Changed)...)
}

func tagTexts(t model.Tag) []textcheck.Text {
	return append(textField("name", t.Name), textField("description", t.Description)...)
}

func vocabTexts(v model.VocabEntry) []textcheck.Text {
	return append(textField("label", v.Label), textField("description", v.Description)...)
}

// namedTextTargets は、<kind>:<id> の指定を記録にする。transition は文の欄が無いので、実在だけ確かめる。
func namedTextTargets(snap store.Snapshot, args []string) (targets []textTarget, missing []string, err error) {
	seen := map[string]bool{}
	for _, arg := range args {
		kind, id, ok := strings.Cut(arg, ":")
		if !ok || id == "" {
			return nil, nil, fmt.Errorf("記録の指定が <kind>:<id> の形ではありません: %q（kind は decision・tx・tag・vocab）", arg)
		}
		if kind == "transition" {
			kind = "tx"
		}
		if seen[kind+":"+id] {
			continue
		}
		seen[kind+":"+id] = true
		found := false
		switch kind {
		case "decision":
			for _, d := range snap.Decisions {
				if d.ID == id {
					found, targets = true, append(targets, textTarget{kind, id, decisionTexts(d)})
				}
			}
		case "tx":
			for _, t := range snap.Transitions {
				if t.ID == id {
					found = true
				}
			}
		case "tag":
			for _, t := range snap.Tags {
				if t.ID == id {
					found, targets = true, append(targets, textTarget{kind, id, tagTexts(t)})
				}
			}
		case "vocab":
			for _, v := range snap.Vocab {
				if v.ID == id {
					found, targets = true, append(targets, textTarget{kind, id, vocabTexts(v)})
				}
			}
		default:
			return nil, nil, fmt.Errorf("記録の kind は decision・tx・tag・vocab のいずれかです: %q", arg)
		}
		if !found {
			missing = append(missing, arg)
		}
	}
	return targets, missing, nil
}

// changedTextTargets は、diff の結果から、足された記録と、文の欄が変わった記録を取る（変わらない記録は渡さない）。
// 渡すのは、その記録の変更後の文の欄すべて。
func changedTextTargets(res diff.Result) []textTarget {
	var targets []textTarget
	add := func(kind, id string, texts []textcheck.Text) {
		targets = append(targets, textTarget{kind, id, texts})
	}
	for _, d := range res.Decisions.Added {
		add("decision", d.ID, decisionTexts(d))
	}
	for _, c := range res.Decisions.Changed {
		if !slices.Equal(decisionTexts(c.Before), decisionTexts(c.After)) {
			add("decision", c.ID, decisionTexts(c.After))
		}
	}
	for _, t := range res.Tags.Added {
		add("tag", t.ID, tagTexts(t))
	}
	for _, c := range res.Tags.Changed {
		if !slices.Equal(tagTexts(c.Before), tagTexts(c.After)) {
			add("tag", c.ID, tagTexts(c.After))
		}
	}
	for _, v := range res.Vocab.Added {
		add("vocab", v.ID, vocabTexts(v))
	}
	for _, c := range res.Vocab.Changed {
		if !slices.Equal(vocabTexts(c.Before), vocabTexts(c.After)) {
			add("vocab", c.ID, vocabTexts(c.After))
		}
	}
	return targets
}

// textRecords は、読み手に渡す記録にする。対象を kind と id の組で重複なしにそろえてから、同じ id が kind をまたいで重なるときだけ、key を kind:id にする
// （読み手が返す key で、どの記録かを一意に決めるため）。
func textRecords(targets []textTarget) []textcheck.Record {
	var unique []textTarget
	seenTarget := map[string]bool{}
	for _, t := range targets {
		if !seenTarget[t.kind+":"+t.id] {
			seenTarget[t.kind+":"+t.id] = true
			unique = append(unique, t)
		}
	}
	count := map[string]int{}
	for _, t := range unique {
		count[t.id]++
	}
	records := make([]textcheck.Record, 0, len(unique))
	for _, t := range unique {
		key := t.id
		if count[t.id] > 1 {
			key = t.kind + ":" + t.id
		}
		records = append(records, textcheck.Record{ID: key, Texts: t.texts})
	}
	return records
}

// decideTextCheck は、decide が保存しようとしている文を読ませる。accept が真なら読み手を呼ばない
// （読み手の指定が無いときは、指定が無いことを返し、何も出さない）。
func decideTextCheck(cmd *cobra.Command, s *store.Store, d model.Decision, accept bool) (textCheckOutcome, error) {
	if accept && (textcheck.Reader{}).Named() != "" {
		return textCheckOutcome{Accepted: true}, nil
	}
	return runTextCheck(cmd, s, []textcheck.Record{{ID: decideTextCheckKey, Texts: decisionTexts(d)}})
}

// textCheckBlockError は、候補があって decide を止めるときの拒み。decide が保存前の拒みに使う出口
// （runWriteGate と同じ reject(<規則>): の行＋エラー）を通す。候補が無ければ nil。
func textCheckBlockError(cmd *cobra.Command, o textCheckOutcome) error {
	if len(o.Result.Candidates) == 0 {
		return nil
	}
	lines := textcheck.Lines(textcheck.Result{Candidates: o.Result.Candidates})
	lines = append(lines, "直すと決めた所だけ直し、直した後は --"+textCheckAcceptFlag+" を付けて decide を打ち直す。直した文をもう一度読ませない（読み手は言い換えるたびに別の候補を出すため）")
	fmt.Fprintf(cmd.ErrOrStderr(), "reject(%s): %s\n", textCheckDecideBlocked, strings.Join(lines, "\n"))
	return fmt.Errorf("reject: 読み手が挙げた候補が %d 件あるため保存しませんでした（%s）", len(o.Result.Candidates), textCheckDecideBlocked)
}

const textCheckDecideBlocked = "text-check-decide-blocked"

// printTextCheckLines は、確かめの行を出す。--json のときは標準出力が JSON なので、標準エラーへ出す。
func printTextCheckLines(cmd *cobra.Command, o textCheckOutcome, asJSON bool) {
	w := cmd.OutOrStdout()
	if asJSON {
		w = cmd.ErrOrStderr()
	}
	for _, l := range o.lines() {
		fmt.Fprintln(w, l)
	}
}
