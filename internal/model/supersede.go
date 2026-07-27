package model

import "fmt"

// 現行性リンク（supersedes・#45 D7）の検証は、decision を作る／編集する全ての面
// （`decide --supersedes`・`decision link`・`review adopt`・viewer の
// `POST /api/decision`）で同一でなければならない。面ごとに書き分けると「CLI では
// 弾かれるのに viewer では通る」宙吊りリンクが生まれる。internal/cli は
// internal/viewer を import する（`scholia view`/`export`）ため逆向きの import は
// 循環する——そこで両者が既に依存している model にモデル層の検証だけを置き、
// 各面はここを呼ぶ。CLI 構文 "<ulid>[:<mode>]" の解析だけは CLI 側に残す
// （viewer は構造化 JSON を受け取るので解析が要らない）。

// ValidateSupersedeTargets は各 link の旧 decision が実在するかを検査する
// （実在照合・#45 D7）。
func ValidateSupersedeTargets(all []Decision, links []SupersedeLink) error {
	if len(links) == 0 {
		return nil
	}
	exists := make(map[string]bool, len(all))
	for _, d := range all {
		exists[d.ID] = true
	}
	for _, l := range links {
		if !exists[l.ID] {
			return fmt.Errorf("supersedes: 旧 decision %q が実在しません", l.ID)
		}
	}
	return nil
}

// AppendSupersedeLinks は existing に candidates を追記し、追加分のみ返す。
// 既存と同一 {id, mode} は冪等 skip・同一 id で mode 違いは error（既存 link の
// 改変＝append-only 破れ）。
func AppendSupersedeLinks(existing, candidates []SupersedeLink) (added []SupersedeLink, err error) {
	byID := make(map[string]SupersedeLink, len(existing))
	for _, l := range existing {
		byID[l.ID] = l
	}
	addedIDs := make(map[string]bool)
	for _, c := range candidates {
		if prev, ok := byID[c.ID]; ok {
			if prev.SupersedeMode() == c.SupersedeMode() {
				continue // 冪等 skip
			}
			return nil, fmt.Errorf("supersedes: 既存 link %s の mode（%s）を %s へ改変することはできません（追記専用・link は append-only）",
				c.ID, prev.SupersedeMode(), c.SupersedeMode())
		}
		if addedIDs[c.ID] {
			continue
		}
		addedIDs[c.ID] = true
		added = append(added, c)
	}
	return added, nil
}

// SupersedeCreatesCycle は「newID の supersedes に candidate 群を足すと、
// decision の supersede 有向グラフ（新→旧）に閉路ができるか」を返す（#45 D7）。
// all は現在の全 decision（newID 自身を含む）。新規作成では newID が未保存なので
// 閉路は構造的に起きないが、link は既存 decision を編集するため検査が要る。
func SupersedeCreatesCycle(all []Decision, newID string, candidates []SupersedeLink) bool {
	// 隣接リスト（id → supersede 先 id 群）を組み、candidates を newID に足す。
	adj := make(map[string][]string, len(all)+1)
	for _, d := range all {
		for _, l := range d.Supersedes {
			adj[d.ID] = append(adj[d.ID], l.ID)
		}
	}
	for _, l := range candidates {
		adj[newID] = append(adj[newID], l.ID)
	}
	// newID から DFS して newID に戻れれば閉路（自己参照は parse 段で弾き済みだが
	// 多段の閉路 A→B→A をここで拾う）。
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	var hasCycle bool
	var visit func(string)
	visit = func(u string) {
		color[u] = gray
		for _, v := range adj[u] {
			switch color[v] {
			case gray:
				hasCycle = true
			case white:
				visit(v)
			}
		}
		color[u] = black
	}
	visit(newID)
	return hasCycle
}
