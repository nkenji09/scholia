package cli

import (
	"strings"
	"testing"

	skills "github.com/nkenji09/scholia/agents/skills"
)

// 名前解決は純関数（resolveSkillDocs）に切り出してあるので、画面や stdout を
// 経由せず入力と出力の対で検査する。
func TestResolveSkillDocs_ShortNameBasenameAndEmbedPath(t *testing.T) {
	docs, err := skillDocs()
	if err != nil {
		t.Fatalf("skillDocs: %v", err)
	}

	const modeling = "_scholia-shared/references/modeling-principles.md"

	cases := []struct {
		name     string
		query    string
		wantHits int
		wantPath string // wantHits==1 のときだけ検査
	}{
		{"短縮名", "modeling-principles", 1, modeling},
		{"拡張子つき basename", "modeling-principles.md", 1, modeling},
		{"embed 相対パス", modeling, 1, modeling},
		{"前後の空白を無視する", "  modeling-principles  ", 1, modeling},
		{"先頭スラッシュを無視する", "/" + modeling, 1, modeling},
		{"もう一方の共有リファレンス", "evaluating-changes", 1, "_scholia-shared/references/evaluating-changes.md"},
		{"パスなら SKILL.md も一意", "scholia-change/SKILL.md", 1, "scholia-change/SKILL.md"},
		{"一致なし", "no-such-doc", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := resolveSkillDocs(docs, tc.query)
			if len(hits) != tc.wantHits {
				t.Fatalf("query %q: hits=%d want %d（%v）", tc.query, len(hits), tc.wantHits, hits)
			}
			if tc.wantHits == 1 && hits[0].Path != tc.wantPath {
				t.Fatalf("query %q: path=%q want %q", tc.query, hits[0].Path, tc.wantPath)
			}
		})
	}
}

// SKILL.md は各スキルに 1 つあるので短縮名 "SKILL" は必ず衝突する。
// 片方を黙って選ばないことが決定事項なので、複数一致が複数のまま返ることを検査する。
func TestResolveSkillDocs_ShortNameCollisionStaysAmbiguous(t *testing.T) {
	docs, err := skillDocs()
	if err != nil {
		t.Fatalf("skillDocs: %v", err)
	}
	hits := resolveSkillDocs(docs, "SKILL")
	if len(hits) < 2 {
		t.Fatalf("SKILL の一致が %d 件しかない（衝突が衝突として返っていない）: %v", len(hits), hits)
	}
}

// show は全文を出す（節を切り出さない・装飾を足さない）。embed のバイト列と
// 完全一致で検査するので、節だけ返す実装や見出しを付け足す実装に変異すれば落ちる。
func TestSkillsShow_WritesWholeFileVerbatim(t *testing.T) {
	want, err := skills.FS.ReadFile("_scholia-shared/references/modeling-principles.md")
	if err != nil {
		t.Fatalf("embed 読み出し: %v", err)
	}

	out, err := run(t, t.TempDir(), "skills", "show", "modeling-principles")
	if err != nil {
		t.Fatalf("skills show が失敗した: %v\noutput:\n%s", err, out)
	}
	if out != string(want) {
		t.Fatalf("stdout が embed の全文と一致しない（len=%d want=%d）", len(out), len(want))
	}
}

// 解決できない名前は片方を選ばず落ちる。候補が出ないと次の一手が分からないので
// 候補一覧まで検査する。
func TestSkillsShow_UnresolvedFailsWithCandidates(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"一致なし", "no-such-doc"},
		{"短縮名が複数に一致", "SKILL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := run(t, t.TempDir(), "skills", "show", tc.query)
			if err == nil {
				t.Fatalf("query %q: エラーにならなかった\noutput:\n%s", tc.query, out)
			}
			if !strings.Contains(err.Error(), "SKILL.md") && !strings.Contains(err.Error(), "modeling-principles") {
				t.Fatalf("query %q: 候補一覧が出ていない: %v", tc.query, err)
			}
		})
	}
}

// ls が共有リファレンスを名前で示す。ここが出なければ show は打たれない。
func TestSkillsLs_ListsSharedReferences(t *testing.T) {
	out, err := run(t, t.TempDir(), "skills", "ls")
	if err != nil {
		t.Fatalf("skills ls が失敗した: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{
		"_scholia-shared/references/modeling-principles.md",
		"_scholia-shared/references/evaluating-changes.md",
		"modeling-principles",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("ls の出力に %q が無い:\n%s", want, out)
		}
	}
}
