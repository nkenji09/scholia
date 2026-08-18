package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// クリーンルームレビュー FAIL-4: internal/activity の純関数は歯止め1・2・5を
// 正しく持つが、それを画面（internal/cli/activity.go の writeActivityText・
// toActivityJSON・parseActivityBoundary）へ渡す配線には専用の検査が無かった。
// レビュアの変異3種（テキスト面/JSON面で Shallow を握り潰す・裸の --since を
// 設定タイムゾーンではなく UTC で解釈する）はすべて緑のまま通っていた。
// ここでは CLI の面を実際に起こし、入力→出力（テキスト・JSON 両方）を
// 値として検査する（CLAUDE.md「配線ガードの書き方」1）。

// gitCommitAllAtT は gitCommitAllT の日時指定版。GIT_AUTHOR_DATE/
// GIT_COMMITTER_DATE で commit 日時を固定する——テストが「走らせた時刻」に
// 依存しないようにするため。
func gitCommitAllAtT(t *testing.T, dir, msg string, at time.Time) {
	t.Helper()
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	ts := at.Format(time.RFC3339)
	commitCmd := exec.Command("git", "commit", "-q", "--allow-empty", "-m", msg)
	commitCmd.Dir = dir
	commitCmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+ts, "GIT_COMMITTER_DATE="+ts)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

// TestCLI_Activity_Shallow_TextAndJSONWithholdNumbers は M25・M27（レビュー
// 表）を CLI の面で固定する。internal/activity.Compute は Shallow=true のとき
// 数を計算しないが、それを守って**画面に出さない**責任は
// writeActivityText／toActivityJSON にある——ここを崩す変異はどちらも緑の
// まま通っていた。
func TestCLI_Activity_Shallow_TextAndJSONWithholdNumbers(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	gitCommitAllAtT(t, dir, "seed", base)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitAllAtT(t, dir, "impl", base.Add(24*time.Hour))

	// 対照: full clone では通常どおり数が出る。
	fullOut, err := run(t, dir, "activity", "--since", "2026-07-01", "--until", "2026-08-18")
	if err != nil {
		t.Fatalf("activity（full）: %v\n%s", err, fullOut)
	}
	if !strings.Contains(fullOut, "実装が動いた日") {
		t.Fatalf("full clone は通常の出力を出すはず:\n%s", fullOut)
	}

	shallowDir := t.TempDir()
	cloneCmd := exec.Command("git", "clone", "--depth", "1", "--no-local", dir, shallowDir)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 1: %v\n%s", err, out)
	}

	textOut, err := run(t, shallowDir, "activity")
	if err != nil {
		t.Fatalf("activity（shallow・text）: %v\n%s", err, textOut)
	}
	// ⚠️ 「実装が動いた日」という語自体は開示文（"実装 commit・実装が動いた日・
	// 最後の実装は出せません"）にも出るので、部分一致では誤検出する。数の行
	// （"  実装が動いた日  <N> / <N> 日"）が実際にあるかを行単位で見る。
	for _, line := range strings.Split(textOut, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "実装が動いた日") {
			t.Errorf("浅い clone のテキスト面が実装活動の数の行を出している（歯止め1がCLI面に届いていない）: %q\n全文:\n%s", line, textOut)
		}
	}
	if !strings.Contains(textOut, "浅い") {
		t.Errorf("浅い clone の開示が出ていない:\n%s", textOut)
	}

	jsonOut, err := run(t, shallowDir, "activity", "--json")
	if err != nil {
		t.Fatalf("activity（shallow・json）: %v\n%s", err, jsonOut)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &decoded); err != nil {
		t.Fatalf("JSON を読めない: %v\n%s", err, jsonOut)
	}
	if shallow, _ := decoded["shallow"].(bool); !shallow {
		t.Errorf("shallow: true のはず: %v", decoded)
	}
	for _, key := range []string{"activeDays", "implCommits", "recordOnlyCommits", "lastActivity", "lastActivityDaysAgo"} {
		if _, ok := decoded[key]; ok {
			t.Errorf("浅い clone の JSON 面が %q を出している（歯止め1がCLI面に届いていない）: %v", key, decoded)
		}
	}
}

// TestCLI_Activity_BareDateSince_UsesConfiguredTimezone は M21（レビュー表）
// を CLI の面で固定する。裸の日付（YYYY-MM-DD）の --since/--until は、
// config.timezone（無ければ UTC）の 0 時として解釈されるべきで、常に UTC
// として解釈してはいけない——さもないと Asia/Tokyo のようなプロジェクトで
// 窓が9時間ずれ、初日 00:00〜09:00 の commit が黙って落ちる（歯止め2 が
// 防ごうとしたズレそのもの）。
func TestCLI_Activity_BareDateSince_UsesConfiguredTimezone(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "config", "set", "timezone", "Asia/Tokyo"); err != nil {
		t.Fatalf("config set timezone: %v\n%s", err, out)
	}
	gitCommitAllAtT(t, dir, "seed", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))

	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	// 2026-08-01T03:00:00+09:00 = 2026-07-31T18:00:00Z。
	// --since 2026-08-01 を「設定タイムゾーン(JST)の0時」
	// （2026-08-01T00:00:00+09:00 = 2026-07-31T15:00:00Z）として正しく解釈すれば、
	// この commit（18:00Z）はそれより後なので窓に入る。
	// 誤って「UTC の0時」（2026-08-01T00:00:00Z）と解釈すると、この commit
	// （JST 03:00・その9時間前）は窓の外に落ちる。
	commitTime := time.Date(2026, 8, 1, 3, 0, 0, 0, jst)
	if err := os.WriteFile(filepath.Join(dir, "impl.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommitAllAtT(t, dir, "impl at JST 03:00 Aug1", commitTime)

	out, err := run(t, dir, "activity", "--since", "2026-08-01", "--until", "2026-08-02", "--json")
	if err != nil {
		t.Fatalf("activity: %v\n%s", err, out)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("JSON を読めない: %v\n%s", err, out)
	}
	implCommits, _ := decoded["implCommits"].(float64)
	if implCommits != 1 {
		t.Errorf("implCommits = %v, want 1（裸の --since が設定タイムゾーン Asia/Tokyo の0時として解釈されていない）:\n%s", implCommits, out)
	}
}
