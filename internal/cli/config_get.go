package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/store"
)

func newConfigGetCmd() *cobra.Command {
	var asJSON, local bool
	cmd := &cobra.Command{
		Use:   "get [<key>]",
		Short: "config を表示する（キー省略で config 全体・--local でこの端末専用の上書きを見る）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}

			if local {
				return getLocalConfigKey(cmd, s, args, asJSON)
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			if len(args) == 0 {
				return emitJSON(cmd, cfg)
			}

			key := args[0]
			val, err := configKeyValue(cfg, key)
			if err != nil {
				return err
			}
			if asJSON {
				return emitJSON(cmd, val)
			}
			fmt.Fprintln(cmd.OutOrStdout(), formatConfigValue(val))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（キー指定時。キー省略時は常に JSON）")
	cmd.Flags().BoolVar(&local, "local", false, "この端末専用の上書き（.scholia/config.local.json）を見る（対象: viewer.port/timezone のみ・未設定は null）")
	return cmd
}

// getLocalConfigKey mirrors newConfigGetCmd's key-scoped output for the
// --local (config.local.json) side — same allowlist setLocalConfigKey
// writes (viewer.port/timezone).
func getLocalConfigKey(cmd *cobra.Command, s *store.Store, args []string, asJSON bool) error {
	o, err := s.LoadLocalConfigOverride()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return emitJSON(cmd, o)
	}

	var val any
	display := "(未設定 — config.json の値を使用)"
	switch args[0] {
	case configKeyViewerPort:
		val = o.ViewerPort
		if o.ViewerPort != 0 {
			display = strconv.Itoa(o.ViewerPort)
		}
	case configKeyTimezone:
		val = o.Timezone
		if o.Timezone != "" {
			display = o.Timezone
		}
	default:
		return fmt.Errorf("config get --local が対象にできるのは %s / %s のみです: %q", configKeyViewerPort, configKeyTimezone, args[0])
	}
	if asJSON {
		return emitJSON(cmd, val)
	}
	fmt.Fprintln(cmd.OutOrStdout(), display)
	return nil
}
