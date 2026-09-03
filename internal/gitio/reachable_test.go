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
