package cli

import (
	"fmt"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
)

// parseSupersedeLinks は "<oldUlid>[:<mode>]" 群を SupersedeLink へ解析する
// （#45 D7）。selfID への自己参照は拒否・mode は3値検証・重複 id は最後の指定を
// 採る前に error にはしない（呼び出し側の validate で重複検査する）。
func parseSupersedeLinks(specs []string, selfID string) ([]model.SupersedeLink, error) {
	var out []model.SupersedeLink
	seen := make(map[string]bool)
	for _, spec := range specs {
		id, mode, err := parseSupersedeSpec(spec)
		if err != nil {
			return nil, err
		}
		if id == selfID {
			return nil, fmt.Errorf("supersedes: decision は自分自身（%s）を supersede できません", selfID)
		}
		if seen[id] {
			return nil, fmt.Errorf("supersedes: 旧 decision %q が重複指定されています", id)
		}
		seen[id] = true
		out = append(out, model.SupersedeLink{ID: id, Mode: mode})
	}
	return out, nil
}

// parseSupersedeSpec は "<id>[:<mode>]" を分解する。mode 省略時は "" を返す
// （保存は "" のまま——derive 側で amend として補完する・model.SupersedeMode）。
func parseSupersedeSpec(spec string) (id, mode string, err error) {
	parts := strings.SplitN(spec, ":", 2)
	id = strings.TrimSpace(parts[0])
	if id == "" {
		return "", "", fmt.Errorf("supersedes: id が空です（%q）", spec)
	}
	if len(parts) == 2 {
		mode = strings.TrimSpace(parts[1])
	}
	if !model.ValidSupersedeMode(mode) {
		return "", "", fmt.Errorf("supersedes: mode %q は supersede|amend|exception のいずれかである必要があります（%q）", mode, spec)
	}
	return id, mode, nil
}

// 実在照合・追記マージ・閉路検査は model.ValidateSupersedeTargets /
// model.AppendSupersedeLinks / model.SupersedeCreatesCycle にある——viewer の
// POST /api/decision も同じ検証を通す必要があり、internal/viewer から
// internal/cli は import できない（逆向きに依存している）ため。ここに残すのは
// CLI 構文の解析（上）と derive（下）だけ。

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
