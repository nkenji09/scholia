package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

// 書き込みの最中にできる一時ファイル（writeJSONAtomic の `.tmp-*.json`）を
// レコードとして読まないこと。
//
// ⚠️ **これは「書きながら読む」ときにだけ起きる。** 単発の LoadAll では一時ファイルは
// 既に rename されているので、その形の検査では原理的に捕まらない。だから
// 「rename される直前の状態」を手で作って**値として**見る。
//
// この欠陥は `internal/viewer` の同時要求ガード（`go test -race`）が引き当てた。
// 症状は2つあり、どちらもここで固定する。
func TestListRecords_IgnoresInFlightTempFiles(t *testing.T) {
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	for _, tg := range []model.Tag{{ID: "a", Name: "A", Kind: "k"}, {ID: "b", Name: "B", Kind: "k"}} {
		if err := s.SaveTag(tg); err != nil {
			t.Fatalf("SaveTag: %v", err)
		}
	}

	// 4カテゴリすべてに「rename される直前」の一時ファイルを置く。
	// 1カテゴリだけ見る形にすると、絞りをそのカテゴリにだけ効かせる実装が素通りする。
	for _, sub := range []string{vocabDir, tagsDir, transitionsDir, decisionsDir} {
		tmp := filepath.Join(s.Dir, sub, ".tmp-inflight.json")
		if err := os.WriteFile(tmp, []byte(`{"id":"phantom"}`), 0o644); err != nil {
			t.Fatalf("一時ファイルの用意: %v", err)
		}
	}

	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("一時ファイルがあると LoadAll が落ちる: %v", err)
	}
	for _, tg := range snap.Tags {
		if tg.ID == "phantom" {
			t.Errorf("一時ファイルが幻のレコードとして読み込まれた（tag）")
		}
	}
	for _, v := range snap.Vocab {
		if v.ID == "phantom" {
			t.Errorf("一時ファイルが幻のレコードとして読み込まれた（vocab）")
		}
	}
	for _, tx := range snap.Transitions {
		if tx.ID == "phantom" {
			t.Errorf("一時ファイルが幻のレコードとして読み込まれた（transition）")
		}
	}
	for _, d := range snap.Decisions {
		if d.ID == "phantom" {
			t.Errorf("一時ファイルが幻のレコードとして読み込まれた（decision）")
		}
	}
	if len(snap.Tags) != 2 {
		t.Errorf("タグが 2 件のはずが %d 件", len(snap.Tags))
	}
}

// 指紋も同じ絞りを通ること。ここが広いと、読んでいないもの（一時ファイル）の
// 出入りで指紋が変わり、無駄な建て直しが起きる。
func TestFingerprint_IgnoresInFlightTempFiles(t *testing.T) {
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveTag(model.Tag{ID: "a", Name: "A", Kind: "k"}); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}
	before, err := s.Fingerprint()
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	tmp := filepath.Join(s.Dir, tagsDir, ".tmp-inflight.json")
	if err := os.WriteFile(tmp, []byte(`{"id":"phantom"}`), 0o644); err != nil {
		t.Fatalf("一時ファイルの用意: %v", err)
	}
	after, err := s.Fingerprint()
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if before != after {
		t.Errorf("一時ファイルの出入りで指紋が変わっている（読んでいないものの変化で建て直すことになる）")
	}
}

// 書きながら読んでも LoadAll が失敗しないこと（元の症状そのもの）。
//
// ⚠️ **これは確率的な検査である。** 一時ファイルが ReadDir と ReadFile の
// 間に消える窓は狭いので、緑になったからといって窓が無いとは言えない。
// 決定的な検査は上の2つが持ち、ここは「実際の書き込み経路で踏んでも落ちない」
// ことを見るだけ（射程・CLAUDE.md 6）。
func TestLoadAll_WhileWriting(t *testing.T) {
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	for i := 0; i < 20; i++ {
		if err := s.SaveTag(model.Tag{ID: string(rune('a'+i)) + "-seed", Name: "seed", Kind: "k"}); err != nil {
			t.Fatalf("SaveTag: %v", err)
		}
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = s.SaveTag(model.Tag{ID: "churn", Name: "撹拌", Kind: "k"})
		}
	}()
	for i := 0; i < 300; i++ {
		if _, err := s.LoadAll(); err != nil {
			close(stop)
			wg.Wait()
			t.Fatalf("書き込みと同時に読むと LoadAll が失敗する: %v", err)
		}
	}
	close(stop)
	wg.Wait()
}
