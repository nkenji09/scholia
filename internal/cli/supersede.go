package cli

import (
	"strings"

	"github.com/nkenji09/scholia/internal/model"
)

// parseSupersedeLinks は "<oldUlid>[:<mode>]" 群を SupersedeLink へ解析する
// （#45 D7）。ここが担うのは CLI 構文の分解だけで、集合としての不変条件
// （mode 3値・自己参照禁止・重複禁止）は model.NormalizeSupersedeLinks に渡す
// ——viewer の構造化 JSON も review が持ってきた宣言も、同じ1関数を通す。
func parseSupersedeLinks(specs []string, selfID string) ([]model.SupersedeLink, error) {
	links := make([]model.SupersedeLink, 0, len(specs))
	for _, spec := range specs {
		id, mode := parseSupersedeSpec(spec)
		links = append(links, model.SupersedeLink{ID: id, Mode: mode})
	}
	return model.NormalizeSupersedeLinks(links, selfID)
}

// parseSupersedeSpec は "<id>[:<mode>]" を分解する。mode 省略時は "" を返す
// （保存は "" のまま——derive 側で amend として補完する・model.SupersedeMode）。
// 値の妥当性は判定しない（呼び出し元が NormalizeSupersedeLinks へ渡す）。
func parseSupersedeSpec(spec string) (id, mode string) {
	parts := strings.SplitN(spec, ":", 2)
	id = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		mode = strings.TrimSpace(parts[1])
	}
	return id, mode
}

// 集合の正規化・実在照合・追記マージ・閉路検査は
// model.NormalizeSupersedeLinks / ValidateSupersedeTargets /
// AppendSupersedeLinks / SupersedeCreatesCycle にある——viewer の
// POST /api/decision も同じ検証を通す必要があり、internal/viewer から
// internal/cli は import できない（逆向きに依存している）ため。ここに残すのは
// CLI 構文の分解（上）と derive（下）だけ。

// supersededIDs は「mode=supersede で他 decision から指された decision の id 集合」
// を返す（#45 D7・derive は保守的に supersede のみ失効扱い）。amend/exception は
// 旧を失効させない。superseded-by の逆リンクもここから derive できる。
func supersededIDs(all []model.Decision) map[string]bool {
	out := make(map[string]bool)
	for _, d := range all {
		for _, l := range d.Supersedes {
			if l.SupersedeMode() == model.ModeSupersede {
				out[l.ID] = true
			}
		}
	}
	return out
}

// supersededByIndex は id → その id を指した (fromID, mode) 群、の逆索引を返す
// （superseded-by バッジ・decision show 用の derive・保存しない）。
type supersededByRef struct {
	FromID string
	Mode   string
}

func supersededByIndex(all []model.Decision) map[string][]supersededByRef {
	out := make(map[string][]supersededByRef)
	for _, d := range all {
		for _, l := range d.Supersedes {
			out[l.ID] = append(out[l.ID], supersededByRef{FromID: d.ID, Mode: l.SupersedeMode()})
		}
	}
	return out
}
