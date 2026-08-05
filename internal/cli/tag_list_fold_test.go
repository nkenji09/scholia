// tag_list_fold_test.go — 「`--json` で 1 タグについて何を渡すか」の判断を、
// 入力と出力の対で検査する（CLAUDE.md「配線ガードの書き方」1）。
//
// # この歯止めが落とす範囲（同 6）
//
// **落ちる:**
//   - `--all` の返り値が入力と 1 つでも違う（どのフィールドでも）
//   - 既定の返り値が、description 以外のフィールドで入力と 1 つでも違う
//   - 既定で description が 1 件でも残る
//   - 入力のスライス／要素をその場で書き換える（共有している値を壊す実装）
//   - 返り値の要素を書き換えると入力側にも波及する（backing array の共有）
//   - 上のどれかが**件数によって変わる**（0 件から、実在する最大より十分上まで）
//   - model.Tag にフィールドが増えたのに標本がそれを埋めていない
//     （＝「他のフィールドは変わらない」の検査がその新フィールドを見ないまま緑になる状態）
//
// **落ちない（射程の外・正直に名乗る）:**
//   - 出力のバイト列そのもの（整形・キーの並び）。それは tag_list_bytes_test.go の golden。
//   - CLI がこの関数を**呼んでいるか**。それも tag_list_bytes_test.go が
//     実際の出力で見る（呼び出しが書かれていることの照合はしない・CLAUDE.md 2）。
package cli

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

// sampleTagsForFold は model.Tag の**全フィールド**を踏む標本。
// 踏み残しがあると「description 以外は変わらない」の検査がそのフィールドを
// 見ないまま緑になるので、下の TestFoldSampleCoversEveryTagField が
// 踏み残しを赤で知らせる。
func sampleTagsForFold() []model.Tag {
	return []model.Tag{
		{
			ID:          "a.full",
			Name:        "全フィールド",
			Kind:        "axis",
			ParentIDs:   []string{"b.parent", "c.parent"},
			Description: "落ちる説明。改行\nと 引用符 \" を含む。",
			Color:       "#3b82f6",
			Ref:         "https://example.invalid/a",
			Total:       true,
			Fulfillment: model.FulfillmentProperty,
		},
		{ID: "b.parent", Name: "説明なし", Kind: "subject"},          // description が空
		{ID: "c.parent", Name: "説明だけ", Description: "説明しか持たない。"}, // kind も親も無い
	}
}

func TestFoldTagDescriptions(t *testing.T) {
	t.Run("--all は入力と完全に一致する", func(t *testing.T) {
		in := sampleTagsForFold()
		got := foldTagDescriptions(in, true)
		if !reflect.DeepEqual(got, in) {
			t.Errorf("--all が入力と違う\ngot:  %+v\nwant: %+v", got, in)
		}
	})

	t.Run("既定は description だけを落とす", func(t *testing.T) {
		in := sampleTagsForFold()
		got := foldTagDescriptions(in, false)
		if len(got) != len(in) {
			t.Fatalf("件数が違う: %d, want %d", len(got), len(in))
		}
		dropped := 0
		for i := range got {
			if got[i].Description != "" {
				t.Errorf("%d 件目に description が残っている: %q", i, got[i].Description)
			}
			if in[i].Description != "" {
				dropped++
			}
			// description を戻したものが入力と完全に一致する＝ description 以外は
			// 1 つも変わっていない。フィールドを 1 つずつ数え上げないので、
			// model.Tag にフィールドが増えてもこの検査はそのまま効く。
			restored := got[i]
			restored.Description = in[i].Description
			if !reflect.DeepEqual(restored, in[i]) {
				t.Errorf("%d 件目が description 以外で変わっている\ngot:  %+v\nwant: %+v", i, restored, in[i])
			}
		}
		if dropped == 0 {
			t.Fatal("標本に description を持つタグが 1 件も無い（この検査は何も見ていない）")
		}
	})

	t.Run("入力を破壊しない", func(t *testing.T) {
		in := sampleTagsForFold()
		before := sampleTagsForFold()
		foldTagDescriptions(in, false)
		if !reflect.DeepEqual(in, before) {
			t.Errorf("既定の呼び出しが入力を書き換えた\ngot:  %+v\nwant: %+v", in, before)
		}
		foldTagDescriptions(in, true)
		if !reflect.DeepEqual(in, before) {
			t.Errorf("--all の呼び出しが入力を書き換えた\ngot:  %+v\nwant: %+v", in, before)
		}
	})

	t.Run("返り値を書き換えても入力へ波及しない", func(t *testing.T) {
		for _, showAll := range []bool{false, true} {
			in := sampleTagsForFold()
			got := foldTagDescriptions(in, showAll)
			if len(got) == 0 {
				t.Fatal("標本が空")
			}
			got[0].Name = "書き換えた"
			got[0].Description = "書き換えた"
			if in[0].Name == "書き換えた" || in[0].Description == "書き換えた" {
				t.Errorf("showAll=%v: 返り値と入力が同じ配列を指している", showAll)
			}
		}
	})

	// ⚠️ 件数で分岐する変異は、標本の上端が本番より下にあると素通りする。
	// ここでは実在する最大の記録集合より十分上（1200 件）まで見る。
	t.Run("件数に依らない", func(t *testing.T) {
		for _, n := range []int{0, 1, 2, 300, 1200} {
			t.Run(fmt.Sprintf("%d 件", n), func(t *testing.T) {
				in := make([]model.Tag, 0, n)
				for i := 0; i < n; i++ {
					in = append(in, model.Tag{
						ID:          fmt.Sprintf("req.bulk-%05d", i),
						Name:        fmt.Sprintf("一括 %05d", i),
						Kind:        "requirement",
						Description: strings.Repeat("落ちる説明。", 8),
					})
				}
				def := foldTagDescriptions(in, false)
				all := foldTagDescriptions(in, true)
				if len(def) != n || len(all) != n {
					t.Fatalf("件数が変わった: 既定 %d / --all %d, want %d", len(def), len(all), n)
				}
				if !reflect.DeepEqual(all, in) {
					t.Errorf("%d 件で --all が入力と違う", n)
				}
				for i := range def {
					if def[i].Description != "" {
						t.Fatalf("%d 件のとき %d 件目の description が残っている", n, i)
					}
					restored := def[i]
					restored.Description = in[i].Description
					if !reflect.DeepEqual(restored, in[i]) {
						t.Fatalf("%d 件のとき %d 件目が description 以外で変わっている", n, i)
					}
				}
			})
		}
	})

	t.Run("nil 入力でも JSON の [] になる形を返す", func(t *testing.T) {
		// json.Encoder は nil スライスを null、空スライスを [] と書く。
		// 変更前の filterTagsByKind は空スライスを返していたので、ここが nil を
		// 返すと `--all` の出力が現行と変わってしまう。
		for _, showAll := range []bool{false, true} {
			if got := foldTagDescriptions(nil, showAll); got == nil {
				t.Errorf("showAll=%v: nil を返した（出力が [] から null へ変わる）", showAll)
			}
		}
	})
}

// TestFoldSampleCoversEveryTagField は、標本が model.Tag の全フィールドを
// 踏んでいることを見る。**新しいフィールドを足した面にガードを置き忘れる**という
// 型（CLAUDE.md 5）を、標本の側から塞ぐ。
func TestFoldSampleCoversEveryTagField(t *testing.T) {
	sample := sampleTagsForFold()
	typ := reflect.TypeOf(model.Tag{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		covered := false
		for _, tag := range sample {
			v := reflect.ValueOf(tag).Field(i)
			if !v.IsZero() {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("標本が model.Tag.%s を 1 件も埋めていない: "+
				"この状態だと「description 以外は変わらない」の検査がこのフィールドを見ない。"+
				"sampleTagsForFold に足すこと", f.Name)
		}
	}
}
