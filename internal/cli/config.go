package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/model"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "config（tagKinds/facetKinds/traceabilityKinds/viewer.port/roots）を操作する",
	}
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	return cmd
}

// configKeys are the keys `config get`/`config set` accept (§6). pmemVersion,
// kinds, idPrefix are excluded: version is not user-editable, kinds go
// through `pmem kind set`, and idPrefix is a naming convention baked in at
// init rather than a runtime setting.
const (
	configKeyTagKinds          = "tagKinds"
	configKeyFacetKinds        = "facetKinds"
	configKeyTraceabilityKinds = "traceabilityKinds"
	configKeyViewerPort        = "viewer.port"
	configKeyRoots             = "roots"
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
	default:
		return nil, fmt.Errorf("未知の config キーです: %q", key)
	}
}

func formatConfigValue(v any) string {
	switch val := v.(type) {
	case []string:
		return strings.Join(val, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}
