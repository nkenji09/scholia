package cli

import (
	"testing"

	"github.com/nkenji09/scholia/internal/gitio"
	"github.com/nkenji09/scholia/internal/model"
)

// ⚠️ 判断（どの decision を対象にするか・候補を1つに定めるか）は純関数
// relinkTargets / resolveRelink に切り出してあるので、**入力と出力の対**で検査する
// （CLAUDE.md「配線ガードの書き方」1）。git を起こす検査より強い。

const (
	relinkHashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"
	relinkHashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2"
	relinkHashC = "ccccccccccccccccccccccccccccccccccccccc3"
)

func relinkDecision(id string, commits ...string) model.Decision {
	return model.Decision{
		ID:      id,
		Target:  model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.x"},
		Why:     "# 見出し\n\n本文",
		At:      "2026-09-03T00:00:00Z",
		Commits: commits,
	}
}

func TestRelinkTargets(t *testing.T) {
	reachable := gitio.NewReachableSet([]string{relinkHashA})

	tests := []struct {
		name     string
		input    []model.Decision
		wantIDs  []string
		wantHash []string // 先頭 target の対象ハッシュ
	}{
		{
			name:    "commits が空なら対象にしない",
			input:   []model.Decision{relinkDecision("01D0")},
			wantIDs: nil,
		},
		{
			name:    "全部辿れるなら対象にしない",
			input:   []model.Decision{relinkDecision("01D1", relinkHashA)},
			wantIDs: nil,
		},
		{
			// 🔴 lint の commit-unreachable と同じ「1つも辿れない」でなければ、
			// 気づかせる範囲と直す範囲がずれる。
			name:    "一部だけ辿れないなら対象にしない",
			input:   []model.Decision{relinkDecision("01D2", relinkHashB, relinkHashA)},
			wantIDs: nil,
		},
		{
			name:     "1つも辿れないなら対象にし、辿れないハッシュを全部並べる",
			input:    []model.Decision{relinkDecision("01D3", relinkHashB, relinkHashC)},
			wantIDs:  []string{"01D3"},
			wantHash: []string{relinkHashB, relinkHashC},
		},
		{
			name: "decision id の昇順で返す",
			input: []model.Decision{
				relinkDecision("01D9", relinkHashB),
				relinkDecision("01D4", relinkHashC),
			},
			wantIDs:  []string{"01D4", "01D9"},
			wantHash: []string{relinkHashC},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relinkTargets(tt.input, reachable)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("target 件数 = %d, want %d（%+v）", len(got), len(tt.wantIDs), got)
			}
			for i, want := range tt.wantIDs {
				if got[i].DecisionID != want {
					t.Errorf("target[%d].DecisionID = %q, want %q", i, got[i].DecisionID, want)
				}
			}
			if tt.wantHash == nil {
				return
			}
			if len(got[0].Hashes) != len(tt.wantHash) {
				t.Fatalf("target[0] のハッシュ数 = %d, want %d", len(got[0].Hashes), len(tt.wantHash))
			}
			for i, want := range tt.wantHash {
				if got[0].Hashes[i].Hash != want {
					t.Errorf("target[0].Hashes[%d] = %q, want %q", i, got[0].Hashes[i].Hash, want)
				}
			}
		})
	}
}

func TestResolveRelink(t *testing.T) {
	tests := []struct {
		name         string
		input        relinkHash
		wantResolved string
		wantReason   bool // 定まらなかった理由が入っていること
	}{
		{
			name:       "元の commit が手元に無ければ定めない",
			input:      relinkHash{Hash: relinkHashB},
			wantReason: true,
		},
		{
			name:       "同題の commit が無ければ定めない",
			input:      relinkHash{Hash: relinkHashB, Subject: "feat: x"},
			wantReason: true,
		},
		{
			// 見出しが偶然一致した別 commit を結ぶ事故は取り消せない（追記専用）。
			name:       "同題が複数あれば定めない",
			input:      relinkHash{Hash: relinkHashB, Subject: "feat: x", Matches: []string{relinkHashA, relinkHashC}},
			wantReason: true,
		},
		{
			name:         "同題が1件なら定める",
			input:        relinkHash{Hash: relinkHashB, Subject: "feat: x", Matches: []string{relinkHashA}},
			wantResolved: relinkHashA,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, reason := resolveRelink(tt.input)
			if resolved != tt.wantResolved {
				t.Errorf("resolved = %q, want %q", resolved, tt.wantResolved)
			}
			if tt.wantReason && reason == "" {
				t.Error("定まらなかったのに理由が空（黙って諦めている）")
			}
			if !tt.wantReason && reason != "" {
				t.Errorf("定まったのに理由が入っている: %q", reason)
			}
		})
	}
}
