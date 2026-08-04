package viewer

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/render"
	"github.com/nkenji09/scholia/internal/store"
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
// 🔴 **fixture は `seedScaledStore`（4カテゴリ・多親・1レコードに規則が複数件・
// 4,200 件）を使う。** 最初はこの検査を最小 fixture（タグ2・遷移1・語彙2・decision 1）
// で回していて、**どのレコードも支配する規則が高々1件**だった。その形では
// 「一括の答えを**先頭1件に切り詰める**」変異が突き合わせを**素通りする**
// （レビュアが実測）。件数と並びが壊れる変異を判別するには、
// **1レコードに規則が複数件ある fixture**でなければならない。
// `seedScaledStore` は `at` を全件同じにしてあるので、支配する規則の並びは
// 元の走査順そのままになり、**走査の順を変える変異も値として見える。**
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

// eachRecord は全レコードの突き合わせを**並列に**回す。
//
// 直列だと 4,200 件の fixture で `-race` 込み 162 秒かかり、CI の
// `go test -race ./...` が現実的でなくなる（実測）。件数を減らして速くはしない
// ——標本の上端を下げると、件数で分岐する形が標本の外に出る。
//
// ⚠️ **並列にすること自体が検査でもある。** 一括の口と1件ずつの口を同時に叩くので、
// 共有スナップショットを書き換える実装は `-race` でここでも落ちる。
func eachRecord[T any](t *testing.T, items []T, fn func(T)) {
	t.Helper()
	workers := runtime.GOMAXPROCS(0)
	ch := make(chan T)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range ch {
				fn(it)
			}
		}()
	}
	for _, it := range items {
		ch <- it
	}
	close(ch)
	wg.Wait()
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
	h, _ := scaledHandler(t)

	tags := getJSON[[]model.Tag](t, h, "/api/tags")
	if len(tags) == 0 {
		t.Fatal("前提が崩れている: タグが0件")
	}
	bulk := getJSON[specAllResponse](t, h, "/api/spec")

	if len(bulk.Reports) != len(tags) {
		t.Errorf("一括の口が全タグを返していない: %d 件（タグは %d 件）", len(bulk.Reports), len(tags))
	}
	eachRecord(t, tags, func(tg model.Tag) {
		one := getJSON[render.SpecReport](t, h, "/api/spec/"+url.PathEscape(tg.ID))
		got, ok := bulk.Reports[tg.ID]
		if !ok {
			t.Errorf("一括の口に tag %q が無い", tg.ID)
			return
		}
		if canonical(t, got) != canonical(t, one) {
			t.Errorf("tag %q で一括と1件ずつの答えが違う\n一括: %s\n1件: %s", tg.ID, canonical(t, got), canonical(t, one))
		}
	})
}

func TestBulkGoverns_MatchesPerRecord(t *testing.T) {
	h, _ := scaledHandler(t)

	bulk := getJSON[governsAllResponse](t, h, "/api/governs?all=1")

	// 前提: この fixture では「支配する規則が2件以上」のレコードが実在する。
	// 1件しか無い fixture では、切り詰める変異が素通りする（上の 🔴）。
	multi := 0
	for _, refs := range bulk.ByRef {
		if len(refs) >= 2 {
			multi++
		}
	}
	if multi == 0 {
		t.Fatal("fixture が弱い: 支配する規則が2件以上のレコードが1つも無い（切り詰める変異を判別できない）")
	}

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
	type probe struct{ ref, query string }
	probes := make([]probe, 0, len(tags)+len(vocab)+len(txs.Transitions))
	for _, tg := range tags {
		probes = append(probes, probe{"tag:" + tg.ID, "tag=" + url.QueryEscape(tg.ID)})
	}
	for _, v := range vocab {
		probes = append(probes, probe{"vocab:" + v.ID, "vocab=" + url.QueryEscape(v.ID)})
	}
	for _, tx := range txs.Transitions {
		probes = append(probes, probe{"transition:" + tx.ID, "tx=" + url.QueryEscape(tx.ID)})
	}
	eachRecord(t, probes, func(p probe) { check(p.ref, p.query) })
}

func TestBulkTransitionDetails_MatchesPerRecord(t *testing.T) {
	h, _ := scaledHandler(t)

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
	eachRecord(t, list.Transitions, func(tx model.Transition) {
		one := getJSON[index.TransitionDetail](t, h, "/api/transitions/"+url.PathEscape(tx.ID))
		got, ok := bulk.Details[tx.ID]
		if !ok {
			t.Errorf("一括の口に transition %q が無い", tx.ID)
			return
		}
		if canonical(t, got) != canonical(t, one) {
			t.Errorf("transition %q で一括と1件ずつの答えが違う\n一括: %s\n1件: %s", tx.ID, canonical(t, got), canonical(t, one))
		}
	})
}

// TestBulkAtFullScale_NotTruncated は、**総当たりより大きい標本**で
// 「一括の口が黙って切り詰めていないか」だけを見る。要求は数本なので安い。
//
// 🔴 **なぜ分けるか。** 総当たりの突き合わせは費用が「件数 × 要求本数」＝
// **件数の二乗**に効くので、いちばん大きい標本では回せない（4,200 件・`-race`
// 込みで 162 秒）。だが「N 件を超えたら切り詰める」形の変異は、標本の上端が
// 低いと素通りする——**上端は下げずに、上端で見る中身を軽いものに変える。**
//
// ここで見るのは2つ:
//   - 全レコードが返っていること（レコード単位の切り詰め）
//   - **1レコードあたりの規則が2件以上あるレコードが残っていること**
//     （リスト単位の切り詰め＝レビュアの B2 変異。件数だけ見ても捕まらない）
func TestBulkAtFullScale_NotTruncated(t *testing.T) {
	if testing.Short() {
		t.Skip("規模の標本は -short では走らせない")
	}
	h, _ := fullScaleHandler(t)

	tags := getJSON[[]model.Tag](t, h, "/api/tags")
	vocab := getJSON[[]model.VocabEntry](t, h, "/api/vocab")
	txs := getJSON[transitionsResponse](t, h, "/api/transitions")

	governs := getJSON[governsAllResponse](t, h, "/api/governs?all=1")
	if want := len(tags) + len(vocab) + len(txs.Transitions); len(governs.ByRef) != want {
		t.Errorf("この規模で一括 governs がレコードを落としている: %d 件（%d 件のはず）", len(governs.ByRef), want)
	}
	maxRefs := 0
	for _, refs := range governs.ByRef {
		if len(refs) > maxRefs {
			maxRefs = len(refs)
		}
	}
	if maxRefs < 3 {
		t.Errorf("この規模で1レコードあたりの規則が最大 %d 件しか無い（切り詰めているか、fixture が弱い）", maxRefs)
	}

	spec := getJSON[specAllResponse](t, h, "/api/spec")
	if len(spec.Reports) != len(tags) {
		t.Errorf("この規模で一括 spec がタグを落としている: %d 件（%d 件のはず）", len(spec.Reports), len(tags))
	}
	detail := getJSON[transitionsResponse](t, h, "/api/transitions?detail=1")
	if len(detail.Details) != len(txs.Transitions) {
		t.Errorf("この規模で一括の詳細が transition を落としている: %d 件（%d 件のはず）", len(detail.Details), len(txs.Transitions))
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

// 規模 fixture は**この test binary で1回だけ**建てて共有する。突き合わせは
// 全レコードを回るので、テストごとに建て直すと生成だけで時間が溶ける。
//
// ⚠️ **`t.TempDir()` は使えない**（最初に呼んだテストの終了時に消えて、
// 2つ目以降が「config.json が無い」で落ちる——実見）。TestMain が後始末する
// package 寿命のディレクトリに建てる。
//
// ⚠️ **共有してよいのは、この file の検査がストアを書き換えないから。**
// 書き換える検査（`TestIndexCache_AtScale`）は自分の t.TempDir() で建てる。
type sharedFixture struct {
	once    sync.Once
	dir     string
	handler http.Handler
	store   *store.Store
}

var (
	midFixture  sharedFixture // 2,100 件・総当たりの突き合わせ用
	fullFixture sharedFixture // 4,200 件・規模の側の検査用
	fixtureDirs []string
)

func TestMain(m *testing.M) {
	code := m.Run()
	for _, d := range fixtureDirs {
		_ = os.RemoveAll(d)
	}
	os.Exit(code)
}

func (f *sharedFixture) get(t *testing.T, sz scaleSize) (http.Handler, *store.Store) {
	t.Helper()
	f.once.Do(func() {
		dir, err := os.MkdirTemp("", "scholia-scaled-*")
		if err != nil {
			return
		}
		f.dir = dir
		fixtureDirs = append(fixtureDirs, dir)
		f.handler, f.store = seedScaledStoreIn(t, dir, sz)
	})
	if f.handler == nil {
		t.Fatal("規模 fixture の生成に失敗した")
	}
	return f.handler, f.store
}

func scaledHandler(t *testing.T) (http.Handler, *store.Store) { return midFixture.get(t, midScale) }
func fullScaleHandler(t *testing.T) (http.Handler, *store.Store) {
	return fullFixture.get(t, fullScale)
}
