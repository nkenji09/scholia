package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// withInjected は ldflags 注入をシミュレートするため version/commit/date を
// 一時的に差し替え、テスト後に復元する。
func withInjected(t *testing.T, v, c, d string) {
	t.Helper()
	ov, oc, od := version, commit, date
	version, commit, date = v, c, d
	t.Cleanup(func() { version, commit, date = ov, oc, od })
}

// 注入版が最優先されること（T-version-report-injected）。
func TestResolveVersionInfo_InjectedWins(t *testing.T) {
	withInjected(t, "v9.9.9", "abc1234", "2026-07-14T00:00:00Z")

	info := resolveVersionInfo()

	if info.Version != "v9.9.9" {
		t.Fatalf("Version = %q, want v9.9.9", info.Version)
	}
	if info.Commit != "abc1234" {
		t.Fatalf("Commit = %q, want abc1234", info.Commit)
	}
	if info.Date != "2026-07-14T00:00:00Z" {
		t.Fatalf("Date = %q, want the injected date", info.Date)
	}
	// GoVersion / Platform は常に埋まる。
	if info.GoVersion == "" || info.Platform == "" {
		t.Fatalf("GoVersion/Platform must always be set: %+v", info)
	}
	if !strings.Contains(info.Platform, "/") {
		t.Fatalf("Platform should be os/arch, got %q", info.Platform)
	}
}

// 未注入（version=dev）でも版はクラッシュせず・空にならないこと
// （T-version-report-buildinfo: ReadBuildInfo fallback、無ければ dev）。
func TestResolveVersionInfo_FallbackNeverEmpty(t *testing.T) {
	withInjected(t, "dev", "", "")

	info := resolveVersionInfo()

	if info.Version == "" {
		t.Fatalf("Version must never be empty; want build-info version or \"dev\"")
	}
	if info.GoVersion == "" || info.Platform == "" {
		t.Fatalf("GoVersion/Platform must always be set: %+v", info)
	}
}

// scholia version コマンドが版を出力すること（既存コマンドに影響しない additive な追加）。
func TestVersionCmd_Human(t *testing.T) {
	withInjected(t, "v1.2.3", "", "")

	out, err := run(t, t.TempDir(), "version")
	if err != nil {
		t.Fatalf("version failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "scholia v1.2.3") {
		t.Fatalf("version output missing injected version:\n%s", out)
	}
}

// --json が machine-readable な形で版を出すこと。
func TestVersionCmd_JSON(t *testing.T) {
	withInjected(t, "v4.5.6", "deadbeef", "2026-07-14T12:00:00Z")

	out, err := run(t, t.TempDir(), "version", "--json")
	if err != nil {
		t.Fatalf("version --json failed: %v\noutput:\n%s", err, out)
	}

	var got versionInfo
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("version --json is not valid JSON: %v\noutput:\n%s", err, out)
	}
	if got.Version != "v4.5.6" {
		t.Fatalf("json Version = %q, want v4.5.6", got.Version)
	}
	if got.Commit != "deadbeef" {
		t.Fatalf("json Commit = %q, want deadbeef", got.Commit)
	}
}
