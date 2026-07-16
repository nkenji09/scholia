package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "config（tagKinds/facetKinds/traceabilityKinds/tagKindLabels/viewer.port/roots）を操作する",
	}
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	return cmd
}

// configKeys are the keys `config get`/`config set` accept (§6). schemaVersion,
// kinds, idPrefix are excluded: version is not user-editable, kinds go
// through `scholia kind set`, and idPrefix is a naming convention baked in at
// init rather than a runtime setting. tagKindLabels (2026-07-11 tweaks3 §2)
// is additive to tagKinds, not a replacement for it — tagKinds alone still
// decides which kinds are valid/declared.
const (
	configKeyTagKinds          = "tagKinds"
	configKeyFacetKinds        = "facetKinds"
	configKeyTraceabilityKinds = "traceabilityKinds"
	configKeyViewerPort        = "viewer.port"
	configKeyRoots             = "roots"
	configKeyTagKindLabels     = "tagKindLabels"
)

func configKeyValue(cfg model.Config, key string) (any, error) {
	switch key {
	case configKeyTagKinds:
		return cfg.TagKinds, nil
	case configKeyFacetKinds:
		return cfg.FacetKinds, nil
	case configKeyTraceabilityKinds:
		return cfg.TraceabilityKinds, nil
	case configKeyViewerPort:
		return cfg.Viewer.Port, nil
	case configKeyRoots:
		return cfg.Roots, nil
	case configKeyTagKindLabels:
		return cfg.TagKindLabels, nil
	default:
		return nil, fmt.Errorf("未知の config キーです: %q", key)
	}
}

func formatConfigValue(v any) string {
	switch val := v.(type) {
	case []string:
		return strings.Join(val, ", ")
	case map[string]string:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, k := range keys {
			pairs = append(pairs, k+"="+val[k])
		}
		return strings.Join(pairs, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}
