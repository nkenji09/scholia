package viewer

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// ===========================================================================
// 条項2 の歯止め — 「1本の要求が、毎回ストア全体を読み直さない」
// （decision 01KZ5N5CJ2VFMZAGSFPSCZAMTZ・語彙 cond.index-built）
// ===========================================================================
//
// ## 何を、どう数えるか
//
// 🔴 **自分で申告するカウンタは置かない。** 直前の単位（単位AU）が
// 「起こす側が自分で申告する」カウンタを置き、申告 2/2/2 に対して実プロセスは
// 7/62/202 だった。ここは**外から観測できる事象**で数える:
//
//   - **読み直したかどうか** … ディスク上の中身を、**指紋（サイズ＋mtime）を
//     変えないまま**書き換えてから同じ要求を投げる。読み直していれば新しい中身が、
//     読み直していなければ古い中身が返る。**応答の値**がそのまま答えになる。
//   - **鮮度を落としていないか** … 普通に書き換えて（＝指紋が変わる）、
//     次の要求に反映されることを応答の値で見る。
//
// 前者は Fingerprint の盲点（store/fingerprint.go の「射程」）を**探針として
// 使っている**。盲点を検査に使うのは、その盲点が実在することを同時に固定する
// ことでもある——だから両方を1つの file に置き、射程として名乗る。
//
// ## このガードが捕まえないもの（射程・CLAUDE.md 6）
//
//  1. **1本あたりの費用が O(N) のまま残ること。** ここが見るのは「ストアを
//     読み直したか」だけで、読み直さずに O(N) の計算をしていても緑になる。
//     （decision の「歯止め」節が同じ限界を名指ししている。）
//  2. **要求の本数。** 条項1 の話で、そちらは `web/pageRequestCost.test.tsx` が
//     値で数える（画面が実際に投げた fetch を外から数える）。
//  3. **指紋の盲点そのもの。** サイズも mtime も変わらない書き換えは検知されない。
//     ここではそれを**探針として使っている**ので、盲点を塞ぐ変更を入れると
//     `TestIndexCache_SecondRequestDoesNotRereadStore` は落ちる（＝盲点が
//     塞がったことに気づける）。塞ぐこと自体は退行ではないので、そのときは
//     この探針を別の手段（例: 読み取り権限を落とす）に置き換えること。

// writeSameStamp は path の中身を new に差し替え、**サイズと mtime を元のまま**に
// 戻す。指紋（store.Fingerprint）から見ると「何も変わっていない」ファイルになる。
func writeSameStamp(t *testing.T, path string, replace func(old []byte) []byte) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	next := replace(old)
	if len(next) != len(old) {
		t.Fatalf("探針が壊れている: 差し替え後のサイズが違う（%d → %d）", len(old), len(next))
	}
	if err := os.WriteFile(path, next, info.Mode().Perm()); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// tagNameFromAPI は GET /api/tags の応答から1件の name を読む。
func tagNameFromAPI(t *testing.T, h http.Handler, tagID string) string {
	t.Helper()
	rec := doRequest(t, h, http.MethodGet, "/api/tags", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/tags: %d %s", rec.Code, rec.Body.String())
	}
	for _, tg := range decodeJSON[[]model.Tag](t, rec) {
		if tg.ID == tagID {
			return tg.Name
		}
	}
	t.Fatalf("tag %q が応答に無い: %s", tagID, rec.Body.String())
	return ""
}

// TestIndexCache_SecondRequestDoesNotRereadStore は、起動後の要求が
// `.scholia/` を読み直していないことを**応答の値**で見る。
func TestIndexCache_SecondRequestDoesNotRereadStore(t *testing.T) {
	h, s := newTestHandler(t)

	if got := tagNameFromAPI(t, h, "subject.auth"); got != "認証" {
		t.Fatalf("前提が崩れている: name=%q", got)
	}

	// ディスク上の name を書き換える。サイズと mtime は元のまま＝指紋は不変。
	path := filepath.Join(s.Dir, "tags", "subject.auth.json")
	writeSameStamp(t, path, func(old []byte) []byte {
		return []byte(strings.Replace(string(old), `"認証"`, `"改竄"`, 1))
	})

	// 読み直していれば "改竄"、読み直していなければ "認証"。
	if got := tagNameFromAPI(t, h, "subject.auth"); got != "認証" {
		t.Errorf("2回目の要求がストアを読み直している（name=%q）——条項2 は起動時に建て、"+
			".scholia/ が変わったときだけ建て直すことを定めている", got)
	}
}

// TestIndexCache_BuiltAtStartup は cond.index-built「**起動時に** .scholia の
// in-memory index が構築済み」を見る。NewHandler から返った時点で建っていれば、
// その後の（指紋を変えない）改竄は**最初の要求から**見えない。
//
// ⚠️ この検査が無いと、「初回要求で建てて以後キャッシュする」遅延構築の実装が
// 素通りする——18 の transition が given に載せているのは「起動時に構築済み」
// であって「いつか構築済み」ではない。
func TestIndexCache_BuiltAtStartup(t *testing.T) {
	h, s := newTestHandler(t) // NewHandler はこの中で呼ばれ、prime() 済み

	path := filepath.Join(s.Dir, "tags", "subject.auth.json")
	writeSameStamp(t, path, func(old []byte) []byte {
		return []byte(strings.Replace(string(old), `"認証"`, `"改竄"`, 1))
	})

	// これがこのハンドラへの**最初の**要求である。
	if got := tagNameFromAPI(t, h, "subject.auth"); got != "認証" {
		t.Errorf("起動時に index が建っていない（name=%q）——最初の要求がディスクを読みに行っている", got)
	}
}

// TestIndexCache_RebuildsOnChange は条項2 の後半「鮮度は落とさない」を見る。
// CLI 相当の外部書き込み・レコードの新設・削除・config の書き換えを、
// それぞれ次の要求が拾うこと。
func TestIndexCache_RebuildsOnChange(t *testing.T) {
	h, s := newTestHandler(t)
	_ = tagNameFromAPI(t, h, "subject.auth") // 1回読ませて建てさせる

	t.Run("既存レコードの書き換え", func(t *testing.T) {
		if err := s.SaveTag(model.Tag{ID: "subject.auth", Name: "認証（改）", Kind: "subject"}); err != nil {
			t.Fatalf("SaveTag: %v", err)
		}
		if got := tagNameFromAPI(t, h, "subject.auth"); got != "認証（改）" {
			t.Errorf("外部の書き換えが次の要求に反映されない: name=%q", got)
		}
	})

	t.Run("レコードの新設", func(t *testing.T) {
		if err := s.SaveTag(model.Tag{ID: "subject.new", Name: "新設", Kind: "subject"}); err != nil {
			t.Fatalf("SaveTag: %v", err)
		}
		if got := tagNameFromAPI(t, h, "subject.new"); got != "新設" {
			t.Errorf("新設レコードが次の要求に出ない: name=%q", got)
		}
	})

	t.Run("レコードの削除", func(t *testing.T) {
		if err := os.Remove(filepath.Join(s.Dir, "tags", "subject.new.json")); err != nil {
			t.Fatalf("remove: %v", err)
		}
		rec := doRequest(t, h, http.MethodGet, "/api/tags", nil)
		for _, tg := range decodeJSON[[]model.Tag](t, rec) {
			if tg.ID == "subject.new" {
				t.Errorf("削除したレコードが次の要求にまだ出る")
			}
		}
	})

	t.Run("config の書き換え", func(t *testing.T) {
		cfg, err := s.LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		cfg.Display.ProductName = "改名した"
		if err := s.SaveConfig(cfg); err != nil {
			t.Fatalf("SaveConfig: %v", err)
		}
		rec := doRequest(t, h, http.MethodGet, "/api/config", nil)
		if !strings.Contains(rec.Body.String(), "改名した") {
			t.Errorf("config の書き換えが次の要求に反映されない: %s", rec.Body.String())
		}
	})
}

// TestIndexCache_ViewerWritesAreVisibleImmediately は viewer 自身の書込
// （PUT /api/config・POST /api/transition）が、**同じプロセスの次の読みに**
// 反映されることを見る。ここが外れると、採用したはずの提案が画面から消える。
func TestIndexCache_ViewerWritesAreVisibleImmediately(t *testing.T) {
	h, _ := newTestHandler(t)
	_ = tagNameFromAPI(t, h, "subject.auth")

	body := []byte(`{"id":"T-cached-new","action":"act.user.login","given":[],"then":["eff.session.issue"],"tags":["req.auth-happy"]}`)
	if rec := doRequest(t, h, http.MethodPost, "/api/transition", body); rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("POST /api/transition: %d %s", rec.Code, rec.Body.String())
	}
	rec := doRequest(t, h, http.MethodGet, "/api/transitions", nil)
	if !strings.Contains(rec.Body.String(), "T-cached-new") {
		t.Errorf("viewer 自身の書込が次の読みに出ない: %s", rec.Body.String())
	}
}

// TestIndexCache_ConcurrentReadsShareSnapshot は、共有された Snapshot / *index.Index
// を複数の要求が同時に持っても壊れないことを見る（`go test -race` と対で意味を持つ）。
//
// ⚠️ **これが無いと、スライスを in-place に並べ替える／append で書き戻す実装が
// 素通りする。** 要求ごとに別の Snapshot を配っていたときは無害だった形が、
// 共有した瞬間に別の要求の応答を壊す。
func TestIndexCache_ConcurrentReadsShareSnapshot(t *testing.T) {
	h, s := newTestHandler(t)

	paths := []string{
		"/api/tags", "/api/vocab", "/api/facets", "/api/transitions",
		"/api/transitions/T-login", "/api/rules", "/api/spec/subject.auth",
		"/api/governs?tag=subject.auth", "/api/traceability", "/api/search?q=login",
		"/api/lint", "/api/config", "/api/reviews",
	}

	var wg sync.WaitGroup
	// 読み側を並べつつ、書き側も同時に走らせる（＝建て直しが読みと重なる）。
	for round := 0; round < 4; round++ {
		for _, p := range paths {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				rec := doRequest(t, h, http.MethodGet, p, nil)
				if rec.Code != http.StatusOK {
					t.Errorf("GET %s: %d %s", p, rec.Code, rec.Body.String())
				}
			}(p)
		}
		wg.Add(1)
		go func(round int) {
			defer wg.Done()
			_ = s.SaveTag(model.Tag{ID: fmt.Sprintf("subject.churn%d", round), Name: "撹拌", Kind: "subject"})
		}(round)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// 本番より上の規模で踏む
// ---------------------------------------------------------------------------

// seedScaledStore は 4 カテゴリすべてを持つストアを組む。
//
// 🔴 **本番の形**（vocab・tags・transitions・decisions の4カテゴリ）で組む。
// 単位AU は `.scholia/tags/` だけの fixture を使い、「カテゴリ2種以上なら旧経路」の
// 変異が**全ゲート緑で、本番では 100% 発火**した。
//
// 🔴 **本番より上の規模**。この repo の `.scholia` は 501 件、利用者の実機は
// その 3.17 倍（≈1,590 件）。ここは 4,200 件で、その上に置く——「N 件までは
// キャッシュする」形の閾値ゲートを標本の中に入れるため。
func seedScaledStore(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	s, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg.Kinds.Action = []string{"user"}
	cfg.Kinds.Effect = []string{"state"}
	cfg.TagKinds = []model.KindDecl{{ID: "subject"}, {ID: "requirement"}}
	cfg.FacetKinds = []string{"subject", "requirement"}
	cfg.TraceabilityKinds = []string{"requirement"}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	const (
		nTags        = 700
		nTransitions = 600
		nVocab       = 1400
		nDecisions   = 1500
	)
	for i := 0; i < nVocab; i++ {
		cat := model.CategoryAction
		if i%2 == 1 {
			cat = model.CategoryEffect
		}
		kind := "user"
		if cat == model.CategoryEffect {
			kind = "state"
		}
		if err := s.SaveVocab(model.VocabEntry{ID: fmt.Sprintf("v.scale%04d", i), Category: cat, Label: fmt.Sprintf("語彙%04d", i), Kind: kind}); err != nil {
			t.Fatalf("SaveVocab: %v", err)
		}
	}
	for i := 0; i < nTags; i++ {
		tg := model.Tag{ID: fmt.Sprintf("subject.scale%04d", i), Name: fmt.Sprintf("主題%04d", i), Kind: "subject"}
		if i > 0 {
			tg.ParentIDs = []string{fmt.Sprintf("subject.scale%04d", i/8)}
		}
		if err := s.SaveTag(tg); err != nil {
			t.Fatalf("SaveTag: %v", err)
		}
	}
	for i := 0; i < nTransitions; i++ {
		if err := s.SaveTransition(model.Transition{
			ID:     fmt.Sprintf("T-scale%04d", i),
			Action: fmt.Sprintf("v.scale%04d", (i*2)%nVocab),
			Then:   []string{fmt.Sprintf("v.scale%04d", (i*2+1)%nVocab)},
			Tags:   []string{fmt.Sprintf("subject.scale%04d", i%nTags)},
		}); err != nil {
			t.Fatalf("SaveTransition: %v", err)
		}
	}
	for i := 0; i < nDecisions; i++ {
		if err := s.CreateDecision(model.Decision{
			ID:     fmt.Sprintf("01SCALEDECISION%011d", i),
			Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: fmt.Sprintf("subject.scale%04d", i%nTags)},
			Why:    fmt.Sprintf("# 規模の標本 %d\n\n本文。\n", i),
			At:     "2026-01-01T00:00:00Z",
		}, store.DecisionCreateOptions{}); err != nil {
			t.Fatalf("CreateDecision: %v", err)
		}
	}

	h, err := NewHandler(s)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h, s
}

// TestIndexCache_AtScale は、上と同じ2つの検査を**本番より上の規模・本番の形**で
// もう一度踏む。件数で分岐する形（「N 件までは建て直す」等）は、小さい fixture
// だけでは標本の外に出る。
func TestIndexCache_AtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("規模の標本は -short では走らせない")
	}
	h, s := seedScaledStore(t)

	const probeTag = "subject.scale0123"
	if got := tagNameFromAPI(t, h, probeTag); got != "主題0123" {
		t.Fatalf("前提が崩れている: name=%q", got)
	}

	path := filepath.Join(s.Dir, "tags", probeTag+".json")
	writeSameStamp(t, path, func(old []byte) []byte {
		return []byte(strings.Replace(string(old), `"主題0123"`, `"主題XXXX"`, 1))
	})
	if got := tagNameFromAPI(t, h, probeTag); got != "主題0123" {
		t.Errorf("この規模ではストアを読み直している（name=%q）", got)
	}

	// 鮮度も同じ規模で見る（キャッシュを効かせた結果、大きいストアで
	// 更新が届かなくなる形を落とす）。
	if err := s.SaveTag(model.Tag{ID: probeTag, Name: "主題ZZZZ", Kind: "subject"}); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}
	if got := tagNameFromAPI(t, h, probeTag); got != "主題ZZZZ" {
		t.Errorf("この規模で鮮度が落ちている: name=%q", got)
	}
}
