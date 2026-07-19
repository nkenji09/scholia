// rules_authoring.go — authoring 規律の advisory lint 8 規則（#45 U2/P4）。
//
// error/warn（自己矛盾の検査）と違い、advisory は「書き方規律の改善提案」で
// あって保存も CI も止めない（severity=info・tier=advisory）。decision の判断
// 欄位（why/changed/ref/at）由来の finding は append-only により是正が原理的に
// 不能なので acknowledge-only 区分（Finding.AcknowledgeOnly）で別掲する。
// 検出仕様・除外パターンは .concierge/kits/kit-bundle2-retrofit-findings.md の
// 実 store 全件実走（331 レコード）で確定したもの——特に dangling-id は素朴実装
// だと 8 件中 7 件が偽陽性になるため、除外3種（E1 族 glob・E2 プレースホルダ・
// E3 category+kind 族参照）が規則仕様の一部である。
package lint

import (
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// Finding.TargetType の値。
const (
	targetTag        = "tag"
	targetVocab      = "vocab"
	targetTransition = "transition"
	targetDecision   = "decision"
)

// advisory は advisory finding のコンストラクタ。decision を対象にした時点で
// acknowledge-only（判断欄位 why/changed/ref/at 由来＝是正不能）に区分する。
func advisory(rule, targetType, targetID, field, quote, suggestion, format string, args ...any) Finding {
	return Finding{
		Rule:            rule,
		Severity:        SeverityInfo,
		Tier:            TierAdvisory,
		Target:          targetID,
		TargetType:      targetType,
		Field:           field,
		Quote:           quote,
		Suggestion:      suggestion,
		AcknowledgeOnly: targetType == targetDecision,
		Message:         fmt.Sprintf(format, args...),
	}
}

// --- 共有: id トークンの境界規則（internal/refs/match.go と同規則） ---

func isIDByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '.' || b == '-' || b == '_':
		return true
	}
	return false
}

func isDelimByte(b byte) bool {
	return b == '.' || b == '-' || b == '_'
}

// idOccursIn は text 中に id が境界安全なリテラル参照として現れるかを返す
// （internal/refs findOccurrences の containment 版。左は id 連続文字で拒否・
// 右は「英数字を含まない末尾区切り連鎖＝文末句読点」のみ許容）。id は ASCII、
// text は UTF-8 だが、マルチバイト文字のバイトは id 連続文字にならないため
// バイト走査で安全。
func idOccursIn(text, id string) bool {
	if id == "" {
		return false
	}
	start := 0
	for {
		rel := strings.Index(text[start:], id)
		if rel < 0 {
			return false
		}
		idx := start + rel
		end := idx + len(id)
		if idx > 0 && isIDByte(text[idx-1]) {
			start = idx + 1
			continue
		}
		if end >= len(text) || !isIDByte(text[end]) {
			return true
		}
		j := end
		hasAlnum := false
		for j < len(text) && isIDByte(text[j]) {
			if !isDelimByte(text[j]) {
				hasAlnum = true
			}
			j++
		}
		if !hasAlnum {
			return true
		}
		start = j
	}
}

// fieldHits は 1 レコード内で複数欄位にまたがるヒットを 1 finding に畳む
// アキュムレータ（record×rule 単位の集計＝kit 実走の数え方と一致させる）。
type fieldHits struct {
	fields []string
	quotes []string
}

func (h *fieldHits) add(field string, quotes []string) {
	if len(quotes) == 0 {
		return
	}
	h.fields = append(h.fields, field)
	for _, q := range quotes {
		if !contains(h.quotes, q) {
			h.quotes = append(h.quotes, q)
		}
	}
}

func (h *fieldHits) empty() bool { return len(h.fields) == 0 }

func (h *fieldHits) fieldList() string { return strings.Join(h.fields, "・") }
func (h *fieldHits) quoteList() string { return strings.Join(h.quotes, "・") }

// --- 1. derived-value-in-desc: axis タグ desc への派生値の書き写し ---
//
// 軸の値（自軸に貼られた condition id）・`値={…}` 列挙構文・total フィールドの
// 複製は、構造（condition の axis タグ・Tag.Total）から派生できる情報の二重
// 書きで、構造が変わると desc が黙って嘘になる。

var (
	derivedEnumPattern  = regexp.MustCompile(`値[=＝]\s*[{｛]`)
	derivedTotalPattern = regexp.MustCompile(`(?i)total=(?:true|false)`)
)

func checkDerivedValueInDesc(snap store.Snapshot) []Finding {
	condsByAxis := make(map[string][]string)
	axisKind := make(map[string]bool)
	for _, t := range snap.Tags {
		if t.Kind == "axis" {
			axisKind[t.ID] = true
		}
	}
	for _, v := range snap.Vocab {
		if v.Category != model.CategoryCondition {
			continue
		}
		for _, tagID := range v.Tags {
			if axisKind[tagID] {
				condsByAxis[tagID] = append(condsByAxis[tagID], v.ID)
			}
		}
	}

	var out []Finding
	for _, t := range snap.Tags {
		if t.Kind != "axis" || t.Description == "" {
			continue
		}
		var quotes []string
		conds := append([]string{}, condsByAxis[t.ID]...)
		sort.Strings(conds)
		for _, c := range conds {
			if idOccursIn(t.Description, c) {
				quotes = append(quotes, c)
			}
		}
		if m := derivedEnumPattern.FindString(t.Description); m != "" {
			quotes = append(quotes, m)
		}
		if m := derivedTotalPattern.FindString(t.Description); m != "" {
			quotes = append(quotes, m)
		}
		if len(quotes) == 0 {
			continue
		}
		q := strings.Join(quotes, "・")
		out = append(out, advisory("derived-value-in-desc", targetTag, t.ID, "description", q,
			"desc は次元名（軸が束ねる分岐の意味）だけにし、値列挙・total・排他の根拠は構造（condition の axis タグ）と own decision に任せる",
			"tag %s: description に自軸の派生値（%s）が書き写されています（構造から導出できる情報の二重書き）", t.ID, q))
	}
	return out
}

// --- 2. stale-tense: desc への時点依存語の残置 ---
//
// 走査対象は tag/vocab の description のみ。label は runtime 状態を書く欄
// （「まだ存在しない」等が正当）、decision.why は「その時点の判断」を書く履歴
// 欄で時点依存語が正——どちらも対象外（kit-bundle2 実走で確定した設計。全語彙
// を why に適用すると #N 等の正当な引用が恒常 acknowledge-only ノイズになる）。

var staleTensePattern = regexp.MustCompile(`現状|現時点|現在は|本タスク|本 ?PR|今回|新設|未実装|未対応|予定|整備中|rev\d|#\d+|Level ?\d`)

func checkStaleTense(snap store.Snapshot) []Finding {
	excludes := compileStalePatternExcludes(snap.Config)
	var out []Finding
	check := func(targetType, id, desc string) {
		if f, ok := staleTenseFinding(targetType, id, desc, excludes); ok {
			out = append(out, f)
		}
	}
	for _, t := range snap.Tags {
		if t.Description != "" {
			check(targetTag, t.ID, t.Description)
		}
	}
	for _, v := range snap.Vocab {
		if v.Description != "" {
			check(targetVocab, v.ID, v.Description)
		}
	}
	return out
}

// staleTenseFinding は 1 レコードの desc に stale-tense finding があれば返す
// （検査コア・全量走査と desc 現在形ゲート三点配線で共有・#45 D7）。
func staleTenseFinding(targetType, id, desc string, excludes []*regexp.Regexp) (Finding, bool) {
	matches := uniquePatternMatches(staleTensePattern, desc)
	matches = filterExcludedMatches(matches, excludes)
	if len(matches) == 0 {
		return Finding{}, false
	}
	q := strings.Join(matches, "・")
	return advisory("stale-tense", targetType, id, "description", q,
		"時点依存語を除き、現在形の「これは何か」だけに書き直す（経緯・工程は decision が保持する）",
		"%s %s: description に時点依存語（%s）が残っています（record が古びると嘘になる書き方）", targetType, id, q), true
}

// TargetDescStaleTense は decision の target（tag/vocab）の desc に対する
// stale-tense 検査（desc 現在形ゲート三点配線・#45 D7）。decide 保存前プレビュー・
// review adopt 応答・add-commit 同一ターンが同じコアで対象 desc の鮮度を見る。
// transition は desc を持たないため対象外（tag/vocab のみ）。
func TargetDescStaleTense(snap store.Snapshot, target model.DecisionTarget) []Finding {
	excludes := compileStalePatternExcludes(snap.Config)
	switch target.Type {
	case model.DecisionTargetTag:
		for _, t := range snap.Tags {
			if t.ID == target.ID && t.Description != "" {
				if f, ok := staleTenseFinding(targetTag, t.ID, t.Description, excludes); ok {
					return []Finding{f}
				}
			}
		}
	case model.DecisionTargetVocab:
		for _, v := range snap.Vocab {
			if v.ID == target.ID && v.Description != "" {
				if f, ok := staleTenseFinding(targetVocab, v.ID, v.Description, excludes); ok {
					return []Finding{f}
				}
			}
		}
	}
	return nil
}

// compileStalePatternExcludes は config.lint.stalePatternExcludes を
// コンパイルする。不正な正規表現は黙って捨てる（advisory の調整設定が lint
// 全体を落とすのは本末転倒）。
func compileStalePatternExcludes(cfg model.Config) []*regexp.Regexp {
	if cfg.Lint == nil {
		return nil
	}
	var out []*regexp.Regexp
	for _, p := range cfg.Lint.StalePatternExcludes {
		if re, err := regexp.Compile(p); err == nil {
			out = append(out, re)
		}
	}
	return out
}

func uniquePatternMatches(re *regexp.Regexp, text string) []string {
	var out []string
	for _, m := range re.FindAllString(text, -1) {
		if !contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}

func filterExcludedMatches(matches []string, excludes []*regexp.Regexp) []string {
	if len(excludes) == 0 {
		return matches
	}
	var out []string
	for _, m := range matches {
		excluded := false
		for _, re := range excludes {
			if re.MatchString(m) {
				excluded = true
				break
			}
		}
		if !excluded {
			out = append(out, m)
		}
	}
	return out
}

// --- 3. prose-ref: 「〜を参照」型のメタ指示 ---
//
// 根拠は構造（タグ → decision）で辿れるため、散文の道案内は desc に埋めない。
// パターンは「を参照」＋句読点/文末/閉じ括弧のみ検出し、動詞活用が続く domain
// 用法（参照する/参照される/参照でき/参照先/参照整合/タグ参照/参照 vocab）は
// 構造的に除外される（実 store の「参照」9 件は全て正当用法＝ヒット 0 の実測）。

var proseRefPattern = regexp.MustCompile(`を参照(?:[。、．，)）\]」』】\s]|$)|参照のこと|を見よ|参照してください`)

func checkProseRef(snap store.Snapshot) []Finding {
	var out []Finding
	check := func(targetType, id, desc string) {
		matches := uniquePatternMatches(proseRefPattern, desc)
		for i, m := range matches {
			matches[i] = strings.TrimSpace(m)
		}
		if len(matches) == 0 {
			return
		}
		q := strings.Join(matches, "・")
		out = append(out, advisory("prose-ref", targetType, id, "description", q,
			"散文の道案内を desc に埋めず、根拠は構造（タグ→decision）で辿れるようにする",
			"%s %s: description に「〜を参照」型のメタ指示（%s）があります", targetType, id, q))
	}
	for _, t := range snap.Tags {
		if t.Description != "" {
			check(targetTag, t.ID, t.Description)
		}
	}
	for _, v := range snap.Vocab {
		if v.Description != "" {
			check(targetVocab, v.ID, v.Description)
		}
	}
	return out
}

// --- 4. why-file-line: 腐る file:line 参照 ---
//
// decision の why/changed（判断欄位＝acknowledge-only）と tag/vocab の
// description を走査する。decision.ref の file:line は既存の ref-freshness
// （warn）の領分なのでここでは重複検査しない。

var fileLinePattern = regexp.MustCompile(`[\w./-]+\.(?:go|ts|tsx|js|py|md|json|vue)(?::\d+)+`)

func checkWhyFileLine(snap store.Snapshot) []Finding {
	var out []Finding
	suggestion := "file:line はコード変更で腐る——永続参照（commit hash・DESIGN 等 versioned 文書の § 参照）に置き換える"

	for _, d := range snap.Decisions {
		var hits fieldHits
		hits.add("why", fileLinePattern.FindAllString(d.Why, -1))
		hits.add("changed", fileLinePattern.FindAllString(d.Changed, -1))
		if hits.empty() {
			continue
		}
		out = append(out, advisory("why-file-line", targetDecision, d.ID, hits.fieldList(), hits.quoteList(), suggestion,
			"decision %s: %s に file:line 参照（%s）があります（判断欄位は append-only のため容認で畳む対象）", d.ID, hits.fieldList(), hits.quoteList()))
	}
	check := func(targetType, id, desc string) {
		matches := uniquePatternMatches(fileLinePattern, desc)
		if len(matches) == 0 {
			return
		}
		q := strings.Join(matches, "・")
		out = append(out, advisory("why-file-line", targetType, id, "description", q, suggestion,
			"%s %s: description に file:line 参照（%s）があります", targetType, id, q))
	}
	for _, t := range snap.Tags {
		if t.Description != "" {
			check(targetTag, t.ID, t.Description)
		}
	}
	for _, v := range snap.Vocab {
		if v.Description != "" {
			check(targetVocab, v.ID, v.Description)
		}
	}
	return out
}

// --- 5. axis-without-decision: 軸タグに decision 0 件 ---
//
// 軸導入（値の畳み方・排他/total の理由）は必ず decision と対、という運用慣行
// の規則化。tag create 直後は正規フロー上常に未充足になるため write-time
// advisory には含めない（U3）——全量走査の lint/retrofit だけが報告する。

func checkAxisWithoutDecision(snap store.Snapshot) []Finding {
	counts := tagDecisionCounts(snap.Decisions)
	var out []Finding
	for _, t := range snap.Tags {
		if t.Kind != "axis" || counts[t.ID] > 0 {
			continue
		}
		out = append(out, advisory("axis-without-decision", targetTag, t.ID, "", "",
			"軸導入の根拠（値の畳み方・排他/total の理由）を decide --on tag:"+t.ID+" で記録する",
			"tag %s: kind=axis ですが own decision が 0 件です（軸導入の根拠が未記録）", t.ID))
	}
	return out
}

// --- 6. duplicate-atom: 同一 action＋given(集合)＋then(列) の複製グループ ---
//
// 正規形は「1 原子＋複数 subject タグ」。given は集合一致（保存時ソート済みだが
// 念のため正規化）・then は順序込み一致。グループごとに 1 finding（代表 =
// 辞書順先頭の transition）。構成遷移が own decision を持つ場合、統合は
// decision target の張替えを要するためその旨を明記する（ユーザー判断）。

func checkDuplicateAtom(snap store.Snapshot) []Finding {
	decided := make(map[string]bool)
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetTransition {
			decided[d.Target.ID] = true
		}
	}

	groups := make(map[string][]string)
	for _, t := range snap.Transitions {
		given := append([]string{}, t.Given...)
		sort.Strings(given)
		key := t.Action + "\x1f" + strings.Join(given, ",") + "\x1f" + strings.Join(t.Then, ",")
		groups[key] = append(groups[key], t.ID)
	}

	var reps []string
	byRep := make(map[string][]string)
	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		sort.Strings(members)
		reps = append(reps, members[0])
		byRep[members[0]] = members
	}
	sort.Strings(reps)

	var out []Finding
	for _, rep := range reps {
		members := byRep[rep]
		var withDecision []string
		for _, id := range members {
			if decided[id] {
				withDecision = append(withDecision, id)
			}
		}
		msg := fmt.Sprintf("transition %s: 同一原子（action＋given 集合＋then 列が一致）の複製グループです（構成: %s）", rep, strings.Join(members, "・"))
		if len(withDecision) > 0 {
			msg += fmt.Sprintf("。decision 付き遷移（%s）を含むため、統合には decision target の張替えを要します（ユーザー判断）", strings.Join(withDecision, "・"))
		}
		f := advisory("duplicate-atom", targetTransition, rep, "", strings.Join(members, "・"),
			"「1 原子＋複数 subject タグ」の正規形へ統合する（1 本を残して多タグ化し、残りは tx rm --why。decision 付きは統合せず容認も可）",
			"%s", msg)
		out = append(out, f)
	}
	return out
}

// --- 7. dangling-id: prose 内の id 様トークンが現存レコードに解決しない ---
//
// 走査対象は decision の why/changed/ref・tag の description・vocab の
// description/label。検出のみで書き換えない（append-only 尊重）。素朴実装は
// 実 store で 8 件中 7 件が偽陽性になるため、除外3種が仕様の本体:
//   E1 族 glob     — トークン直後が `*`（例 `T-comment-*`・`eff.emit.*`）
//   E2 プレースホルダ — 最終セグメントが xxx/foo 等（built-in＋config 追加可）
//   E3 category+kind — vocab カテゴリ prefix＋宣言 kind に完全一致する族参照
//                      （例 `eff.log`＝「eff.log 系の効果」）
// id 様トークンの候補判定は「既存レコード id から推定した prefix 群＋idPrefix
// ＋idPolicy 宣言」を用いる（トークン境界は internal/refs と同規則）。

var builtinPlaceholderSegments = []string{"xxx", "yyy", "foo", "bar", "foobar", "foo-bar", "example", "sample", "dummy"}

func checkDanglingID(snap store.Snapshot) []Finding {
	existing := make(map[string]bool)
	var declared []string
	collect := func(id string) {
		existing[id] = true
		declared = append(declared, id)
	}
	for _, v := range snap.Vocab {
		collect(v.ID)
	}
	for _, t := range snap.Tags {
		collect(t.ID)
	}
	for _, t := range snap.Transitions {
		collect(t.ID)
	}
	for _, d := range snap.Decisions {
		existing[d.ID] = true // decision ULID は prefix 推定には使わない（区切り無し）
	}

	prefixes := danglingPrefixSet(snap.Config, declared)
	placeholders := placeholderSegmentSet(snap.Config)
	familyRefs := categoryKindFamilyRefs(snap.Config)

	scan := func(text string) []string {
		return danglingTokens(text, prefixes, existing, placeholders, familyRefs)
	}

	var out []Finding
	suggestion := "現存 id へ直すか、族参照（`〜-*`）／プレースホルダ表記に改める（rename 済みなら新 id へ。検出のみ・自動書き換えはしない）"
	for _, t := range snap.Tags {
		if toks := scan(t.Description); len(toks) > 0 {
			q := strings.Join(toks, "・")
			out = append(out, advisory("dangling-id", targetTag, t.ID, "description", q, suggestion,
				"tag %s: description 内の id 様トークン（%s）が現存レコードに解決しません", t.ID, q))
		}
	}
	for _, v := range snap.Vocab {
		var hits fieldHits
		hits.add("description", scan(v.Description))
		hits.add("label", scan(v.Label))
		if hits.empty() {
			continue
		}
		out = append(out, advisory("dangling-id", targetVocab, v.ID, hits.fieldList(), hits.quoteList(), suggestion,
			"vocab %s: %s 内の id 様トークン（%s）が現存レコードに解決しません", v.ID, hits.fieldList(), hits.quoteList()))
	}
	for _, d := range snap.Decisions {
		var hits fieldHits
		hits.add("why", scan(d.Why))
		hits.add("changed", scan(d.Changed))
		hits.add("ref", scan(d.Ref))
		if hits.empty() {
			continue
		}
		out = append(out, advisory("dangling-id", targetDecision, d.ID, hits.fieldList(), hits.quoteList(), suggestion,
			"decision %s: %s 内の id 様トークン（%s）が現存レコードに解決しません（判断欄位は append-only のため容認で畳む対象）", d.ID, hits.fieldList(), hits.quoteList()))
	}
	return out
}

// danglingPrefixSet は id 様トークンの候補判定に使う prefix 集合を返す。
// 既存レコード id の「最初の区切り（. / -）まで」＋config.idPrefix＋idPolicy
// 宣言値。決定的な順序（sort）で返す。
func danglingPrefixSet(cfg model.Config, ids []string) []string {
	set := make(map[string]bool)
	add := func(p string) {
		if p != "" {
			set[p] = true
		}
	}
	for _, id := range ids {
		for k := 0; k < len(id); k++ {
			if id[k] == '.' || id[k] == '-' {
				add(id[:k+1])
				break
			}
		}
	}
	add(cfg.IDPrefix.Condition)
	add(cfg.IDPrefix.Action)
	add(cfg.IDPrefix.Effect)
	if cfg.IDPolicy != nil {
		add(cfg.IDPolicy.Transition)
		for _, p := range cfg.IDPolicy.Vocab {
			add(p)
		}
		for _, p := range cfg.IDPolicy.TagByKind {
			add(p)
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// placeholderSegmentSet は E2 のプレースホルダ語彙（built-in＋config 追加分）。
func placeholderSegmentSet(cfg model.Config) map[string]bool {
	set := make(map[string]bool, len(builtinPlaceholderSegments))
	for _, s := range builtinPlaceholderSegments {
		set[s] = true
	}
	if cfg.Lint != nil {
		for _, s := range cfg.Lint.PlaceholderSegments {
			set[s] = true
		}
	}
	return set
}

// categoryKindFamilyRefs は E3 の「category prefix＋宣言 kind」完全一致
// トークン集合（`eff.log` 等＝kind 族への参照であって個別 id ではない）。
func categoryKindFamilyRefs(cfg model.Config) map[string]bool {
	set := make(map[string]bool)
	for prefix, category := range map[string]string{
		cfg.IDPrefix.Condition: model.CategoryCondition,
		cfg.IDPrefix.Action:    model.CategoryAction,
		cfg.IDPrefix.Effect:    model.CategoryEffect,
	} {
		if prefix == "" {
			continue
		}
		for _, kind := range cfg.KindsFor(category) {
			set[prefix+kind] = true
		}
	}
	return set
}

// danglingTokens は text から id 様トークン（prefix 群のいずれかで始まる境界
// 安全な連続列）を抽出し、現存 id に解決せず除外3種にも該当しないものを返す。
func danglingTokens(text string, prefixes []string, existing, placeholders, familyRefs map[string]bool) []string {
	var out []string
	n := len(text)
	i := 0
	for i < n {
		if !isIDByte(text[i]) {
			i++
			continue
		}
		j := i
		for j < n && isIDByte(text[j]) {
			j++
		}
		run := text[i:j]
		star := j < n && text[j] == '*'
		i = j
		if star {
			continue // E1: 族 glob（`T-comment-*`・`eff.emit.*`）
		}
		tok := strings.Trim(run, ".-_") // 文末句読点・囲みの区切り文字を落とす
		if tok == "" {
			continue
		}
		matched := false
		for _, p := range prefixes {
			if strings.HasPrefix(tok, p) && len(tok) > len(p) {
				matched = true
				break
			}
		}
		if !matched || existing[tok] {
			continue
		}
		if isPlaceholderToken(tok, placeholders) { // E2
			continue
		}
		if familyRefs[tok] { // E3
			continue
		}
		if !contains(out, tok) {
			out = append(out, tok)
		}
	}
	return out
}

// isPlaceholderToken は E2 判定: 最終セグメント（最後の `.` 以降、または
// 最後の区切り以降）がプレースホルダ語彙に含まれるか。
func isPlaceholderToken(tok string, placeholders map[string]bool) bool {
	lastDot := tok
	if idx := strings.LastIndex(tok, "."); idx >= 0 {
		lastDot = tok[idx+1:]
	}
	lastSeg := tok
	if idx := strings.LastIndexAny(tok, ".-_"); idx >= 0 {
		lastSeg = tok[idx+1:]
	}
	return placeholders[lastDot] || placeholders[lastSeg]
}

// --- 8. dead-doc-ref: repo 内に解決しない文書参照 ---
//
// 検出: (i) `*.md` パス様トークンが versioned ファイルに解決しない（相対パス
// →basename fallback）・(ii) `/tmp/` パス＝常時ヒット・(iii) gitignored パス
// （versioned 集合に無いため (i) に吸収される）・(iv) `<docname> §…` 形で
// `<docname>.md` が repo に無い。§ 形で dead と判った docname は、素の言及
// （「design-options が指摘する欠落」型）も store 全体から検出する。
// 除外: 解決する参照（DESIGN §N 等）・http(s) URL（外部到達性は検査しない）。
// 解決基盤は `git ls-files`（gitignored を dead 扱いにできる）。git が使えない
// store では filesystem 走査に fallback する。Snapshot.Root が空（手組み
// snapshot）のときは検査しない。

var (
	mdPathPattern     = regexp.MustCompile(`[A-Za-z0-9._/-]+\.md`)
	tmpPathPattern    = regexp.MustCompile(`/tmp/[A-Za-z0-9._/-]+`)
	sectionRefPattern = regexp.MustCompile(`([A-Za-z0-9._-]+)(?:\s|　)*§`)
	asciiLetter       = regexp.MustCompile(`[A-Za-z]`)
	alnumPattern      = regexp.MustCompile(`[A-Za-z0-9]`)
)

type repoDocSet struct {
	files     map[string]bool // project-root 相対 slash パス
	basenames map[string]bool
}

func (r repoDocSet) resolves(tok string) bool {
	clean := strings.TrimPrefix(tok, "./")
	if r.files[clean] {
		return true
	}
	base := clean
	if idx := strings.LastIndex(clean, "/"); idx >= 0 {
		base = clean[idx+1:]
	}
	return r.basenames[base]
}

// loadRepoDocs は解決基盤のファイル集合を返す。git repo なら `git ls-files`
// （＝versioned のみ・gitignored は dead 扱い）、それ以外は filesystem 走査
// （.git 配下は除外）。
func loadRepoDocs(root string) repoDocSet {
	docs := repoDocSet{files: make(map[string]bool), basenames: make(map[string]bool)}
	addPath := func(rel string) {
		rel = filepath.ToSlash(rel)
		docs.files[rel] = true
		base := rel
		if idx := strings.LastIndex(rel, "/"); idx >= 0 {
			base = rel[idx+1:]
		}
		docs.basenames[base] = true
	}

	if out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output(); err == nil {
		listed := false
		for _, rel := range strings.Split(string(out), "\x00") {
			if rel == "" {
				continue
			}
			addPath(rel)
			listed = true
		}
		if listed {
			return docs
		}
	}

	// fallback: 非 git store（gitignore 判定は不能——存在するファイルは全て解決扱い）
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 読めない部分は解決基盤から外れるだけ
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, relErr := filepath.Rel(root, path); relErr == nil {
			addPath(rel)
		}
		return nil
	})
	return docs
}

func checkDeadDocRef(snap store.Snapshot) []Finding {
	if snap.Root == "" {
		return nil
	}
	docs := loadRepoDocs(snap.Root)

	type scanField struct {
		targetType string
		targetID   string
		field      string
		text       string
	}
	var fields []scanField
	for _, t := range snap.Tags {
		fields = append(fields, scanField{targetTag, t.ID, "description", t.Description})
	}
	for _, v := range snap.Vocab {
		fields = append(fields, scanField{targetVocab, v.ID, "description", v.Description})
	}
	for _, d := range snap.Decisions {
		fields = append(fields, scanField{targetDecision, d.ID, "why", d.Why})
		fields = append(fields, scanField{targetDecision, d.ID, "changed", d.Changed})
		fields = append(fields, scanField{targetDecision, d.ID, "ref", d.Ref})
	}

	// pass 1: § 形で dead と判る docname を store 全体から収集する
	deadDocnames := make(map[string]bool)
	for _, f := range fields {
		for _, dn := range sectionDocnames(f.text) {
			if !docs.basenames[dn+".md"] {
				deadDocnames[dn] = true
			}
		}
	}
	sortedDead := make([]string, 0, len(deadDocnames))
	for dn := range deadDocnames {
		sortedDead = append(sortedDead, dn)
	}
	sort.Strings(sortedDead)

	// pass 2: レコード×欄位ごとにトークンを集める
	deadTokens := func(text string) []string {
		var toks []string
		addTok := func(t string) {
			if !contains(toks, t) {
				toks = append(toks, t)
			}
		}
		for _, tok := range mdPathTokens(text) {
			if strings.HasPrefix(tok, "/tmp/") {
				continue // /tmp 側で報告
			}
			if !docs.resolves(tok) {
				addTok(tok)
			}
		}
		for _, tok := range tmpPathTokens(text) {
			addTok(tok)
		}
		for _, dn := range sectionDocnames(text) {
			if deadDocnames[dn] {
				addTok(dn + " §")
			}
		}
		for _, dn := range sortedDead {
			if contains(toks, dn+" §") {
				continue // 同一レコードに § 形があれば素の言及は畳む
			}
			if idOccursIn(text, dn) {
				addTok(dn)
			}
		}
		return toks
	}

	type recordKey struct{ targetType, targetID string }
	hitsByRecord := make(map[recordKey]*fieldHits)
	var order []recordKey
	for _, f := range fields {
		toks := deadTokens(f.text)
		if len(toks) == 0 {
			continue
		}
		key := recordKey{f.targetType, f.targetID}
		h, ok := hitsByRecord[key]
		if !ok {
			h = &fieldHits{}
			hitsByRecord[key] = h
			order = append(order, key)
		}
		h.add(f.field, toks)
	}

	var out []Finding
	suggestion := "参照先を永続物（repo 内 versioned 文書の § 参照・commit hash 等）へ置き換え、消えた根拠の要旨は decision に再記録する"
	for _, key := range order {
		h := hitsByRecord[key]
		msg := fmt.Sprintf("%s %s: %s 内の文書参照（%s）が repo の versioned ファイルに解決しません", key.targetType, key.targetID, h.fieldList(), h.quoteList())
		if key.targetType == targetDecision {
			msg += "（判断欄位は append-only のため容認で畳む対象）"
		}
		out = append(out, advisory("dead-doc-ref", key.targetType, key.targetID, h.fieldList(), h.quoteList(), suggestion, "%s", msg))
	}
	return out
}

// mdPathTokens は text 中の `*.md` パス様トークンを返す。右境界が英数字のもの
// （.mdx 等）・URL 由来（`://` 直後＝`//` 始まり）・stem に英数字が無いものは
// 除外する。
func mdPathTokens(text string) []string {
	var out []string
	for _, loc := range mdPathPattern.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		if end < len(text) && alnumPattern.MatchString(string(text[end])) {
			continue // .mdx など別拡張子の一部
		}
		tok := text[start:end]
		if strings.HasPrefix(tok, "//") || (start > 0 && text[start-1] == ':') {
			continue // URL（外部到達性は検査しない）
		}
		stem := strings.TrimSuffix(tok, ".md")
		if !alnumPattern.MatchString(stem) {
			continue
		}
		if !contains(out, tok) {
			out = append(out, tok)
		}
	}
	return out
}

// tmpPathTokens は `/tmp/` パス（常時 dead）を返す。左境界がパス連続文字なら
// 相対パス中の tmp セグメントとみなして除外する。
func tmpPathTokens(text string) []string {
	var out []string
	for _, loc := range tmpPathPattern.FindAllStringIndex(text, -1) {
		start := loc[0]
		if start > 0 {
			b := text[start-1]
			if isIDByte(b) || b == '/' {
				continue
			}
		}
		tok := text[loc[0]:loc[1]]
		if !contains(out, tok) {
			out = append(out, tok)
		}
	}
	return out
}

// sectionDocnames は `<docname> §…` 形の docname 候補（英字を含む長さ 2 以上の
// トークン）を返す。
func sectionDocnames(text string) []string {
	var out []string
	for _, m := range sectionRefPattern.FindAllStringSubmatch(text, -1) {
		dn := strings.Trim(m[1], ".-_")
		if len(dn) < 2 || !asciiLetter.MatchString(dn) {
			continue
		}
		if !contains(out, dn) {
			out = append(out, dn)
		}
	}
	return out
}
