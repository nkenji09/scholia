package cli

import (
	"strings"
	"testing"
)

func TestSpec_TextOutputResolvesLabelsAndDecisions(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	out, err := run(t, dir, "spec", "subject.auth")
	if err != nil {
		t.Fatalf("spec: %v\n%s", err, out)
	}
	for _, want := range []string{"認証", "T-happy", "WHEN ログイン送信", "GIVEN 資格情報が正当", "THEN トークン発行"} {
		if !strings.Contains(out, want) {
			t.Fatalf("spec output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "T-untagged") {
		t.Fatalf("spec subject.auth should not include T-untagged (no subject.auth tag):\n%s", out)
	}
}

func TestSpec_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	out, err := run(t, dir, "spec", "subject.auth", "--json")
	if err != nil {
		t.Fatalf("spec --json: %v\n%s", err, out)
	}
	for _, want := range []string{`"tag"`, `"entries"`, `"T-happy"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("spec --json output missing %q:\n%s", want, out)
		}
	}
}

func TestSpec_UnknownTagIsError(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	if _, err := run(t, dir, "spec", "does.not.exist"); err == nil {
		t.Fatalf("expected error for unknown subject tag")
	}
}
