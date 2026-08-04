package viewer

import (
	"sync"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/store"
)

// indexCache は `.scholia` の読み込みスナップショットと派生 index を、
// **プロセスの寿命のあいだ持ち続ける**（decision 01KZ5N5CJ2VFMZAGSFPSCZAMTZ
// 条項2・語彙 cond.index-built「起動時に .scholia の in-memory index が構築済み」）。
//
// 以前はここで毎要求 LoadAll＋index.Build していた。理由は「CLI や PUT /api/config
// による同時編集を次の要求に反映する」で、それ自体は要件である。だが実測で、
// タグ 82 件の画面 1 枚が投げる 175 本がそれぞれ 501 ファイルを読み直しており、
// **本数 × 全読み** で画面あたり O(N²) になっていた（同 decision の「なぜ」）。
//
// 鮮度は落とさない。落とす代わりに**変化を見て**建て直す——store.Fingerprint()
// は中身を読まずに stat だけで「いまの .scholia」を表すので、変わっていなければ
// 建てたものをそのまま返し、変わっていれば建て直す。CLI での同時編集も、
// viewer 自身の書込（PUT /api/config・POST /api/decision・POST /api/transition・
// DELETE /api/transitions/{id}）も、次の要求で指紋が変わるので反映される。
//
// ⚠️ **射程**（CLAUDE.md 6）: 建て直しの判断は Fingerprint の射程をそのまま
// 引き継ぐ。捕まえないもの（サイズも mtime も変わらない書き換え等）は
// store/fingerprint.go の doc コメントに書いてある。
//
// ⚠️ **返す Snapshot / *index.Index は共有される（複数の要求が同時に持つ）。**
// どちらも読み取り専用として扱うこと——スライスを in-place に並べ替えたり
// append で書き戻したりすると、別の要求から見えるデータが壊れる。
// これは値として検査してある（snapshot_test.go の同時要求ガード＋`go test -race`）。
type indexCache struct {
	store *store.Store

	mu     sync.Mutex
	loaded bool
	// fingerprint は snap/ix を建てた時点の .scholia の指紋。
	fingerprint string
	snap        store.Snapshot
	ix          *index.Index
}

func newIndexCache(s *store.Store) *indexCache {
	return &indexCache{store: s}
}

// load は現行のスナップショットと index を返す。`.scholia` が前回から変わって
// いなければ、建てたものをそのまま返す（ファイルは1つも読まない）。
func (c *indexCache) load() (store.Snapshot, *index.Index, error) {
	fp, fpErr := c.store.Fingerprint()

	c.mu.Lock()
	defer c.mu.Unlock()

	// 指紋が取れなかったときは「分からない」なので建て直す側へ倒す。
	if fpErr == nil && c.loaded && c.fingerprint == fp {
		return c.snap, c.ix, nil
	}

	snap, err := c.store.LoadAll()
	if err != nil {
		// 建て直しに失敗したら、古いものを配らずエラーを返す（壊れた JSON を
		// 直している最中に、直す前の内容を「現行」として配らないため）。
		c.loaded = false
		return store.Snapshot{}, nil, err
	}
	ix := index.Build(&snap)

	c.snap, c.ix = snap, ix
	if fpErr == nil {
		c.fingerprint, c.loaded = fp, true
	} else {
		c.loaded = false
	}
	return snap, ix, nil
}

// prime は起動時に1回だけ index を建てる（cond.index-built「**起動時に**
// 構築済み」）。失敗しても致命ではない——最初の要求が同じ経路で建て直し、
// そこで初めてエラーを返す（起動を止めると、壊れた1件のせいで viewer が
// 上がらなくなり、直すための画面にも到達できなくなる）。
func (c *indexCache) prime() {
	_, _, _ = c.load()
}

func containsStr(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
