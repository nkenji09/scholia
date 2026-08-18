// Package commitcheck は「decision に結ぶ commit hash が実在するか」を照合する
// （decision 01M09FHEQH7PVZ2BTKGXY5YMNN 変更5・「落とせる範囲」②）。
//
// これを入れる前、scholia は結ぶ hash を**一切検査していなかった**——
// `scholia decision add-commit <id> "これはハッシュではない"` が成功し、その
// 日本語の文字列がそのまま commits[] に保存された（実測）。`decide --commit` も
// 同じだった。
//
// ⚠️ 上の一行は**当時の呼び出し形**である（`--kind` は同じ決定で必須になったので、
// いま同じ文字列を打つと種別の指定が無いところで先に落ちる）。**手本ではない。**
//
// # 2つの検査を分ける（結論が変わる）
//
//	形の検査（LooksLikeHash）  … git を要らない純関数。どこでも効く。
//	実在の照合（Repo.Verify）  … git を要る。git 管理下でなければできない。
//
// **分けるのは、git が無い場所でも「これはハッシュではない」を弾けるからである。**
// 2つをまとめて「git が無ければ素通り」にすると、実測で踏んだ壊れ方そのものが
// git 管理外のストアで残る。
//
// # git が使えないとき——弾かない。素通りもしない。名乗る。
//
// git コマンドが無い／ストアが git 管理下でないとき、実在は**照合できない**。
// ここで採る答えは3つのうちの「名乗る」である。
//
//   - **弾く**は採らない。git 管理下でないストアで実装来歴を1件も結べなくなる。
//     照合できないことは、その hash が偽物であることの証拠ではない。
//   - **素通り**は採らない。呼んだ側は検査が走ったと読む。`scholia activity` が
//     浅い clone で数を出さずに名乗るのと同じ理由で、**取れているつもり**を
//     作らない（decision 01M09FHDJCV2WWFC7Z8331B0YQ）。
//   - **名乗る**を採る。保存はする。そのうえで「実在は照合していない」と出す。
//
// ⚠️ **したがって、この歯止めが落とす範囲は git の有無で変わる**（CLAUDE.md 6）:
//
//	git 管理下      … 形が違う hash・実在しない hash の両方が落ちる
//	git 管理外・git 無し … 形が違う hash だけが落ちる（形は合っているが実在
//	                       しない hash は通る。通ったことを黙らない）
//
// **落ちないもの（射程の外・正直に名乗る）:**
//   - ⚠️ **「その commit が本当にこの decision の実装（あるいは是正）である」
//     ことは確かめられない。** 実在する commit なら何でも通る——無関係の commit を
//     結ぶ変異は素通りする。ここが落とすのは「実在しない hash」だけである。
//   - ⚠️ **16 進の名前を持つブランチ/タグが、たまたま自分の名前を接頭辞に持つ
//     commit を指している場合**は通る。ただしそのとき保存されるのは「その名前を
//     短縮 hash として解決した結果」と同じ commit なので、害は無い。
//   - `.scholia/decisions/*.json` を store を通さず直接書く経路（エディタ・別ツール）。
//     保存の口に置く歯止めはファイルシステムそのものを守れない（`scholia lint` の領分）。
package commitcheck

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/nkenji09/scholia/internal/activity"
)

// Verdict は1つの hash についての結論（4値）。
type Verdict string

const (
	// VerdictExists は git 管理下で commit として解決した。
	VerdictExists Verdict = "exists"
	// VerdictMissing は git 管理下だが commit として解決しなかった。
	VerdictMissing Verdict = "missing"
	// VerdictMalformed は git のオブジェクト名の形をしていない（git を要らずに決まる）。
	VerdictMalformed Verdict = "malformed"
	// VerdictUnverifiable は形は合っているが、照合する相手（git）が無い。
	VerdictUnverifiable Verdict = "unverifiable"
)

// Rejects は「この結論なら保存を止めるべきか」を返す純関数。
func (v Verdict) Rejects() bool {
	return v == VerdictMalformed || v == VerdictMissing
}

// hashMinLen / hashMaxLen は受け付ける長さ。7 は git の既定の短縮長、
// 64 は SHA-256 リポジトリの完全長。
const (
	hashMinLen = 7
	hashMaxLen = 64
)

// LooksLikeHash は「git のオブジェクト名の形をしているか」を返す純関数
// （16 進数字だけで hashMinLen〜hashMaxLen 文字）。
//
// ⚠️ **ブランチ名・タグ名・`HEAD` は通さない。** それらは git では解決するが、
// commits[] に残るのは**動かない値**であるべきで、`HEAD` を保存すると指す先が
// 後から変わる。ここで形を要求することで、`decide --commit HEAD` は
// 「解決したが保存してはいけない値」として保存前に止まる。
//
// ⚠️ **形だけでは足りない。** 16 進 7〜64 文字の値は**ブランチ名やタグ名にもできる**
// ので、`git rev-parse --verify <h>^{commit}` は **ref を先に解決する**
// ——**曖昧エラーにはならず、exit 0 で「打った人が指していない commit」を返す**
// （実測: `c8d45c0` という名前のブランチを別 commit へ向けると、`c8d45c0…` では
// なくブランチの先が返った。git の警告は出ない）。だから解決したあとに
// **前方一致を確かめる**（Repo.Resolve）。ここは形の判定だけを担う。
func LooksLikeHash(s string) bool {
	if len(s) < hashMinLen || len(s) > hashMaxLen {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// Classify は「形の判定」と「git 側の答え」を組み合わせて結論を出す純関数
// （CLAUDE.md「配線ガードの書き方」1: 判断を画面と git から切り離し、入力と
// 出力の対で検査できる形にする）。
//
// gitManaged が false のとき resolved は見ない。
func Classify(hash string, gitManaged, resolved bool) Verdict {
	if !LooksLikeHash(hash) {
		return VerdictMalformed
	}
	if !gitManaged {
		return VerdictUnverifiable
	}
	if !resolved {
		return VerdictMissing
	}
	return VerdictExists
}

// Result は1つの hash の照合結果。
type Result struct {
	Hash    string
	Verdict Verdict
	// Canonical は git が解決した完全 hash（VerdictExists のときだけ非空）。
	// **短縮 hash と完全 hash が別の出来事として数えられるのを防ぐため**に返す
	// ——保存する値をここへ寄せると、後から文字列の完全一致で畳める
	// （decision 01M09FHEQH7PVZ2BTKGXY5YMNN「落とせない」節が
	// 「是正は commit hash で…重複を畳める」と書いている性質は、
	// 保存される値が同じ commit につき1つに定まって初めて成り立つ）。
	Canonical string
}

// Repo は照合の相手。「git 管理下か」を1度だけ解決して持つ。
type Repo struct {
	gitRoot string
	managed bool
}

// Open は projectRoot を含む git リポジトリを解決する。
//
// ⚠️ **リポジトリ根の解決は git 自身にさせる**（activity.ResolveGitContext）。
// 自前に `filepath.Rel` で計算すると、t.TempDir() 等がシンボリックリンク越しの
// パス（macOS の /var → /private/var）を返す場面で一致しない。単位BC が同じ
// 助走を既に書いてあるので、**ここでは二重に書かない**。
//
// 解決できないとき（git が PATH に無い・git 管理下でない）は managed=false の
// Repo を返す。これは異常ではなく、照合できないという正当な状態である。
func Open(projectRoot string) Repo {
	gitRoot, _, err := activity.ResolveGitContext(projectRoot)
	if err != nil {
		return Repo{}
	}
	return Repo{gitRoot: gitRoot, managed: true}
}

// Managed は git 管理下として解決できたかを返す（false のとき実在は照合しない）。
func (r Repo) Managed() bool { return r.managed }

// Verify は1つの hash を照合し、結論だけを返す。
func (r Repo) Verify(hash string) Verdict {
	return r.Resolve(hash).Verdict
}

// Resolve は1つの hash を照合し、結論と（解決したなら）完全 hash を返す。
//
// 🔴 **解決したものが「その hash」であることまで確かめる。** git は
// `<名前>^{commit}` を **ref 優先**で解決するので、**16 進の名前を持つブランチ/タグが
// 在ると、打った人が指した commit ではなくその ref の先が返る**——しかも
// exit 0 で、警告は stderr にすら出ない（実測）。返った値をそのまま
// `commits[]`・`applied[]` に書くと、**追記専用のフィールドに別の commit が
// 焼き付いて後から消せない。**
//
// 判定は前方一致1つで足りる: hash として解決したなら完全 hash は必ず入力を
// 接頭辞に持つ（短縮 hash の定義）。**ref として解決されたときだけ、これが破れる。**
func (r Repo) Resolve(hash string) Result {
	if !LooksLikeHash(hash) {
		return Result{Hash: hash, Verdict: VerdictMalformed} // git を呼ばずに決まる
	}
	if !r.managed {
		return Result{Hash: hash, Verdict: VerdictUnverifiable}
	}
	canonical := r.resolveToCommit(hash)
	if canonical != "" && !strings.HasPrefix(canonical, strings.ToLower(hash)) {
		// ref として解決された（16 進の名前を持つブランチ/タグ）。
		// **通さない。** 打った人が指した commit は、この repo には無い。
		return Result{Hash: hash, Verdict: VerdictMissing}
	}
	return Result{Hash: hash, Verdict: Classify(hash, true, canonical != ""), Canonical: canonical}
}

// VerifyAll は hashes を順に照合する。
func (r Repo) VerifyAll(hashes []string) []Result {
	out := make([]Result, 0, len(hashes))
	for _, h := range hashes {
		out = append(out, r.Resolve(h))
	}
	return out
}

// Canonical は hash が指す commit の完全 hash を返す（解決しなければ ""）。
// **保存する値をここへ寄せるための口**で、model の正規化関数がこれを受け取る。
func (r Repo) Canonical(hash string) string {
	return r.Resolve(hash).Canonical
}

// resolveToCommit は hash が commit オブジェクトとして解決するかを git に聞き、
// 解決したなら**完全 hash**を返す（解決しなければ ""）。
//
// `^{commit}` を付けるのは、tree や blob の hash を通さないため
// （`cat-file -t` で型を見て自分で比べるより、git 自身に peel させるほうが短い）。
// `--quiet` は解決しないときの "fatal: ..." を黙らせる——解決しないこと自体は
// この関数にとって正当な入力である。
//
// ⚠️ **出力を捨てない。** 以前は `cmd.Run()` の成否だけを見ていたが、それだと
// 短縮 hash を渡したときに「解決した」ことしか分からず、**保存される値が短縮の
// まま**残った。同じ commit が短縮と完全で2つの文字列として保存されると、
// 完全一致で畳む仕組み（applied[] の重複判定）が効かない——実測で是正が
// 2件に上振れした。
func (r Repo) resolveToCommit(hash string) string {
	cmd := exec.Command("git", "-C", r.gitRoot, "rev-parse", "--verify", "--quiet", hash+"^{commit}")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// RejectError は「保存前に止めた」こと。面ごとに文言を書き分けないよう、
// どの hash がなぜ止まったかを型で持つ。
type RejectError struct {
	Results []Result
}

func (e *RejectError) Error() string {
	var lines []string
	// 同じ値は1回だけ言う。commits[] と applied[] の両方に同じ hash が載る
	// 呼び出し（`add-commit --kind correction`）で、同じ文が2回出ていた。
	said := make(map[string]bool, len(e.Results))
	for _, res := range e.Results {
		if !res.Verdict.Rejects() || said[res.Hash] {
			continue
		}
		said[res.Hash] = true
		switch res.Verdict {
		case VerdictMalformed:
			lines = append(lines, fmt.Sprintf("%q は git の commit hash の形をしていません（16 進 %d〜%d 文字。ブランチ名や HEAD は使えません——保存されるのは後から動かない値である必要があります）",
				res.Hash, hashMinLen, hashMaxLen))
		case VerdictMissing:
			lines = append(lines, fmt.Sprintf("%q はこのリポジトリに実在しません（commit として解決しませんでした。先に commit してから結んでください）", res.Hash))
		}
	}
	return "結ぶ commit を保存前に止めました: " + strings.Join(lines, " / ")
}

// Check は hashes を照合し、止めるべきものが1つでもあれば RejectError を返す。
// 止めるものが無ければ nil（VerdictUnverifiable は止めない——名乗るのは呼んだ側）。
func (r Repo) Check(hashes []string) error {
	results := r.VerifyAll(hashes)
	rejected := false
	for _, res := range results {
		if res.Verdict.Rejects() {
			rejected = true
		}
	}
	if !rejected {
		return nil
	}
	return &RejectError{Results: results}
}
