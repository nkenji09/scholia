package usage

import (
	"sort"
	"testing"
)

// minLevel は「その項目を最初に記録する段」の**宣言**。
//
// ⚠️ これは Records の写しではなく、Records とは別に手で書いた期待値である。
// 片方だけを書き換えれば下の検査が落ちる。項目を足したらここにも足すこと
// （足し忘れると TestRecords_EveryFieldIsPlacedInTheTable が落ちる）。
var minLevel = map[Field]Level{
	FieldLevel:        Masked,
	FieldTimestamp:    Masked,
	FieldCommand:      Masked,
	FieldFlagNames:    Masked,
	FieldSelectorKind: Masked,
	FieldArgCount:     Masked,
	FieldExitCode:     Masked,
	FieldStdoutBytes:  Masked,
	FieldDurationUs:   Masked,
	FieldCaller:       Masked,
	FieldSessionID:    Masked,
	FieldToolVersion:  Masked,

	FieldRecordIDs:   Normal,
	FieldProjectRoot: Normal,

	FieldFlagValues:    Detailed,
	FieldFreeTextLens:  Detailed,
	FieldStderrBytes:   Detailed,
	FieldDurationParts: Detailed,
}

// TestRecords_EveryFieldIsPlacedInTheTable は、4 段 × 全項目の**対すべて**を検査する。
//
// 正本の歯止め 2「検査は 4 段 × 全項目の対で行う。表を通らない経路を作らない」。
// 段を足すときは AllLevels に、項目を足すときは AllFields と minLevel に載る。
func TestRecords_EveryFieldIsPlacedInTheTable(t *testing.T) {
	fields := AllFields()
	if len(fields) == 0 {
		t.Fatal("AllFields が空。項目が 1 つも無い")
	}
	for _, f := range fields {
		want, declared := minLevel[f]
		if !declared {
			t.Fatalf("項目 %q が minLevel に載っていない。"+
				"どの段から記録するかを宣言すること（CLAUDE.md 5: 新しく作った面にはガードを置き忘れる）", f.Key())
		}
		for _, l := range AllLevels() {
			got := Records(l, f)
			expect := l != Off && l >= want
			if got != expect {
				t.Errorf("Records(%s, %q) = %v, want %v", l, f.Key(), got, expect)
			}
		}
	}
	// 逆向き: 宣言に載っているのに実在しない項目が残っていたら、それも間違い。
	present := map[Field]bool{}
	for _, f := range fields {
		present[f] = true
	}
	for f := range minLevel {
		if !present[f] {
			t.Errorf("minLevel に載っている項目 %d は AllFields に無い", int(f))
		}
	}
}

// TestRecords_OffRecordsNothing はオフが全項目を記録しないこと。
// オフは「観測しない・書かない」段なので、例外は 1 つも無い。
func TestRecords_OffRecordsNothing(t *testing.T) {
	for _, f := range AllFields() {
		if Records(Off, f) {
			t.Errorf("Records(off, %q) = true。オフは何も記録しない", f.Key())
		}
	}
}

// TestRecords_LevelsAreNested は段が包含関係にあること（マスク ⊂ 通常 ⊂ 詳細）。
//
// 上の段で記録しなくなる項目があると、「詳細にしたのに落ちた」という読めない段差ができる。
func TestRecords_LevelsAreNested(t *testing.T) {
	levels := AllLevels()
	for _, f := range AllFields() {
		for i := 1; i < len(levels); i++ {
			lower, upper := levels[i-1], levels[i]
			if Records(lower, f) && !Records(upper, f) {
				t.Errorf("項目 %q が %s では記録され %s では記録されない（段は包含関係のはず）",
					f.Key(), lower, upper)
			}
		}
	}
}

// TestRecords_MaskedNeverRecordsAProjectNamingField は、マスクが
// 「プロジェクトが名付けたものを指しうる項目」を 1 つも記録しないこと。
//
// これは Records と NamesProject という**別々に書かれた 2 つの宣言**の突き合わせである。
// マスクの段の定義（この環境の外へ出してもプロジェクトの中身が復元できない）が、
// 項目の置き場所と食い違ったら落ちる。
func TestRecords_MaskedNeverRecordsAProjectNamingField(t *testing.T) {
	for _, f := range AllFields() {
		if Records(Masked, f) && f.NamesProject() {
			t.Errorf("マスクが %q を記録している。この項目はプロジェクトが名付けたものを指しうる", f.Key())
		}
	}
}

// TestFields_KeysAreUniqueAndNonEmpty は JSON キーの重複と欠落を見る。
// キーが重なると、行の中で後勝ちになって片方が消える。
func TestFields_KeysAreUniqueAndNonEmpty(t *testing.T) {
	seen := map[string]bool{}
	var keys []string
	for _, f := range AllFields() {
		k := f.Key()
		if k == "" {
			t.Errorf("項目 %d に JSON キーが無い", int(f))
			continue
		}
		if seen[k] {
			t.Errorf("JSON キー %q が重複している", k)
		}
		seen[k] = true
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) != len(AllFields()) {
		t.Errorf("キーの数 %d が項目の数 %d と合わない", len(keys), len(AllFields()))
	}
}

func TestParseLevel(t *testing.T) {
	cases := []struct {
		raw       string
		wantLevel Level
		wantOK    bool
	}{
		{"off", Off, true},
		{"masked", Masked, true},
		{"normal", Normal, true},
		{"detailed", Detailed, true},
		{"  Detailed  ", Detailed, true}, // 前後の空白と大小は畳む
		{"MASKED", Masked, true},
		{"", Off, false},         // 空文字はオフに倒す（ただし解釈できていない）
		{"verbose", Off, false},  // 未知の値もオフ
		{"1", Off, false},        // 真偽値のつもりの値もオフ
		{"detail", Off, false},   // 惜しい綴りも通さない
		{"masked ,", Off, false}, // 組み合わせは無い
	}
	for _, c := range cases {
		gotLevel, gotOK := ParseLevel(c.raw)
		if gotLevel != c.wantLevel || gotOK != c.wantOK {
			t.Errorf("ParseLevel(%q) = (%s, %v), want (%s, %v)",
				c.raw, gotLevel, gotOK, c.wantLevel, c.wantOK)
		}
	}
}

func TestLevelNames_CoverAllLevels(t *testing.T) {
	names := LevelNames()
	if len(names) != len(AllLevels()) {
		t.Fatalf("段の名前が %d 個、段が %d 個", len(names), len(AllLevels()))
	}
	for _, l := range AllLevels() {
		if _, ok := ParseLevel(l.String()); !ok {
			t.Errorf("段 %s の名前 %q を ParseLevel が解釈できない", l, l.String())
		}
	}
}
