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

// ---------------------------------------------------------------------------
// 正規化（短縮 hash と完全 hash を同じ1件に寄せる）
// ---------------------------------------------------------------------------
//
// # ここが落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:** 新しく足す値が完全 hash へ寄らない／寄せた結果として重複する
// 要素が落ちない／**既存要素が1バイトでも触られる**（append-only 破れ）。
//
// **落ちない:** 解決できない値（git 管理外・git 不在・手元に無い commit）。
// canon が "" を返すので元の値が残る——これは「照合していない」と名乗る領域である。

// fakeCanon は「先頭一致する完全 hash があればそれへ寄せる」解決器。
// git を呼ばずに、短縮 hash の解決だけを再現する。
func fakeCanon(full ...string) Canonicalizer {
	return func(h string) string {
		for _, f := range full {
			if len(h) <= len(f) && f[:len(h)] == h {
				return f
			}
		}
		return ""
	}
}

func TestNormalizeCommits(t *testing.T) {
	const full = "a0d00a36c865c64a094b86d8363a86c88ffd060e"
	canon := fakeCanon(full)

	t.Run("完全のあとに短縮を足しても1件のまま", func(t *testing.T) {
		got := NormalizeCommits([]string{full}, []string{full, full[:8]}, canon)
		if len(got) != 1 || got[0] != full {
			t.Fatalf("同じ commit は1件に畳むべき: %v", got)
		}
	})

	t.Run("短縮のあとに完全を足しても1件のまま", func(t *testing.T) {
		// 既存が短縮で保存されている場合も、比較のためだけに解決して畳む
		//（既存要素そのものは触らない）。
		got := NormalizeCommits([]string{full[:8]}, []string{full[:8], full}, canon)
		if len(got) != 1 || got[0] != full[:8] {
			t.Fatalf("既存要素は触らず、後から来た完全 hash を畳むべき: %v", got)
		}
	})

	t.Run("同一呼び出しの中の短縮と完全も畳む", func(t *testing.T) {
		got := NormalizeCommits(nil, []string{full[:8], full}, canon)
		if len(got) != 1 || got[0] != full {
			t.Fatalf("1件（完全 hash）になるべき: %v", got)
		}
	})

	t.Run("新しく足す値は完全 hash へ寄る", func(t *testing.T) {
		got := NormalizeCommits(nil, []string{full[:8]}, canon)
		if len(got) != 1 || got[0] != full {
			t.Fatalf("完全 hash へ寄せるべき: %v", got)
		}
	})

	t.Run("既存要素は1バイトも触らない（append-only）", func(t *testing.T) {
		// 既存が短縮でも、寄せ直したら既存要素の改変＝append-only 破れになる。
		got := NormalizeCommits([]string{full[:8]}, []string{full[:8], "1234567"}, canon)
		if len(got) != 2 || got[0] != full[:8] {
			t.Fatalf("既存要素の値も位置も変えてはいけない: %v", got)
		}
	})

	t.Run("解決できない値はそのまま残る", func(t *testing.T) {
		got := NormalizeCommits(nil, []string{"1234567", "89abcde"}, canon)
		if len(got) != 2 || got[0] != "1234567" || got[1] != "89abcde" {
			t.Fatalf("解決できない値は渡されたまま: %v", got)
		}
	})

	t.Run("canon が nil でも壊れない（git を知らない呼び出し）", func(t *testing.T) {
		got := NormalizeCommits(nil, []string{"1234567"}, nil)
		if len(got) != 1 || got[0] != "1234567" {
			t.Fatalf("nil canon では何も寄せない: %v", got)
		}
	})
}

func TestNormalizeAppliedMarks(t *testing.T) {
	const full = "a0d00a36c865c64a094b86d8363a86c88ffd060e"
	const at = "2026-08-18T00:00:00Z"
	canon := fakeCanon(full)
	mark := func(h string) AppliedMark {
		return AppliedMark{Kind: AppliedCorrection, At: at, Commit: h}
	}

	t.Run("同じ commit を指す是正の印は1件に畳む", func(t *testing.T) {
		prev := []AppliedMark{mark(full)}
		got := NormalizeAppliedMarks(prev, append(append([]AppliedMark(nil), prev...), mark(full[:8])), canon)
		if len(got) != 1 {
			t.Fatalf("同じ commit の是正は1件のはず: %+v", got)
		}
	})

	t.Run("新しい印の commit は完全 hash へ寄る", func(t *testing.T) {
		got := NormalizeAppliedMarks(nil, []AppliedMark{mark(full[:8])}, canon)
		if len(got) != 1 || got[0].Commit != full {
			t.Fatalf("完全 hash へ寄せるべき: %+v", got)
		}
	})

	t.Run("既存の印は1バイトも触らない", func(t *testing.T) {
		prev := []AppliedMark{mark(full[:8])}
		got := NormalizeAppliedMarks(prev, append(append([]AppliedMark(nil), prev...), mark("1234567")), canon)
		if len(got) != 2 || got[0].Commit != full[:8] {
			t.Fatalf("既存の印の値も位置も変えてはいけない: %+v", got)
		}
	})

	t.Run("commit を持たない印（矛盾・却下）は素通り", func(t *testing.T) {
		conflict := AppliedMark{Kind: AppliedConflict, At: at}
		got := NormalizeAppliedMarks(nil, []AppliedMark{conflict, conflict}, canon)
		// 指し先の無い矛盾には畳む鍵が無い——2件のまま（既存の仕様どおり）。
		if len(got) != 2 {
			t.Fatalf("指し先の無い矛盾は畳めない: %+v", got)
		}
	})
}
