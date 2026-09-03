// reachable.go — 「この commit は HEAD から辿れるか」を答える集合
// （decision 01M1JY0APWXHFZ1TKWST7VPS9N）。
//
// 記録されたハッシュは**短縮形のことがある**（保存の口が完全ハッシュへ寄せる
// ようになる前の記録が実データに残っている）。完全一致だけで引くと、短縮で
// 記録された結線が一律「辿れない」になる——**誤りの向きは偽陽性側**で、
// 記録は正しいのに finding が出る。だから前方一致で引ける形にして持つ。
package gitio

import (
	"sort"
	"strings"
)

// FullHashLen は git の完全ハッシュ（SHA-1）の文字数。
const FullHashLen = 40

// ReachableSet は HEAD から辿れる commit の集合（完全ハッシュ）。
// 短縮ハッシュは前方一致で引く。
type ReachableSet struct {
	full   map[string]struct{}
	sorted []string // 前方一致の二分探索用（昇順）
}

// NewReachableSet は完全ハッシュの並びから集合を作る（テストから値で組める）。
func NewReachableSet(hashes []string) ReachableSet {
	full := make(map[string]struct{}, len(hashes))
	sorted := make([]string, 0, len(hashes))
	for _, h := range hashes {
		h = strings.ToLower(h)
		if h == "" {
			continue
		}
		if _, dup := full[h]; dup {
			continue
		}
		full[h] = struct{}{}
		sorted = append(sorted, h)
	}
	sort.Strings(sorted)
	return ReachableSet{full: full, sorted: sorted}
}

// Contains は完全ハッシュまたは短縮ハッシュが集合の commit を指すかを返す。
func (s ReachableSet) Contains(hash string) bool {
	if hash == "" {
		return false
	}
	h := strings.ToLower(hash)
	if len(h) >= FullHashLen {
		_, ok := s.full[h]
		return ok
	}
	// 短縮ハッシュ: 昇順に並べた完全ハッシュ列で、h を接頭辞に持つ最初の要素を探す。
	i := sort.SearchStrings(s.sorted, h)
	return i < len(s.sorted) && strings.HasPrefix(s.sorted[i], h)
}

// ReachableFromHead は HEAD から辿れる commit の集合を git から導く。
//
// derived が false なら「検査しない」段——git が起動できない／git 管理下でない／
// commit が1件も無い／**浅い clone**。failure が非 nil なら「git 管理下で commit も
// あるのに導出そのものが落ちた」（01M0AJDYJSEVCSYEV0HDPSTWFZ の第4段）。
//
// ⚠️ **浅い clone で降りるのは、偽の finding を出さないためである。** rev-list は
// 浅い境界で切れるので、境界より古い祖先が「辿れない」に見える。
// その代わり、浅い clone では「異常なし」と「検査していない」が区別できない。
func ReachableFromHead(projectRoot string) (set ReachableSet, derived bool, failure error) {
	if !Installed() {
		return ReachableSet{}, false, nil
	}
	gitRoot, _, err := ResolveContext(projectRoot)
	if err != nil {
		return ReachableSet{}, false, nil
	}
	if has, err := HasAnyCommit(gitRoot); err != nil || !has {
		return ReachableSet{}, false, nil
	}
	if shallow, err := Run(gitRoot, "rev-parse", "--is-shallow-repository"); err == nil &&
		strings.TrimSpace(string(shallow)) == "true" {
		return ReachableSet{}, false, nil
	}
	out, err := Run(gitRoot, "rev-list", "HEAD")
	if err != nil {
		return ReachableSet{}, false, err
	}
	return NewReachableSet(strings.Fields(string(out))), true, nil
}
