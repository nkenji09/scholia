package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func newConfigSetCmd() *cobra.Command {
	var asJSON, local bool
	cmd := &cobra.Command{
		Use: "set <key> <value>",
		Short: "config の値を更新する（tagKinds/facetKinds/traceabilityKinds/tagKindLabels/viewer.port/roots/timezone）。" +
			"tagKindLabels の value は kind=label のカンマ区切り（例: requirement=要件,concern=関心事）。" +
			"--local で viewer.port/timezone だけをこの端末専用の上書き（.scholia/config.local.json・gitignore 対象）に書く",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			s, err := openStore()
			if err != nil {
				return err
			}

			if local {
				return setLocalConfigKey(cmd, s, key, value, asJSON)
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			switch key {
			case configKeyTagKinds:
				// #45 D9: CSV を id 集合として解釈する。既存 object 宣言は id が残る
				// 限り Label/Description/Behaviors を保持し、新規 id は string 追加、
				// 除去は使用中検査後（behaviors 等のメタ編集は本コマンド外）。
				ids := splitNonEmpty(value)
				removed := diffStrings(cfg.TagKindIDs(), ids)
				if len(removed) > 0 {
					snap, err := s.LoadAll()
					if err != nil {
						return err
					}
					if inUse := tagsUsingKinds(snap.Tags, removed); len(inUse) > 0 {
						return fmt.Errorf(
							"kind %s は %d 件の tag で使用中のため tagKinds から外せません: %s",
							strings.Join(removed, ","), len(inUse), strings.Join(inUse, ", "))
					}
				}
				cfg.TagKinds = mergeTagKindIDs(cfg.TagKinds, ids)
			case configKeyFacetKinds:
				cfg.FacetKinds = splitNonEmpty(value)
			case configKeyTraceabilityKinds:
				cfg.TraceabilityKinds = splitNonEmpty(value)
			case configKeyViewerPort:
				port, err := strconv.Atoi(value)
				if err != nil {
					return fmt.Errorf("viewer.port は数値である必要があります: %w", err)
				}
				cfg.Viewer.Port = port
			case configKeyRoots:
				cfg.Roots = splitNonEmpty(value)
			case configKeyTagKindLabels:
				labels, err := parseLabelMap(value)
				if err != nil {
					return err
				}
				cfg.TagKindLabels = labels
			case configKeyTimezone:
				if value != "" {
					if err := model.ValidateTimezone(value); err != nil {
						return err
					}
				}
				cfg.Timezone = value
			default:
				return fmt.Errorf("未知の config キーです: %q", key)
			}

			if err := s.SaveConfig(cfg); err != nil {
				return err
			}

			if asJSON {
				return emitJSON(cmd, cfg)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config.%s を更新しました: %s\n", key, value)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後の config 全体を JSON で出力する")
	cmd.Flags().BoolVar(&local, "local", false, "この端末専用の上書き（.scholia/config.local.json）に書く（対象: viewer.port/timezone のみ）")
	return cmd
}

// setLocalConfigKey handles `config set --local` — a closed allowlist
// (viewer.port/timezone: req.comfortable-viewer.config-editing amend) of
// keys with zero shared-data impact, written to config.local.json instead
// of config.json. An empty value clears that key's override (falls back to
// the project config.json value), matching the project-config path's own
// "empty clears" convention for optional string keys.
func setLocalConfigKey(cmd *cobra.Command, s *store.Store, key, value string, asJSON bool) error {
	o, err := s.LoadLocalConfigOverride()
	if err != nil {
		return err
	}

	switch key {
	case configKeyViewerPort:
		if value == "" {
			o.ViewerPort = 0
		} else {
			port, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("viewer.port は数値である必要があります: %w", err)
			}
			o.ViewerPort = port
		}
	case configKeyTimezone:
		if value != "" {
			if err := model.ValidateTimezone(value); err != nil {
				return err
			}
		}
		o.Timezone = value
	default:
		return fmt.Errorf(
			"config set --local が対象にできるのは %s / %s のみです（データの意味論に関わる設定は端末ごとの上書き対象外）: %q",
			configKeyViewerPort, configKeyTimezone, key)
	}

	if err := s.SaveLocalConfigOverride(o); err != nil {
		return err
	}

	if asJSON {
		return emitJSON(cmd, o)
	}
	if value == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "config.local.%s の上書きを解除しました\n", key)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "config.local.%s を更新しました: %s\n", key, value)
	}
	return nil
}

// parseLabelMap parses config set's tagKindLabels value format
// ("kind=label,kind=label", the same comma-separated convention
// splitNonEmpty uses for slice-valued keys, extended with "=" per entry
// since this key is a map). An empty value clears the map (matching the
// splitNonEmpty("") == nil behavior the slice-valued keys already have).
func parseLabelMap(value string) (map[string]string, error) {
	entries := splitNonEmpty(value)
	if len(entries) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		k, v, ok := strings.Cut(e, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("tagKindLabels の値は kind=label のカンマ区切りである必要があります（不正な項目: %q）", e)
		}
		out[k] = v
	}
	return out, nil
}

// mergeTagKindIDs は「新しい id 集合」を既存 KindDecl 群に反映する（#45 D9・
// config set tagKinds の id 集合解釈）。出力は ids の順序どおりで、id が既存宣言に
// あれば object メタデータ（Label/Description/Behaviors）ごと保持し、無ければ
// string 宣言（id のみ）として追加する。ids に無い既存宣言は落とす（除去）。
func mergeTagKindIDs(existing []model.KindDecl, ids []string) []model.KindDecl {
	byID := make(map[string]model.KindDecl, len(existing))
	for _, d := range existing {
		byID[d.ID] = d
	}
	out := make([]model.KindDecl, 0, len(ids))
	for _, id := range ids {
		if d, ok := byID[id]; ok {
			out = append(out, d)
		} else {
			out = append(out, model.KindDecl{ID: id})
		}
	}
	return out
}

func tagsUsingKinds(tags []model.Tag, kinds []string) []string {
	want := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	var out []string
	for _, t := range tags {
		if want[t.Kind] {
			out = append(out, t.ID)
		}
	}
	sort.Strings(out)
	return out
}
