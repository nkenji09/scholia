package cli

import (
	"strings"
	"testing"
)

func TestCLI_KindSetUpdatesDeclarationAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	mustRun(t, dir, "kind", "set", "action", "user,api,extra")
	out := mustRun(t, dir, "kind", "get", "action")
	for _, want := range []string{"user", "api", "extra"} {
		if !strings.Contains(out, want) {
			t.Fatalf("kind get action missing %q, got:\n%s", want, out)
		}
	}

	listOut := mustRun(t, dir, "kind", "list")
	if !strings.Contains(listOut, "user, api, extra") {
		t.Fatalf("kind list did not reflect updated action kinds:\n%s", listOut)
	}
}

func TestCLI_KindSetRejectsInvalidCategory(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	if _, err := run(t, dir, "kind", "set", "bogus", "a,b"); err == nil {
		t.Fatalf("expected error for invalid category")
	}
}

func TestCLI_KindSetRejectsRemovingInUseKind(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.a", "--label", "a", "--kind", "user")

	if _, err := run(t, dir, "kind", "set", "action", "api,lifecycle,system,cron,webhook"); err == nil {
		t.Fatalf("expected error when removing an in-use kind")
	}

	// 未使用の kind の追加/削除は許可される。
	mustRun(t, dir, "kind", "set", "action", "user,api,lifecycle,system,cron,webhook,extra")
}

func TestCLI_ConfigGetShowsWholeConfigOrOneKey(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	whole := mustRun(t, dir, "config", "get")
	if !strings.Contains(whole, "pmemVersion") {
		t.Fatalf("expected config get with no key to dump the whole config:\n%s", whole)
	}

	tagKinds := mustRun(t, dir, "config", "get", "tagKinds")
	if !strings.Contains(tagKinds, "requirement") {
		t.Fatalf("expected config get tagKinds to show declared tag kinds:\n%s", tagKinds)
	}

	if _, err := run(t, dir, "config", "get", "bogusKey"); err == nil {
		t.Fatalf("expected error for unknown config key")
	}
}

func TestCLI_ConfigSetUpdatesEachSupportedKey(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	mustRun(t, dir, "config", "set", "viewer.port", "5001")
	if out := mustRun(t, dir, "config", "get", "viewer.port"); !strings.Contains(out, "5001") {
		t.Fatalf("expected viewer.port to be updated, got:\n%s", out)
	}

	mustRun(t, dir, "config", "set", "roots", "extra-root")
	if out := mustRun(t, dir, "config", "get", "roots"); !strings.Contains(out, "extra-root") {
		t.Fatalf("expected roots to be updated, got:\n%s", out)
	}

	mustRun(t, dir, "config", "set", "facetKinds", "subject,requirement")
	if out := mustRun(t, dir, "config", "get", "facetKinds"); !strings.Contains(out, "subject") {
		t.Fatalf("expected facetKinds to be updated, got:\n%s", out)
	}

	mustRun(t, dir, "config", "set", "traceabilityKinds", "requirement")
	if out := mustRun(t, dir, "config", "get", "traceabilityKinds"); !strings.Contains(out, "requirement") {
		t.Fatalf("expected traceabilityKinds to be updated, got:\n%s", out)
	}
}

func TestCLI_ConfigSetRejectsUnknownKeyAndBadPort(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	if _, err := run(t, dir, "config", "set", "bogusKey", "x"); err == nil {
		t.Fatalf("expected error for unknown config key")
	}
	if _, err := run(t, dir, "config", "set", "viewer.port", "not-a-number"); err == nil {
		t.Fatalf("expected error for non-numeric viewer.port")
	}
}

func TestCLI_ConfigSetRejectsRemovingInUseTagKind(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "req.a", "--name", "a", "--kind", "requirement")

	if _, err := run(t, dir, "config", "set", "tagKinds", "concern,subject"); err == nil {
		t.Fatalf("expected error when removing an in-use tagKind")
	}

	// 未使用の tagKind の削除は許可される。
	mustRun(t, dir, "config", "set", "tagKinds", "requirement,concern,subject,extra")
}
