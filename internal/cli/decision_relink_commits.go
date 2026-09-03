package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/gitio"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// newDecisionRelinkCommitsCmd は「結んだ commit がどれも HEAD から辿れない」
// decision について、着地先の候補を探して見せる（decision 01M1JY0APWXHFZ1TKWST7VPS9N）。
//
// # 何を頼りに探すのか
//
// **手元に残っている commit の見出しを読み、同じ見出しを持つ commit を HEAD の
// 系譜から探す。** squash merge では、潰した commit の見出しが squash 後の本文に
// 並ぶのが GitHub の既定なので、同じ探し方が効く（ただしこれは導出であって、
// squash 運用のストアで実測してはいない）。作業ブランチの作り直し（rebase）では
// 見出しがそのまま残るので確実に効く——実測: この repo の辿れない 91 件は
// 91 件すべてが候補1件に定まった。
//
// # なぜ既定で書かないのか
//
// `commits[]` は追記専用で、**誤って足したハッシュは永久に消せない**
// （01KXS4BB6B8FNFXQV9PEDK336S）。見出しが偶然一致した別の commit を結ぶ事故は
// 取り消せないので、既定は「これが辿れない・これが候補だ」と見せるだけにし、
// 人が納得してから --apply で確定する。
//
// # 落ちない範囲（射程・正直に名乗る）
//
//   - 🔴 **手元にオブジェクトが残っていなければ救済できない。** 新しく clone した
//     後・GC の後は見出しすら読めない。**この口は「切れてから直す」ためのもので、
//     気づく手立ては lint の commit-unreachable が担う**——対で使う。
//   - **見出しを書き換えて取り込んだ場合は見つからない。** 「特定できず」に並べる。
//   - **同じ見出しの commit が複数あれば1つに定めない。** 選ぶのは人の仕事である。
//   - **見出しが一致しただけで、それが同じ変更である保証は無い。** 中身は見ていない。
func newDecisionRelinkCommitsCmd() *cobra.Command {
	var apply bool
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "relink-commits",
		Short: "取り込みで祖先から外れた commits[] の結線を、着地先の候補へ結び直す",
		Long: "結んだ commit がどれも HEAD から辿れない decision を集め、\n" +
			"同じ見出しを持つ commit を HEAD の系譜から探して候補として見せる。\n\n" +
			"既定では1バイトも書かない。--apply を付けたときだけ、候補が1件に\n" +
			"定まったものを commits[] へ追記する（追記専用・取り消せない）。",
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
			projectRoot := snap.Root
			reachable, derived, failure := gitio.ReachableFromHead(projectRoot)
			if failure != nil {
				return fmt.Errorf("HEAD から辿れる commit を導出できません: %w", failure)
			}
			if !derived {
				return fmt.Errorf("HEAD から辿れる commit を導出できません（git 管理下でない・commit が1件も無い・浅い clone のいずれか）。この口は git の履歴を頼りに候補を探すため、導出できないと何も答えられません")
			}
			gitRoot, _, err := gitio.ResolveContext(projectRoot)
			if err != nil {
				return err
			}

			targets := relinkTargets(snap.Decisions, reachable)
			for i := range targets {
				for j := range targets[i].Hashes {
					h := &targets[i].Hashes[j]
					h.Subject = commitSubject(gitRoot, h.Hash)
					if h.Subject != "" {
						h.Matches = commitsWithSubject(gitRoot, h.Subject)
					}
					h.Resolved, h.Reason = resolveRelink(*h)
				}
			}

			if apply {
				if err := applyRelink(s, targets); err != nil {
					return err
				}
			}
			if asJSON {
				return emitJSON(cmd, targets)
			}
			printRelinkReport(cmd, targets, apply)
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "候補が1件に定まったものを commits[] へ追記する（追記専用・取り消せない）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "結果を JSON で出力する")
	return cmd
}

// relinkHash は辿れなくなった結線1件と、その着地先の候補。
type relinkHash struct {
	Hash string `json:"hash"`
	// Subject は手元に元の commit が残っていれば、その見出し（1行目）。
	// 残っていなければ空——**候補を探す手がかりがこれしか無い。**
	Subject string `json:"subject,omitempty"`
	// Matches は同じ見出しを持つ、HEAD から辿れる commit。
	Matches  []string `json:"matches,omitempty"`
	Resolved string   `json:"resolved,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Applied  bool     `json:"applied,omitempty"`
}

// relinkTarget は付け替えの対象（decision 1件ぶん）。
type relinkTarget struct {
	DecisionID string       `json:"decision"`
	Hashes     []relinkHash `json:"hashes"`
}

// relinkTargets は「commits[] に載っているどのハッシュも HEAD から辿れない」
// decision を集める純関数（git を呼ばない）。
//
// ⚠️ **判定は lint の commit-unreachable と同じ「1つも辿れない」でなければならない。**
// ここだけ「1件でも辿れない」にすると、finding が出ていない decision まで書き換える
// 口になる——**気づかせる範囲と直す範囲がずれる。**
func relinkTargets(decisions []model.Decision, reachable gitio.ReachableSet) []relinkTarget {
	var out []relinkTarget
	for _, d := range decisions {
		if len(d.Commits) == 0 {
			continue
		}
		hashes := make([]relinkHash, 0, len(d.Commits))
		for _, h := range d.Commits {
			if reachable.Contains(h) {
				hashes = nil
				break
			}
			hashes = append(hashes, relinkHash{Hash: h})
		}
		if len(hashes) == 0 {
			continue
		}
		out = append(out, relinkTarget{DecisionID: d.ID, Hashes: hashes})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DecisionID < out[j].DecisionID })
	return out
}

// resolveRelink は集めた候補から結び直し先を決める純関数。
// 定まらなかったときは、なぜ定まらなかったかを返す（黙って諦めない）。
func resolveRelink(h relinkHash) (resolved, reason string) {
	switch {
	case h.Subject == "":
		return "", "元の commit がこの clone に残っていません（見出しを読めないので候補を探せません）"
	case len(h.Matches) == 0:
		return "", "同じ見出しの commit が HEAD の系譜にありません（見出しを書き換えて取り込んだ可能性があります）"
	case len(h.Matches) > 1:
		return "", fmt.Sprintf("同じ見出しの commit が %d 件あり、1つに定まりません（選ぶのは人の仕事です）", len(h.Matches))
	default:
		return h.Matches[0], ""
	}
}

// commitSubject は commit の見出し（1行目）を返す。手元に無ければ空。
func commitSubject(gitRoot, hash string) string {
	out, err := gitio.Run(gitRoot, "log", "-1", "--format=%s", hash)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// commitsWithSubject は「その見出しを本文に含む」HEAD から辿れる commit を返す。
//
// ⚠️ **見出しの一致は本文全体に掛ける**（--grep は subject と body の両方を見る）。
// squash merge は潰した commit の見出しを本文に並べるので、subject だけを見ると
// squash 後の commit を取り逃がす。
func commitsWithSubject(gitRoot, subject string) []string {
	out, err := gitio.Run(gitRoot, "log", "HEAD", "--fixed-strings", "--grep", subject, "--format=%H")
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}

// applyRelink は候補が1件に定まったハッシュを commits[] へ追記する。
// 種別は実装（implementation）——付け替えは是正ではないので applied[] に印は付けない。
func applyRelink(s *store.Store, targets []relinkTarget) error {
	for i := range targets {
		t := &targets[i]
		var add []string
		for j := range t.Hashes {
			if r := t.Hashes[j].Resolved; r != "" {
				add = append(add, r)
			}
		}
		if len(add) == 0 {
			continue
		}
		d, err := s.LoadDecision(t.DecisionID)
		if err != nil {
			return fmt.Errorf("decision %q を読み込めません: %w", t.DecisionID, err)
		}
		d.Commits = dedupeAppend(d.Commits, add)
		if _, err := s.UpdateDecision(d); err != nil {
			return fmt.Errorf("decision %q を更新できません: %w", t.DecisionID, err)
		}
		for j := range t.Hashes {
			if t.Hashes[j].Resolved != "" {
				t.Hashes[j].Applied = true
			}
		}
	}
	return nil
}

func printRelinkReport(cmd *cobra.Command, targets []relinkTarget, applied bool) {
	w := cmd.OutOrStdout()
	if len(targets) == 0 {
		fmt.Fprintln(w, "結んだ commit がどれも HEAD から辿れない decision はありません")
		return
	}
	resolved, unresolved := 0, 0
	fmt.Fprintf(w, "結線が切れている decision: %d 件\n\n", len(targets))
	for _, t := range targets {
		fmt.Fprintf(w, "decision %s\n", t.DecisionID)
		for _, h := range t.Hashes {
			subject := h.Subject
			if subject == "" {
				subject = "（見出しを読めません）"
			}
			fmt.Fprintf(w, "  %s  %s\n", shortHash(h.Hash), subject)
			if h.Resolved != "" {
				resolved++
				mark := "→"
				if h.Applied {
					mark = "✔"
				}
				fmt.Fprintf(w, "    %s %s へ結び直す\n", mark, shortHash(h.Resolved))
				continue
			}
			unresolved++
			fmt.Fprintf(w, "    ✗ 特定できず: %s\n", h.Reason)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "特定できた: %d 件 / 特定できず: %d 件\n", resolved, unresolved)
	if applied {
		fmt.Fprintf(w, "特定できたものを commits[] へ追記しました（追記専用・取り消せません）\n")
		return
	}
	fmt.Fprintln(w, "何も書いていません。--apply を付けると、特定できたものを commits[] へ追記します（追記専用・取り消せません）")
}

// shortHash は表示用の短縮（lint の同名関数と同じ長さに揃える）。
func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}
