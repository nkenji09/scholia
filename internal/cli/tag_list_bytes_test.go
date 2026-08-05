// tag_list_bytes_test.go — `tag list` の各面が渡すバイト列の歯止め
// （01KZ5ACN6P279S96D5M3AHY9HZ）。
//
// # このファイルの歯止めが落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:**
//   - `--json --all` のバイト列が golden と 1 バイトでも違う
//     （平坦・入れ子〔`--tree`〕・絞り込み〔`--kind`〕・入れ子＋絞り込み の 4 つとも）。
//     ⚠️ **golden は「この変更の前」の出力ではなくなった**——
//     01KZ7V637RNMPXJMVACYV6V1AS で採り直してある（下の「golden の出自」）
//   - テキストの面（素の一覧 / `--tree` / `--kind` / `--tree --kind`）のバイト列が変わる
//   - 既定の `--json` が「`--all` の出力から description の欄だけを抜いたもの」と
//     1 バイトでも違う（＝ description 以外が 1 つでも消えた・変わった・
//     description が 1 件でも残っている・整形が変わった、のいずれか）
//   - **`tag list` に足された「新しい面」が description を畳んでいない**——
//     面をここで列挙せず、bool フラグを cobra から数え上げてその全部分集合を回す
//     （TestTagListEveryDiscoveredFaceFolds）。**新しい bool フラグが足されれば、
//     その面も自動で回る。** 判断を通さずに store を開き直す面でも落ちる。
//   - 上のどれかが**件数によって変わる**（0 件から、実在が確認できている最大の
//     タグ集合〔257 件〕の 4.7 倍＝1200 件まで見る。本 repo は 83 件）
//
// **落ちない（射程の外・正直に名乗る）:**
//   - repo の外の消費者。ここで見ているのはこの repo が出すバイト列だけ。
//   - golden の標本に現れない model.Tag のフィールド。フィールドの網羅は
//     tag_list_fold_test.go が reflect で見る（標本が全フィールドを埋めていない
//     ことを、そちらが赤で知らせる）。
//   - 🔴 **bool フラグ以外で表現される面**（位置引数・文字列フラグの値で分岐する面）
//     **が、`loadTagListTags` を使わず自分で store を開き直した場合。**
//     cobra から数え上げられるのは bool フラグまでで、そこは届かない。
//     ⚠️ **実際に変異を書いて緑を実見してある**（M-R）。この 1 件は名乗るだけで塞いでいない。
//     なお `loadTagListTags` を使う限りは、位置引数の面でも畳んだ値しか受け取れない。
//   - `--json` を付けない面が description を**テキストとして**出すこと。
//     いまは畳んだ値しか届かないので出しようがない（出しても空文字になる）が、
//     「空文字が出る」こと自体を落とす検査は置いていない。
//
// # golden の出自と、採り直しの手順
//
// 初出の golden は 01KZ5ACN6P279S96D5M3AHY9HZ を**入れる前のコード**の出力だった。
// 採り直しは
//
//	SCHOLIA_GOLDEN_UPDATE=1 go test ./internal/cli -run 'TestTagListBytes$'
//
// で、`tagListGoldenCases.args` を走らせて記録する。**外部のスクリプトは要らない。**
//
// 🔴 **採り直しは、同時に「変わったのは空白だけか」を機械で見る**（updateTagListGoldens）。
// 手順書に「確かめること」と書くだけでは飛ばせてしまい、飛ばせば**欄が消えても
// 気づけないまま golden が正典になる。** 面ごとに次を出す:
//
//   - JSON の面: 旧と新をトークン列で比べ、**キー順・重複キー・数値リテラルの綴りまで
//     保って**「空白の置き方だけが変わった」かを判定する（jsonSameIgnoringWhitespace）。
//     ⚠️ **`jq -S .` は使わない**——`-S` がキー順を正規化するので、順序が変わっても
//     一致してしまう。
//   - テキストの面: JSON ではないので**生バイト**で比べる。
//
// 値が変わるのが正しい変更のときだけ `SCHOLIA_GOLDEN_ALLOW_VALUE_CHANGE=1` を併せて立てる
// （立てた事実がログに残る）。
//
// ⚠️ **`01KZ7V637RNMPXJMVACYV6V1AS`（`--json` の整形をやめる・全 52 サブコマンド）で
// 採り直した。** `--json` の 4 本は**空白だけが変わり、欄は 1 つも変わっていない**。
// テキストの 3 本は 1 バイトも変わっていない。
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	pflag "github.com/spf13/pflag"

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

// tagListGoldenCases は golden を持つ面と、その引き方。
//
// ⚠️ **かつては「変更前に採る引数（capture）」と「変更後に突き合わせる引数（check）」の
// 対だった。** capture が `--all` を書かなかったのは、01KZ5ACN6P279S96D5M3AHY9HZ を
// 入れる前のコードでも走る形にしておくためで、あの単位が着地した時点で役目を終えている
// （いまの `tag list --json` は description を畳むので、capture 側で採り直すと
// json_full が「full ではない」ものになる）。**引き方は 1 つに戻した。**
var tagListGoldenCases = []struct {
	name string
	args []string
}{
	{"plain", []string{"tag", "list"}},
	{"tree", []string{"tag", "list", "--tree"}},
	{"kind", []string{"tag", "list", "--kind", "requirement"}},
	{"json_full", []string{"tag", "list", "--json", "--all"}},
	{"tree_json_full", []string{"tag", "list", "--tree", "--json", "--all"}},
	{"kind_json_full", []string{"tag", "list", "--kind", "requirement", "--json", "--all"}},
	{"tree_kind_json_full", []string{"tag", "list", "--tree", "--kind", "requirement", "--json", "--all"}},
}

// jsonFaces は golden を持つ 4 面。**これは「面の網羅」ではない**——
// 網羅は TestTagListEveryDiscoveredFaceFolds が cobra から数え上げて担う。
// ここは「変更前のバイト列を記録してある面」の対応表で、golden がある面にだけ
// バイト単位の期待値を当てるためのものである。
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

// updateTagListGoldens は golden を採り直す。
//
// 🔴 **採り直しと同時に「変わったのは空白だけか」を機械で見る。**
// これを手順書の 1 行にしておくと飛ばせる——飛ばせば、**欄が消えても気づけないまま
// golden が正典になる。** 比べ方は `jsonSameIgnoringWhitespace`（goldencmp_test.go）で、
// キー順・重複キー・数値リテラルの綴りまで保つ。⚠️ **`jq -S .` は使えない**
// （`-S` がキー順を正規化してしまう）。
//
// JSON として読めない golden（テキストの面）は**生バイト**で比べる。
//
// 値が変わるのが正しい変更のときは、`SCHOLIA_GOLDEN_ALLOW_VALUE_CHANGE=1` を
// 併せて立てる。**立てた事実がログに残る。**
func updateTagListGoldens(t *testing.T, dir string) {
	t.Helper()
	const allowValueChangeEnv = "SCHOLIA_GOLDEN_ALLOW_VALUE_CHANGE"
	allowValueChange := os.Getenv(allowValueChangeEnv) != ""
	if allowValueChange {
		t.Logf("⚠️ %s が立っている——値が変わっても採り直しを止めない", allowValueChangeEnv)
	}
	if err := os.MkdirAll(filepath.Join("testdata", "tag_list"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range tagListGoldenCases {
		path := tagListGoldenPath(tc.name)
		old, readErr := os.ReadFile(path) // 初回は存在しない
		out := mustRun(t, dir, tc.args...)
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		if readErr != nil {
			t.Logf("golden %s を %v から新規に採った（%d バイト・比べる相手が無い）", tc.name, tc.args, len(out))
			continue
		}

		var verdict string
		switch {
		case bytes.Equal(old, []byte(out)):
			verdict = "1 バイトも変わっていない"
		case json.Valid(old) && json.Valid([]byte(out)):
			same, why := jsonSameIgnoringWhitespace(old, []byte(out))
			if same {
				verdict = "空白の置き方だけが変わった（欄・順序・値は同一）"
			} else {
				verdict = "★値が変わった: " + why
				if !allowValueChange {
					t.Errorf("golden %s は空白以外も変わった。この単位の射程外の変更かもしれない: %s\n"+
						"（意図した変更なら %s=1 を併せて立てること）", tc.name, why, allowValueChangeEnv)
				}
			}
		default:
			verdict = "★テキストの golden のバイト列が変わった（JSON ではないので空白の差も差である）"
			if !allowValueChange {
				t.Errorf("golden %s（テキスト）のバイト列が変わった。`--json` の単位はテキストを変えないはず\n"+
					"（意図した変更なら %s=1 を併せて立てること）", tc.name, allowValueChangeEnv)
			}
		}
		t.Logf("golden %s を %v から採り直した（%d → %d バイト・%s）",
			tc.name, tc.args, len(old), len(out), verdict)
	}
}

// TestTagListBytes は各面のバイト列を golden と突き合わせる。
func TestTagListBytes(t *testing.T) {
	dir := t.TempDir()
	seedTagListFixture(t, dir)

	if os.Getenv(goldenUpdateEnv) != "" {
		updateTagListGoldens(t, dir)
		t.Skip("golden を採り直したので照合はしない")
	}

	for _, tc := range tagListGoldenCases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustRun(t, dir, tc.args...)
			want := readTagListGolden(t, tc.name)
			if got != want {
				t.Errorf("%v の出力が golden(%s) と違う\n--- got (%d バイト) ---\n%s\n--- want (%d バイト) ---\n%s",
					tc.args, tc.name, len(got), got, len(want), want)
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
		t.Run("既定は description の欄だけが抜けた形/"+face.name, func(t *testing.T) {
			full := readTagListGolden(t, face.golden)
			want := withoutDescriptionFields(full)
			if want == full {
				t.Fatalf("golden %s に description の欄が 1 つも無い（標本が壊れている）", face.golden)
			}
			got := mustRun(t, dir, face.args...)
			if got != want {
				t.Errorf("%v の出力が「description の欄だけを抜いた golden」と違う\n--- got ---\n%s\n--- want ---\n%s",
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

// withoutDescriptionFields は golden から description の欄だけを取り除く。
//
// ⚠️ **行では切れない。** `--json` は整形しない（01KZ7V637RNMPXJMVACYV6V1AS 条項1）
// ので、出力は全体で 1 行である。**文字列の中に居るかどうかを追いながら**
// `"description":<値>` の span を探し、前後どちらかのカンマごと落とす。
// 文字列を追うのは、description の値そのものが `"description":` という並びを
// 含みうるため——素朴な検索だと、そこで切って残りを壊す。
func withoutDescriptionFields(compact string) string {
	const key = `"description":`
	var out strings.Builder
	i := 0
	for i < len(compact) {
		if compact[i] == '"' {
			end := endOfJSONString(compact, i)
			if strings.HasPrefix(compact[i:], key) {
				// 値（model.Tag.Description は string なので必ず文字列）の終わりまでが span。
				ve := endOfJSONString(compact, i+len(key))
				s := out.String()
				switch {
				case strings.HasSuffix(s, ","): // 前にカンマ → 前を落とす
					out.Reset()
					out.WriteString(strings.TrimSuffix(s, ","))
				case ve < len(compact) && compact[ve] == ',': // 後ろにカンマ → 後ろを落とす
					ve++
				}
				i = ve
				continue
			}
			out.WriteString(compact[i:end])
			i = end
			continue
		}
		out.WriteByte(compact[i])
		i++
	}
	return out.String()
}

// endOfJSONString は compact[start] から始まる JSON 文字列リテラルの終端（閉じ
// 引用符の次）を返す。escape（`\"` / `\\`）を跨ぐ。
func endOfJSONString(compact string, start int) int {
	i := start + 1
	for i < len(compact) {
		switch compact[i] {
		case '\\':
			i += 2
			continue
		case '"':
			return i + 1
		}
		i++
	}
	return len(compact)
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

// TestTagListEveryDiscoveredFaceFolds は、`tag list` の**面の側を cobra から
// 数え上げて**、どの引き方でも既定が description を畳んでいることを見る。
//
// 🔴 **面をここで列挙しない。** 手書きの列挙（jsonFaces）は 4 面を数えていただけで、
// **5 つ目の面は誰も見ていなかった**——クリーンルームレビューが「新しい面が畳む前の
// タグを直に出す」変異を入れ、既存の歯止めは 1 つも落ちなかった。
// 列挙で追う限り差し戻しは終わらない（CLAUDE.md 2）ので、列挙を足すのではなく
// **面の集合を宣言から機械的に取る。**
//
// 取り方: bool フラグを `Flags().VisitAll` で拾い（`--all` は開く側なので除く）、
// その**全部分集合** × **config.tagKinds の全値＋無指定**を回す。
// **新しい bool フラグが足されれば、この検査が自動でその面も回す。**
//
// 見るのは 2 つだけで、出力の形（配列でも入れ子でも）に依らない:
//  1. 既定の出力に `"description":` が 1 つも無い
//  2. `--all` の出力から description の欄だけを抜いたものと、既定の出力が 1 バイトも違わない
func TestTagListEveryDiscoveredFaceFolds(t *testing.T) {
	dir := t.TempDir()
	seedTagListFixture(t, dir)

	// 面を表す bool フラグを宣言から拾う（手で並べない）。
	var boolFlags []string
	newTagListCmd().Flags().VisitAll(func(f *pflag.Flag) {
		if f.Value.Type() != "bool" || f.Name == "all" || f.Name == "help" {
			return
		}
		boolFlags = append(boolFlags, f.Name)
	})
	if len(boolFlags) == 0 {
		t.Fatal("bool フラグを 1 つも拾えていない（この検査は何も見ていない）")
	}
	sort.Strings(boolFlags)

	// 絞り込みの値も宣言（config.tagKinds）から拾う。
	kindValues := append([]string{""}, declaredTagKinds(t, dir)...)

	facesSeen, facesCarryingDescription := 0, 0
	for mask := 0; mask < 1<<len(boolFlags); mask++ {
		for _, kind := range kindValues {
			args := []string{"tag", "list"}
			for i, name := range boolFlags {
				if mask&(1<<i) != 0 {
					args = append(args, "--"+name)
				}
			}
			if kind != "" {
				args = append(args, "--kind", kind)
			}

			def, err := run(t, dir, args...)
			if err != nil {
				// 組み合わせとして成り立たない引き方は、タグを 1 件も出さない。
				// 素通りと混ぜないよう、記録だけ残して次へ。
				t.Logf("面 %v は失敗した（出力を持たないので検査対象外）: %v", args[2:], err)
				continue
			}
			all, err := run(t, dir, append(append([]string{}, args...), "--all")...)
			if err != nil {
				t.Errorf("面 %v は既定で成功したのに --all で失敗した: %v", args[2:], err)
				continue
			}
			facesSeen++

			t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
				if strings.Contains(def, `"description":`) {
					t.Errorf("既定の出力に description が残っている\n--- got ---\n%s", def)
				}
				if want := withoutDescriptionFields(all); def != want {
					t.Errorf("既定が「--all から description の欄だけを抜いたもの」と違う\n--- got ---\n%s\n--- want ---\n%s",
						def, want)
				}
			})
			if strings.Contains(all, `"description":`) {
				facesCarryingDescription++
			}
		}
	}

	t.Logf("cobra から数え上げた面: %d 通り（bool フラグ %v × kind %d 値）",
		facesSeen, boolFlags, len(kindValues))
	if facesCarryingDescription == 0 {
		t.Fatal("--all で description を出す面が 1 つも無い（標本か検査が壊れている）")
	}
}

// declaredTagKinds は標本の config.tagKinds を読む。絞り込みの値も手で並べない
// ——kind を足した人が、この検査の対象からその値だけ漏らすことがないように。
func declaredTagKinds(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".scholia", "config.json"))
	if err != nil {
		t.Fatalf("config.json が読めない: %v", err)
	}
	var cfg model.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("config.json が読めない: %v", err)
	}
	kinds := cfg.TagKindIDs()
	if len(kinds) == 0 {
		t.Fatal("config.tagKinds が空（この検査は絞り込みの面を 1 つも見ない）")
	}
	return kinds
}
