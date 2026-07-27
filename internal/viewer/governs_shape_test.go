package viewer

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/render"
)

// 追補 01KYJP68V2GR4QJ8HNW6HEP00T 条項3（live と静的で開示の答えが割れない）の歯止め。
//
// 条項3 は「両方を同じ形にしたので構造上割れようがない」で守っているつもりだったが、
// レビューで **その「同じ形」を固定しているものが何も無い** と指摘された（2回目
// should-1）。実際、静的書き出し側だけを本文つき（index.GovernsEntry）に戻す変異を
// 当てても go build / go vet / go test / npm test はすべて緑のまま通り、その状態で
// 書き出した静的 HTML は tx.viewer.search-restore の開示を 19件 → 20件（置き換え済み
// を混ぜた数）と表示した——**live とだけ答えが割れる**。
//
// 型が割れると何が起きるか:
//   - 本文つきに戻ると、消費側（web/src/components/browse/inheritedSummary.ts）が
//     読む `decisionId` が undefined になり、効力の判定が総崩れになる。
//   - 形が違えば消費側に2つのコードパスが要る。そこが割れの入口になる。
//
// ここは「2つの面が同じ要素型を返す」ことだけを見る。中身の一致（276レコード全件で
// live と焼き込みが一致すること）は実機計測が担う——型が同じでも詰め方を間違えれば
// 割れうるが、詰めているのは両方とも index.RefsOf(index.GovernsFor*) の1本である。

func TestGovernsShape_LiveAndStaticExportShareElementType(t *testing.T) {
	// live: []index.GovernsRef → 要素型は index.GovernsRef
	live := reflect.TypeOf(governsResponse{}.Entries).Elem()
	// static: map[string][]index.GovernsRef → 値がスライス、その要素型を取る
	staticField, ok := render.StaticGovernsType()
	if !ok {
		t.Fatal("render 側の governs フィールドが見つからない（フィールド名が変わった？）")
	}
	static := staticField.Elem().Elem()
	if live != static {
		t.Fatalf("要素型が割れている: live=%s / static=%s\n"+
			"live の GET /api/governs と静的書き出しの焼き込みは同じ形でなければならない"+
			"（追補 01KYJP68V2GR4QJ8HNW6HEP00T 条項3）。", live, static)
	}
	// 本文を運ぶ型に戻っていないこと（絞る前は index.GovernsEntry だった・条項2）。
	if live != reflect.TypeOf(index.GovernsRef{}) {
		t.Fatalf("要素型 = %s, want index.GovernsRef（本文を運ばない形・条項2）", live)
	}
}

// 応答の JSON が「参照だけ」であること——本文のフィールドが生えていたら、絞ったはずの
// 焼き込みがまた太っている（条項2）。型だけでなく実際の JSON キーで見る。
func TestGovernsShape_ResponseCarriesNoDecisionBody(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/governs?tag=subject.auth", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var raw struct {
		Entries []map[string]json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Entries) == 0 {
		t.Fatal("entries が空——このフィクスチャでは1件返るはず")
	}
	allowed := map[string]bool{"decisionId": true, "provenance": true, "viaTag": true}
	for _, e := range raw.Entries {
		for k := range e {
			if !allowed[k] {
				t.Fatalf("応答に %q が入っている。開示が要るのは decision id と出自だけで、"+
					"本文は decisions 側にある（追補 01KYJP68V2GR4QJ8HNW6HEP00T 条項2）", k)
			}
		}
	}
}
