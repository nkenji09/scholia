package gitio

import (
	"strings"
	"testing"
)

const (
	hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"
	hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2"
	hashC = "ccccccccccccccccccccccccccccccccccccccc3"
)

// 短縮ハッシュで記録された結線を「辿れない」と誤判定しないこと。保存の口が
// 完全ハッシュへ寄せるようになる前の記録が実データに残っている。
func TestReachableSetContains(t *testing.T) {
	set := NewReachableSet([]string{hashA, hashB})

	tests := []struct {
		hash string
		want bool
	}{
		{hashA, true},
		{hashB, true},
		{hashC, false},
		{"aaaaaaa", true},  // 短縮（前方一致）
		{"AAAAAAA", true},  // 大文字（git は小文字で出すが記録側は分からない）
		{"ccccccc", false}, // 短縮だが集合に無い
		{"", false},        // 空
		{strings.Repeat("a", FullHashLen), false}, // 完全長だが別の commit
	}

	for _, tt := range tests {
		if got := set.Contains(tt.hash); got != tt.want {
			t.Errorf("Contains(%q) = %v, want %v", tt.hash, got, tt.want)
		}
	}
}

// 大文字で記録されたハッシュも引けること（集合側の正規化）。
func TestReachableSetNormalizesCase(t *testing.T) {
	set := NewReachableSet([]string{strings.ToUpper(hashA)})
	if !set.Contains(hashA) {
		t.Error("大文字で作った集合が小文字の完全ハッシュを引けない")
	}
}

func TestReachableSetEmpty(t *testing.T) {
	if NewReachableSet(nil).Contains(hashA) {
		t.Error("空集合が hashA を含むと答えた")
	}
}

// トレーラを運ぶ見出しを、NUL の構造を壊さずに読めること
// （01M1N02SRH9BAMT82B7GMTGQJH）。
//
// ⚠️ ここは**値で検査する**——実際の git 出力の形（番兵＋NUL 区切り）をそのまま
// 組んで、返る Commit を突き合わせる。git を起こす検査より安く、同じ穴を見る。
func TestParseNameStatusZCarriesTrailers(t *testing.T) {
	// <mark>hash<tmark>id1<sep>id2 \0 "\nM" \0 path \0
	out := CommitMark + "abc123" + TrailerMark + "01AAA\x1f01BBB" + "\x00" +
		"\nM" + "\x00" + ".scholia/tags/req.x.json" + "\x00" +
		CommitMark + "def456" + TrailerMark + "" + "\x00" +
		"\nM" + "\x00" + ".scholia/vocab/cond.y.json" + "\x00"

	commits, err := ParseNameStatusZ([]byte(out))
	if err != nil {
		t.Fatalf("読めません: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("commit 数 = %d, want 2", len(commits))
	}
	if commits[0].Hash != "abc123" {
		t.Errorf("hash = %q, want abc123（トレーラが hash に混ざっている）", commits[0].Hash)
	}
	if len(commits[0].Trailers) != 2 || commits[0].Trailers[0] != "01AAA" || commits[0].Trailers[1] != "01BBB" {
		t.Errorf("trailers = %v, want [01AAA 01BBB]", commits[0].Trailers)
	}
	// 🔴 パスの読み取りが壊れていないこと（見出しを広げた副作用を捕まえる）。
	if len(commits[0].Changes) != 1 || commits[0].Changes[0].Path != ".scholia/tags/req.x.json" {
		t.Errorf("changes = %+v（見出しを広げてパスの位置がずれた）", commits[0].Changes)
	}
	if len(commits[1].Trailers) != 0 {
		t.Errorf("トレーラ無しが %v になった（空文字を1件と数えている）", commits[1].Trailers)
	}
}

// 番兵が無い（トレーラを要求していない）出力も、これまでどおり読めること。
func TestParseNameStatusZWithoutTrailerMark(t *testing.T) {
	out := CommitMark + "abc123" + "\x00" + "\nM" + "\x00" + ".scholia/tags/req.x.json" + "\x00"
	commits, err := ParseNameStatusZ([]byte(out))
	if err != nil {
		t.Fatalf("読めません: %v", err)
	}
	if len(commits) != 1 || commits[0].Hash != "abc123" || len(commits[0].Trailers) != 0 {
		t.Fatalf("後方互換が壊れている: %+v", commits)
	}
}
