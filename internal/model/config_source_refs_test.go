package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfig_SourceRefsOmittedWhenUnset(t *testing.T) {
	cfg := DefaultConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "sourceRefs") {
		t.Fatalf("DefaultConfig() must not emit sourceRefs (additive, backward compatible), got %s", data)
	}

	// An older config.json with no sourceRefs field must decode to nil,
	// not a zero-value struct — this is what lets callers tell "unset" from
	// "explicitly set to empty" if that distinction ever matters.
	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.SourceRefs != nil {
		t.Fatalf("expected SourceRefs to decode as nil, got %+v", decoded.SourceRefs)
	}
}

func TestConfig_SourceRefsRoundTripsWhenSet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SourceRefs = &SourceRefs{Scan: []string{"src"}, Exclude: []string{"src/generated"}}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.SourceRefs == nil {
		t.Fatalf("expected SourceRefs to round-trip, got nil")
	}
	if len(decoded.SourceRefs.Scan) != 1 || decoded.SourceRefs.Scan[0] != "src" {
		t.Fatalf("unexpected Scan: %+v", decoded.SourceRefs.Scan)
	}
	if len(decoded.SourceRefs.Exclude) != 1 || decoded.SourceRefs.Exclude[0] != "src/generated" {
		t.Fatalf("unexpected Exclude: %+v", decoded.SourceRefs.Exclude)
	}
}
