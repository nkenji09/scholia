// tag_list_bytes_test.go — `tag list` の各面が渡すバイト列の歯止め
// （01KZ5ACN6P279S96D5M3AHY9HZ）。
//
// # このファイルの歯止めが落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:**
//   - `--json --all` のバイト列が、この変更の前と 1 バイトでも違う
//     （平坦・入れ子〔`--tree`〕・絞り込み〔`--kind`〕の 3 つとも）
//   - テキストの面（素の一覧 / `--tree` / `--kind`）のバイト列が変わる
//   - 既定の `--json` が「`--all` の出力から description の行だけを抜いたもの」と
//     1 バイトでも違う（＝ description 以外が 1 つでも消えた・変わった・
//     description が 1 件でも残っている・整形が変わった、のいずれか）。
//     **入れ子の面にも同じ期待値を当てる。**
//   - `--json` の 4 つの面（平坦・入れ子・絞り込み・入れ子＋絞り込み）のどれかで、
//     既定と `--all` が description 以外で違う
//   - 上のどれかが**件数によって変わる**（0 件から、実在する最大の記録集合より
//     十分上の件数まで見る）
//
// **落ちない（射程の外・正直に名乗る）:**
//   - repo の外の消費者。ここで見ているのはこの repo が出すバイト列だけ。
//   - golden の標本に現れない model.Tag のフィールド。フィールドの網羅は
//     tag_list_json_test.go が reflect で見る（標本が全フィールドを埋めていない
//     ことを、そちらが赤で知らせる）。
//   - `--json` を付けない面での `--all` の効き目（何も畳んでいないので no-op）。
//     それは TestTagListTextFacesIgnoreAll が別に見る。
//
// # golden の出自
//
// golden は**この変更を入れる前のコード**の出力そのものである。採り直しは
//
//	SCHOLIA_GOLDEN_UPDATE=1 go test ./internal/cli -run TestTagListBytes
//
// で、`capture` 側の引数（`--all` を含まない＝変更前にも通る形）を走らせて記録する。
// `--json` の整形を変える単位が将来来たら、この golden は正しく赤くなる——
// そのときは、その単位が採り直して「何が変わったか」を差分で示すこと。
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

const goldenUpdateEnv = "SCHOLIA_GOLDEN_UPDATE"

// seedTagListFixture は `tag list` の標本を作る。
//
// ⚠️ **本番の `.scholia` は必ず vocab・tags・transitions・decisions の 4 カテゴリを持つ。**
// tags だけの標本は「他のカテゴリが在るときにだけ効く」変異を素通りさせる
// （実測で、本番 100% 発火の変異が tags だけの標本を通り抜けた）。
//
// タグ側は model.Tag の各フィールドを一通り踏む——description の有無・多親・
// color・ref・total・fulfillment。踏んでいないフィールドがあると、
// 「description 以外は 1 つも変わらない」の検査がそのフィールドを見ないまま緑になる。
func seedTagListFixture(t *testing.T, dir string) {
	t.Helper()
	must := func(args ...string) {
		t.Helper()
		if out, err := run(t, dir, args...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", args, err, out)
		}
	}
	must("init")
	must("config", "set", "tagKinds", "requirement,concern,subject,axis")

	must("vocab", "add", "condition", "cond.valid", "--label", "前提が成り立つ")
	must("vocab", "add", "action", "act.submit", "--label", "送信する", "--kind", "user")
	must("vocab", "add", "effect", "eff.token", "--label", "トークンを発行する", "--kind", "state")

	must("tag", "create", "subject.core", "--name", "中核", "--kind", "subject",
		"--desc", "説明を持つ親タグ。改行を含む。\n2 行目。")
	must("tag", "create", "subject.side", "--name", "傍系", "--kind", "subject") // description なし
	must("tag", "create", "req.a", "--name", "要件A", "--kind", "requirement",
		"--parent", "subject.core",
		"--desc", "引用符 \" と < > & と ASCII mixed 日本語 を含む説明。")
	must("tag", "create", "req.b", "--name", "要件B", "--kind", "requirement",
		"--parent", "subject.core", "--parent", "subject.side",
		"--desc", "多親のタグの説明。")
	must("tag", "create", "concern.x", "--name", "関心X", "--kind", "concern",
		"--color", "#3b82f6", "--ref", "https://example.invalid/x", "--desc", "色と参照を持つ。")
	must("tag", "create", "axis.k", "--name", "軸K", "--kind", "axis", "--total", "--desc", "軸の説明。")
	must("tag", "edit", "req.a", "--fulfillment", "property")

	must("tx", "add", "T-a", "--action", "act.submit", "--given", "cond.valid", "--then", "eff.token",
		"--tags", "req.a,subject.core")
	must("tx", "add", "T-b", "--action", "act.submit", "--then", "eff.token")

	must("decide", "--on", "tag:req.a", "--why", "# 標本用の見出し\n\n標本を 4 カテゴリにするための本文。")
	must("decide", "--on", "transition:T-a", "--why", "# 標本用の見出し 2\n\n同上。")
}

// tagListGoldenCases は「変更前に採る引数」と「変更後に突き合わせる引数」の対。
//
// capture に `--all` を書かないのは、**変更前のコードでも走る形**にしておくため。
// json_full / kind_json_full の対がそのまま、正本の
// 「`--all` を付けたときの出力は現行と 1 バイトも変わらない」である。
var tagListGoldenCases = []struct {
	name    string
	capture []string
	check   []string
}{
	{"plain", []string{"tag", "list"}, []string{"tag", "list"}},
	{"tree", []string{"tag", "list", "--tree"}, []string{"tag", "list", "--tree"}},
	{"kind", []string{"tag", "list", "--kind", "requirement"}, []string{"tag", "list", "--kind", "requirement"}},
	{"json_full", []string{"tag", "list", "--json"}, []string{"tag", "list", "--json", "--all"}},
	{"tree_json_full",
		[]string{"tag", "list", "--tree", "--json"},
		[]string{"tag", "list", "--tree", "--json", "--all"}},
	{"kind_json_full",
		[]string{"tag", "list", "--kind", "requirement", "--json"},
		[]string{"tag", "list", "--kind", "requirement", "--json", "--all"}},
	{"tree_kind_json_full",
		[]string{"tag", "list", "--tree", "--kind", "requirement", "--json"},
		[]string{"tag", "list", "--tree", "--kind", "requirement", "--json", "--all"}},
}

// jsonFaces は `--json` の面（平坦・入れ子・絞り込み）。**この列挙は
// 「面ごとに検査を書く」ためのものではない**——判断は面が分かれる前に 1 度だけ
// 通っているので、ここは「その 1 つの判断がどの面にも届いているか」の確認である。
var jsonFaces = []struct {
	name   string
	args   []string
	golden string // 変更前のバイト列を記録した golden（空なら byte 比較はしない）
}{
	{"平坦", []string{"tag", "list", "--json"}, "json_full"},
	{"入れ子", []string{"tag", "list", "--tree", "--json"}, "tree_json_full"},
	{"絞り込み", []string{"tag", "list", "--kind", "requirement", "--json"}, "kind_json_full"},
	{"入れ子＋絞り込み", []string{"tag", "list", "--tree", "--kind", "requirement", "--json"}, "tree_kind_json_full"},
}

func tagListGoldenPath(name string) string {
	return filepath.Join("testdata", "tag_list", name+".golden")
}

func readTagListGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(tagListGoldenPath(name))
	if err != nil {
		t.Fatalf("golden %s が読めない（採り直しは %s=1 go test ./internal/cli -run TestTagListBytes）: %v",
			name, goldenUpdateEnv, err)
	}
	return string(b)
}

// TestTagListBytes は各面のバイト列を golden と突き合わせる。
func TestTagListBytes(t *testing.T) {
	dir := t.TempDir()
	seedTagListFixture(t, dir)

	if os.Getenv(goldenUpdateEnv) != "" {
		if err := os.MkdirAll(filepath.Join("testdata", "tag_list"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, tc := range tagListGoldenCases {
			out := mustRun(t, dir, tc.capture...)
			if err := os.WriteFile(tagListGoldenPath(tc.name), []byte(out), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("golden %s を %v から採り直した（%d バイト）", tc.name, tc.capture, len(out))
		}
		t.Skip("golden を採り直したので照合はしない")
	}

	for _, tc := range tagListGoldenCases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustRun(t, dir, tc.check...)
			want := readTagListGolden(t, tc.name)
			if got != want {
				t.Errorf("%v の出力が golden(%s) と違う\n--- got (%d バイト) ---\n%s\n--- want (%d バイト) ---\n%s",
					tc.check, tc.name, len(got), got, len(want), want)
			}
		})
	}

	// 既定の `--json` は「`--all` の出力から description の行だけを抜いたもの」と
	// バイト単位で一致する。正本の 2 つの条件——「description が消える」と
	// 「他のフィールドは 1 つも変わらない」——を、そのままバイト列の期待値にしている。
	// **入れ子（`--tree --json`）にも同じ期待値を当てる。**
	for _, face := range jsonFaces {
		if face.golden == "" {
			continue
		}
		t.Run("既定は description の行だけが抜けた形/"+face.name, func(t *testing.T) {
			full := readTagListGolden(t, face.golden)
			want := withoutDescriptionLines(full)
			if want == full {
				t.Fatalf("golden %s に description の行が 1 つも無い（標本が壊れている）", face.golden)
			}
			got := mustRun(t, dir, face.args...)
			if got != want {
				t.Errorf("%v の出力が「description の行だけを抜いた golden」と違う\n--- got ---\n%s\n--- want ---\n%s",
					face.args, got, want)
			}
			// 抜いた側がなお JSON として読めること（末尾カンマの処理漏れを落とす）。
			var decoded any
			if err := json.Unmarshal([]byte(got), &decoded); err != nil {
				t.Fatalf("既定の出力が JSON として読めない: %v\n%s", err, got)
			}
		})
	}
}

// withoutDescriptionLines は golden から description の行だけを取り除く。
//
// json.Encoder + SetIndent が書く文字列フィールドは必ず 1 行に収まる（改行は
// \n へ escape される）ので、行単位で抜ける。description がその object の
// 最後のキーだったときだけ、直前の行の末尾カンマも落とす。
func withoutDescriptionLines(golden string) string {
	lines := strings.Split(golden, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if !strings.HasPrefix(strings.TrimSpace(ln), `"description": `) {
			out = append(out, ln)
			continue
		}
		if !strings.HasSuffix(ln, ",") && len(out) > 0 {
			out[len(out)-1] = strings.TrimSuffix(out[len(out)-1], ",")
		}
	}
	return strings.Join(out, "\n")
}

// TestTagListJSONDefaultDropsOnlyDescription は、既定と `--all` の出力を
// **値として**突き合わせる。golden はバイト列を固定するので整形を変える単位が
// 来ると赤くなるが、こちらは整形に依らず「何が渡っているか」を見る。
//
// ⚠️ 件数を変えて同じことを見る。件数で分岐する変異は、標本の上端が本番より
// 下にあると素通りする（実測で「301 件超で退行」する変異が上端 200 の標本を
// 通り抜けた）。ここでは実在する最大の記録集合より十分上まで見る。
func TestTagListJSONDefaultDropsOnlyDescription(t *testing.T) {
	for _, extra := range []int{0, 1, 2, 300, 1200} {
		t.Run(fmt.Sprintf("追加 %d 件", extra), func(t *testing.T) {
			dir := t.TempDir()
			seedTagListFixture(t, dir)
			seedBulkTags(t, dir, extra)

			for _, face := range jsonFaces {
				t.Run(face.name, func(t *testing.T) {
					def := collectTagObjects(t, mustRun(t, dir, face.args...))
					all := collectTagObjects(t, mustRun(t, dir, append(append([]string{}, face.args...), "--all")...))

					if len(def) != len(all) {
						t.Fatalf("件数が違う: 既定 %d 件 / --all %d 件", len(def), len(all))
					}
					if len(all) == 0 {
						t.Fatal("標本が空（この検査は何も見ていない）")
					}
					describedInAll := 0
					for i := range all {
						if _, ok := def[i]["description"]; ok {
							t.Errorf("既定の %d 件目に description が残っている: %v", i, def[i]["id"])
						}
						if _, ok := all[i]["description"]; ok {
							describedInAll++
							delete(all[i], "description")
						}
						// description を除いた残りが、キーも値も並びも完全に一致する。
						if !reflect.DeepEqual(def[i], all[i]) {
							t.Errorf("%d 件目が description 以外で違う\n既定: %v\n--all: %v", i, def[i], all[i])
						}
					}
					if describedInAll == 0 {
						t.Fatal("--all 側に description を持つタグが 1 件も無い（標本が壊れている）")
					}
				})
			}
		})
	}
}

// TestTagListTextFacesIgnoreAll は、**テキストの**面が `--all` の有無で
// 変わらないことを見る。これらは元から description を出していない
// （正本「1 バイトも変わらない」）。
//
// ⚠️ `--tree --json` はここに**居ない**。入れ子も `--json` の面なので、
// `--all` で description が戻る側である（TestTagListJSONDefaultDropsOnlyDescription）。
func TestTagListTextFacesIgnoreAll(t *testing.T) {
	dir := t.TempDir()
	seedTagListFixture(t, dir)

	for _, args := range [][]string{
		{"tag", "list"},
		{"tag", "list", "--tree"},
		{"tag", "list", "--kind", "requirement"},
		{"tag", "list", "--tree", "--kind", "requirement"},
	} {
		t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
			plain := mustRun(t, dir, args...)
			withAll := mustRun(t, dir, append(append([]string{}, args...), "--all")...)
			if plain != withAll {
				t.Errorf("%v が --all の有無で変わった\n--- なし ---\n%s\n--- あり ---\n%s", args, plain, withAll)
			}
		})
	}
}

// collectTagObjects は `--json` の出力からタグの object を**文書順に**取り出す。
// 平坦（タグの配列）と入れ子（`{tag, children}` の森）のどちらの形でも同じ列を返すので、
// 面ごとに別の検査を書かなくて済む。
func collectTagObjects(t *testing.T, out string) []map[string]any {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("JSON として読めない: %v\n%s", err, out)
	}
	return walkTagObjects(decoded)
}

func walkTagObjects(v any) []map[string]any {
	switch x := v.(type) {
	case []any:
		var out []map[string]any
		for _, e := range x {
			out = append(out, walkTagObjects(e)...)
		}
		return out
	case map[string]any:
		if tag, ok := x["tag"]; ok { // 入れ子のノード
			out := walkTagObjects(tag)
			return append(out, walkTagObjects(x["children"])...)
		}
		return []map[string]any{x} // タグそのもの
	}
	return nil
}

// seedBulkTags は tags ディレクトリへ直接 n 件書く。CLI 経由だと 1 件ごとに
// store 全体を読み書きするので、件数を上げた標本が作れない。
func seedBulkTags(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("req.bulk-%05d", i)
		tag := model.Tag{
			ID:          id,
			Name:        fmt.Sprintf("一括 %05d", i),
			Kind:        "requirement",
			Description: strings.Repeat("この説明は既定の出力から落ちる。", 8),
		}
		b, err := json.MarshalIndent(tag, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".scholia", "tags", id+".json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
