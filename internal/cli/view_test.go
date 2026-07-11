package cli

import "testing"

// TestIsLocalHost covers the boundary --host's LAN-exposure warning fires
// on (2026-07-11 handoff: opt-in --host flag). `view`'s RunE itself blocks
// on ListenAndServe until interrupted, so it isn't exercised via the
// run()/mustRun() cobra-execute harness the rest of this package uses —
// this pure-function boundary check is the light test the handoff asked
// for instead.
func TestIsLocalHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"localhost", true},
		{"::1", true},
		{"0.0.0.0", false},
		{"192.168.1.23", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isLocalHost(c.host); got != c.want {
			t.Errorf("isLocalHost(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

// TestViewCmd_HostFlagDefaultsToLoopback checks the flag is registered and
// defaults to the pre-existing behavior (127.0.0.1) without actually
// running the command (RunE starts a real listener and blocks).
func TestViewCmd_HostFlagDefaultsToLoopback(t *testing.T) {
	cmd := newViewCmd()
	f := cmd.Flags().Lookup("host")
	if f == nil {
		t.Fatal("expected a --host flag to be registered")
	}
	if f.DefValue != "127.0.0.1" {
		t.Fatalf("--host default = %q, want 127.0.0.1 (default behavior must stay unchanged)", f.DefValue)
	}
}
