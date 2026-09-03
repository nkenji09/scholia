package lint

import (
	"testing"

	"github.com/nkenji09/scholia/internal/gitio"

	"github.com/nkenji09/scholia/internal/model"
)

// ⚠️ 判断（出すか出さないか）は純関数 commitUnreachableFindings / ReachableSet に
// 切り出してあるので、**入力と出力の対**で検査する（CLAUDE.md「配線ガードの書き方」1）。
// git を起こしてソースを覗く検査より強い。

const (
	hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"
	hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2"
	hashC = "ccccccccccccccccccccccccccccccccccccccc3"
)

func decisionWith(id string, commits ...string) model.Decision {
	return model.Decision{
		ID:      id,
		Target:  model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.x"},
		Why:     "# 見出し\n\n本文",
		At:      "2026-09-03T00:00:00Z",
		Commits: commits,
	}
}

func withRefs(d model.Decision, refs ...string) model.Decision {
	d.Refs = refs
	return d
}

func TestCommitUnreachableFindings(t *testing.T) {
	reachable := gitio.NewReachableSet([]string{hashA})

	tests := []struct {
		name    string
		input   []model.Decision
		wantIDs []string
	}{
		{
			name:    "commits が空なら出さない",
			input:   []model.Decision{decisionWith("01D0")},
			wantIDs: nil,
		},
		{
			name:    "全部辿れるなら出さない",
			input:   []model.Decision{decisionWith("01D1", hashA)},
			wantIDs: nil,
		},
		{
			// 🔴 ここが規則の要。1件でも辿れれば出さないので、着地ハッシュを
			// 足した瞬間に finding が消える（acknowledges を積まずに済む）。
			name:    "一部だけ辿れないなら出さない",
			input:   []model.Decision{decisionWith("01D2", hashB, hashA)},
			wantIDs: nil,
		},
		{
			name:    "1つも辿れないなら出す",
			input:   []model.Decision{decisionWith("01D3", hashB, hashC)},
			wantIDs: []string{"01D3"},
		},
		{
			// 🔴 refs を持つなら黙る（01M1K4WPN3HXNVE0CR0T6NQK33）。commits は
			// 追記専用で消せないので、ここで黙らないと refs へ移した decision が
			// 永久に鳴り続ける。
			name:    "refs があるなら、commits が全部辿れなくても出さない",
			input:   []model.Decision{withRefs(decisionWith("01D6", hashB, hashC), "https://example.test/pull/1")},
			wantIDs: nil,
		},
		{
			name:    "refs が空文字だけでも、要素があるなら出さない",
			input:   []model.Decision{withRefs(decisionWith("01D7", hashB), "#123")},
			wantIDs: nil,
		},
		{
			name: "decision id の昇順で返す",
			input: []model.Decision{
				decisionWith("01D9", hashB),
				decisionWith("01D4", hashC),
			},
			wantIDs: []string{"01D4", "01D9"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commitUnreachableFindings(tt.input, reachable)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("finding 件数 = %d, want %d（%+v）", len(got), len(tt.wantIDs), got)
			}
			for i, want := range tt.wantIDs {
				if got[i].Target != want {
					t.Errorf("finding[%d].Target = %q, want %q", i, got[i].Target, want)
				}
				if got[i].Rule != RuleCommitUnreachable {
					t.Errorf("finding[%d].Rule = %q, want %q", i, got[i].Rule, RuleCommitUnreachable)
				}
				if got[i].Severity != SeverityInfo || got[i].Tier != TierAdvisory {
					t.Errorf("finding[%d] severity/tier = %v/%v, want info/advisory", i, got[i].Severity, got[i].Tier)
				}
				// AcknowledgeOnly を立てない＝「容認でしか解けない印」ではない。
				// 着地ハッシュを足せば消えるので、是正可能な finding として扱う。
				if got[i].AcknowledgeOnly {
					t.Errorf("finding[%d].AcknowledgeOnly = true, want false（relink で消えるため）", i)
				}
			}
		})
	}
}

// 空集合（辿れる commit が1つも無い）でも落ちないこと。
func TestReachableSetEmpty(t *testing.T) {
	set := gitio.NewReachableSet(nil)
	if set.Contains(hashA) {
		t.Error("空集合が hashA を含むと答えた")
	}
	got := commitUnreachableFindings([]model.Decision{decisionWith("01D5", hashA)}, set)
	if len(got) != 1 {
		t.Fatalf("finding 件数 = %d, want 1", len(got))
	}
}
