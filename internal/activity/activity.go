// Package activity は「実装活動」を git 履歴から導出する（`scholia activity`・
// decision 01M09FHDJCV2WWFC7Z8331B0YQ・req.practice-observability）。
//
// 何も保存しない。呼ばれるたびに git を読み、数えて、返す。だから機構を足す前の
// 期間についても同じ数が出せる——ただし記録ディレクトリの名前が過去に変わっている
// と、改名より前の期間で水増しされる（WindowPredatesStore で開示する。決定本文の
// 「導出なので、機構が入る前の期間についても同じ数が出る」節）。
package activity

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Window は数える区間 [Since, Until)。
type Window struct {
	Since time.Time
	Until time.Time
}

// Options は Compute の入力。
type Options struct {
	// GitRoot は git リポジトリの根（`git rev-parse --show-toplevel`）。
	GitRoot string
	// RelPrefix はストアのプロジェクトルートが GitRoot から見て何本下にあるか
	// （`git rev-parse --show-prefix`・根そのものなら ""）。ResolveGitContext が
	// git 自身に解決させる——decision-stale の固定接頭辞バグ（根より下のストアで
	// 何も拾えない）を、同じ形で再発させないための唯一の理由がここにある。
	RelPrefix string
	// StoreDirName は記録ディレクトリの現在名（store.DirName・".scholia"）。
	StoreDirName string
	Window       Window
	// DecisionTimes は各 decision の at（呼び出し側で既にパース済みの瞬間）。
	// git の履歴からではなくレコード自身の at から数える——改名が「decision 追加」
	// に化けて過大計上される実測済みの壊れ方（82 件）を避けるため。
	DecisionTimes []time.Time
	// Loc は「実装が動いた日」を数える暦日の基準タイムゾーン（呼び出し側の既定は
	// config.timezone・空なら UTC）。
	Loc *time.Location
	// Now は「最後の実装からの経過」を測る基準時刻。呼び出し側が渡す
	// （Compute 自身は time.Now を呼ばない＝入力と出力の対で検査できる）。
	Now time.Time
}

// Report は Compute の出力。何も保存しない値なので、次に呼ばれれば作り直される。
type Report struct {
	Since, Until time.Time
	WindowDays   int

	// Shallow が true のとき、以下の git 由来カウンタ（ActiveDays・ImplCommits・
	// RecordOnlyCommits・LastActivity・LastActivityDaysAgo）は計算していない
	// （nil/ゼロ値）。浅い clone は full clone よりずっと少ない commit しか
	// 見えないため、実測せずに数を出すと「実装が止まっている」に化ける
	// （実測: 同じ 30 日窓で full clone 268/16 → depth 1 clone 1/1）。
	Shallow bool

	ActiveDays        int
	ImplCommits       int
	RecordOnlyCommits int
	LastActivity      *time.Time
	// LastActivityDaysAgo は Now を基準にした経過日数（窓に依存しない）。
	LastActivityDaysAgo *int

	// DecisionsInWindow は git を経由しない（DecisionTimes から数える）ので、
	// Shallow でも常に埋まる。
	DecisionsInWindow int

	// StoreFirstSeen は記録ディレクトリ（現在名）が最初に現れた commit の日時。
	// 一度も無ければ nil（新規ストアでまだ commit されていない等）。
	StoreFirstSeen *time.Time
	// WindowPredatesStore は Since が StoreFirstSeen より前であること
	// （StoreFirstSeen が nil なら常に false＝判定材料が無い）。
	WindowPredatesStore bool
}

// ResolveGitContext は projectRoot を含む git リポジトリの根と、その根から見た
// projectRoot の相対パスを、git 自身に解決させる。
//
// filepath.Rel で自前に計算しない理由: t.TempDir() 等がシンボリックリンク越しの
// パス（macOS の /var → /private/var）を返すことがあり、素朴な文字列比較は
// 一致しない。`git rev-parse --show-prefix` は git 自身が同じ内部表現で答えるので
// この不一致が起きない。
func ResolveGitContext(projectRoot string) (gitRoot, relPrefix string, err error) {
	rootOut, err := exec.Command("git", "-C", projectRoot, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	gitRoot = strings.TrimSpace(string(rootOut))

	prefixOut, err := exec.Command("git", "-C", projectRoot, "rev-parse", "--show-prefix").Output()
	if err != nil {
		return "", "", fmt.Errorf("git rev-parse --show-prefix: %w", err)
	}
	relPrefix = strings.TrimSuffix(strings.TrimSpace(string(prefixOut)), "/")
	return gitRoot, relPrefix, nil
}

// Compute は実装活動を git から数えて返す。何も保存しない。
func Compute(opts Options) (Report, error) {
	loc := opts.Loc
	if loc == nil {
		loc = time.UTC
	}

	rep := Report{
		Since:             opts.Window.Since,
		Until:             opts.Window.Until,
		WindowDays:        windowDays(opts.Window),
		DecisionsInWindow: countDecisionsInWindow(opts.DecisionTimes, opts.Window),
	}

	shallow, err := isShallow(opts.GitRoot)
	if err != nil {
		return Report{}, fmt.Errorf("git rev-parse --is-shallow-repository: %w", err)
	}
	rep.Shallow = shallow
	if shallow {
		// これ以降は git 由来の数を一切埋めない（黙って嘘の数を出さない）。
		return rep, nil
	}

	rootSpec := rootPathspec(opts.RelPrefix)
	storeSpec := storePathspec(opts.RelPrefix, opts.StoreDirName)

	commits, err := windowCommits(opts.GitRoot, rootSpec, opts.Window.Since, opts.Window.Until)
	if err != nil {
		return Report{}, fmt.Errorf("git log（窓内の commit 走査）: %w", err)
	}
	impl, recordOnly, activeDays := classify(commits, storeSpec, loc)
	rep.ImplCommits = impl
	rep.RecordOnlyCommits = recordOnly
	rep.ActiveDays = activeDays

	last, err := lastActivity(opts.GitRoot, rootSpec, storeSpec)
	if err != nil {
		return Report{}, fmt.Errorf("git log（最後の実装活動・窓に依存しない）: %w", err)
	}
	rep.LastActivity = last
	if last != nil {
		days := int(opts.Now.Sub(*last).Hours() / 24)
		rep.LastActivityDaysAgo = &days
	}

	firstSeen, err := storeFirstSeen(opts.GitRoot, storeSpec)
	if err != nil {
		return Report{}, fmt.Errorf("git log（記録ディレクトリの初出）: %w", err)
	}
	rep.StoreFirstSeen = firstSeen
	if firstSeen != nil && opts.Window.Since.Before(*firstSeen) {
		rep.WindowPredatesStore = true
	}

	return rep, nil
}

func windowDays(w Window) int {
	return int(w.Until.Sub(w.Since).Hours()/24 + 0.5)
}

func countDecisionsInWindow(times []time.Time, w Window) int {
	n := 0
	for _, t := range times {
		if !t.Before(w.Since) && t.Before(w.Until) {
			n++
		}
	}
	return n
}

func rootPathspec(relPrefix string) string {
	if relPrefix == "" || relPrefix == "." {
		return "."
	}
	return relPrefix
}

func storePathspec(relPrefix, storeDirName string) string {
	if relPrefix == "" || relPrefix == "." {
		return storeDirName
	}
	return filepath.ToSlash(filepath.Join(relPrefix, storeDirName))
}

func isShallow(gitRoot string) (bool, error) {
	out, err := exec.Command("git", "-C", gitRoot, "rev-parse", "--is-shallow-repository").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// rawCommit は1件の commit の解析結果（git 呼び出しから切り離した純データ）。
type rawCommit struct {
	hash  string
	when  time.Time
	paths []string
}

// windowCommitsFormat の区切り文字。パスの先頭に来ることが無い制御文字を使い、
// commit 見出し行と path 行を取り違えない（decision-stale が使う「40桁 hex か」の
// 判定より頑丈——path が偶然 hex 40 桁に見える取り違えを構造的に無くす）。
const commitHeaderMark = "\x01"
const fieldSep = "\x1f"

// windowCommits は [since, until) の非マージ commit を、rootSpec 配下の変更に
// 限って返す（monorepo でストアのプロジェクトルート配下だけを数える・decision の
// 「粒度をストア全体に留める理由」節）。
//
// ⚠️ 窓の端は RFC3339（時刻とタイムゾーンまで明示）で渡す。日付だけを渡すと、
// git はそれを「その日の“いま”の時刻」に解決し、走らせた時刻で結果が変わる
// （実測済みの罠・count2.sh v1 の bug）。
//
// ⚠️ git の --since/--until は両端とも含む（実測済み: ちょうど --until の時刻の
// commit も出る）。半開区間 [since, until) にするため、--until へ渡す値は
// 「until より真に小さい、最大の秒」に切り詰める（commit の日時は秒精度なので）。
// これは `until を 1 秒引いてから RFC3339 整形（＝端数秒を切り捨て）`とは違う
// ——実測: Now() 由来の until（例 04:33:32.24）に「1 秒引いてから切り捨て」を
// 適用すると 04:33:31 になり、04:33:32 ちょうどの commit（実際には until より
// 前）まで巻き込んで落としてしまった（scholia init 直後に scholia activity を
// 打つ smoke test で実見。単体検査が使っていた「端数秒ゼロの until」では
// 再現しなかった）。正しくは「1 ナノ秒引いてから秒に切り捨てる」
// （floor(until − ε)）——端数が有る until（32.24）は 32 に切り詰まり
// （32 の commit は正しく含む）、端数が無い until（32.00 ちょうど）は 31 に
// 切り詰まる（32 ちょうどの commit を正しく除く）。表示用の Report.Until は
// 元の値のままで、ここでのズラしは git への問い合わせにだけ効く。
//
// ⚠️ --since は素の形だと「commit 日時が単調減少している」前提の早期打ち切りを
// 持つ——HEAD から遡る途中で --since より古い commit に当たった時点で、それより
// 前も全部古いはずだと判断して走査を止める。commit 日時が履歴の並びと一致しない
// （rebase・cherry-pick・時計のずれ等）と、窓の中にある commit を黙って見落とす。
// 実測: HEAD の日時だけを窓の外（過去）にずらすと、窓の中に2件あるのに0件になった
// （TestCompute_WindowBoundary_HalfOpen が red で実見・変異ではなく実データで踏んだ）。
// `--since-as-filter` は単調性を仮定しない単純な bool フィルタで、これを使う。
// `--until` 側にこの罠は無い（今日より新しい commit に当たっても、それより古い
// commit がまだ窓に入り得るため打ち切れない・実測: --until-as-filter という
// フラグ自体が無い＝git 側もこの罠が --since 側だけだと扱っている）。
func windowCommits(gitRoot, rootSpec string, since, until time.Time) ([]rawCommit, error) {
	untilFloor := until.Add(-time.Nanosecond).Truncate(time.Second)
	cmd := exec.Command("git", "-C", gitRoot, "log", "--no-merges",
		"--since-as-filter="+since.Format(time.RFC3339),
		"--until="+untilFloor.Format(time.RFC3339),
		"--format="+commitHeaderMark+"%H"+fieldSep+"%cI",
		"--name-only",
		"--", rootSpec)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseWindowCommits(string(out))
}

// parseWindowCommits は windowCommits の生出力を解釈する純関数（git を呼ばない・
// 入力と出力の対で検査できる）。
func parseWindowCommits(raw string) ([]rawCommit, error) {
	var out []rawCommit
	for _, block := range strings.Split(raw, commitHeaderMark) {
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		parts := strings.SplitN(lines[0], fieldSep, 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("git log の commit 見出しを解釈できません: %q", lines[0])
		}
		when, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			return nil, fmt.Errorf("commit 日時を解釈できません %q: %w", parts[1], err)
		}
		c := rawCommit{hash: parts[0], when: when}
		for _, ln := range lines[1:] {
			if ln == "" {
				continue
			}
			c.paths = append(c.paths, ln)
		}
		out = append(out, c)
	}
	return out, nil
}

// classify は各 commit を実装活動／記録のみに分ける純関数。
//
// 記録ディレクトリの外を1つ以上変更した commit だけが実装活動——記録と記録外を
// 同じ commit で触ったもの（mixed）も実装活動に数える（記録外の変更が実在する
// ので水増しにならない）。記録だけの commit は数えない（循環を避ける・実測で
// 混ぜると +25.6% 水増しされた）。
func classify(commits []rawCommit, storeSpec string, loc *time.Location) (implCommits, recordOnlyCommits, activeDays int) {
	prefix := storeSpec + "/"
	days := make(map[string]struct{})
	for _, c := range commits {
		if len(c.paths) == 0 {
			continue
		}
		isImpl := false
		for _, p := range c.paths {
			if !strings.HasPrefix(p, prefix) {
				isImpl = true
				break
			}
		}
		if isImpl {
			implCommits++
			days[c.when.In(loc).Format("2006-01-02")] = struct{}{}
		} else {
			recordOnlyCommits++
		}
	}
	return implCommits, recordOnlyCommits, len(days)
}

// lastActivity は「窓に依存しない」最後の実装活動——rootSpec 配下で、記録
// ディレクトリを除いた変更を持つ最新の非マージ commit。無ければ nil。
func lastActivity(gitRoot, rootSpec, storeSpec string) (*time.Time, error) {
	cmd := exec.Command("git", "-C", gitRoot, "log", "-1", "--no-merges",
		"--format=%cI", "--", rootSpec, ":(exclude)"+storeSpec)
	return firstLineAsTime(cmd)
}

// storeFirstSeen は記録ディレクトリ（現在名）を最初に変更した commit の日時。
// ストアが git 履歴に一度も現れていなければ nil（新規ストアでまだ commit
// されていない等）。
func storeFirstSeen(gitRoot, storeSpec string) (*time.Time, error) {
	cmd := exec.Command("git", "-C", gitRoot, "log", "--format=%cI", "--reverse", "--", storeSpec)
	return firstLineAsTime(cmd)
}

func firstLineAsTime(cmd *exec.Cmd) (*time.Time, error) {
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	line, _, _ := bytes.Cut(out, []byte("\n"))
	s := strings.TrimSpace(string(line))
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("commit 日時を解釈できません %q: %w", s, err)
	}
	return &t, nil
}
