package cli

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// これらはリリースビルド時に goreleaser が ldflags(-X)で上書きする。
// 変数パスは .goreleaser.yaml の -X github.com/nkenji09/scholia/internal/cli.<name> と厳密一致させること。
// 既定値 "dev"（version）／空（commit・date）のままなら未注入とみなし、
// runtime/debug.ReadBuildInfo に fallback する（model: T-version-report-buildinfo）。
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// versionInfo は scholia version の出力（人間可読／--json 共通）の形。
type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	Date      string `json:"date,omitempty"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// resolveVersionInfo は版の解決順を実装する（decision: req.release-versioning）：
//
//	(a) ldflags 注入された version が既定値 "dev" 以外ならそれを最優先（T-version-report-injected）
//	(b) 未注入なら runtime/debug.ReadBuildInfo() の Main.Version（go install …@vX の
//	    module 版や "(devel)"）に fallback（T-version-report-buildinfo）
//	(c) いずれも無ければ "dev"
//
// commit/date も未注入時は build info の vcs.revision / vcs.time で補う。
func resolveVersionInfo() versionInfo {
	v, c, d := version, commit, date

	if v == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" {
				v = bi.Main.Version
			}
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					if c == "" {
						c = s.Value
					}
				case "vcs.time":
					if d == "" {
						d = s.Value
					}
				}
			}
		}
	}

	return versionInfo{
		Version:   v,
		Commit:    c,
		Date:      d,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// newVersionCmd は scholia version コマンド（additive・既存コマンドに影響しない）。
func newVersionCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "scholia の版を表示する",
		Long: `scholia バイナリの版を表示する。

版の解決順:
  1. リリースビルド（goreleaser）が ldflags で焼き込んだ版タグ
  2. 未注入なら runtime/debug.ReadBuildInfo 由来の版
     （go install …@vX の module 版・vcs.revision/time）
  3. いずれも無ければ "dev"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := resolveVersionInfo()

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "scholia %s\n", info.Version)
			if info.Commit != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  commit:  %s\n", info.Commit)
			}
			if info.Date != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  built:   %s\n", info.Date)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  go:      %s\n", info.GoVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "  os/arch: %s\n", info.Platform)
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}
