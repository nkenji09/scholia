package index

import (
	"github.com/nkenji09/scholia/internal/store"
)

// 一括取得の口が使う組み立て（decision 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1
// 「1画面が投げる要求の本数を、レコード件数に比例させない」）。
//
// ここに置くのは、**live の一括エンドポイントと静的書き出しが同じ1つの核を
// 通る**ようにするため。`scholia export --html` は前から全レコードぶんの map を
// 焼いていて（internal/render/export.go）、その形と live の答えが割れると
// 「この記録を支配する規則は何か」に面ごとに違う答えが返る余地が復活する
// （面間整合原則 D10b-2・01KYKS4Y56FAHRVCWKMQJK4RT6 条項5）。
//
// 中身は 1件用の GovernsFor*/BuildTransitionDetail をそのまま回しているだけで、
// 選択規則をここに再実装していない。

// AllGoverns は tag / transition / vocab の全レコードについて、それを支配する
// decision の参照を record ref（"tag:<id>" 等）でひいた map を返す。
// key の綴りは静的書き出しの焼き込み（staticData.Governs）と同一。
func AllGoverns(snap *store.Snapshot, ix *Index) (map[string][]GovernsRef, error) {
	// 表は1回だけ建てて使い回す（1件用の口は毎回自分で建てる・snapLookups の doc）。
	lk := newSnapLookups(snap)
	out := make(map[string][]GovernsRef, len(snap.Tags)+len(ix.TransitionByID)+len(snap.Vocab))
	for _, t := range snap.Tags {
		entries, err := governsForTagWith(snap, lk, t.ID)
		if err != nil {
			return nil, err
		}
		out["tag:"+t.ID] = RefsOf(entries)
	}
	for _, t := range ix.AllTransitions() {
		entries, err := governsForTransitionWith(snap, lk, t.ID)
		if err != nil {
			return nil, err
		}
		out["transition:"+t.ID] = RefsOf(entries)
	}
	for _, v := range snap.Vocab {
		entries, err := governsForVocabWith(snap, lk, v.ID)
		if err != nil {
			return nil, err
		}
		out["vocab:"+v.ID] = RefsOf(entries)
	}
	return out, nil
}

// AllTransitionDetails は全 transition の詳細を id でひいた map を返す。
// 1件用の BuildTransitionDetail をそのまま回す（形は
// `GET /api/transitions/{id}` の応答と同一）。
func AllTransitionDetails(snap *store.Snapshot, ix *Index) (map[string]TransitionDetail, error) {
	lk := newSnapLookups(snap)
	out := make(map[string]TransitionDetail, len(ix.TransitionByID))
	for _, t := range ix.AllTransitions() {
		detail, ok, err := buildTransitionDetailWith(snap, lk, ix, t.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			out[t.ID] = detail
		}
	}
	return out, nil
}
