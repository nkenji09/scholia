package diff

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// blobReader は「ある git ref 上の path の中身」を返す読み手。読み方だけを
// 差し替えるための境界であり、返す中身・返すエラーは読み方によらず同じでなければ
// ならない（gitblob_test.go の突き合わせ検査がそれを守る）。
type blobReader interface {
	// read は ref 上の path の中身をバイト列そのままで返す。JSON である必要も
	// UTF-8 である必要もない（symlink なら「リンク先の文字列」が返る）。
	read(path string) (string, error)
	// close は読み手が抱えたプロセス・パイプを片付ける。
	close()
}

// blobOpener は blobReader を1つ開く。loadRefSnapshotWith がこれを引数で受け取る
// ことで、テストが新旧の読み方を同じ入力に対して並べて突き合わせられる。
type blobOpener func(repoRoot, ref string) blobReader

// showBlobReader は path 1件ごとに `git show <ref>:<path>` を1プロセス起こす。
// 以前はこれが唯一の読み方だった。いまも次の3つの役目で残っている:
//
//  1. batch が blob として答えなかった1件を、以前と同じエラー文言で読み直す
//  2. batch を始められない環境での全面フォールバック
//  3. batch の正しさを突き合わせる相手（テスト）
type showBlobReader struct {
	repoRoot string
	ref      string
}

func newShowBlobReader(repoRoot, ref string) blobReader {
	return &showBlobReader{repoRoot: repoRoot, ref: ref}
}

func (r *showBlobReader) read(path string) (string, error) {
	return runGit(r.repoRoot, "show", r.ref+":"+path)
}

func (r *showBlobReader) close() {}

// batchBlobReader は `git cat-file --batch` を1プロセスだけ起こし、そこへ path を
// 1件ずつ投げて読む。プロセス生成が O(ファイル数) から O(1) になるのが目的で、
// 返す中身は showBlobReader と1バイトも変わらない。
//
// 要求はまとめて書かず、1件投げて1件読み切ってから次を投げる。まとめ書きは
// (a) 相手の stdout が詰まったところで相互待ちになりうる (b) 貯めた分だけ
// メモリを食う、の2つを抱える。1件ずつなら常駐するのは「いま読んでいる1件」
// だけで、.scholia が大きくなっても増えない。
type batchBlobReader struct {
	repoRoot string
	ref      string

	started bool
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader

	// fallback は batch が答えられなかった1件を読み直す相手。エラーの文言まで
	// 以前と同じにするため、異常系は必ずここを通す。
	fallback *showBlobReader
	// broken は応答が解釈できずストリームの同期を保証できなくなった状態。以後は
	// 全件 fallback で読む（黙って中身を取り違えるより、1件1プロセスの方がよい）。
	broken bool
	// fallbacks は fallback へ回した件数。ふつうのレコードがここに落ちていたら
	// 「batch を通しているつもりで1件1プロセスに戻っていた」ということなので、
	// テストがこれを 0 で見張る（速度ではなく経路を見る）。
	fallbacks int
}

func newBatchBlobReader(repoRoot, ref string) blobReader {
	return &batchBlobReader{
		repoRoot: repoRoot,
		ref:      ref,
		fallback: &showBlobReader{repoRoot: repoRoot, ref: ref},
	}
}

// start は最初に read が呼ばれたときだけ git を起こす（読む対象が1件も無い ref で
// プロセスを無駄にしないため）。起こせなければ broken にして全面フォールバックする。
func (r *batchBlobReader) start() {
	if r.started {
		return
	}
	r.started = true

	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = r.repoRoot
	// stderr は捕まえない（Stderr==nil は os.DevNull へ繋がる）。以前は
	// bytes.Buffer に取ってエラー文へ埋めていたが、(a) その値は read が
	// 捨てている——ask が err を返した先で readViaFallback に回るだけで、
	// 利用者に届くのは `git show` が出す方のエラーである——のに
	// (b) os/exec の複写ゴルーチンが書いている buffer を cmd.Wait() より前に
	// 読むデータ競合を抱えていた。使われない文字列のための競合だったので消した。
	stdin, err := cmd.StdinPipe()
	if err != nil {
		r.broken = true
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		r.broken = true
		return
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		r.broken = true
		return
	}
	r.cmd, r.stdin, r.stdout = cmd, stdin, bufio.NewReader(stdout)
}

func (r *batchBlobReader) read(path string) (string, error) {
	r.start()
	if r.broken {
		return r.readViaFallback(path)
	}
	// 要求は1行1件。改行・NUL を含む path は行の区切りを壊す。ls-tree は
	// そういう名前を引用して返すので blobTarget の .json 判定を通らないが、
	// この境界の側でも塞いでおく。
	if strings.ContainsAny(path, "\n\x00") {
		return r.readViaFallback(path)
	}

	content, ok, err := r.ask(path)
	if err != nil {
		// ストリームの同期が保証できない。以後は全件 `git show` で読む。
		r.shutdown()
		r.broken = true
		return r.readViaFallback(path)
	}
	if !ok {
		// git が blob として答えなかった（missing・blob 以外）。エラーの文言を
		// 以前と同じにするため、この1件だけ `git show` で読み直す。
		return r.readViaFallback(path)
	}
	return content, nil
}

func (r *batchBlobReader) readViaFallback(path string) (string, error) {
	r.fallbacks++
	return r.fallback.read(path)
}

// ask は1件投げて応答を1件読む。
//
//	ok=false, err=nil : git が blob として答えなかった。ストリームは同期したまま。
//	err != nil        : 同期が保証できない（以後 batch を使ってはならない）。
func (r *batchBlobReader) ask(path string) (content string, ok bool, err error) {
	if _, err := io.WriteString(r.stdin, r.ref+":"+path+"\n"); err != nil {
		return "", false, err
	}
	header, err := r.stdout.ReadString('\n')
	if err != nil {
		return "", false, fmt.Errorf("git cat-file --batch: %w", err)
	}
	size, ok := parseBatchHeader(header)
	if !ok {
		return "", false, nil
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r.stdout, buf); err != nil {
		return "", false, fmt.Errorf("git cat-file --batch: %w", err)
	}
	// 中身の直後に git が付ける LF を1バイト捨てる。ここを読み残すと次の応答の
	// 先頭とずれ、以降すべての path が別の中身とすり替わる。
	b, err := r.stdout.ReadByte()
	if err != nil {
		return "", false, fmt.Errorf("git cat-file --batch: %w", err)
	}
	if b != '\n' {
		return "", false, fmt.Errorf("git cat-file --batch: 中身の後に LF が来ませんでした")
	}
	return string(buf), true, nil
}

// parseBatchHeader は `git cat-file --batch` の応答1行目を読み、blob として
// 答えたときだけ (size, true) を返す。
//
//	"<oid> blob <size>"   -> (size, true)
//	"<入力> missing"       -> (0, false)  ref 上にその path が無い
//	"<oid> commit <size>"  -> (0, false)  gitlink など blob でないもの
//
// blob 以外を false にするのは、`git show <ref>:<path>` が blob 以外に対しては
// 中身をそのまま出さない（commit ならログを整形して出す）ためである。ここで
// false にして `git show` へ回せば、出力もエラーも以前と同じになる。
//
// oid が16進であることまで見るのは、応答が「入力をそのまま echo する」形の
// エラー行でもありうるからで、`.scholia/x blob 12.json` のような名前が
// たまたま3語に割れて blob と誤読されるのを防ぐ。
func parseBatchHeader(line string) (size int64, ok bool) {
	fields := strings.Fields(strings.TrimSuffix(line, "\n"))
	if len(fields) != 3 || fields[1] != "blob" || !isHex(fields[0]) {
		return 0, false
	}
	n, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func (r *batchBlobReader) shutdown() {
	if r.cmd == nil {
		return
	}
	r.stdin.Close()
	// 応答を読み切らないまま終わらせるので、残りを捨ててから Wait する
	// （StdoutPipe は「全部読み終える前に Wait してはいけない」）。
	io.Copy(io.Discard, r.stdout)
	r.cmd.Wait()
	r.cmd = nil
}

func (r *batchBlobReader) close() { r.shutdown() }

// blobTarget は `git ls-tree -r --name-only` が返した1行を、diff が中身を読む
// 対象かどうかに振り分ける（§4 が比較するのは vocab/tag/transition/decision）。
//
// read=true になるのは「.json で終わり、relDir の直下ではなく1段以上下にある」行:
//
//	.scholia/tags/x.json         -> ("tags", true)
//	.scholia/tags/sub/x.json     -> ("tags", true)   カテゴリの下は何段でもよい
//	.scholia/config.json         -> ("", false)      カテゴリ直下でない（§4 の対象外）
//	.scholia/tags/x.md           -> ("", false)      .json でない
//	".scholia/tags/\346..\.json" -> ("", false)      引用された非 ASCII パス
//
// 最後の1つは意図した除外ではなく、core.quotePath（既定 on）が非 ASCII を含む
// 名前を引用符ごと返し、その行が `.json` で終わらなくなることの帰結である。
// ここに書いてあるのは現状の振る舞いであって、こうあるべきという判断ではない。
// 読み方を差し替えても同じ行が同じように落ちることを、この関数を値で検査して固定する。
//
// カテゴリ名が既知の4つでないときも read=true を返す。以前の実装は中身を読んで
// から switch で捨てていたので、読む・読まないの境目をずらさないためである。
func blobTarget(relDir, path string) (category string, read bool) {
	if !strings.HasSuffix(path, ".json") {
		return "", false
	}
	rest := strings.TrimPrefix(path, relDir+"/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", false
	}
	return parts[0], true
}
