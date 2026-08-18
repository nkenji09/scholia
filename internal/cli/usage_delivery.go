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
package cli

import (
	"context"
	"reflect"
	"sort"
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

// deliveredRecord は「本文が渡ったと数える候補」1 件。
//
// body は本文そのもの（`--json` の出力口が「本当にバイト列へ出たか」を確かめるために使う）。
// **本文の欄を持たない型（遷移）では空**——遷移は id・action・given・then が
// レコードの全部で、載った時点で畳みようがなく全部が渡っている。
type deliveredRecord struct {
	id   string
	body string
}

// recordCandidate は、その 1 つの値がレコードの型なら候補を返す。
//
// **本文の欄が空なら候補にしない。** 畳んだ出力（`tag list` の既定は description を
// 空にして渡す）を数えないためであり、同時に「本文が無いものは本文が渡りようがない」
// という素直な帰結でもある。
func recordCandidate(v reflect.Value) (deliveredRecord, bool) {
	switch r := v.Interface().(type) {
	case model.Decision:
		if r.Why == "" {
			return deliveredRecord{}, false
		}
		return deliveredRecord{id: r.ID, body: r.Why}, true
	case model.Tag:
		if r.Description == "" {
			return deliveredRecord{}, false
		}
		return deliveredRecord{id: r.ID, body: r.Description}, true
	case model.VocabEntry:
		if r.Description == "" {
			return deliveredRecord{}, false
		}
		return deliveredRecord{id: r.ID, body: r.Description}, true
	case model.Transition:
		return deliveredRecord{id: r.ID}, true
	}
	return deliveredRecord{}, false
}

// deliveredRecordsMaxDepth は入れ子の上限。出力形は木なので実際には届かないが、
// 万一環になっても計測が本業を止めないための止め木である
// （記録の失敗が本業を落とさない・`01KYSKM4T0RWRY1N7407KZSZ17` 条項 11）。
const deliveredRecordsMaxDepth = 64

// deliveredRecords は**出力に組み立てた値**を歩いて、本文が渡った候補を id 昇順で返す。
//
// ⚠️ **見るのは「JSON に出る欄」だけである**——公開されていない欄と `json:"-"` の欄は
// 出力に現れないので歩かない。
//
// ⚠️ **面ごとに列挙しない。** 面が新しく出力形を作っても、その中にレコードの型が
// 載っていれば拾う。逆に、本文を持たない出力形（`inheritedOut` 等）へ落とした
// レコードは拾わない——**判定は綴りではなく型と欄の値で決まる**（CLAUDE.md 2）。
//
// 🔴 **ここは「候補」までしか出せない。** Go の値には出るが JSON には出ない欄がある
// ——外側の欄が埋め込みの同名欄を覆う場合である（`specOutput` は自分の `Entries` で
// `render.SpecReport` の `Entries` を覆っており、覆われた側には畳む前の decision が
// 入っている）。だから `--json` の出力口は、候補の**本文が本当にバイト列へ出たか**を
// 確かめてから積む（jsonio.go の noteDeliveredJSON）。
func deliveredRecords(v any) []deliveredRecord {
	found := map[string]deliveredRecord{}
	walkForDelivered(reflect.ValueOf(v), 0, found)
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

func walkForDelivered(v reflect.Value, depth int, found map[string]deliveredRecord) {
	if depth > deliveredRecordsMaxDepth || !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return
		}
		walkForDelivered(v.Elem(), depth+1, found)
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return
		}
		for i := 0; i < v.Len(); i++ {
			walkForDelivered(v.Index(i), depth+1, found)
		}
	case reflect.Map:
		if v.IsNil() {
			return
		}
		for _, k := range v.MapKeys() {
			walkForDelivered(v.MapIndex(k), depth+1, found)
		}
	case reflect.Struct:
		if v.CanInterface() {
			if r, ok := recordCandidate(v); ok {
				if _, dup := found[r.id]; !dup {
					found[r.id] = r
				}
				// レコードの中に別のレコードは入らないので、ここで降りるのをやめる。
				return
			}
		}
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() || f.Tag.Get("json") == "-" {
				continue // JSON に出ない欄は「渡っていない」
			}
			walkForDelivered(v.Field(i), depth+1, found)
		}
	}
}
