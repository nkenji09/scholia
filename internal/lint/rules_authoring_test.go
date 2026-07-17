package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// --- derived-value-in-desc ---

func TestDerivedValueInDescDetectsCondIDsEnumAndTotal(t *testing.T) {
	snap := store.Snapshot{
		Config: model.DefaultConfig(),
		Tags: []model.Tag{
			{ID: "axis.a", Kind: "axis", Description: "分岐を束ねる軸。値＝{cond.x, cond.y}。total=true。"},
			{ID: "axis.clean", Kind: "axis", Description: "次元名だけのきれいな軸。"},
			{ID: "req.r", Kind: "requirement", Description: "値＝{列挙してもよい} requirement は対象外。"},
		},
		Vocab: []model.VocabEntry{
			{ID: "cond.x", Category: model.CategoryCondition, Label: "x", Tags: []string{"axis.a"}},
			{ID: "cond.y", Category: model.CategoryCondition, Label: "y", Tags: []string{"axis.a"}},
		},
	}
	findings := checkDerivedValueInDesc(snap)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (axis.a only), got %+v", findings)
	}
	f := findings[0]
	if f.Target != "axis.a" || f.TargetType != "tag" || f.Field != "description" {
		t.Fatalf("unexpected finding location: %+v", f)
	}
	if f.Severity != SeverityInfo || f.Tier != TierAdvisory || f.AcknowledgeOnly {
		t.Fatalf("advisory 区分が違う: %+v", f)
	}
	for _, want := range []string{"cond.x", "cond.y", "値＝{", "total=true"} {
		if !strings.Contains(f.Quote, want) {
			t.Fatalf("quote missing %q: %+v", want, f)
		}
	}
}

// --- stale-tense ---

func TestStaleTenseScansDescOnlyAndHonorsConfigExcludes(t *testing.T) {
	snap := store.Snapshot{
		Config: model.DefaultConfig(),
		Tags: []model.Tag{
			{ID: "req.a", Kind: "requirement", Description: "現状は未実装（#12・rev3）。"},
		},
		Vocab: []model.VocabEntry{
			// label は runtime 状態を書く欄なので走査しない
			{ID: "cond.b", Category: model.CategoryCondition, Label: "現状まだ存在しないとき", Description: "新設の条件。"},
		},
		Decisions: []model.Decision{
			// why は時点の判断を書く履歴欄なので走査しない
			{ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", Target: model.DecisionTarget{Type: "tag", ID: "req.a"}, Why: "現状こうするのが最善と判断した（#45）"},
		},
	}
	findings := checkStaleTense(snap)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (req.a desc + cond.b desc), got %+v", findings)
	}
	if findings[0].Target != "req.a" || findings[1].Target != "cond.b" {
		t.Fatalf("unexpected targets: %+v", findings)
	}
	for _, want := range []string{"現状", "未実装", "#12", "rev3"} {
		if !strings.Contains(findings[0].Quote, want) {
			t.Fatalf("quote missing %q: %+v", want, findings[0])
		}
	}

	// config.lint.stalePatternExcludes: 検出語がマッチしたら除外する
	snap.Config.Lint = &model.LintConfig{StalePatternExcludes: []string{`^#\d+$`, `^rev\d$`, `^未実装$`, `^現状$`}}
	findings = checkStaleTense(snap)
	if len(findings) != 1 || findings[0].Target != "cond.b" {
		t.Fatalf("excludes should drop req.a entirely, got %+v", findings)
	}
}

// --- prose-ref ---

func TestProseRefDetectsMetaDirectiveButNotDomainUsage(t *testing.T) {
	snap := store.Snapshot{
		Config: model.DefaultConfig(),
		Tags: []model.Tag{
			{ID: "req.hit1", Kind: "requirement", Description: "詳細は設計文書を参照。"},
			{ID: "req.hit2", Kind: "requirement", Description: "背景（decision を参照）に基づく。"},
			{ID: "req.hit3", Kind: "requirement", Description: "経緯は履歴を参照してください"},
			{ID: "req.ok", Kind: "requirement", Description: "タグ参照の整合を保つ。req.a が参照する語彙。参照先を辿る。値を参照する仕組み。参照 vocab の解決。"},
		},
	}
	findings := checkProseRef(snap)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %+v", findings)
	}
	targets := []string{findings[0].Target, findings[1].Target, findings[2].Target}
	if targets[0] != "req.hit1" || targets[1] != "req.hit2" || targets[2] != "req.hit3" {
		t.Fatalf("unexpected targets %v", targets)
	}
}

// --- why-file-line ---

func TestWhyFileLineDecisionFieldsAreAcknowledgeOnly(t *testing.T) {
	snap := store.Snapshot{
		Config: model.DefaultConfig(),
		Decisions: []model.Decision{
			{ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", Target: model.DecisionTarget{Type: "tag", ID: "req.a"},
				Why: "internal/flow/text.go:58 の分岐を直した", Changed: "web/src/app.tsx:12 を更新"},
			{ID: "01BBBBBBBBBBBBBBBBBBBBBBBB", Target: model.DecisionTarget{Type: "tag", ID: "req.a"},
				Why: "analyze.go を全面に見直した（行番号なしは対象外）"},
		},
		Tags: []model.Tag{
			{ID: "req.a", Kind: "requirement", Description: "internal/lint/lint.go:104 が根拠。"},
		},
	}
	findings := checkWhyFileLine(snap)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %+v", findings)
	}
	d := findings[0]
	if d.Target != "01AAAAAAAAAAAAAAAAAAAAAAAA" || !d.AcknowledgeOnly || d.Field != "why・changed" {
		t.Fatalf("decision finding wrong: %+v", d)
	}
	if !strings.Contains(d.Quote, "internal/flow/text.go:58") || !strings.Contains(d.Quote, "web/src/app.tsx:12") {
		t.Fatalf("decision quote wrong: %+v", d)
	}
	tag := findings[1]
	if tag.Target != "req.a" || tag.AcknowledgeOnly || tag.Field != "description" {
		t.Fatalf("tag finding wrong: %+v", tag)
	}
}

// --- axis-without-decision ---

func TestAxisWithoutDecisionFlagsOnlyUndecidedAxisTags(t *testing.T) {
	snap := store.Snapshot{
		Config: model.DefaultConfig(),
		Tags: []model.Tag{
			{ID: "axis.decided", Kind: "axis"},
			{ID: "axis.bare", Kind: "axis"},
			{ID: "req.bare", Kind: "requirement"},
		},
		Decisions: []model.Decision{
			{ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", Target: model.DecisionTarget{Type: "tag", ID: "axis.decided"}, Why: "軸導入の根拠"},
		},
	}
	findings := checkAxisWithoutDecision(snap)
	if len(findings) != 1 || findings[0].Target != "axis.bare" {
		t.Fatalf("expected only axis.bare, got %+v", findings)
	}
}

// --- duplicate-atom ---

func TestDuplicateAtomGroupsBySetGivenAndOrderedThen(t *testing.T) {
	snap := store.Snapshot{
		Config: model.DefaultConfig(),
		Transitions: []model.Transition{
			{ID: "T-a1", Action: "act.a", Given: []string{"cond.1", "cond.2"}, Then: []string{"eff.1"}},
			{ID: "T-a2", Action: "act.a", Given: []string{"cond.2", "cond.1"}, Then: []string{"eff.1"}}, // given は集合一致
			{ID: "T-b", Action: "act.a", Given: []string{"cond.1"}, Then: []string{"eff.1"}},
			{ID: "T-c1", Action: "act.c", Given: nil, Then: []string{"eff.1", "eff.2"}},
			{ID: "T-c2", Action: "act.c", Given: nil, Then: []string{"eff.2", "eff.1"}}, // then は順序込み＝別原子
		},
		Decisions: []model.Decision{
			{ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", Target: model.DecisionTarget{Type: "transition", ID: "T-a2"}, Why: "w"},
		},
	}
	findings := checkDuplicateAtom(snap)
	if len(findings) != 1 {
		t.Fatalf("expected 1 duplicate group, got %+v", findings)
	}
	f := findings[0]
	if f.Target != "T-a1" || f.Quote != "T-a1・T-a2" {
		t.Fatalf("unexpected group representative/members: %+v", f)
	}
	if !strings.Contains(f.Message, "decision 付き遷移（T-a2）") {
		t.Fatalf("decision-attached note missing: %+v", f)
	}
	if f.AcknowledgeOnly {
		t.Fatalf("duplicate-atom は判断欄位由来ではないので acknowledge-only にしない: %+v", f)
	}
}

// --- dangling-id ---

func danglingSnap() store.Snapshot {
	return store.Snapshot{
		Config: model.DefaultConfig(),
		Vocab: []model.VocabEntry{
			{ID: "cond.exists", Category: model.CategoryCondition, Label: "在る"},
		},
		Tags:        []model.Tag{{ID: "req.exists", Kind: "requirement"}},
		Transitions: []model.Transition{{ID: "T-exists", Action: "act.a", Then: []string{"eff.a"}}},
	}
}

func TestDanglingIDDetectsUnresolvedTokensWithExclusions(t *testing.T) {
	snap := danglingSnap()
	snap.Decisions = []model.Decision{
		{ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", Target: model.DecisionTarget{Type: "tag", ID: "req.exists"},
			// 真ヒット: cond.missing（文末句読点付き）・T-gone
			// 除外: E1 族 glob（T-comment-*）・E2 プレースホルダ（T-xxx・req.foobar・
			// req.foo-bar）・E3 category+kind（eff.log）・境界（xcond.zzz は別語）
			Why:     "cond.exists は保つ。cond.missing. は消えた。T-comment-* 族と T-xxx、req.foobar・req.foo-bar は例示。then は eff.log のみ。xcond.zzz は無関係。",
			Changed: "T-gone を改名した"},
	}
	findings := checkDanglingID(snap)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %+v", findings)
	}
	f := findings[0]
	if f.Target != "01AAAAAAAAAAAAAAAAAAAAAAAA" || !f.AcknowledgeOnly || f.Field != "why・changed" {
		t.Fatalf("unexpected finding: %+v", f)
	}
	if f.Quote != "cond.missing・T-gone" {
		t.Fatalf("quote = %q, want cond.missing・T-gone", f.Quote)
	}
}

func TestDanglingIDScansVocabLabelAndConfigExtensions(t *testing.T) {
	snap := danglingSnap()
	snap.Vocab = append(snap.Vocab, model.VocabEntry{
		ID: "cond.other", Category: model.CategoryCondition,
		Label:       "T-gone2 が消えたとき",
		Description: "cond.tbd は将来語彙（config 拡張プレースホルダ）。",
	})
	snap.Config.Lint = &model.LintConfig{PlaceholderSegments: []string{"tbd"}}
	// idPolicy 宣言 prefix もトークン候補になる（tx. の transition は 1 件も無い store）
	snap.Config.IDPolicy = &model.IDPolicy{Transition: "tx."}
	snap.Tags[0].Description = "tx.Widget.clear は未作成。"

	findings := checkDanglingID(snap)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (tag desc + vocab label), got %+v", findings)
	}
	if findings[0].Target != "req.exists" || findings[0].Quote != "tx.Widget.clear" {
		t.Fatalf("idPolicy 宣言 prefix のトークンを拾えていない: %+v", findings[0])
	}
	if findings[1].Target != "cond.other" || findings[1].Field != "label" || findings[1].Quote != "T-gone2" {
		t.Fatalf("vocab label の走査が違う（cond.tbd は E2 config 拡張で除外されるべき）: %+v", findings[1])
	}
}

// --- dead-doc-ref ---

func TestDeadDocRefResolvesAgainstProjectFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"DESIGN.md", "docs/guide.md"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	snap := store.Snapshot{
		Root:   root,
		Config: model.DefaultConfig(),
		Tags: []model.Tag{
			{ID: "req.a", Kind: "requirement",
				Description: "DESIGN §3 と docs/guide.md と guide.md は解決する。missing.md が無い。" +
					"https://example.com/gone.md は外部。/tmp/scratch.txt に置いた。old-notes §2 の判断。"},
		},
		Vocab: []model.VocabEntry{
			// § 形で dead と判った docname（old-notes）は素の言及も検出する
			{ID: "cond.b", Category: model.CategoryCondition, Label: "b", Description: "old-notes が指摘した欠落。"},
		},
		Decisions: []model.Decision{
			{ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", Target: model.DecisionTarget{Type: "tag", ID: "req.a"},
				Why: "経緯", Ref: ".concierge/decision.md"},
		},
	}
	findings := checkDeadDocRef(snap)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %+v", findings)
	}
	tag := findings[0]
	if tag.Target != "req.a" || tag.AcknowledgeOnly {
		t.Fatalf("unexpected tag finding: %+v", tag)
	}
	for _, want := range []string{"missing.md", "/tmp/scratch.txt", "old-notes §"} {
		if !strings.Contains(tag.Quote, want) {
			t.Fatalf("tag quote missing %q: %+v", want, tag)
		}
	}
	for _, reject := range []string{"guide.md", "DESIGN", "example.com"} {
		if strings.Contains(tag.Quote, reject) {
			t.Fatalf("解決する参照/URL を誤検出: %q in %+v", reject, tag)
		}
	}
	if v := findings[1]; v.Target != "cond.b" || v.Quote != "old-notes" {
		t.Fatalf("bare docname 検出が違う: %+v", v)
	}
	if d := findings[2]; d.Target != "01AAAAAAAAAAAAAAAAAAAAAAAA" || !d.AcknowledgeOnly || d.Quote != ".concierge/decision.md" {
		t.Fatalf("decision ref finding wrong: %+v", d)
	}
}

func TestDeadDocRefSkipsWhenRootUnknown(t *testing.T) {
	snap := store.Snapshot{
		Config: model.DefaultConfig(),
		Tags:   []model.Tag{{ID: "req.a", Description: "missing.md を参照する。"}},
	}
	if findings := checkDeadDocRef(snap); findings != nil {
		t.Fatalf("Root 未設定の手組み snapshot では検査しない: %+v", findings)
	}
}
