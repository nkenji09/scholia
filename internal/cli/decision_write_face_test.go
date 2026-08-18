// decision_write_face_test.go — 「面が出す decision は、保存された decision と
// 一致する」ことの歯止め（decision 01M09FHEQH7PVZ2BTKGXY5YMNN・
// クリーンルームレビュー 指摘④）。
//
// # なぜ要るか
//
// 保存の口は**結ぶ commit を完全 hash へ寄せる**（短縮 hash と完全 hash が別の
// 出来事として数えられるのを防ぐため）ので、**渡した値と保存された値は同じとは
// 限らない。** だから口は保存後の値を返し、面はそれを使う——と決めた。
//
// ⚠️ **その性質を守る検査が無かった。** レビュアが面を「保存前の値を出す」形へ
// 変異させても**全緑のまま通った**——`--json` は2件と言い、記録は1件のまま、
// という食い違いが検出されなかった。
//
// # 何に対して落ちるのか（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:**
//   - `--json` の面が、**保存されたものと違う decision を出した**——欄がずれていても、
//     commits[] の値が寄る前のものでも、印の数が違っても落ちる。
//     ⚠️ 見るのは**バイト列ではなく値**（保存済みファイルを読み直して突き合わせる）。
//   - 🔴 **面の一覧をここに書いていない。** 「`--json` を持つ面」を cobra の木から
//     数え上げ、**実際に走らせて `.scholia/decisions/` が変わったか**で
//     「decision を書く面」を判定する。**新しい書き込み面を足した人が、この検査に
//     何も書かなくても対象になる。**
//
// **落ちない（射程の外・正直に名乗る）:**
//   - `--json` を持たない面（テキストだけの出力）。突き合わせる機械可読な値が無い。
//   - `jsonFaceInvocations` に引き方が無い面——ただしそれは
//     TestEveryJSONFaceIsExercised が別途落とす。
//   - **出力に decision を含めない書き込み面**（`.scholia/decisions/` は変えるが
//     応答に decision を載せない面）。載せていないものは食い違いようがないので、
//     ここでは「対象外」として数え、件数をログに出す。
//   - viewer の面（cobra の木にぶら下がっていない）。そちらは
//     internal/viewer のテストが持つ。
//   - 🔴 **その面の引き方で「渡した値」と「保存された値」が食い違いえないとき。**
//     ⚠️ **実測で踏んだ**: `decision add-commit` の引き方が**完全** hash だったので
//     正規化しても値が変わらず、面を「保存前の値を出す」形へ変異させても
//     **緑のまま通った**。だから引き方の側を**短縮** hash に直してある
//     （placeholderShortHash）。同じ理由で `decision applied` はいまも射程の外である
//     ——この口は commit を持たない印（矛盾・却下）しか受けないので、
//     保存の前後で値が変わりえない。**将来 applied 側に正規化が入ったら、
//     その面の引き方も食い違いが出る形にする必要がある。**
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestJSONWriteFacesEmitTheSavedDecision は、`--json` の面を走らせて
// 「decision ファイルが変わったか」を実測し、変わった面については
// **出したものと保存されたものが一致する**ことを見る。
func TestJSONWriteFacesEmitTheSavedDecision(t *testing.T) {
	template, ids := seedJSONFaceFixture(t)
	t.Setenv("EDITOR", "true")
	t.Setenv("HOME", t.TempDir())

	writeFaces, checked, noRecord := 0, 0, 0
	for _, face := range discoverJSONFaces(t) {
		extra, ok := jsonFaceInvocations[face]
		if !ok {
			continue // TestEveryJSONFaceIsExercised が別途落とす
		}
		t.Run(face, func(t *testing.T) {
			dir := copyFixture(t, template)
			t.Chdir(dir)

			before := readDecisionFiles(t, dir)
			args := append(strings.Fields(face), ids.resolve(extra)...)
			args = append(args, "--json")
			stdout, _, err := runSplit(t, dir, args...)
			if err != nil || stdout == "" {
				return // 成り立たない引き方（TestEveryJSONFaceGoesThroughTheSingleExit が扱う）
			}
			after := readDecisionFiles(t, dir)

			changed := changedDecisionIDs(before, after)
			if len(changed) == 0 {
				return // decision を書かない面
			}
			writeFaces++

			// 出力の中から「変わった decision と同じ id を持つオブジェクト」を探す。
			// 面ごとに封筒の形が違う（record 埋め込み・フラット）ので、
			// **形を決め打ちせず値で探す。**
			for _, id := range changed {
				emitted, found := findObjectWithID(decodeAny(t, stdout), id)
				if !found {
					noRecord++
					t.Logf("この面は保存した decision を出力に載せていない（食い違いようがない）: %s", face)
					continue
				}
				checked++
				saved := decodeAny(t, after[id])
				if diff := compareDecisionFields(emitted, saved); diff != "" {
					t.Errorf("面が出した decision が、保存された decision と違う: %s\n%s\n"+
						"出力:\n%s\n保存:\n%s", face, diff, stdout, after[id])
				}
			}
		})
	}
	t.Logf("decision を書いた面: %d／突き合わせた decision: %d／出力に載せていなかった: %d",
		writeFaces, checked, noRecord)
	if checked == 0 {
		t.Fatal("1 件も突き合わせていない（この検査は何も見ていない）")
	}
}

// compareDecisionFields は「出したもの」と「保存されたもの」を欄ごとに比べる。
//
// ⚠️ **バイト列を比べない。** 面は derive した欄（effect・supersededBy）を足す
// ので、丸ごとの一致は要求できない。**保存された decision が持つ欄のすべてが、
// 出力に同じ値で載っていること**を見る（出力に余分な欄があるのは許す）。
func compareDecisionFields(emitted, saved any) string {
	e, ok1 := emitted.(map[string]any)
	s, ok2 := saved.(map[string]any)
	if !ok1 || !ok2 {
		return "JSON オブジェクトとして読めない"
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var diffs []string
	for _, k := range keys {
		got, present := e[k]
		if !present {
			diffs = append(diffs, "  欄 "+k+" が出力に無い")
			continue
		}
		if !reflect.DeepEqual(got, s[k]) {
			diffs = append(diffs, "  欄 "+k+" の値が違う: 出力="+jsonString(got)+" 保存="+jsonString(s[k]))
		}
	}
	return strings.Join(diffs, "\n")
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "<marshal 失敗>"
	}
	return string(b)
}

// findObjectWithID は JSON の木を歩いて、`id` が target のオブジェクトを返す。
// 封筒の形（record 埋め込み・フラット・配列）に依存しない。
func findObjectWithID(v any, target string) (any, bool) {
	switch t := v.(type) {
	case map[string]any:
		if id, ok := t["id"].(string); ok && id == target {
			return t, true
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if got, ok := findObjectWithID(t[k], target); ok {
				return got, true
			}
		}
	case []any:
		for _, e := range t {
			if got, ok := findObjectWithID(e, target); ok {
				return got, true
			}
		}
	}
	return nil, false
}

func decodeAny(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("JSON として読めない: %v\n%s", err, s)
	}
	return v
}

// readDecisionFiles は `.scholia/decisions/*.json` を id → 中身で返す。
func readDecisionFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	glob := filepath.Join(dir, ".scholia", "decisions", "*.json")
	paths, err := filepath.Glob(glob)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimSuffix(filepath.Base(p), ".json")] = string(b)
	}
	return out
}

// changedDecisionIDs は追加・変更された decision の id を返す（削除は見ない）。
func changedDecisionIDs(before, after map[string]string) []string {
	var out []string
	for id, body := range after {
		if before[id] != body {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
