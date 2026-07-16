package cli

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

const (
	updateRepoOwner  = "nkenji09"
	updateRepoName   = "scholia"
	updateAPILatest  = "https://api.github.com/repos/" + updateRepoOwner + "/" + updateRepoName + "/releases/latest"
	updateDownload   = "https://github.com/" + updateRepoOwner + "/" + updateRepoName + "/releases/latest/download/"
	updateGoInstall  = "go install github.com/" + updateRepoOwner + "/" + updateRepoName + "/cmd/scholia@latest"
	updateReleaseURL = "https://github.com/" + updateRepoOwner + "/" + updateRepoName + "/releases/latest"
)

// これらは実ネットワーク I/O・自己置換・実行環境検出の seam。テストでは
// 差し替えて実際の通信・ファイル置換や実行中 OS/arch に依らない分岐検証を
// 行う（$EDITOR の seam パターン (editor.go) に倣う）。
var (
	fetchLatestTag       = fetchLatestTagViaGitHub
	downloadReleaseAsset = downloadReleaseAssetViaHTTP
	replaceRunningBinary = replaceRunningBinaryAtomic
	updateGOOS           = runtime.GOOS
	updateGOARCH         = runtime.GOARCH
	osExecutable         = os.Executable
)

// newUpdateCmd は scholia update コマンド（decision: req.self-update）。
// 導入方式/OS を検出し、自己置換できる場合のみ「取得→checksum 検証→置換」を行う。
func newUpdateCmd() *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "scholia を最新版に更新する",
		Long: `scholia を最新版に更新する。

導入方式/OS によって挙動が変わる:
  - リリース版バイナリ（darwin/linux）: 最新版を取得し、checksum を検証してから
    実行中バイナリを自己置換する。
  - go install / ソースからのビルド: 自己置換せず、` + "`" + updateGoInstall + "`" + ` を案内する。
  - windows: 実行中の .exe を上書きできないため、手動更新を案内する。`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, checkOnly)
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "取得・置換せず、更新の有無だけを報告する")
	return cmd
}

func runUpdate(cmd *cobra.Command, checkOnly bool) error {
	out := cmd.OutOrStdout()

	// T-update-guide-source: source/go install 由来は自己置換しない。
	if isSourceInstall() {
		fmt.Fprintln(out, "scholia は go install（source）経由の導入のため、自己更新できません。")
		fmt.Fprintln(out, "以下のコマンドで更新してください:")
		fmt.Fprintf(out, "  %s\n", updateGoInstall)
		return nil
	}

	// T-update-guide-windows: 実行中の .exe は上書きできない。
	if updateGOOS == "windows" {
		fmt.Fprintln(out, "Windows では実行中の scholia.exe を自己置換できません。")
		fmt.Fprintln(out, "以下のいずれかで更新してください:")
		fmt.Fprintf(out, "  - %s から手動ダウンロード\n", updateReleaseURL)
		fmt.Fprintln(out, "  - お使いのパッケージマネージャで更新")
		return nil
	}

	current := resolveVersionInfo().Version
	latest, err := fetchLatestTag()
	if err != nil {
		return err
	}
	upToDate := normalizeTag(current) == normalizeTag(latest)

	// T-update-check: --check は取得・置換せず可否だけ報告する。
	if checkOnly {
		if upToDate {
			fmt.Fprintf(out, "現在版 %s は最新です。\n", current)
		} else {
			fmt.Fprintf(out, "更新があります: %s → %s\n", current, latest)
		}
		return nil
	}

	// T-update-already-latest
	if upToDate {
		fmt.Fprintf(out, "既に最新版です（%s）。\n", current)
		return nil
	}

	// T-update-self-replace: 取得→checksum 検証→自己置換→報告。
	archive := archiveFileName(updateGOOS, updateGOARCH)
	archiveData, err := downloadReleaseAsset(archive)
	if err != nil {
		return err
	}
	checksums, err := downloadReleaseAsset("checksums.txt")
	if err != nil {
		return err
	}
	if err := verifyArchiveChecksum(archive, archiveData, checksums); err != nil {
		return fmt.Errorf("checksum 検証に失敗しました（中断・元のバイナリは変更していません）: %w", err)
	}
	newBinary, err := extractBinaryFromTarGz(archiveData)
	if err != nil {
		return fmt.Errorf("アーカイブの展開に失敗しました（中断・元のバイナリは変更していません）: %w", err)
	}
	if err := replaceRunningBinary(newBinary); err != nil {
		return fmt.Errorf("自己置換に失敗しました（元のバイナリは変更していません）: %w", err)
	}
	fmt.Fprintf(out, "scholia を %s から %s に更新しました。\n", current, latest)
	return nil
}

// isSourceInstall は導入方式が source/go install かどうかを判定する
// （cond.install-source-goinstall）。goreleaser のリリースビルドは常に
// version を実タグへ ldflags 注入するため、注入されていない（"dev" のまま）
// ことがそれ自体で source/go install の十分な兆候になる。加えて
// runtime/debug.ReadBuildInfo がモジュール版（go install pkg@vX 由来）を
// 示す場合も同様に扱う。
func isSourceInstall() bool {
	if version == "dev" {
		return true
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			return true
		}
	}
	return false
}

// normalizeTag は "v1.2.3" と goreleaser の {{.Version}}（"v" 無し）表記の
// 差異を吸収して比較できるようにする。
func normalizeTag(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// archiveFileName は .goreleaser.yaml の archives.name_template
// （"{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"）と厳密一致させる。
func archiveFileName(goos, goarch string) string {
	return fmt.Sprintf("scholia_%s_%s.tar.gz", goos, goarch)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func fetchLatestTagViaGitHub() (string, error) {
	resp, err := http.Get(updateAPILatest)
	if err != nil {
		return "", fmt.Errorf("最新版の確認に失敗しました: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API がエラーを返しました: %s", resp.Status)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("GitHub API の応答を解釈できません: %w", err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("GitHub API の応答に tag_name がありません")
	}
	return rel.TagName, nil
}

func downloadReleaseAssetViaHTTP(name string) ([]byte, error) {
	url := updateDownload + name
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("%s の取得に失敗しました: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s の取得に失敗しました: %s", name, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// verifyArchiveChecksum は checksums.txt（sha256sum 形式: "<hex>  <filename>"）
// から対象アーカイブの期待値を引き、実測 sha256 と照合する。
func verifyArchiveChecksum(filename string, archive, checksums []byte) error {
	want, err := lookupChecksum(checksums, filename)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 が一致しません（got %s, want %s）", got, want)
	}
	return nil
}

func lookupChecksum(checksums []byte, filename string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(checksums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		if fields[1] == filename {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("checksums.txt を読み込めません: %w", err)
	}
	return "", fmt.Errorf("checksums.txt に %s のエントリがありません", filename)
}

// extractBinaryFromTarGz は tar.gz アーカイブ内の scholia 実行ファイルを取り出す。
func extractBinaryFromTarGz(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip の展開に失敗しました: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar の展開に失敗しました: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) == "scholia" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("アーカイブ内に scholia バイナリが見つかりません")
}

// replaceRunningBinaryAtomic は検証済みの新バイナリを実行中バイナリと同じ
// ディレクトリに書き出し、os.Rename で atomic に置換する（同一ファイルシステム内の
// rename は成功時に必ず完了した状態になり、unix では稼働中の実行ファイルでも置換できる）。
// 途中で失敗した場合は一時ファイルを片付けるのみで、元のバイナリには触れない。
func replaceRunningBinaryAtomic(newBinary []byte) error {
	exe, err := osExecutable()
	if err != nil {
		return fmt.Errorf("実行ファイルのパスを取得できません: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".scholia-update-*")
	if err != nil {
		return fmt.Errorf("一時ファイルを作成できません: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		return fmt.Errorf("新しいバイナリの書き込みに失敗しました: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("新しいバイナリの書き込みに失敗しました: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("実行権限の付与に失敗しました: %w", err)
	}
	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("置換に失敗しました: %w", err)
	}
	return nil
}
