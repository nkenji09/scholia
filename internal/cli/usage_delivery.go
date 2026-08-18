// usage_delivery.go — 「この起動で**本文が渡った**記録」を集める入れ物と、その判定
// （01M09FHFG4PVTGN4CA10N7BQZK）。
//
// # 集める場所
//
// 🔴 **ストアから読んだところではなく、出力に組み立てたところで集める。**
// 規則の一覧はすべての decision を走査してから絞るので、読んだところで集めると
// 全件が「渡った」になり、この項目が答えるはずの問い（どの記録が引かれていないか）が消える。
//
// # 入れ物をどこに置くか
//
// 🔴 **パッケージ変数に置かない。** viewer はスナップショットと派生 index を
// **プロセス全体で共有**している（`01KZ5N5CJ2VFMZAGSFPSCZAMTZ` 条項 2）。
// 同型の入れ物をパッケージ変数に置けば、常駐プロセス（`scholia view`）の中で
// **別の要求のぶんが同じ入れ物へ積まれる。** 入れ物は 1 起動につき 1 つ作り、
// cobra の context に載せて配る——`scholia view` の HTTP ハンドラは cobra の
// コマンドを持たないので、この入れ物に手が届かない（画面を数えないという条項 5 が、
// 配線の形として保たれる）。
//
// ⚠️ 計測がオフのときは context に載らない。取り出しは nil を返し、積む側は何もしない
// （`01KYSKM4T0RWRY1N7407KZSZ17` 条項 10「オフのときは何もしない」）。
// nil でも安全に呼べるのは、面の側に「オフかどうか」の分岐を書かせないためである。
//
// # 何を「本文が渡った」と数えるか
//
// ⚠️ **新しい判定を作らない。** 道具の側には既に区別がある——本文つきの出力形は
// レコードの型そのもの（`model.Decision` 等）を載せ、存在だけの出力形は
// **本文の欄を持たない別の型**（`inheritedOut`・`withdrawnOut`）へ落としている。
// `tag list` の既定も、同じレコード型のまま**本文の欄を空にして**渡している
// （`foldTagDescriptions`）。だから判定は「その値がレコードの型か」と
// 「本文の欄が空でないか」の 2 つで足りる（deliveredRecords）。
//
// ⚠️ **ただし「出力に組み立てた値」と「JSON に出るもの」は同じではない。**
// 外側の欄が埋め込みの同名の欄を覆うと、覆われた側は Go の値には居るのに 1 バイトも出ない。
// 歩く側がここを見なかったために、**出ていない本文を「渡った」と書いた**
// （差し戻し 1 回目 FAIL-2）。いまは deliveredRecords が覆いを見て歩く。
package cli

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

// deliveryKey は context に入れ物を載せるときの鍵（この package の外からは触れない）。
type deliveryKey struct{}

// deliveryLog は 1 起動ぶんの「本文が渡った記録」の集合。
//
// ⚠️ **mutex を持たせてあるのは、同じ起動の中で並行に書かれても壊れないため**である。
// いまの読み取り面はどれも 1 つの goroutine から書くが、その前提は面を足す人には見えない。
// 入れ物の側で閉じておく（`-race` がこの前提を検査する）。
type deliveryLog struct {
	mu   sync.Mutex
	seen map[string]bool
}

// note は id を積む。**nil レシーバで何もしない**（＝計測オフ）。
func (d *deliveryLog) note(ids ...string) {
	if d == nil || len(ids) == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen == nil {
		d.seen = make(map[string]bool, len(ids))
	}
	for _, id := range ids {
		if id != "" {
			d.seen[id] = true
		}
	}
}

// ids は積まれた id を昇順・重複なしで返す。nil レシーバでは空。
//
// 並べ替えるのは集計の都合ではなく**検査のため**である——渡った順は面の描き方に
// 依存するので、値で照合できる形（集合）に落としておく。
func (d *deliveryLog) ids() []string {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, 0, len(d.seen))
	for id := range d.seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// withDeliveryLog は入れ物を載せた context を返す。
func withDeliveryLog(ctx context.Context, d *deliveryLog) context.Context {
	return context.WithValue(ctx, deliveryKey{}, d)
}

// deliveryLogFrom は実行中のコマンドから入れ物を取り出す。載っていなければ nil
// （＝計測オフ・note は何もしない）。
func deliveryLogFrom(cmd *cobra.Command) *deliveryLog {
	if cmd == nil {
		return nil
	}
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}
	d, _ := ctx.Value(deliveryKey{}).(*deliveryLog)
	return d
}

// noteDelivered は**人が読む面**が「本文を渡した」と申告する口。
//
// 渡すのは、その面が実際に書き出した値そのもの（畳んだ後の値）である。
// `--json` の面はここを呼ばない——出力口（emitJSONTo）が同じ判定を通す。
func noteDelivered(cmd *cobra.Command, v any) {
	d := deliveryLogFrom(cmd)
	if d == nil {
		return
	}
	for _, r := range deliveredRecords(v) {
		d.note(r.id)
	}
}

// ---------------------------------------------------------------------------
// 判定（純関数・入力と出力の対で検査する・CLAUDE.md 1）
// ---------------------------------------------------------------------------

// deliveredRecord は「本文が渡ったと数える」レコード 1 件。
type deliveredRecord struct {
	id string
	// bodyKey はそのレコードの本文が載る JSON の欄名（遷移は本文の欄を持たないので空）。
	// 外側の欄がこの名前を覆っていれば本文は出力に出ないので、そのときは数えない。
	bodyKey string
}

// レコードの本文が載る JSON の欄名。
const (
	bodyKeyWhy         = "why"
	bodyKeyDescription = "description"
)

// recordCandidate は、その 1 つの値がレコードの型なら「渡った」1 件を返す。
//
// **本文の欄が空なら数えない。** 畳んだ出力（`tag list` の既定は description を
// 空にして渡す）を数えないためであり、同時に「本文が無いものは本文が渡りようがない」
// という素直な帰結でもある。
func recordCandidate(v reflect.Value) (deliveredRecord, bool) {
	switch r := v.Interface().(type) {
	case model.Decision:
		if r.Why == "" {
			return deliveredRecord{}, false
		}
		return deliveredRecord{id: r.ID, bodyKey: bodyKeyWhy}, true
	case model.Tag:
		if r.Description == "" {
			return deliveredRecord{}, false
		}
		return deliveredRecord{id: r.ID, bodyKey: bodyKeyDescription}, true
	case model.VocabEntry:
		if r.Description == "" {
			return deliveredRecord{}, false
		}
		return deliveredRecord{id: r.ID, bodyKey: bodyKeyDescription}, true
	case model.Transition:
		return deliveredRecord{id: r.ID}, true
	}
	return deliveredRecord{}, false
}

// deliveredRecordsMaxDepth は入れ子の上限。出力形は木なので実際には届かないが、
// 万一環になっても計測が本業を止めないための止め木である
// （記録の失敗が本業を落とさない・`01KYSKM4T0RWRY1N7407KZSZ17` 条項 11）。
const deliveredRecordsMaxDepth = 64

// deliveredRecords は**出力に組み立てた値**を歩いて、本文が渡ったレコードを id 昇順で返す。
//
// ⚠️ **見るのは「JSON に出る欄」だけである。** 公開されていない欄と `json:"-"` の欄は歩かない。
// 🔴 **外側の欄が覆っている埋め込みの欄も歩かない**——ここが差し戻し 1 回目で落ちた所である。
// `specOutput` は自分の `entries` で `render.SpecReport` の `entries` を覆っており、
// 覆われた側には**畳む前の**（取り下げ分も本文つきの）decision が入っている。
// 覆いを見ずに歩くと、**出力に 1 バイトも出ていない本文を「渡った」と書く。**
//
// ⚠️ **初版はこれを「書くバイト列に本文が現れるか」で確かめていた。**
// その確かめは**部分文字列の一致**なので、**本文が 1 文字も違わない 2 件があると区別できず**、
// 出ていない側まで数えた（レビュアが実測で再現し、対照実験で確定させた）。
// 出力の側から確かめるのをやめ、**覆いを見て歩く**形にしたのは、
// 「どのゲートの内側にいるか」を見て回るのをやめ、**出ない値をそもそも拾わない**ためである
// （CLAUDE.md 3 の一般化）。副産物として、計測が有効なとき `--json` を 2 度組み立てる必要も消えた。
//
// ⚠️ **面ごとに列挙しない。** 面が新しく出力形を作っても、その中にレコードの型が
// 載っていれば拾う。逆に、本文を持たない出力形（`inheritedOut` 等）へ落とした
// レコードは拾わない——**判定は綴りではなく型と欄の値で決まる**（CLAUDE.md 2）。
//
// **この歩き方が合わない場合（名乗る）:**
//   - **同じ深さで名前が衝突する 2 つの埋め込み。** `encoding/json` は両方を落とすが、ここは両方歩く。
//     いまそういう出力形は無い。
//   - **`json.Marshaler` を自分で実装した型。** 欄から出力を導けない。
//     いま `--json` の経路にあるのは `model.KindDecl` だけで、レコードを含まない。
func deliveredRecords(v any) []deliveredRecord {
	found := map[string]deliveredRecord{}
	walkForDelivered(reflect.ValueOf(v), 0, nil, found)
	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]deliveredRecord, 0, len(ids))
	for _, id := range ids {
		out = append(out, found[id])
	}
	return out
}

// walkForDelivered は値を歩く。shadowed は「この構造体より外側の欄に覆われていて、
// JSON に出ない欄の名前」の集合（埋め込みを降りるときだけ受け継ぐ）。
func walkForDelivered(v reflect.Value, depth int, shadowed map[string]bool, found map[string]deliveredRecord) {
	if depth > deliveredRecordsMaxDepth || !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return
		}
		walkForDelivered(v.Elem(), depth+1, shadowed, found)
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return
		}
		for i := 0; i < v.Len(); i++ {
			walkForDelivered(v.Index(i), depth+1, shadowed, found)
		}
	case reflect.Map:
		// ⚠️ 鍵は歩かない。`encoding/json` の map の鍵は文字列・整数・TextMarshaler に
		// 限られるので、レコードの型が鍵になることはない。
		if v.IsNil() {
			return
		}
		for _, k := range v.MapKeys() {
			walkForDelivered(v.MapIndex(k), depth+1, shadowed, found)
		}
	case reflect.Struct:
		walkStructForDelivered(v, depth, shadowed, found)
	}
}

func walkStructForDelivered(v reflect.Value, depth int, shadowed map[string]bool, found map[string]deliveredRecord) {
	if v.CanInterface() {
		if r, ok := recordCandidate(v); ok {
			// 本文の欄が外側に覆われていれば、本文は出力に出ない。
			if r.bodyKey != "" && shadowed[r.bodyKey] {
				return
			}
			if _, dup := found[r.id]; !dup {
				found[r.id] = r
			}
			// レコードの中に別のレコードは入らないので、ここで降りるのをやめる。
			return
		}
	}
	t := v.Type()

	// この構造体が**自分で宣言している**欄の JSON 名。埋め込みから昇格してくる
	// 同名の欄は、これに覆われて JSON に出ない（`encoding/json` の優先規則）。
	var own map[string]bool
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, tagged, skip := jsonFieldName(f)
		if skip || !emittedField(f) || (f.Anonymous && !tagged) {
			continue // 名前を持たない埋め込みは昇格する側なので、覆う側には数えない
		}
		if own == nil {
			own = map[string]bool{}
		}
		own[name] = true
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, tagged, skip := jsonFieldName(f)
		if skip || !emittedField(f) {
			continue // `json:"-"` と、出力に現れない欄
		}
		if f.Anonymous && !tagged {
			// 昇格する埋め込み。外側（own）と、さらに外側（shadowed）に覆われた名前を持って降りる。
			walkForDelivered(v.Field(i), depth+1, unionNames(shadowed, own), found)
			continue
		}
		if shadowed[name] {
			continue // 外側の同名の欄に覆われていて、JSON に出ない
		}
		// 名前を持つ欄の中身は自分の名前空間に入るので、覆いは受け継がない。
		walkForDelivered(v.Field(i), depth+1, nil, found)
	}
}

// emittedField は、その欄が JSON に出るかを返す（`encoding/json` の規則に合わせる）。
//
// ⚠️ **公開されていない埋め込みでも、型が構造体なら中身は昇格して出る。**
// ここを「公開された欄だけ」で切ると、**出ているのに歩かない**——
// この repo では `searchMatchOut` が小文字の別名を埋め込んでいる。
func emittedField(f reflect.StructField) bool {
	if f.IsExported() {
		return true
	}
	if !f.Anonymous {
		return false
	}
	t := f.Type
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

// jsonFieldName は欄の JSON 名と、名前が tag で明示されているか、出力に現れない欄かを返す。
func jsonFieldName(f reflect.StructField) (name string, tagged, skip bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	if i := strings.Index(tag, ","); i >= 0 {
		tag = tag[:i]
	}
	if tag == "" {
		return f.Name, false, false
	}
	return tag, true, false
}

func unionNames(a, b map[string]bool) map[string]bool {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make(map[string]bool, len(a)+len(b))
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}
