package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// この検査が守るのは「読み方を差し替えても、返る中身と返るエラーが変わらないこと」
// だけである。速さは一切見ていない（実行時間は機械と負荷で揺れるので、緑を根拠に
// できない）。落ちるのは次のときで、それ以外では落ちない:
//
//   - `git cat-file --batch` の応答の読み取りが1バイトでもずれたとき
//   - blob 以外・存在しない object を、以前と違う結末（違う中身・違うエラー）にしたとき
//   - ls-tree の行を読む・読まないの振り分けが変わったとき
//
// 落ちないもの: 速度が戻ること、プロセス数が増えること（後者は
// TestBatchBlobReader_HandlesOrdinaryRecordsWithoutFallback が別に見る）。

// blobFixtureRepo は「読み方が違えば結末が変わりうる」ものを一通り置いた git repo を作る。
// 返り値の第3要素は `git ls-tree -r --name-only HEAD -- .scholia` が返す行そのまま
// （非 ASCII の引用行も含む・振り分け前）。
func blobFixtureRepo(t *testing.T) (dir string, relDir string, paths []string) {
	t.Helper()
	dir, _ = gitTestRepo(t)
	tags := filepath.Join(dir, ".scholia", "tags")

	writeFile(t, filepath.Join(tags, "plain.json"), `{"id":"plain"}`+"\n")
	// 末尾に改行が無い（読み手が1バイト足す/削ると割れる）
	writeFile(t, filepath.Join(tags, "no-trailing-newline.json"), `{"id":"nonl"}`)
	// 空ファイル: 中身は 0 バイト。unmarshal は必ず失敗する。
	writeFile(t, filepath.Join(tags, "empty.json"), "")
	// binary: NUL・不正 UTF-8 を含む。string 化で壊れないことを見る。
	writeFile(t, filepath.Join(tags, "binary.json"), "\x00\x01\x02binary\xff\xfe")
	// 中身の途中に LF がある（応答の区切りと紛れる）
	writeFile(t, filepath.Join(tags, "multiline.json"), "{\n  \"id\": \"multi\"\n}\n")
	// 大きい: bufio の内部バッファ（既定 4KB）を何度もまたぐ
	writeFile(t, filepath.Join(tags, "large.json"), fmt.Sprintf(`{"id":"large","label":%q}`, strings.Repeat("あa", 1<<20))+"\n")
	// 名前に空白（cat-file の応答行が空白で割れる形）
	writeFile(t, filepath.Join(tags, "with space.json"), `{"id":"space"}`+"\n")
	// 名前が「<何か> blob <数字>」の形（応答行の誤読を誘う）
	writeFile(t, filepath.Join(tags, "x blob 12.json"), `{"id":"blobnum"}`+"\n")
	// 非 ASCII: ls-tree が core.quotePath で引用して返す行
	writeFile(t, filepath.Join(tags, "日本.json"), `{"id":"ja"}`+"\n")
	// .json でない・カテゴリ直下でない・未知カテゴリ
	writeFile(t, filepath.Join(tags, "note.md"), "not json\n")
	writeFile(t, filepath.Join(dir, ".scholia", "config.json"), `{"idPolicy":{}}`+"\n")
	writeFile(t, filepath.Join(tags, "sub", "nested.json"), `{"id":"nested"}`+"\n")
	writeFile(t, filepath.Join(dir, ".scholia", "unknowncat", "u.json"), `{"id":"u"}`+"\n")

	// symlink: blob の中身は「リンク先の文字列」で、改行が付かない。
	if err := os.Symlink("plain.json", filepath.Join(tags, "link.json")); err != nil {
		t.Logf("symlink を作れないので symlink の検査は省く: %v", err)
	}

	commitAll(t, dir, "blob fixtures")

	out, err := runGit(dir, "ls-tree", "-r", "--name-only", "HEAD", "--", ".scholia")
	if err != nil {
		t.Fatalf("ls-tree: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	if len(paths) < 10 {
		t.Fatalf("fixture が足りない: ls-tree = %v", paths)
	}
	return dir, ".scholia", paths
}

// TestBlobReaders_AgreeByteForByte は ls-tree が返した全行について、新旧の読み手が
// 同じ中身・同じエラーを返すことをバイトで突き合わせる。引用された非 ASCII 行も
// そのまま渡す（両方とも同じように失敗するはずのもの）。
func TestBlobReaders_AgreeByteForByte(t *testing.T) {
	dir, _, paths := blobFixtureRepo(t)

	oldReader := newShowBlobReader(dir, "HEAD")
	defer oldReader.close()
	newReader := newBatchBlobReader(dir, "HEAD")
	defer newReader.close()

	for _, path := range paths {
		wantContent, wantErr := oldReader.read(path)
		gotContent, gotErr := newReader.read(path)

		if errString(wantErr) != errString(gotErr) {
			t.Errorf("%s: エラーが違う\n  git show      = %q\n  cat-file batch = %q", path, errString(wantErr), errString(gotErr))
			continue
		}
		if wantContent != gotContent {
			t.Errorf("%s: 中身が違う（%d バイト vs %d バイト）\n  git show       先頭64B = %q\n  cat-file batch 先頭64B = %q",
				path, len(wantContent), len(gotContent), head64(wantContent), head64(gotContent))
		}
	}
}

// TestBatchBlobReader_StaysInSyncAfterMissingObject は「1件が読めなかった後も、
// 続く path が別の中身とすり替わらない」ことを見る。cat-file --batch は1プロセスを
// 使い回すので、応答を1件読み損ねると以後すべてがずれる——この repo が繰り返し
// 踏んできた「静かに間違った値を返す」型そのものになる。
func TestBatchBlobReader_StaysInSyncAfterMissingObject(t *testing.T) {
	dir, _ := gitTestRepo(t)
	tags := filepath.Join(dir, ".scholia", "tags")
	writeFile(t, filepath.Join(tags, "a.json"), `{"id":"a-unique-content"}`+"\n")
	writeFile(t, filepath.Join(tags, "gone.json"), `{"id":"gone-unique-content"}`+"\n")
	writeFile(t, filepath.Join(tags, "z.json"), `{"id":"z-unique-content"}`+"\n")
	commitAll(t, dir, "seed")

	// object 本体だけを消す。tree にはエントリが残るので ls-tree はこの path を
	// 返し続け、読もうとしたときだけ失敗する。
	removeLooseObject(t, dir, "HEAD:.scholia/tags/gone.json")

	oldReader := newShowBlobReader(dir, "HEAD")
	defer oldReader.close()
	newReader := newBatchBlobReader(dir, "HEAD")
	defer newReader.close()

	order := []string{".scholia/tags/a.json", ".scholia/tags/gone.json", ".scholia/tags/z.json"}
	for _, path := range order {
		wantContent, wantErr := oldReader.read(path)
		gotContent, gotErr := newReader.read(path)
		if errString(wantErr) != errString(gotErr) {
			t.Fatalf("%s: エラーが違う\n  git show       = %q\n  cat-file batch = %q", path, errString(wantErr), errString(gotErr))
		}
		if wantContent != gotContent {
			t.Fatalf("%s: 中身が違う\n  git show       = %q\n  cat-file batch = %q", path, wantContent, gotContent)
		}
	}
	// 読めなかった1件が本当にエラーになっていること（この検査自体が空振りしていないこと）。
	if _, err := oldReader.read(".scholia/tags/gone.json"); err == nil {
		t.Fatal("fixture が効いていない: object を消したのに git show が成功した")
	}
}

// TestBatchBlobReader_HandlesOrdinaryRecordsWithoutFallback は「ふつうのレコードが
// batch で読めていること」を落ちる形で持つ。
//
// 落ちるのは、ふつうの .json レコードが1件でも `git show` へ回されたとき——つまり
// 「batch を通しているつもりで、実は以前と同じく1件1プロセスに戻っていた」場合。
// これは実行時間ではなく経路を見ているので、機械の速さで揺れない。
// 落ちないもの: 別の原因で遅くなること、プロセスが1つ増えること。
func TestBatchBlobReader_HandlesOrdinaryRecordsWithoutFallback(t *testing.T) {
	dir, _ := gitTestRepo(t)
	tags := filepath.Join(dir, ".scholia", "tags")
	var want []string
	for i := 0; i < 20; i++ {
		p := filepath.Join(tags, fmt.Sprintf("t%02d.json", i))
		writeFile(t, p, fmt.Sprintf(`{"id":"t%02d"}`+"\n", i))
		want = append(want, fmt.Sprintf(".scholia/tags/t%02d.json", i))
	}
	commitAll(t, dir, "seed")

	r := newBatchBlobReader(dir, "HEAD").(*batchBlobReader)
	defer r.close()
	for _, path := range want {
		if _, err := r.read(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if r.fallbacks != 0 {
		t.Errorf("ふつうのレコード %d 件のうち %d 件が git show へ回った（batch 経路が効いていない）", len(want), r.fallbacks)
	}
	if r.broken {
		t.Error("batch が broken になった（1プロセスで読み切れていない）")
	}
}

// ============================================================================
// 配線ガード — 実際に起きた git のプロセスを、外から数える
// ============================================================================
//
// 🔴 前の版は runGit の中で自分をインクリメントする「申告カウンタ」だった。
// それは「runGit を通る」という1つの綴りに依存していて、ループの中へ直に
// exec.Command("git", "show", ...) と書けば申告は平らなまま実プロセスだけが
// 件数に比例する。クリーンルームレビューが実際にその変異を通し、全ゲート緑・
// 出力バイト同一のまま 0.73s -> 5.56s に戻した（申告 2/2/2 に対し実プロセス
// 7/62/202）。CLAUDE.md 2 が言う穴の一段深いところに同じ穴があった。
//
// いまは PATH の先頭に「git」という名前の影武者を置き、起きたプロセスを
// 外から数える。見ているのは「git のプロセスが起きた」という事象そのもので、
// 呼び出し側の書き方に依らない。

// gitShim は PATH の先頭に置いた偽の git。実行のたびに argv をログへ1行積み、
// 本物の git へ exec する（＝振る舞いは変えず、回数だけを外から観測する）。
type gitShim struct{ logPath string }

// installGitShim は PATH の先頭へ影武者を置く。body は本物の git の絶対パスを
// 受け取り、sh スクリプトの本文を返す。
func installGitShim(t *testing.T, body func(realGit, logPath string) string) *gitShim {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("影武者は sh スクリプトなので Windows では置けない")
	}
	// ⚠️ PATH を書き換える前に本物を解決する（後だと影武者自身を掴む）。
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "invocations.log")
	shimPath := filepath.Join(dir, "git")
	if err := os.WriteFile(shimPath, []byte(body(realGit, logPath)), 0o755); err != nil {
		t.Fatalf("影武者を書けない: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	g := &gitShim{logPath: logPath}
	// ⚠️ 影武者が本当に間に入っていることを確かめる。ここを省くと、
	// 差し替えに失敗しても「0 本で一定」で緑になり、検査が空振りする。
	before := g.count(t)
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Fatalf("影武者経由の git が動かない: %v", err)
	}
	if g.count(t) != before+1 {
		t.Fatalf("影武者が PATH に効いていない（この検査は空振りする）: log=%s", logPath)
	}
	return g
}

// count はこれまでに起きた git プロセスの本数。
func (g *gitShim) count(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile(g.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("影武者のログを読めない: %v", err)
	}
	return bytes.Count(b, []byte("\n"))
}

// shQuote は sh スクリプトへ埋めるためのシングルクォート。
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// countingGitShim は「数えてから本物へ渡す」影武者。
func countingGitShim(t *testing.T) *gitShim {
	return installGitShim(t, func(realGit, logPath string) string {
		return "#!/bin/sh\n" +
			"printf '%s\\n' \"$*\" >> " + shQuote(logPath) + "\n" +
			"exec " + shQuote(realGit) + " \"$@\"\n"
	})
}

// TestLoadRefSnapshot_RealGitProcessesDoNotGrowWithRecordCount は配線ガードである。
//
// 見ている性質（1つだけ）:
//
//	本番の入口 loadRefSnapshot が起こす「実際の git プロセスの本数」が、
//	レコード件数を振っても変わらないこと。
//
// 🔴 振っている軸を明示する（ここが射程のすべてである）:
//
//	レコード件数 … 5 / 60 / 200
//	ref         … HEAD / HEAD~1 / ブランチ名 / 完全 SHA
//
// **落ちる**もの — 上の軸の上で、実プロセス数が件数に比例したとき。
// 読み方をどう綴っても落ちる: runGit 経由でも、ループの中へ直に
// exec.Command("git", ...) と書いても、batch を1件ごとに起こし直しても、
// 配線を旧読み手へ戻しても、非 HEAD の ref だけ旧読み方に落としても、
// 件数が閾値を超えたときだけ旧読み方に落としても（閾値が 200 以下なら）。
//
// 🔴 **落ちない**もの — ここは塞ぎ切れていないので名乗る（CLAUDE.md 6）:
//
//   - **標本の外側にゲートを置く変異**。これは原理的に通る。件数を 200 までしか
//     振っていないので「n>300 で切り替える」は見えないし、ref を4通りしか
//     試していないので「ref=="main" のときだけ旧読み方」も見えない。
//     ⚠️ 軸を増やしても標本抽出であることは変わらない。**綴りを列挙して
//     1つずつ塞ぐ方向へは行かない**（CLAUDE.md 2）。捕まえたい性質は
//     「git のプロセスが件数に比例して起きないこと」の1つである。
//   - **PATH を経由しない git の起動**（絶対パスで /usr/bin/git を直に叩く等）。
//     影武者は PATH の先頭に居るので、名前解決を通らない起動は数えられない。
//   - 件数に依らない定数本のプロセス増。
//   - プロセス以外の原因（ls-tree 自体・JSON の unmarshal・store の読み込み）での遅さ。
//   - **実行時間そのもの**。一度も測っていない（機械と負荷で揺れるため）。
func TestLoadRefSnapshot_RealGitProcessesDoNotGrowWithRecordCount(t *testing.T) {
	shim := countingGitShim(t)

	sizes := []int{5, 60, 200}
	refLabels := []string{"HEAD", "HEAD~1", "branch", "sha"}
	// counts[refLabel][n] = 実プロセス本数
	counts := map[string]map[int]int{}
	for _, l := range refLabels {
		counts[l] = map[int]int{}
	}

	for _, n := range sizes {
		dir := recordRepo(t, n)
		sha := strings.TrimSpace(mustGit(t, dir, "rev-parse", "HEAD"))
		refs := map[string]string{"HEAD": "HEAD", "HEAD~1": "HEAD~1", "branch": "b1", "sha": sha}

		for _, label := range refLabels {
			before := shim.count(t)
			snap, err := loadRefSnapshot(dir, ".scholia", refs[label])
			if err != nil {
				t.Fatalf("n=%d ref=%s: loadRefSnapshot: %v", n, label, err)
			}
			// ⚠️ 空振り検出: 読めていなければプロセスも起きず「一定」で緑になる。
			if len(snap.Tags) != n {
				t.Fatalf("n=%d ref=%s: %d 件のはずが %d 件しか読めていない（この検査が空振りしている）",
					n, label, n, len(snap.Tags))
			}
			counts[label][n] = shim.count(t) - before
		}
	}

	for _, label := range refLabels {
		base := counts[label][sizes[0]]
		for _, n := range sizes[1:] {
			if counts[label][n] != base {
				t.Errorf("ref=%s: レコード件数で実 git プロセス数が変わった —— %d 件で %d 本、%d 件で %d 本。"+
					"件数に比例している＝1件1プロセスの読み方に戻っている",
					label, sizes[0], base, n, counts[label][n])
			}
		}
		t.Logf("ref=%-7s  実 git プロセス: 5 件=%d / 60 件=%d / 200 件=%d 本",
			label, counts[label][5], counts[label][60], counts[label][200])
	}
}

// recordRepo は tags を n 件持つ git repo を作る。HEAD~1 も .scholia を持つよう
// 2 コミット積み、ブランチ b1 を HEAD に置く（ref を振るため）。
func recordRepo(t *testing.T, n int) string {
	t.Helper()
	dir, _ := gitTestRepo(t)
	for i := 0; i < n; i++ {
		writeFile(t, filepath.Join(dir, ".scholia", "tags", fmt.Sprintf("t%03d.json", i)),
			fmt.Sprintf(`{"id":"t%03d","kind":"requirement","label":"t"}`+"\n", i))
	}
	commitAll(t, dir, "records")
	// .scholia の外に1件足して2つ目のコミットを作る（HEAD~1 でも同じ n 件が読める）。
	writeFile(t, filepath.Join(dir, "README.md"), "hello\n")
	commitAll(t, dir, "readme")
	mustGit(t, dir, "branch", "b1")
	return dir
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

// TestBatchBlobReader_FallsBackWhenBatchProcessDies は、batch を起こせたのに
// 応答を1行も読めなかった経路を踏む。
//
// この経路は2つの理由で検査が要る:
//
//  1. 🔴 **データ競合が在った場所**である。以前ここは cmd.Wait() より前に
//     os/exec の複写ゴルーチンが書いている bytes.Buffer を読んでいた。
//     -race が緑だったのは、この経路を踏む検査が1つも無かったからである。
//     ⚠️ この検査は go test -race で走らせて初めて意味を持つ。
//  2. 全件フォールバックが本当に正しい結果を返すことを、値で確かめる。
//
// `git cat-file` だけが即死する影武者を PATH に置いて再現する。
func TestBatchBlobReader_FallsBackWhenBatchProcessDies(t *testing.T) {
	installGitShim(t, func(realGit, logPath string) string {
		// cat-file だけ即座に終了させる（stdout が即 EOF になり、応答行が読めない）。
		return "#!/bin/sh\n" +
			"printf '%s\\n' \"$*\" >> " + shQuote(logPath) + "\n" +
			"case \"$1\" in cat-file) exit 0 ;; esac\n" +
			"exec " + shQuote(realGit) + " \"$@\"\n"
	})

	dir, _ := gitTestRepo(t)
	for i := 0; i < 5; i++ {
		writeFile(t, filepath.Join(dir, ".scholia", "tags", fmt.Sprintf("t%d.json", i)),
			fmt.Sprintf(`{"id":"t%d","kind":"requirement","label":"t"}`+"\n", i))
	}
	commitAll(t, dir, "seed")

	want, wantErr := loadRefSnapshotWith(dir, ".scholia", "HEAD", newShowBlobReader)
	got, gotErr := loadRefSnapshotWith(dir, ".scholia", "HEAD", newBatchBlobReader)

	if errString(wantErr) != errString(gotErr) {
		t.Fatalf("エラーが違う\n  git show       = %q\n  cat-file 即死  = %q", errString(wantErr), errString(gotErr))
	}
	if !snapshotsEqual(want, got) {
		t.Fatalf("batch が死んだときの snapshot が違う\n  git show      = %s\n  cat-file 即死 = %s",
			snapshotJSON(t, want), snapshotJSON(t, got))
	}
	if len(got.Tags) != 5 {
		t.Fatalf("5 件のはずが %d 件（この検査が空振りしている）", len(got.Tags))
	}

	// ⚠️ 本当に「batch が死んで全件フォールバックした」経路を踏んだことを確かめる。
	// 踏んでいなければ、上の一致は何も証明していない。
	r := newBatchBlobReader(dir, "HEAD").(*batchBlobReader)
	defer r.close()
	if _, err := r.read(".scholia/tags/t0.json"); err != nil {
		t.Fatalf("フォールバックが読めていない: %v", err)
	}
	if !r.broken {
		t.Error("batch が死んだのに broken になっていない（想定した経路を踏んでいない）")
	}
	if r.fallbacks != 1 {
		t.Errorf("fallbacks = %d, want 1（1件目から git show へ回るはず）", r.fallbacks)
	}
}

// TestLoadRefSnapshotWith_BatchMatchesPerFileShow は snapshot まるごとを新旧の
// 読み方で組み、値とエラー文字列を突き合わせる。読み手単体の突き合わせでは
// 見えない「どこで打ち切るか」（最初に失敗した path でそのエラーを返す）まで含めて
// 同じであることを見る。
func TestLoadRefSnapshotWith_BatchMatchesPerFileShow(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{"ordinary", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "req.x.json"), `{"id":"req.x","kind":"requirement","label":"x"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "transitions", "T-1.json"), `{"id":"T-1","action":"act.a","given":[],"then":["eff.a"]}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "decisions", "d1.json"), `{"id":"d1","target":{"type":"transition","id":"T-1"},"why":"w","at":"2026-01-01T00:00:00Z"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "config.json"), `{"idPolicy":{}}`+"\n")
		}},
		{"empty-file-aborts", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "a.json"), `{"id":"a","kind":"requirement","label":"a"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "b-empty.json"), "")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "c.json"), `{"id":"c","kind":"requirement","label":"c"}`+"\n")
		}},
		{"binary-aborts", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "a.json"), `{"id":"a","kind":"requirement","label":"a"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "b-binary.json"), "\x00\x01\x02\xff")
		}},
		{"symlink-aborts", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "a.json"), `{"id":"a","kind":"requirement","label":"a"}`+"\n")
			if err := os.Symlink("a.json", filepath.Join(dir, ".scholia", "tags", "b-link.json")); err != nil {
				t.Skipf("symlink を作れない: %v", err)
			}
		}},
		{"non-ascii-path-skipped", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "a.json"), `{"id":"a","kind":"requirement","label":"a"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "日本.json"), `{"id":"ja","kind":"requirement","label":"ja"}`+"\n")
		}},
		{"large-and-odd-names", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "large.json"),
				fmt.Sprintf(`{"id":"large","kind":"requirement","label":%q}`, strings.Repeat("xあ", 1<<19))+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "with space.json"), `{"id":"sp","kind":"requirement","label":"sp"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "x blob 12.json"), `{"id":"bn","kind":"requirement","label":"bn"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "sub", "nested.json"), `{"id":"nst","kind":"requirement","label":"nst"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "unknowncat", "u.json"), `{"id":"u"}`+"\n")
		}},
		{"missing-object-aborts", func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "a.json"), `{"id":"a","kind":"requirement","label":"a"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "b-gone.json"), `{"id":"b-gone-unique","kind":"requirement","label":"b"}`+"\n")
			writeFile(t, filepath.Join(dir, ".scholia", "tags", "c.json"), `{"id":"c","kind":"requirement","label":"c"}`+"\n")
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, _ := gitTestRepo(t)
			tc.setup(t, dir)
			commitAll(t, dir, "seed")
			if tc.name == "missing-object-aborts" {
				removeLooseObject(t, dir, "HEAD:.scholia/tags/b-gone.json")
			}

			wantSnap, wantErr := loadRefSnapshotWith(dir, ".scholia", "HEAD", newShowBlobReader)
			gotSnap, gotErr := loadRefSnapshotWith(dir, ".scholia", "HEAD", newBatchBlobReader)

			if errString(wantErr) != errString(gotErr) {
				t.Fatalf("エラーが違う\n  git show       = %q\n  cat-file batch = %q", errString(wantErr), errString(gotErr))
			}
			if !snapshotsEqual(wantSnap, gotSnap) {
				t.Fatalf("snapshot が違う\n  git show       = %s\n  cat-file batch = %s",
					snapshotJSON(t, wantSnap), snapshotJSON(t, gotSnap))
			}
		})
	}
}

// TestLoadRefSnapshotWith_BatchMatchesPerFileShowOnRealStore はこの repo 自身の
// .scholia（本番と同じ規模・同じ中身）で新旧を突き合わせる。合成 fixture が
// 取りこぼす形——実データにしか無い綴り——をここで踏む。
func TestLoadRefSnapshotWith_BatchMatchesPerFileShowOnRealStore(t *testing.T) {
	if testing.Short() {
		t.Skip("short モードでは実ストアを読まない")
	}
	repoRoot, err := gitRepoRoot(".")
	if err != nil {
		t.Skipf("git repo ではない: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".scholia")); err != nil {
		t.Skipf(".scholia が無い: %v", err)
	}

	wantSnap, wantErr := loadRefSnapshotWith(repoRoot, ".scholia", "HEAD", newShowBlobReader)
	gotSnap, gotErr := loadRefSnapshotWith(repoRoot, ".scholia", "HEAD", newBatchBlobReader)

	if errString(wantErr) != errString(gotErr) {
		t.Fatalf("エラーが違う\n  git show       = %q\n  cat-file batch = %q", errString(wantErr), errString(gotErr))
	}
	if !snapshotsEqual(wantSnap, gotSnap) {
		t.Fatalf("実ストアで snapshot が食い違った\n  git show       = %s\n  cat-file batch = %s",
			snapshotJSON(t, wantSnap), snapshotJSON(t, gotSnap))
	}
	total := len(wantSnap.Vocab) + len(wantSnap.Tags) + len(wantSnap.Transitions) + len(wantSnap.Decisions)
	if total == 0 {
		t.Fatal("実ストアから1件も読めていない（この検査が空振りしている）")
	}
	t.Logf("実ストア %d 件で新旧一致", total)
}

func TestBlobTarget(t *testing.T) {
	cases := []struct {
		path         string
		wantCategory string
		wantRead     bool
		note         string
	}{
		{".scholia/vocab/cond.a.json", "vocab", true, "カテゴリ直下の記録"},
		{".scholia/tags/req.x.json", "tags", true, ""},
		{".scholia/transitions/T-1.json", "transitions", true, ""},
		{".scholia/decisions/01K.json", "decisions", true, ""},
		{".scholia/tags/sub/deep/x.json", "tags", true, "カテゴリの下は何段でもよい"},
		{".scholia/unknowncat/u.json", "unknowncat", true, "未知カテゴリも読む（読む・読まないの境目を変えない）"},
		{".scholia/tags/with space.json", "tags", true, "空白を含む名前"},
		{".scholia/config.json", "", false, "カテゴリ直下でない（§4 の対象外）"},
		{".scholia/README.md", "", false, ".json でない"},
		{".scholia/tags/note.md", "", false, ".json でない"},
		{".scholia/tags/x.jsonx", "", false, ".json で終わらない"},
		{`".scholia/tags/\346\227\245\346\234\254.json"`, "", false,
			"core.quotePath が引用した非 ASCII パス。引用符で終わるので .json 判定に落ちる（現状の振る舞い）"},
		{".scholia", "", false, "relDir そのもの"},
	}
	for _, tc := range cases {
		gotCategory, gotRead := blobTarget(".scholia", tc.path)
		if gotCategory != tc.wantCategory || gotRead != tc.wantRead {
			t.Errorf("blobTarget(%q) = (%q, %v), want (%q, %v)  %s",
				tc.path, gotCategory, gotRead, tc.wantCategory, tc.wantRead, tc.note)
		}
	}
}

func TestParseBatchHeader(t *testing.T) {
	cases := []struct {
		line     string
		wantSize int64
		wantOK   bool
		note     string
	}{
		{"3614ab9c0cd212ec0ca5c26cc64cd14f5a3e47b1 blob 11\n", 11, true, "ふつうの blob"},
		{"3614ab9c0cd212ec0ca5c26cc64cd14f5a3e47b1 blob 0\n", 0, true, "空 blob"},
		{strings.Repeat("a", 64) + " blob 7\n", 7, true, "SHA-256 の oid"},
		{"HEAD:.scholia/tags/nope.json missing\n", 0, false, "存在しない path"},
		{"HEAD:.scholia/tags/x blob 12.json missing\n", 0, false,
			"入力を echo するエラー行が、たまたま <何か> blob <数字> の形に割れる"},
		{"3614ab9c0cd212ec0ca5c26cc64cd14f5a3e47b1 commit 250\n", 0, false, "blob でない（gitlink など）→ git show へ回す"},
		{"3614ab9c0cd212ec0ca5c26cc64cd14f5a3e47b1 tree 130\n", 0, false, "tree → git show へ回す"},
		{"3614ab9c0cd212ec0ca5c26cc64cd14f5a3e47b1 blob -1\n", 0, false, "負の size"},
		{"3614ab9c0cd212ec0ca5c26cc64cd14f5a3e47b1 blob xx\n", 0, false, "size が数字でない"},
		{"notahexoid blob 11\n", 0, false, "oid が16進でない"},
		{"\n", 0, false, "空行"},
		{"", 0, false, "空"},
	}
	for _, tc := range cases {
		gotSize, gotOK := parseBatchHeader(tc.line)
		if gotSize != tc.wantSize || gotOK != tc.wantOK {
			t.Errorf("parseBatchHeader(%q) = (%d, %v), want (%d, %v)  %s",
				tc.line, gotSize, gotOK, tc.wantSize, tc.wantOK, tc.note)
		}
	}
}

// removeLooseObject は rev（"HEAD:path" 形式）が指す object 本体を .git/objects から
// 消す。tree のエントリは残るので ls-tree には出続け、読もうとしたときだけ失敗する。
func removeLooseObject(t *testing.T, dir, rev string) {
	t.Helper()
	out, err := runGit(dir, "rev-parse", rev)
	if err != nil {
		t.Fatalf("rev-parse %s: %v", rev, err)
	}
	oid := strings.TrimSpace(out)
	if len(oid) < 3 {
		t.Fatalf("rev-parse %s が oid を返さなかった: %q", rev, oid)
	}
	loose := filepath.Join(dir, ".git", "objects", oid[:2], oid[2:])
	if err := os.Remove(loose); err != nil {
		t.Skipf("loose object を消せない（pack 済み？）: %v", err)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func head64(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

// snapshotsEqual は refSnapshot を値で比べる。
//
// ⚠️ ここで fmt の %v を使ってはいけない。model.Transition はポインタ欄を持つので、
// %v は中身ではなくアドレスを印字し、同じ内容でも毎回違う文字列になる（最初に
// この形で書いたら、実装が正しいのに実ストアの検査だけが落ちた）。
// reflect.DeepEqual はポインタの先まで辿るので、この用途ではこちらが正しい。
func snapshotsEqual(a, b refSnapshot) bool {
	return reflect.DeepEqual(a, b)
}

// snapshotJSON は食い違ったときに中身を見せるための整形（アドレスを出さない）。
func snapshotJSON(t *testing.T, s refSnapshot) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Sprintf("<marshal 失敗: %v>", err)
	}
	return string(b)
}
