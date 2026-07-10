// Package viewer serves the embedded SPA and its JSON API over local HTTP
// (pmem view / export --html). It is a thin layer over internal/{store,
// index,lint,diff,render}: derived-view logic (effective-tag filtering,
// facet-tree construction, rules selection) lives in internal/index
// (FilterTransitions/BuildFacetNodes/UntaggedTransitions/
// SelectRulesDecisions) and is shared with internal/cli — do not
// reimplement it here; call the internal/index functions instead (§7/§9:
// CLI and viewer must call the same core).
package viewer
