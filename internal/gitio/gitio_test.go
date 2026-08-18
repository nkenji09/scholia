package gitio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/gittest"
)

// TestParseNameStatusZ は、git の出力の解釈を**入力と出力の対**で検査する
// （CLAUDE.md「配線ガードの書き方」1）。値で見るので、実装の綴りを変えても
// 答えが変われば落ちる。
//
// 落ちない範囲: ここは**解釈だけ**を見る。git が実際にこの形で出すことは
// TestParseNameStatusZ_MatchesRealGitOutput が実物で確かめる。
func TestParseNameStatusZ(t *testing.T) {
	const hashA = "0563ddd9b1ba2fd5ccd4dde16727e896907998f3"
	const hashB = "ae5ec8f8ad41320e852d9d79ab91de9245000913"
	const hashC = "41aa764af4475d44f35527550157a099ea39476b"

	tests := []struct {
		name string
		raw  string
		want []Commit
	}{
		{
			name: "非 ASCII のパスが引用されずに読める",
			raw:  CommitMark + hashB + "\x00" + "\nM\x00.scholia/tags/subject.核心.json\x00",
			want: []Commit{{Hash: hashB, Changes: []Change{
				{Status: "M", Path: ".scholia/tags/subject.核心.json"},
			}}},
		},
		{
			name: `引用符とバックスラッシュを含むパスも生のまま読める`,
			raw:  CommitMark + hashB + "\x00" + "\nM\x00" + `.scholia/tags/quote"back\slash.json` + "\x00",
			want: []Commit{{Hash: hashB, Changes: []Change{
				{Status: "M", Path: `.scholia/tags/quote"back\slash.json`},
			}}},
		},
		{
			name: "rename はパスを2つ持ち、取るのは新しいほう",
			raw: CommitMark + hashA + "\x00" + "\nA\x00.scholia/decisions/d1.json\x00" +
				"R100\x00.scholia/tags/ascii.json\x00.scholia/tags/renamed 名前.json\x00",
			want: []Commit{{Hash: hashA, Changes: []Change{
				{Status: "A", Path: ".scholia/decisions/d1.json"},
				{Status: "R100", Path: ".scholia/tags/renamed 名前.json"},
			}}},
		},
		{
			name: "ファイルを1つも変えていない commit は Changes が空のまま次へ進む",
			raw: CommitMark + hashC + "\x00" +
				CommitMark + hashB + "\x00" + "\nM\x00.scholia/tags/ascii.json\x00",
			want: []Commit{
				{Hash: hashC},
				{Hash: hashB, Changes: []Change{{Status: "M", Path: ".scholia/tags/ascii.json"}}},
			},
		},
		{
			name: "commit 見出しの直後だけ status に改行が付く（2つ目以降は付かない）",
			raw: CommitMark + hashB + "\x00" + "\nM\x00a.json\x00" + "M\x00b.json\x00" +
				"D\x00c.json\x00",
			want: []Commit{{Hash: hashB, Changes: []Change{
				{Status: "M", Path: "a.json"},
				{Status: "M", Path: "b.json"},
				{Status: "D", Path: "c.json"},
			}}},
		},
		{
			name: "出力が空なら commit も無い",
			raw:  "",
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseNameStatusZ([]byte(tc.raw))
			if err != nil {
				t.Fatalf("ParseNameStatusZ: %v", err)
			}
			if !equalCommits(got, tc.want) {
				t.Fatalf("解釈が違う:\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// TestParseNameStatusZ_HeaderlessInputIsAnError は、番兵で始まらない出力を
// 「commit ゼロ」に縮退させないことを見る。縮退させると、形が変わったときに
// **黙って0件**になる——この単位が塞いだ穴と同じ形である。
func TestParseNameStatusZ_HeaderlessInputIsAnError(t *testing.T) {
	if _, err := ParseNameStatusZ([]byte("M\x00a.json\x00")); err == nil {
		t.Fatal("commit 見出しの無い出力は error にするはず")
	}
}

// TestParseNameStatusZ_MatchesRealGitOutput は、**実物の git を走らせて**その
// 出力を解釈し、期待値そのものと突き合わせる。
//
// 合成サンプルだけを見ていると、git 側の形が変わったことに気づけない。
// ⚠️ 期待値は「ASCII の対照と同じ答え」ではなく**期待する答えそのもの**を書く
// ——対照と比べる形は、両方が同時に壊れたときに差が出ない。
func TestParseNameStatusZ_MatchesRealGitOutput(t *testing.T) {
	dir := t.TempDir()
	gittest.InitRepo(t, dir)

	writeFile(t, dir, ".scholia/tags/subject.ascii.json", "{}\n")
	writeFile(t, dir, ".scholia/tags/subject.核心.json", "{}\n")
	gittest.Run(t, dir, "add", "-A")
	gittest.Run(t, dir, "commit", "-q", "-m", "add")

	writeFile(t, dir, ".scholia/tags/subject.ascii.json", `{"a":1}`+"\n")
	writeFile(t, dir, ".scholia/tags/subject.核心.json", `{"a":1}`+"\n")
	gittest.Run(t, dir, "add", "-A")
	gittest.Run(t, dir, "commit", "-q", "-m", "modify")

	out, err := Run(dir, "log", "-n1", "-M", "--name-status", "-z", LogFormatArg)
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	commits, err := ParseNameStatusZ(out)
	if err != nil {
		t.Fatalf("ParseNameStatusZ: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("commit 1 件のはずが %d 件: %+v", len(commits), commits)
	}
	got := map[string]string{}
	for _, ch := range commits[0].Changes {
		got[ch.Path] = ch.Status
	}
	want := map[string]string{
		".scholia/tags/subject.ascii.json": "M",
		".scholia/tags/subject.核心.json":    "M",
	}
	if len(got) != len(want) {
		t.Fatalf("変更の件数が違う:\n got %v\nwant %v", got, want)
	}
	for p, st := range want {
		if got[p] != st {
			t.Fatalf("パス %q の status が違う: got %q want %q（全体 %v）", p, got[p], st, got)
		}
	}
}

// TestRun_EmbedsStderr は、git が失敗したときに git 自身が書いた理由が
// error に残ることを見る。残らないと、呼び出し元には exit status しか届かない。
func TestRun_EmbedsStderr(t *testing.T) {
	dir := t.TempDir() // git repo ではない

	_, err := Run(dir, "log", "-n1")
	if err == nil {
		t.Fatal("git repo でないディレクトリでの git log は失敗するはず")
	}
	// ⚠️ git の文言そのものを照合しない（版と locale で変わる）。見るのは
	// 「素の exit status より長い＝標準エラーが埋まっている」ことである。
	if !strings.Contains(err.Error(), "exit status") {
		t.Fatalf("git の終了状態が error に残っていない: %v", err)
	}
	if len(err.Error()) <= len("exit status 128") {
		t.Fatalf("標準エラーが error に埋まっていない（exit status だけ残っている）: %v", err)
	}
	t.Logf("埋まった理由: %v", err)
}

// TestResolveContext_ResolvesPrefixBelowRepoRoot は、ストアがリポジトリ根の下に
// あるときの相対位置を git 自身に解決させることを見る。固定接頭辞で判定する形は、
// この位置で必ず外れる。
func TestResolveContext_ResolvesPrefixBelowRepoRoot(t *testing.T) {
	dir := t.TempDir()
	gittest.InitRepo(t, dir)
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	gitRoot, relPrefix, err := ResolveContext(sub)
	if err != nil {
		t.Fatalf("ResolveContext: %v", err)
	}
	if relPrefix != "sub" {
		t.Fatalf("relPrefix = %q, want %q", relPrefix, "sub")
	}
	// gitRoot は git が答えた根（シンボリックリンク解決の差があるので、
	// 末尾の要素だけ突き合わせる）。
	if filepath.Base(gitRoot) != filepath.Base(dir) {
		t.Fatalf("gitRoot = %q, want ...%s", gitRoot, filepath.Base(dir))
	}

	if _, _, err := ResolveContext(t.TempDir()); err == nil {
		t.Fatal("git 管理下でないディレクトリでは error になるはず")
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func equalCommits(a, b []Commit) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Hash != b[i].Hash || len(a[i].Changes) != len(b[i].Changes) {
			return false
		}
		for j := range a[i].Changes {
			if a[i].Changes[j] != b[i].Changes[j] {
				return false
			}
		}
	}
	return true
}

// TestHasAnyCommit は「commit が1件でもあるか」の判定を、unborn HEAD と
// commit のある repo の対で見る。
//
// この判定は「導出が落ちた」と「見るものが無い」を分けるために要る——git は
// どちらも同じ exit 128 の fatal で返すので、終了状態だけを見ると必ず混ざる。
func TestHasAnyCommit(t *testing.T) {
	dir := t.TempDir()
	gittest.InitRepo(t, dir) // git init 直後＝unborn HEAD

	has, err := HasAnyCommit(dir)
	if err != nil {
		t.Fatalf("unborn HEAD は異常ではない（error にしてはいけない）: %v", err)
	}
	if has {
		t.Fatal("commit が1件も無いのに has=true")
	}

	writeFile(t, dir, "a.txt", "x\n")
	gittest.Run(t, dir, "add", "-A")
	gittest.Run(t, dir, "commit", "-q", "-m", "first")

	if has, err = HasAnyCommit(dir); err != nil || !has {
		t.Fatalf("commit があるのに has=%v err=%v", has, err)
	}

	// git 管理下でないディレクトリも「commit ゼロ」に写る（呼び出し元は
	// どちらでも黙るので、ここで分ける必要は無い）。
	if has, err = HasAnyCommit(t.TempDir()); err != nil || has {
		t.Fatalf("git 管理下でないディレクトリ: has=%v err=%v", has, err)
	}
}
