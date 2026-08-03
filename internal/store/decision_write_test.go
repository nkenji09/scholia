package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

const headlessWhy = "見出しの無い why\n\n本文"
const headedWhy = "# 見出し\n\n本文"

func newDecisionStore(t *testing.T) *Store {
	t.Helper()
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	return s
}

func decisionFiles(t *testing.T, s *Store) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(s.Dir, decisionsDir))
	if err != nil {
		t.Fatalf("read decisions dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func decision(id, why string) model.Decision {
	return model.Decision{
		ID:     id,
		Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
		Why:    why,
		At:     "2026-01-01T00:00:00Z",
	}
}

// 新規作成の口は見出しの無い why を拒み、ファイルを 1 つも作らない
// （01KZ06SYR3APGF3JD4NQRFTEEN 変更1）。
func TestCreateDecisionRejectsMissingHeading(t *testing.T) {
	s := newDecisionStore(t)

	err := s.CreateDecision(decision("01D1", headlessWhy), DecisionCreateOptions{})
	if err == nil {
		t.Fatalf("見出しの無い why は拒むべき")
	}
	var rej *DecisionRejectError
	if !asDecisionReject(err, &rej) || rej.Rule != RuleDecisionHeading {
		t.Fatalf("拒否は DecisionRejectError(%s) であるべき: %v", RuleDecisionHeading, err)
	}
	if rej.Reason == "" {
		t.Fatalf("何を満たしていないかを持つべき（viewer が id 抜きで文言を組み直す）")
	}
	if got := decisionFiles(t, s); len(got) != 0 {
		t.Fatalf("拒んだのにファイルが残っている: %v", got)
	}

	if err := s.CreateDecision(decision("01D1", headedWhy), DecisionCreateOptions{}); err != nil {
		t.Fatalf("見出しのある why は通るべき: %v", err)
	}
	if got := decisionFiles(t, s); len(got) != 1 {
		t.Fatalf("保存されるべき: %v", got)
	}
}

// 逃し弁は明示に渡したときだけ効く（CLI の --allow・変更4）。
func TestCreateDecisionAllowRule(t *testing.T) {
	s := newDecisionStore(t)
	if err := s.CreateDecision(decision("01D1", headlessWhy),
		DecisionCreateOptions{AllowRules: []string{RuleDecisionHeading}}); err != nil {
		t.Fatalf("--allow 相当を渡したら通るべき: %v", err)
	}
	// 別の規則名を渡しても、見出しの拒否は解除されない。
	if err := s.CreateDecision(decision("01D2", headlessWhy),
		DecisionCreateOptions{AllowRules: []string{"id-policy"}}); err == nil {
		t.Fatalf("別規則の allow で見出しの拒否が解除されてはならない")
	}
}

// 口を 2 つに割った本体（変更3）: 新規作成の口は既存を踏まず、更新の口は
// 新規を作れない。この 2 つが成り立つ限り、**新規作成の口を通らずに decision を
// 作ることができない。**
func TestDecisionPortsSplitCreateFromUpdate(t *testing.T) {
	s := newDecisionStore(t)
	if err := s.CreateDecision(decision("01D1", headedWhy), DecisionCreateOptions{}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 新規作成の口は既存 id を受けない（append-only を新規の顔で踏み潰せない）。
	if err := s.CreateDecision(decision("01D1", headedWhy), DecisionCreateOptions{}); err == nil {
		t.Fatalf("既存 id への CreateDecision は拒むべき")
	}

	// 更新の口は存在しない id を受けない＝この口から新規は作れない。
	before := decisionFiles(t, s)
	if err := s.UpdateDecision(decision("01D2", headlessWhy)); err == nil {
		t.Fatalf("存在しない id への UpdateDecision は拒むべき")
	}
	if got := decisionFiles(t, s); !reflect.DeepEqual(got, before) {
		t.Fatalf("更新の口からファイルが増えた: before=%v after=%v", before, got)
	}

	// 既存の更新には見出しの拒否を当てない（既存 173 件を遡って壊さない）。
	d := decision("01D1", headedWhy)
	d.Commits = []string{"abc1234"}
	if err := s.UpdateDecision(d); err != nil {
		t.Fatalf("既存の書き戻しは通るべき: %v", err)
	}
	got, err := s.LoadDecision("01D1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Commits) != 1 {
		t.Fatalf("更新が反映されていない: %+v", got)
	}
}

// **面を列挙しない歯止め**（CLAUDE.md 1・3、01KZ06SYR3APGF3JD4NQRFTEEN 変更3）。
//
// 「decide / review adopt / viewer の 3 面が配線されているか」を数えると、
// 4 面目を足した人が置き忘れたときに何も落ちない——この repo が 4 単位連続で
// 落とした型である。代わりに **「decision を書ける口が 2 つしかないこと」** を
// 検査する。3 つ目の口が生えたら、それが何面から呼ばれているかに関係なく落ちる。
//
// ⚠️ **この検査が落ちる範囲**（CLAUDE.md 6）:
// `*store.Store` の**公開メソッドのうち、引数に model.Decision（単体・スライス）を
// 取るもの**に落ちる。落ちないもの:
//   - Decision を引数に取らずに decision ファイルを書くメソッド（例: 一括取り込み）
//   - store を通さず `.scholia/decisions/*.json` を直接書くコード
//
// 後者は保存の口に置く歯止めの射程外である（`scholia lint` の領分）。
func TestNoThirdDecisionWritePort(t *testing.T) {
	want := map[string]bool{"CreateDecision": true, "UpdateDecision": true}

	decisionType := reflect.TypeOf(model.Decision{})
	storeType := reflect.TypeOf(&Store{})
	var found []string
	for i := 0; i < storeType.NumMethod(); i++ {
		m := storeType.Method(i)
		takesDecision := false
		for j := 0; j < m.Type.NumIn(); j++ {
			in := m.Type.In(j)
			if in == decisionType || (in.Kind() == reflect.Slice && in.Elem() == decisionType) {
				takesDecision = true
			}
		}
		if takesDecision {
			found = append(found, m.Name)
		}
	}
	for _, name := range found {
		if !want[name] {
			t.Errorf("decision を書ける口が増えている: %s。"+
				"新規作成は CreateDecision（見出しを拒む）・更新は UpdateDecision の 2 つだけにすること"+
				"——口が増えると、そこを通る面には見出しの歯止めが効かない", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("%s が見つからない（口の名前が変わったなら、この検査の期待も一緒に直すこと）", name)
	}
}

func asDecisionReject(err error, target **DecisionRejectError) bool {
	for err != nil {
		if e, ok := err.(*DecisionRejectError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// 拒否の文言は「何を直せばよいか」を持つ（形式を書かないと、書き手は
// 保存できない理由が分からないまま append-only の欄の前で止まる）。
func TestCreateDecisionRejectMessageStatesTheForm(t *testing.T) {
	s := newDecisionStore(t)
	err := s.CreateDecision(decision("01D1", headlessWhy), DecisionCreateOptions{})
	if err == nil {
		t.Fatal("拒むべき")
	}
	for _, want := range []string{"#", "見出し", "80"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("拒否の文言に %q が無い: %s", want, err.Error())
		}
	}
}
