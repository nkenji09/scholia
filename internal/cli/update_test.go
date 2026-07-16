package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withUpdateSeams は update.go の seam 一式を差し替え、テスト後に復元する。
func withUpdateSeams(t *testing.T) {
	t.Helper()
	origFetch, origDownload, origReplace := fetchLatestTag, downloadReleaseAsset, replaceRunningBinary
	origGOOS, origGOARCH := updateGOOS, updateGOARCH
	t.Cleanup(func() {
		fetchLatestTag, downloadReleaseAsset, replaceRunningBinary = origFetch, origDownload, origReplace
		updateGOOS, updateGOARCH = origGOOS, origGOARCH
	})
}

func makeTarGzWithBinary(t *testing.T, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "scholia", Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// T-update-guide-source: version=dev（source/go install 由来）は自己置換せず案内する。
func TestUpdate_SourceInstallGuidesGoInstallAndDoesNotReplace(t *testing.T) {
	withInjected(t, "dev", "", "")
	withUpdateSeams(t)

	called := false
	fetchLatestTag = func() (string, error) { called = true; return "v9.9.9", nil }
	replaceRunningBinary = func([]byte) error { t.Fatalf("replaceRunningBinary should not be called"); return nil }

	out, err := run(t, t.TempDir(), "update")
	if err != nil {
		t.Fatalf("update failed: %v\noutput:\n%s", err, out)
	}
	if called {
		t.Fatalf("fetchLatestTag should not be called for source install")
	}
	if !strings.Contains(out, "go install github.com/nkenji09/scholia/cmd/scholia@latest") {
		t.Fatalf("output missing go install guidance:\n%s", out)
	}
}

// T-update-guide-windows: windows は自己置換せず手動更新を案内する。
func TestUpdate_WindowsGuidesManualAndDoesNotReplace(t *testing.T) {
	withInjected(t, "v1.0.0", "", "")
	withUpdateSeams(t)
	updateGOOS = "windows"

	fetchLatestTag = func() (string, error) { t.Fatalf("fetchLatestTag should not be called on windows"); return "", nil }
	replaceRunningBinary = func([]byte) error { t.Fatalf("replaceRunningBinary should not be called on windows"); return nil }

	out, err := run(t, t.TempDir(), "update")
	if err != nil {
		t.Fatalf("update failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "手動") && !strings.Contains(out, "releases") {
		t.Fatalf("output missing manual-update guidance:\n%s", out)
	}
}

// T-update-already-latest: 現在版=最新版なら取得・置換せず「既に最新」を報告する。
func TestUpdate_AlreadyLatestDoesNotDownloadOrReplace(t *testing.T) {
	withInjected(t, "1.2.3", "", "") // goreleaser の {{.Version}} は v prefix 無し
	withUpdateSeams(t)
	updateGOOS, updateGOARCH = "linux", "amd64"

	fetchLatestTag = func() (string, error) { return "v1.2.3", nil }
	downloadReleaseAsset = func(name string) ([]byte, error) {
		t.Fatalf("downloadReleaseAsset should not be called when already latest")
		return nil, nil
	}
	replaceRunningBinary = func([]byte) error {
		t.Fatalf("replaceRunningBinary should not be called when already latest")
		return nil
	}

	out, err := run(t, t.TempDir(), "update")
	if err != nil {
		t.Fatalf("update failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "既に最新版") {
		t.Fatalf("output missing already-latest report:\n%s", out)
	}
}

// T-update-check: --check は取得・置換せず可否だけを報告する。
func TestUpdate_CheckFlagReportsWithoutDownloadOrReplace(t *testing.T) {
	withInjected(t, "1.2.3", "", "")
	withUpdateSeams(t)
	updateGOOS, updateGOARCH = "linux", "amd64"

	fetchLatestTag = func() (string, error) { return "v1.3.0", nil }
	downloadReleaseAsset = func(name string) ([]byte, error) {
		t.Fatalf("downloadReleaseAsset should not be called with --check")
		return nil, nil
	}
	replaceRunningBinary = func([]byte) error {
		t.Fatalf("replaceRunningBinary should not be called with --check")
		return nil
	}

	out, err := run(t, t.TempDir(), "update", "--check")
	if err != nil {
		t.Fatalf("update --check failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "1.2.3") || !strings.Contains(out, "v1.3.0") {
		t.Fatalf("output missing version comparison:\n%s", out)
	}
}

// T-update-check: --check でも既に最新なら「最新」である旨を報告する。
func TestUpdate_CheckFlagReportsUpToDate(t *testing.T) {
	withInjected(t, "1.2.3", "", "")
	withUpdateSeams(t)
	updateGOOS, updateGOARCH = "linux", "amd64"

	fetchLatestTag = func() (string, error) { return "v1.2.3", nil }

	out, err := run(t, t.TempDir(), "update", "--check")
	if err != nil {
		t.Fatalf("update --check failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "最新") {
		t.Fatalf("output missing up-to-date report:\n%s", out)
	}
}

// T-update-self-replace: 更新あり→DL→checksum検証→自己置換→新版報告。
func TestUpdate_SelfReplaceDownloadsVerifiesAndReplaces(t *testing.T) {
	withInjected(t, "1.2.3", "", "")
	withUpdateSeams(t)
	updateGOOS, updateGOARCH = "linux", "amd64"

	newBinaryContent := []byte("fake-new-scholia-binary")
	archive := makeTarGzWithBinary(t, newBinaryContent)
	archiveName := archiveFileName("linux", "amd64")
	checksums := []byte(fmt.Sprintf("%s  %s\n", sha256Hex(archive), archiveName))

	fetchLatestTag = func() (string, error) { return "v1.3.0", nil }
	downloadReleaseAsset = func(name string) ([]byte, error) {
		switch name {
		case archiveName:
			return archive, nil
		case "checksums.txt":
			return checksums, nil
		default:
			t.Fatalf("unexpected asset requested: %s", name)
			return nil, nil
		}
	}

	var replaced []byte
	replaceRunningBinary = func(b []byte) error {
		replaced = b
		return nil
	}

	out, err := run(t, t.TempDir(), "update")
	if err != nil {
		t.Fatalf("update failed: %v\noutput:\n%s", err, out)
	}
	if !bytes.Equal(replaced, newBinaryContent) {
		t.Fatalf("replaceRunningBinary got %q, want %q", replaced, newBinaryContent)
	}
	if !strings.Contains(out, "1.2.3") || !strings.Contains(out, "v1.3.0") {
		t.Fatalf("output missing before/after version:\n%s", out)
	}
}

// checksum 不一致→中断・自己置換しない（元バイナリ非破壊）。
func TestUpdate_ChecksumMismatchAbortsWithoutReplace(t *testing.T) {
	withInjected(t, "1.2.3", "", "")
	withUpdateSeams(t)
	updateGOOS, updateGOARCH = "linux", "amd64"

	archive := makeTarGzWithBinary(t, []byte("fake-new-scholia-binary"))
	archiveName := archiveFileName("linux", "amd64")
	// わざと壊れたハッシュを checksums.txt に書く。
	checksums := []byte(fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), archiveName))

	fetchLatestTag = func() (string, error) { return "v1.3.0", nil }
	downloadReleaseAsset = func(name string) ([]byte, error) {
		switch name {
		case archiveName:
			return archive, nil
		case "checksums.txt":
			return checksums, nil
		default:
			t.Fatalf("unexpected asset requested: %s", name)
			return nil, nil
		}
	}
	replaceRunningBinary = func([]byte) error {
		t.Fatalf("replaceRunningBinary should not be called on checksum mismatch")
		return nil
	}

	out, err := run(t, t.TempDir(), "update")
	if err == nil {
		t.Fatalf("expected error on checksum mismatch, output:\n%s", out)
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error should mention checksum, got: %v", err)
	}
}

// isSourceInstall: version が実タグならソース導入と判定しない。
func TestIsSourceInstall(t *testing.T) {
	withInjected(t, "dev", "", "")
	if !isSourceInstall() {
		t.Fatalf("version=dev should be detected as source install")
	}

	withInjected(t, "1.2.3", "", "")
	if isSourceInstall() {
		t.Fatalf("version=1.2.3 (release) should not be detected as source install")
	}
}

// replaceRunningBinaryAtomic: 実ファイルを atomic rename で置換できること
// （seam の実装自体の単体テスト。CLI 経路は上の seam 差し替えテストでカバー）。
func TestReplaceRunningBinaryAtomic(t *testing.T) {
	dir := t.TempDir()
	exePath := filepath.Join(dir, "scholia-fake-exe")
	if err := os.WriteFile(exePath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write fake exe: %v", err)
	}

	origExecutable := osExecutable
	osExecutable = func() (string, error) { return exePath, nil }
	t.Cleanup(func() { osExecutable = origExecutable })

	if err := replaceRunningBinaryAtomic([]byte("new-binary")); err != nil {
		t.Fatalf("replaceRunningBinaryAtomic failed: %v", err)
	}

	data, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("read replaced exe: %v", err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("exe content = %q, want %q", data, "new-binary")
	}
	info, err := os.Stat(exePath)
	if err != nil {
		t.Fatalf("stat replaced exe: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("replaced exe should be executable, mode=%v", info.Mode())
	}
}
