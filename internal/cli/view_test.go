package cli

import (
	"errors"
	"net"
	"strings"
	"testing"
)

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

// TestClassifyListenErr_PortInUse drives a real EADDRINUSE by binding the
// same port twice, then checks the resulting error names the cause (port
// in use) and the next step (--port / stop the other process), per
// §T-view-start-port-in-use.
func TestClassifyListenErr_PortInUse(t *testing.T) {
	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind first listener: %v", err)
	}
	defer first.Close()

	_, bindErr := net.Listen("tcp", first.Addr().String())
	if bindErr == nil {
		t.Fatal("expected second Listen on the same address to fail")
	}

	port := first.Addr().(*net.TCPAddr).Port
	got := classifyListenErr(bindErr, port)
	if got == nil {
		t.Fatal("classifyListenErr returned nil for a real bind failure")
	}
	if !strings.Contains(got.Error(), "使用中です") || !strings.Contains(got.Error(), "--port") {
		t.Errorf("classifyListenErr(EADDRINUSE) = %q, want a message naming the cause (使用中) and the next step (--port)", got.Error())
	}
}

// TestClassifyListenErr_OtherErr checks non-EADDRINUSE bind failures (e.g.
// permission denied) still get a clear, non-nil error instead of being
// mistaken for port contention.
func TestClassifyListenErr_OtherErr(t *testing.T) {
	got := classifyListenErr(errors.New("permission denied"), 80)
	if got == nil {
		t.Fatal("classifyListenErr returned nil for a generic bind failure")
	}
	if strings.Contains(got.Error(), "使用中です") {
		t.Errorf("classifyListenErr(generic err) = %q, should not claim port is in use", got.Error())
	}
	if !strings.Contains(got.Error(), "bind に失敗しました") {
		t.Errorf("classifyListenErr(generic err) = %q, want it to say bind failed", got.Error())
	}
}
