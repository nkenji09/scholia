package model

import (
	"reflect"
	"strings"
	"testing"
)

// NormalizeSupersedeLinks は link 集合の不変条件（mode 3値・id 非空・自己参照
// 禁止・重複禁止）を1箇所で担う。CLI の "<ulid>[:<mode>]" 解析・viewer の
// 構造化 JSON・review が持ってきた宣言の3経路がここを通るので、ここが緩むと
// 3経路すべてが同時に緩む。
func TestNormalizeSupersedeLinks(t *testing.T) {
	cases := []struct {
		name    string
		links   []SupersedeLink
		selfID  string
		want    []SupersedeLink
		wantErr string // 部分一致。空なら成功を期待する
	}{
		{
			name:  "空集合は nil",
			links: nil,
			want:  nil,
		},
		{
			name:   "3値と mode 省略（既定 amend）は通る",
			links:  []SupersedeLink{{ID: "a", Mode: ModeSupersede}, {ID: "b", Mode: ModeAmend}, {ID: "c", Mode: ModeException}, {ID: "d"}},
			selfID: "self",
			want:   []SupersedeLink{{ID: "a", Mode: ModeSupersede}, {ID: "b", Mode: ModeAmend}, {ID: "c", Mode: ModeException}, {ID: "d"}},
		},
		{
			name:    "3値でない mode は拒否",
			links:   []SupersedeLink{{ID: "a", Mode: "bogus"}},
			wantErr: "supersede|amend|exception",
		},
		{
			// レビュー指摘の実害シナリオ: SupersedeMode() は未知の mode を
			// そのまま返し、--current は mode=="supersede" だけを畳むので、
			// 綴り誤りが通ると旧 decision が現行のまま残る。
			name:    "綴り誤り supersedes は拒否",
			links:   []SupersedeLink{{ID: "a", Mode: "supersedes"}},
			wantErr: "supersede|amend|exception",
		},
		{
			name:    "空 id は拒否",
			links:   []SupersedeLink{{ID: "", Mode: ModeAmend}},
			wantErr: "id が空です",
		},
		{
			name:    "自己参照は拒否",
			links:   []SupersedeLink{{ID: "self", Mode: ModeAmend}},
			selfID:  "self",
			wantErr: "自分自身",
		},
		{
			name:    "同一 id の重複は拒否（mode 違い）",
			links:   []SupersedeLink{{ID: "a", Mode: ModeAmend}, {ID: "a", Mode: ModeSupersede}},
			wantErr: "重複指定",
		},
		{
			name:    "同一 id の重複は拒否（mode 同一でも）",
			links:   []SupersedeLink{{ID: "a", Mode: ModeAmend}, {ID: "a", Mode: ModeAmend}},
			wantErr: "重複指定",
		},
		{
			// selfID が空＝指す側の id がまだ無い場面（提案時の宣言）。
			// 自己参照検査だけを飛ばし、他の検査は効かせる。
			name:  "selfID が空なら自己参照検査は飛ばす",
			links: []SupersedeLink{{ID: "a", Mode: ModeAmend}},
			want:  []SupersedeLink{{ID: "a", Mode: ModeAmend}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeSupersedeLinks(tc.links, tc.selfID)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("エラーを期待したが nil（got = %+v）", got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %q, want contains %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got = %+v, want %+v", got, tc.want)
			}
		})
	}
}
