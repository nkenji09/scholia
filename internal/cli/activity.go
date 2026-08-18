package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/activity"
	"github.com/nkenji09/scholia/internal/store"
)

// newActivityCmd は `scholia activity`（decision 01M09FHDJCV2WWFC7Z8331B0YQ・
// req.practice-observability）。記録が動いていない同じ症状が休眠（実装が止まって
// いる）なのか不健全（実装は動いているのに記録が動かない）なのかを見分けるため、
// 実装活動を git から導出して記録側の decision 件数と並べて出す。何も保存しない。
func newActivityCmd() *cobra.Command {
	var days int
	var sinceStr, untilStr string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "実装活動を git から導出して表示する（保存しない・休眠と不健全を見分ける材料）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			local, err := s.LoadLocalConfigOverride()
			if err != nil {
				return err
			}

			tzName := snap.Config.EffectiveTimezone(local)
			if tzName == "" {
				tzName = "UTC"
			}
			loc, err := time.LoadLocation(tzName)
			if err != nil {
				return fmt.Errorf("config.timezone %q を解決できません: %w", tzName, err)
			}

			now := time.Now()
			until := now.In(loc)
			if untilStr != "" {
				until, err = parseActivityBoundary(untilStr, loc)
				if err != nil {
					return fmt.Errorf("--until: %w", err)
				}
			}
			since := until.AddDate(0, 0, -days)
			if sinceStr != "" {
				since, err = parseActivityBoundary(sinceStr, loc)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
			}
			if !since.Before(until) {
				return fmt.Errorf("--since は --until より前である必要があります（since=%s until=%s）",
					since.Format(time.RFC3339), until.Format(time.RFC3339))
			}

			gitRoot, relPrefix, gitErr := activity.ResolveGitContext(snap.Root)
			if gitErr != nil {
				return writeNotGitManaged(cmd, asJSON)
			}

			decisionTimes := make([]time.Time, 0, len(snap.Decisions))
			for _, d := range snap.Decisions {
				t, err := time.Parse(time.RFC3339, d.At)
				if err != nil {
					return fmt.Errorf("decision %s の at %q を解釈できません: %w", d.ID, d.At, err)
				}
				decisionTimes = append(decisionTimes, t)
			}

			report, err := activity.Compute(activity.Options{
				GitRoot:       gitRoot,
				RelPrefix:     relPrefix,
				StoreDirName:  store.DirName,
				Window:        activity.Window{Since: since, Until: until},
				DecisionTimes: decisionTimes,
				Loc:           loc,
				Now:           now,
			})
			if err != nil {
				return err
			}

			if asJSON {
				return emitJSON(cmd, toActivityJSON(report, tzName))
			}
			writeActivityText(cmd.OutOrStdout(), report, tzName, loc)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 90, "窓の長さ（日）。--since が無いとき --until から遡って使う既定")
	cmd.Flags().StringVar(&sinceStr, "since", "", "窓の開始（YYYY-MM-DD または RFC3339。既定: --until から --days 日前）")
	cmd.Flags().StringVar(&untilStr, "until", "", "窓の終了（YYYY-MM-DD または RFC3339。既定: 今）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}

// parseActivityBoundary は --since/--until を解釈する。RFC3339（時刻・
// タイムゾーン込み）をまず試し、次に日付だけ（YYYY-MM-DD）を loc の 00:00:00 として
// 解釈する。⚠️ どちらの形でも、git へ渡す時点では時刻・タイムゾーンまで確定した
// 値になる（日付だけを git にそのまま渡すと「走らせた“いま”の時刻」に解決される
// 罠を避けるため・activity.windowCommits 参照）。
func parseActivityBoundary(s string, loc *time.Location) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(loc), nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q を日付として解釈できません（YYYY-MM-DD または RFC3339）: %w", s, err)
	}
	return t, nil
}

func writeNotGitManaged(cmd *cobra.Command, asJSON bool) error {
	if asJSON {
		return emitJSON(cmd, activityJSON{GitManaged: false})
	}
	fmt.Fprintln(cmd.OutOrStdout(), "実装活動（git から導出・保存しない）")
	fmt.Fprintln(cmd.OutOrStdout(), "⚠️ git 管理下ではありません。実装活動の数は出せません（decision-stale と同型・数を出さず名乗るだけに留める）。")
	return nil
}

func writeActivityText(w io.Writer, rep activity.Report, tzName string, loc *time.Location) {
	fmt.Fprintln(w, "実装活動（git から導出・保存しない）")
	fmt.Fprintf(w, "  窓            %d 日  [%s, %s)  %s\n",
		rep.WindowDays, rep.Since.In(loc).Format("2006-01-02"), rep.Until.In(loc).Format("2006-01-02"), tzName)

	if rep.Shallow {
		fmt.Fprintf(w, "  同じ窓の decision  %d 件\n", rep.DecisionsInWindow)
		fmt.Fprintln(w, "⚠️ 浅い clone です。実装 commit・実装が動いた日・最後の実装は出せません（full clone が必要）。")
		return
	}

	fmt.Fprintf(w, "  実装が動いた日  %d / %d 日\n", rep.ActiveDays, rep.WindowDays)
	fmt.Fprintf(w, "  実装 commit    %d 件（記録だけを触った commit %d 件は数えていない）\n", rep.ImplCommits, rep.RecordOnlyCommits)
	if rep.LastActivity != nil {
		fmt.Fprintf(w, "  最後の実装     %s（%d 日前）\n", rep.LastActivity.In(loc).Format("2006-01-02"), *rep.LastActivityDaysAgo)
	} else {
		fmt.Fprintln(w, "  最後の実装     なし")
	}
	fmt.Fprintf(w, "  同じ窓の decision  %d 件\n", rep.DecisionsInWindow)

	if rep.WindowPredatesStore {
		fmt.Fprintf(w, "⚠️ 窓の開始（%s）は記録ディレクトリの初出（%s）より前です。記録ディレクトリの改名等により、この期間の実装活動が水増しされている可能性があります。\n",
			rep.Since.In(loc).Format("2006-01-02"), rep.StoreFirstSeen.In(loc).Format("2006-01-02"))
	}
}

// activityJSON は `scholia activity --json` の応答封筒。GitManaged=false の
// ときは他の欄を持たない。Shallow=true のときは git 由来の欄（ActiveDays 以下）
// を省く——decisionsInWindow は git を経由しないので常に埋まる。
type activityJSON struct {
	GitManaged bool `json:"gitManaged"`

	Timezone   string    `json:"timezone,omitempty"`
	Since      time.Time `json:"since,omitzero"`
	Until      time.Time `json:"until,omitzero"`
	WindowDays int       `json:"windowDays,omitempty"`
	Shallow    bool      `json:"shallow,omitempty"`

	ActiveDays          *int       `json:"activeDays,omitempty"`
	ImplCommits         *int       `json:"implCommits,omitempty"`
	RecordOnlyCommits   *int       `json:"recordOnlyCommits,omitempty"`
	LastActivity        *time.Time `json:"lastActivity,omitempty"`
	LastActivityDaysAgo *int       `json:"lastActivityDaysAgo,omitempty"`

	DecisionsInWindow int `json:"decisionsInWindow"`

	WindowPredatesStore bool       `json:"windowPredatesStore,omitempty"`
	StoreFirstSeen      *time.Time `json:"storeFirstSeen,omitempty"`
}

func toActivityJSON(rep activity.Report, tzName string) activityJSON {
	out := activityJSON{
		GitManaged:          true,
		Timezone:            tzName,
		Since:               rep.Since,
		Until:               rep.Until,
		WindowDays:          rep.WindowDays,
		Shallow:             rep.Shallow,
		DecisionsInWindow:   rep.DecisionsInWindow,
		WindowPredatesStore: rep.WindowPredatesStore,
		StoreFirstSeen:      rep.StoreFirstSeen,
	}
	if !rep.Shallow {
		activeDays, implCommits, recordOnly := rep.ActiveDays, rep.ImplCommits, rep.RecordOnlyCommits
		out.ActiveDays = &activeDays
		out.ImplCommits = &implCommits
		out.RecordOnlyCommits = &recordOnly
		out.LastActivity = rep.LastActivity
		out.LastActivityDaysAgo = rep.LastActivityDaysAgo
	}
	return out
}
