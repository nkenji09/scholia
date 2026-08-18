// Package gitio は git を起動し、その出力からパスを取り出す入口である
// （decision 01M0AJDYP9524AKXFN6J3FXBYJ 変更1）。
//
// # なぜ入口を1つにするのか
//
// git は既定（`core.quotePath=true`）で、非 ASCII を含むパスを C 形式の引用で出す
// （`".scholia/tags/subject.\346\240\270\345\277\203.json"`）。出力のパスを前方一致で
// 判定する側は、引用符が1つ頭に付くだけで**必ず外れる**。誤りの向きは見落とす側で、
// 検査は「異常なし」を返す。実測: 同じ手順で ASCII 名のタグは検知され、
// 非 ASCII 名のタグは検知されなかった。
//
// **この package は `-z`（NUL 区切り）で読む。** 引用という機構そのものを通らないので、
// **どの文字が引用されるかを1つずつ数え上げなくてよくなる**
// （CLAUDE.md「配線ガードの書き方」2 と同じ考え方）。`-c core.quotePath=false` では
// 名前に `"`・バックスラッシュ・制御文字そのものが含まれる場合に引用が残る（実測）。
//
// あわせて、git が失敗したときに git 自身が標準エラーへ書いた理由を error に埋める。
// `exec.Cmd.Output()` は失敗時に `*exec.ExitError` を返すだけで、呼び出し側には
// `exit status 128` しか残らない。
//
// # 落ちない範囲（射程・正直に名乗る）
//
//   - 🔴 **新しく git の出力を読む面を足した人が、この入口を通さなくても落ちない。**
//     「この入口を呼んでいるか」を見る検査は**ソース文字列の照合**にしかならず、
//     同じ意味を別の綴りで書けば通る（別名で束ねる・別のラッパを作る・直に書く）。
//     綴りを1つずつ列挙して塞ぐ形は列挙が終わらないので採らない。
//     同型の名乗りが internal/gittest にもある（「その package を一度も import しない
//     新しい package は原理的に防げない」）。
//     **代わりに置いてあるのは、値で落ちる検査**——病的な名前（非 ASCII・引用が要る
//     文字・空白）のレコードを含むストアで、面が返す答えを期待値そのものと突き合わせる。
//     ⚠️ その検査も**新しく足した面には効かない**（CLAUDE.md 5 の穴）。
//   - **git が exit 0 のまま部分的に間違った出力を返す形。** 落ちたことにならない。
//   - **`internal/diff` と `internal/activity` の読み方。** この package を作った単位では
//     `internal/lint` だけを乗せ替えた。`internal/activity` は git を起動する側だけを
//     ここへ寄せてあり、読み方は `-c core.quotePath=false` のままである
//     （`-z` にすると `"` を含むパスの分類が変わる＝振る舞いが変わるため）。
package gitio

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Run は `git -C dir <args...>` を走らせ、標準出力を返す。
//
// 失敗したときは、git が標準エラーへ書いた理由（"fatal: ..."）を error に埋める
// ——ここで埋めないと、呼び出し元には `exit status 128` しか残らない。
func Run(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

// Installed は git を PATH から起動できるかを返す。
//
// 「git が無い」を「git はあるが落ちた」から分けるために要る——git は
// 「git repo でない」も「repo だが読めない」も**同じ exit 128 の fatal** で返すので、
// 起動できたかどうかだけが、文言を読まずに取れる区別である。
func Installed() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// ResolveContext は git 管理下かどうかを解決し、リポジトリ根と、そこから見た
// dir の相対位置を返す。
//
// ⚠️ **リポジトリ根とストアの相対位置は git 自身に解決させる。** 固定の接頭辞
// （`.scholia/`）で判定すると、リポジトリ根より下にストアがあるとき何も拾えない
// ——実測: `sub/` の下にストアを置くと、非 ASCII が1文字も無くても
// `decision-stale` は0件になった。
func ResolveContext(dir string) (gitRoot, relPrefix string, err error) {
	rootOut, err := Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	gitRoot = strings.TrimSpace(string(rootOut))

	prefixOut, err := Run(dir, "rev-parse", "--show-prefix")
	if err != nil {
		return "", "", fmt.Errorf("git rev-parse --show-prefix: %w", err)
	}
	relPrefix = strings.TrimSuffix(strings.TrimSpace(string(prefixOut)), "/")
	return gitRoot, relPrefix, nil
}

// --- 出力の読み方（git を呼ばない純関数・入力と出力の対で検査する） ---

// CommitMark は commit 見出しの先頭に置く番兵。
//
// パスの先頭に来ることが無い制御文字を使い、commit 見出しと status/path を
// 取り違えない。「40 桁の 16 進に見えるか」で見分ける形は、**パスが偶然その形に
// 見えれば取り違える**。
const CommitMark = "\x01"

// LogFormatArg は `git log` に渡す `--format`（番兵つき）。
const LogFormatArg = "--format=" + CommitMark + "%H"

// Change は 1 commit の中の 1 つの変更。
type Change struct {
	// Status は git の status 文字列（"M"・"A"・"D"・"R100" など）。
	Status string
	// Path は変更されたパス（rename/copy は**新しいほう**）。
	Path string
}

// Commit は `--name-status -z` の 1 commit 分。
type Commit struct {
	Hash    string
	Changes []Change
}

// ParseNameStatusZ は `git log LogFormatArg --name-status -z` の出力を解釈する。
//
// 出力の形（実測）:
//
//	<CommitMark><hash>\0 "\nM"\0 <path>\0 "M"\0 <path>\0 <CommitMark><hash>\0 …
//
// commit 見出しの直後の status には改行が1つ付く（git が見出しと差分の間に
// 入れる区切り）。ファイルを1つも変えていない commit では status が1つも続かず、
// 次の見出しがそのまま来る。
//
// rename（R…）と copy（C…）は**パスを2つ**持つ（旧・新）。
//
// ⚠️ status とパスの区別は**位置で決める**（中身は見ない）。`-z` のパスは
// 生バイトなので、改行や引用符を含む名前が来ても取り違えない。
func ParseNameStatusZ(out []byte) ([]Commit, error) {
	fields := strings.Split(string(out), "\x00")
	var commits []Commit
	for i := 0; i < len(fields); {
		f := fields[i]
		if f == "" {
			i++
			continue
		}
		if strings.HasPrefix(f, CommitMark) {
			commits = append(commits, Commit{Hash: strings.TrimPrefix(f, CommitMark)})
			i++
			continue
		}
		if len(commits) == 0 {
			return nil, fmt.Errorf("git の出力が commit 見出しで始まっていません: %q", f)
		}
		status := strings.TrimPrefix(f, "\n")
		paths := 1
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			paths = 2 // 旧パスと新パス
		}
		if i+paths >= len(fields) {
			return nil, fmt.Errorf("status %q に対応するパスが足りません", status)
		}
		c := &commits[len(commits)-1]
		c.Changes = append(c.Changes, Change{Status: status, Path: fields[i+paths]})
		i += paths + 1
	}
	return commits, nil
}
