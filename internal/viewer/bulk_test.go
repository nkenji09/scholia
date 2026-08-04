package viewer

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/render"
)

// ===========================================================================
// 一括の口が、1件ずつの口と**同じ答え**を返すこと
// （decision 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1）
// ===========================================================================
//
// 条項1 は本数を減らす話だが、減らした結果として**答えが変わってはいけない**
// （decision の「変更」節: 表示内容は変えない）。一括の口が別の導出を持てば、
// 「この記録を支配する規則は何か」に面ごとに違う答えが返る余地が復活する
// （01KYKS4Y56FAHRVCWKMQJK4RT6 条項5・面間整合原則 D10b-2）。
//
// ここは**全レコードについて**1件ずつの応答と一括の応答を突き合わせる。
// 「代表1件だけ見る」形にすると、特定の形のレコード（規則0件・多親・vocab 経由の
// 実効タグ等）だけ答えが割れる実装が素通りする。
//
// ⚠️ **このガードが捕まえないもの**: Go 側の答えそのものの正しさ（1件ずつの口が
// 間違っていれば一括も同じだけ間違う。そちらは index/render 側のテストが持つ）。

func getJSON[T any](t *testing.T, h http.Handler, target string) T {
	t.Helper()
	rec := doRequest(t, h, http.MethodGet, target, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", target, rec.Code, rec.Body.String())
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("GET %s: JSON decode: %v", target, err)
	}
	return v
}

// canonical は2つの値を JSON に落として比べるための正規形（map の順序差を消す）。
func canonical(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestBulkSpec_MatchesPerRecord(t *testing.T) {
	h, _ := newTestHandler(t)

	tags := getJSON[[]model.Tag](t, h, "/api/tags")
	if len(tags) == 0 {
		t.Fatal("前提が崩れている: タグが0件")
	}
	bulk := getJSON[specAllResponse](t, h, "/api/spec")

	if len(bulk.Reports) != len(tags) {
		t.Errorf("一括の口が全タグを返していない: %d 件（タグは %d 件）", len(bulk.Reports), len(tags))
	}
	for _, tg := range tags {
		one := getJSON[render.SpecReport](t, h, "/api/spec/"+url.PathEscape(tg.ID))
		got, ok := bulk.Reports[tg.ID]
		if !ok {
			t.Errorf("一括の口に tag %q が無い", tg.ID)
			continue
		}
		if canonical(t, got) != canonical(t, one) {
			t.Errorf("tag %q で一括と1件ずつの答えが違う\n一括: %s\n1件: %s", tg.ID, canonical(t, got), canonical(t, one))
		}
	}
}

func TestBulkGoverns_MatchesPerRecord(t *testing.T) {
	h, _ := newTestHandler(t)

	bulk := getJSON[governsAllResponse](t, h, "/api/governs?all=1")

	tags := getJSON[[]model.Tag](t, h, "/api/tags")
	vocab := getJSON[[]model.VocabEntry](t, h, "/api/vocab")
	txs := getJSON[transitionsResponse](t, h, "/api/transitions")

	if want := len(tags) + len(vocab) + len(txs.Transitions); len(bulk.ByRef) != want {
		t.Errorf("一括の口が全レコードを返していない: %d 件（tag+vocab+tx は %d 件）", len(bulk.ByRef), want)
	}

	check := func(ref, query string) {
		t.Helper()
		one := getJSON[governsResponse](t, h, "/api/governs?"+query)
		got, ok := bulk.ByRef[ref]
		if !ok {
			t.Errorf("一括の口に %q が無い", ref)
			return
		}
		if canonical(t, got) != canonical(t, one.Entries) {
			t.Errorf("%s で一括と1件ずつの答えが違う\n一括: %s\n1件: %s", ref, canonical(t, got), canonical(t, one.Entries))
		}
	}
	for _, tg := range tags {
		check("tag:"+tg.ID, "tag="+url.QueryEscape(tg.ID))
	}
	for _, v := range vocab {
		check("vocab:"+v.ID, "vocab="+url.QueryEscape(v.ID))
	}
	for _, tx := range txs.Transitions {
		check("transition:"+tx.ID, "tx="+url.QueryEscape(tx.ID))
	}
}

func TestBulkTransitionDetails_MatchesPerRecord(t *testing.T) {
	h, _ := newTestHandler(t)

	list := getJSON[transitionsResponse](t, h, "/api/transitions")
	if len(list.Transitions) == 0 {
		t.Fatal("前提が崩れている: transition が0件")
	}
	// ⚠️ detail を渡さない応答に details が混ざっていないこと（後方互換）。
	if list.Details != nil {
		t.Errorf("detail を渡していないのに details が載っている（既存の呼び手の JSON が変わる）")
	}

	bulk := getJSON[transitionsResponse](t, h, "/api/transitions?detail=1")
	if len(bulk.Details) != len(list.Transitions) {
		t.Errorf("一括の口が全 transition の詳細を返していない: %d 件（transition は %d 件）", len(bulk.Details), len(list.Transitions))
	}
	// 一覧そのものは detail の有無で変わらない。
	if canonical(t, bulk.Transitions) != canonical(t, list.Transitions) {
		t.Errorf("detail=1 で一覧の中身が変わっている")
	}
	for _, tx := range list.Transitions {
		one := getJSON[index.TransitionDetail](t, h, "/api/transitions/"+url.PathEscape(tx.ID))
		got, ok := bulk.Details[tx.ID]
		if !ok {
			t.Errorf("一括の口に transition %q が無い", tx.ID)
			continue
		}
		if canonical(t, got) != canonical(t, one) {
			t.Errorf("transition %q で一括と1件ずつの答えが違う\n一括: %s\n1件: %s", tx.ID, canonical(t, got), canonical(t, one))
		}
	}
}

// TestBulkGoverns_RejectsMixedSelectors は `all` と1件指定の同時指定を拒むこと。
// 黙って片方を優先すると、呼び手は「絞ったつもりで全件を受け取る」。
func TestBulkGoverns_RejectsMixedSelectors(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/governs?all=1&tag=subject.auth", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("all と tag の同時指定が %d で通っている", rec.Code)
	}
}

// TestGoverns_SingleSelectorContractUnchanged は、`all` を足したことで
// 従来の契約（tag/tx/vocab のいずれか1つ必須）が壊れていないこと。
func TestGoverns_SingleSelectorContractUnchanged(t *testing.T) {
	h, _ := newTestHandler(t)
	for _, q := range []string{"", "tag=subject.auth&tx=T-login"} {
		rec := doRequest(t, h, http.MethodGet, "/api/governs?"+q, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET /api/governs?%s が %d（400 のはず）", q, rec.Code)
		}
	}
}
