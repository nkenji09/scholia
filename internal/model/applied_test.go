package model

import "testing"

// 印1件が満たすべき不変条件を、**入力と出力の対**で検査する
// （CLAUDE.md「配線ガードの書き方」1）。画面も git もファイルも起こさない。
//
// # ここが落とす範囲（CLAUDE.md 6）
//
// **落ちる:** 種別が3値でない／時刻が空／種別と指し先の組み合わせが噛み合って
// いない（是正に commit が無い・是正に decision の指し先がある・矛盾や却下に
// commit がある・却下に指し先が無い）／自分自身を指す。
//
// **落ちない:** 「その印が事実か」——commit が本当にその decision の是正か、
// 記録が本当に結論を決めたか。ここは形しか見ない（決定本文の「落とせない」節）。
func TestValidateAppliedMark(t *testing.T) {
	const self = "01SELF"
	const at = "2026-08-18T00:00:00Z"

	cases := []struct {
		name    string
		mark    AppliedMark
		wantErr string // "" なら通ること
	}{
		{"是正: commit を指す", AppliedMark{Kind: AppliedCorrection, At: at, Commit: "abc1234"}, ""},
		{"矛盾: 指し先あり", AppliedMark{Kind: AppliedConflict, At: at, Decision: "01OTHER"}, ""},
		{"矛盾: 何も着地しなかった（指し先なし）", AppliedMark{Kind: AppliedConflict, At: at}, ""},
		{"却下: 却下を記録した decision を指す", AppliedMark{Kind: AppliedRejection, At: at, Decision: "01OTHER"}, ""},

		{"種別が3値でない", AppliedMark{Kind: "adopted", At: at}, AppliedErrInvalidKind},
		{"種別が空", AppliedMark{At: at}, AppliedErrInvalidKind},
		{"時刻が空", AppliedMark{Kind: AppliedConflict}, AppliedErrMissingAt},
		{"是正なのに commit が無い", AppliedMark{Kind: AppliedCorrection, At: at}, AppliedErrMissingCommit},
		{"是正なのに decision を指す", AppliedMark{Kind: AppliedCorrection, At: at, Commit: "abc1234", Decision: "01OTHER"}, AppliedErrUnexpectedTarget},
		{"矛盾なのに commit を持つ", AppliedMark{Kind: AppliedConflict, At: at, Commit: "abc1234"}, AppliedErrUnexpectedCommit},
		{"却下なのに commit を持つ", AppliedMark{Kind: AppliedRejection, At: at, Commit: "abc1234", Decision: "01OTHER"}, AppliedErrUnexpectedCommit},
		{"却下なのに指し先が無い", AppliedMark{Kind: AppliedRejection, At: at}, AppliedErrMissingDecision},
		{"自分自身を指す", AppliedMark{Kind: AppliedConflict, At: at, Decision: self}, AppliedErrSelfReference},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateAppliedMark(c.mark, self)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("通るべき印が落ちた: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("落ちるべき印が通った: %+v", c.mark)
			}
			ae, ok := err.(*AppliedError)
			if !ok {
				t.Fatalf("AppliedError であるべき: %T %v", err, err)
			}
			if ae.Kind != c.wantErr {
				t.Fatalf("違反の種類が違う: want=%s got=%s（%v）", c.wantErr, ae.Kind, err)
			}
			if ae.Error() == "" {
				t.Fatalf("文言が空（何を直せばよいか読めない）")
			}
		})
	}
}

// selfID が空のときは自己参照検査を飛ばす（指す側の id がまだ決まっていない場面）。
func TestValidateAppliedMark_SkipsSelfCheckWithoutSelfID(t *testing.T) {
	m := AppliedMark{Kind: AppliedConflict, At: "2026-08-18T00:00:00Z", Decision: "01ANY"}
	if err := ValidateAppliedMark(m, ""); err != nil {
		t.Fatalf("selfID 無しでは自己参照を見ないはず: %v", err)
	}
}

// 指し先の decision が実在すること（decide --supersedes と同型）。
func TestValidateAppliedTargets(t *testing.T) {
	all := []Decision{{ID: "01A"}, {ID: "01B"}}
	at := "2026-08-18T00:00:00Z"

	ok := []AppliedMark{
		{Kind: AppliedRejection, At: at, Decision: "01A"},
		{Kind: AppliedConflict, At: at},                      // 指し先なしは素通り
		{Kind: AppliedCorrection, At: at, Commit: "abc1234"}, // commit は照合対象外
	}
	if err := ValidateAppliedTargets(all, ok); err != nil {
		t.Fatalf("実在する指し先が落ちた: %v", err)
	}

	bad := []AppliedMark{{Kind: AppliedRejection, At: at, Decision: "01MISSING"}}
	err := ValidateAppliedTargets(all, bad)
	if err == nil {
		t.Fatal("実在しない指し先は落ちるべき")
	}
	if ae, okAs := err.(*AppliedError); !okAs || ae.Kind != AppliedErrMissingTarget {
		t.Fatalf("missing-target であるべき: %v", err)
	}
}

// 重複は指し先で畳む。**指し先を持たない矛盾は畳めない**——これは仕様であって
// 不具合ではないので、畳まれないことを検査に書く（決定本文の「落とせない」節）。
func TestAppendAppliedMarks_DedupesByTarget(t *testing.T) {
	at := "2026-08-18T00:00:00Z"
	existing := []AppliedMark{
		{Kind: AppliedCorrection, At: at, Commit: "abc1234"},
		{Kind: AppliedRejection, At: at, Decision: "01A"},
		{Kind: AppliedConflict, At: at},
	}

	// 同じ commit・同じ decision は冪等 skip（時刻が違っても同じ出来事）。
	added := AppendAppliedMarks(existing, []AppliedMark{
		{Kind: AppliedCorrection, At: "2026-09-01T00:00:00Z", Commit: "abc1234"},
		{Kind: AppliedRejection, At: "2026-09-01T00:00:00Z", Decision: "01A"},
	})
	if len(added) != 0 {
		t.Fatalf("指し先が同じ印は畳むべき: %+v", added)
	}

	// 種別が違えば別の出来事（同じ decision を「矛盾」でも指しうる）。
	added = AppendAppliedMarks(existing, []AppliedMark{{Kind: AppliedConflict, At: at, Decision: "01A"}})
	if len(added) != 1 {
		t.Fatalf("種別が違う印は畳まないべき: %+v", added)
	}

	// ⚠️ 指し先の無い矛盾には畳む鍵が無い。2回打てば2件になる。
	added = AppendAppliedMarks(existing, []AppliedMark{{Kind: AppliedConflict, At: at}})
	if len(added) != 1 {
		t.Fatalf("指し先の無い矛盾は畳めない（畳んだら、それは鍵を捏造している）: %+v", added)
	}

	// 同一呼び出しの中の重複も畳む。
	added = AppendAppliedMarks(nil, []AppliedMark{
		{Kind: AppliedCorrection, At: at, Commit: "def5678"},
		{Kind: AppliedCorrection, At: at, Commit: "def5678"},
	})
	if len(added) != 1 {
		t.Fatalf("同一呼び出し内の重複も畳むべき: %+v", added)
	}
}

// 3値の一覧は書き写さず1か所から引く（CLI のフラグ説明・使用記録の分類表が
// ここを読む）。数え上げが空なら、それらの表は何も宣言していないことになる。
func TestAppliedKinds(t *testing.T) {
	kinds := AppliedKinds()
	if len(kinds) != 3 {
		t.Fatalf("3値のはず: %v", kinds)
	}
	for _, k := range kinds {
		if !ValidAppliedKind(k) {
			t.Fatalf("一覧に載っているのに妥当でない: %s", k)
		}
	}
	if ValidAppliedKind("refinement") {
		t.Fatal("精緻化は applied[] の種別ではない（記録の側が変わるので印が要らない）")
	}
}

func TestCountApplied(t *testing.T) {
	at := "2026-08-18T00:00:00Z"
	marks := []AppliedMark{
		{Kind: AppliedCorrection, At: at, Commit: "abc1234"},
		{Kind: AppliedCorrection, At: at, Commit: "def5678"},
		{Kind: AppliedConflict, At: at},
	}
	if got := CountApplied(marks, AppliedCorrection); got != 2 {
		t.Fatalf("是正は2件のはず: %d", got)
	}
	if got := CountApplied(marks, AppliedRejection); got != 0 {
		t.Fatalf("却下は0件のはず: %d", got)
	}
}
