package cli

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/nkenji09/scholia/internal/model"
)

// argClass は「その引数の値を、どこまで記録してよいか」の分類。
//
// ⚠️ **既定は classFreeText（値を記録しない）である。** 分類し忘れた引数は
// 詳細でも長さしか残らない——安全側に倒れる。
type argClass int

const (
	// classFreeText: 値は記録しない。詳細で**長さ**だけ残す。
	// 自由文（--why・--desc・--body 等）と、分類のついていない引数がここに入る。
	classFreeText argClass = iota
	// classToolVocab: 道具の側が閉じた集合として定めた値（組み込みの列挙値）。詳細で値を残す。
	// ⚠️ 閉じた集合そのものを values に持ち、**そこに無い値は自由文へ倒す**
	// ——分類を間違えても、書けるのは宣言した語彙だけになる。
	classToolVocab
	// classRecordID: プロジェクトが名付けたレコードを指す値。通常以上で残す。
	classRecordID
)

// 選択子の種類。組み込みの列挙値なので、マスクでも行に残せる。
const (
	selTag        = "tag"
	selTransition = "transition"
	selVocab      = "vocab"
	selDecision   = "decision"
	selReview     = "review"
	selRecord     = "record" // 型を問わないレコード指定（refs rewrite・refs scan --id）
)

// targetPrefixSelectors は `--on tag:<id>` 形式の前置から取れる選択子。
// 前置は道具の側の語彙なので、値そのものを残さないマスクでも「種類」だけは残せる。
var targetPrefixSelectors = map[string]string{
	"tag":        selTag,
	"transition": selTransition,
	"vocab":      selVocab,
}

// argSpec は 1 つの引数（フラグ or 位置引数）についての宣言。
type argSpec struct {
	class argClass
	// values は classToolVocab のときの閉じた集合。空なら値は残さない（自由文と同じ扱い）。
	values []string
	// selector はその引数が指すレコードの種類（選択子の種類）。空なら選択子ではない。
	selector string
	// selectorFromPrefix は、値の "<種類>:<id>" 前置から選択子を取ることを示す（--on）。
	selectorFromPrefix bool
}

// カテゴリ（condition|action|effect）は道具の側の閉じた集合。
var categoryValues = []string{"condition", "action", "effect"}

// stringFlagSpecs は**文字列を取るフラグ**の分類。
//
// ⚠️ 真偽・数値のフラグはここに載せない——型から分かるので構造的に扱う（下の flagValue）。
// ⚠️ ここに無い文字列フラグは classFreeText に倒れる。**倒れること自体は安全**だが、
// 新しいフラグが黙って値を落とすのは分かりにくいので、
// TestUsage_EveryStringFlagIsClassified が未分類を落とす。
var stringFlagSpecs = map[string]argSpec{
	// --- レコードを指す（通常以上で値を残す） ---
	"tag":         {class: classRecordID, selector: selTag},
	"tags":        {class: classRecordID, selector: selTag},
	"parent":      {class: classRecordID, selector: selTag},
	"add":         {class: classRecordID, selector: selTag},
	"rm":          {class: classRecordID, selector: selTag},
	"set":         {class: classRecordID, selector: selTag},
	"tx":          {class: classRecordID, selector: selTransition},
	"into":        {class: classRecordID, selector: selTransition},
	"vocab":       {class: classRecordID, selector: selVocab},
	"action":      {class: classRecordID, selector: selVocab},
	"given":       {class: classRecordID, selector: selVocab},
	"then":        {class: classRecordID, selector: selVocab},
	"establishes": {class: classRecordID, selector: selVocab},
	"supersedes":  {class: classRecordID, selector: selDecision},
	"id":          {class: classRecordID, selector: selRecord},
	"to":          {class: classRecordID, selector: selRecord},
	"on":          {class: classRecordID, selectorFromPrefix: true},

	// --- 道具の側の閉じた集合（詳細で値を残す） ---
	"sort":        {class: classToolVocab, values: []string{"chrono", "target"}},
	"type":        {class: classToolVocab, values: []string{"tag", "transition", "vocab", "decision"}},
	"category":    {class: classToolVocab, values: categoryValues},
	"fulfillment": {class: classToolVocab, values: []string{model.FulfillmentTransitions, model.FulfillmentProperty}},
	"allow":       {class: classToolVocab, values: []string{"exclusive-violation", "total-kind-mismatch", "id-policy"}},

	// --- 自由文（詳細で長さだけ） ---
	// ⚠️ kind / facet / owner は config が宣言する集合＝**プロジェクトが名付けたもの**なので、
	// 道具の側の語彙ではない。値は残さず、選択子の種類だけ名乗る。
	"kind":                    {class: classFreeText},
	"facet":                   {class: classFreeText, selector: "facet"},
	"owner":                   {class: classFreeText},
	"why":                     {class: classFreeText},
	"changed":                 {class: classFreeText},
	"ref":                     {class: classFreeText},
	"body":                    {class: classFreeText},
	"desc":                    {class: classFreeText},
	"description":             {class: classFreeText},
	"desc-file":               {class: classFreeText},
	"name":                    {class: classFreeText},
	"label":                   {class: classFreeText},
	"alt-label":               {class: classFreeText},
	"reason":                  {class: classFreeText},
	"color":                   {class: classFreeText},
	"source":                  {class: classFreeText},
	"commit":                  {class: classFreeText},
	"acknowledges":            {class: classFreeText},
	"rule":                    {class: classFreeText},
	"allow-decision-retrofit": {class: classFreeText},
	"html":                    {class: classFreeText},
	"host":                    {class: classFreeText},
	"dir":                     {class: classFreeText},
}

// positionalSpecs はコマンドごとの**位置引数**の分類。キーは cobra の CommandPath()。
//
// スライスの各要素が位置に対応し、**最後の要素は残りの位置すべてに適用する**（可変長引数）。
// 空スライスは「位置引数を取らない」の宣言である。
//
// ⚠️ ここに無いコマンドは位置引数の値を一切記録しない（安全側）。
// TestUsage_EveryRunnableSurfaceIsClassified が未分類の面を落とす
// （CLAUDE.md 5: 新しく作った面には、ガードを置き忘れる）。
var positionalSpecs = map[string][]argSpec{
	"scholia config get":             {{class: classFreeText}},
	"scholia config infer-id-policy": {},
	"scholia config set":             {{class: classFreeText}, {class: classFreeText}},
	"scholia decide":                 {},
	"scholia decision add-commit":    {{class: classRecordID, selector: selDecision}, {class: classFreeText}},
	"scholia decision link":          {{class: classRecordID, selector: selDecision}},
	"scholia decision list":          {},
	"scholia decision show":          {{class: classRecordID, selector: selDecision}},
	"scholia diff":                   {{class: classFreeText}}, // git の ref
	"scholia export":                 {},
	"scholia flow":                   {{class: classRecordID, selector: selVocab}},
	"scholia gaps":                   {{class: classRecordID, selector: selVocab}},
	"scholia init":                   {},
	"scholia kind get":               {{class: classToolVocab, values: categoryValues}},
	"scholia kind list":              {},
	"scholia kind set":               {{class: classToolVocab, values: categoryValues}, {class: classFreeText}},
	"scholia lint":                   {},
	"scholia lint baseline update":   {},
	"scholia list":                   {},
	"scholia refs rewrite":           {{class: classRecordID, selector: selRecord}, {class: classRecordID, selector: selRecord}},
	"scholia refs scan":              {},
	"scholia retrofit":               {},
	"scholia review add":             {},
	"scholia review adopt":           {{class: classRecordID, selector: selReview}},
	"scholia review list":            {},
	"scholia review reject":          {{class: classRecordID, selector: selReview}},
	"scholia review rm":              {{class: classRecordID, selector: selReview}},
	"scholia rules":                  {},
	"scholia search":                 {{class: classFreeText}}, // 検索語は自由文
	"scholia show decision":          {{class: classRecordID, selector: selDecision}},
	"scholia show tag":               {{class: classRecordID, selector: selTag}},
	"scholia show tx":                {{class: classRecordID, selector: selTransition}},
	"scholia show vocab":             {{class: classRecordID, selector: selVocab}},
	"scholia skills install":         {},
	"scholia skills ls":              {},
	"scholia skills show":            {{class: classFreeText}},
	"scholia spec":                   {{class: classRecordID, selector: selTag}},
	"scholia tag create":             {{class: classRecordID, selector: selTag}},
	"scholia tag edit":               {{class: classRecordID, selector: selTag}},
	"scholia tag list":               {},
	"scholia tag rename":             {{class: classRecordID, selector: selTag}, {class: classRecordID, selector: selTag}},
	"scholia tag rm":                 {{class: classRecordID, selector: selTag}},
	"scholia tx add":                 {{class: classRecordID, selector: selTransition}},
	"scholia tx edit":                {{class: classRecordID, selector: selTransition}},
	"scholia tx merge":               {{class: classRecordID, selector: selTransition}},
	"scholia tx rename":              {{class: classRecordID, selector: selTransition}},
	"scholia tx rm":                  {{class: classRecordID, selector: selTransition}},
	"scholia tx tag":                 {{class: classRecordID, selector: selTransition}},
	"scholia update":                 {},
	"scholia version":                {},
	"scholia view":                   {},
	"scholia vocab add":              {{class: classToolVocab, values: categoryValues}, {class: classRecordID, selector: selVocab}},
	"scholia vocab edit":             {{class: classRecordID, selector: selVocab}},
	"scholia vocab owner-migrate":    {},
	"scholia vocab rename":           {{class: classRecordID, selector: selVocab}},
	"scholia vocab rm":               {{class: classRecordID, selector: selVocab}},
	"scholia vocab tag":              {{class: classRecordID, selector: selVocab}},
}

// numericFlagTypes は pflag の型名のうち、値そのものが数である（＝道具の側の語彙と同じく
// プロジェクトを指さない）もの。真偽と合わせて、表に載せなくても構造的に扱える。
var numericFlagTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "count": true,
}

// invocationShape は 1 回の呼び出しの形。段に依らない観測結果で、取捨は usage.Records が行う。
type invocationShape struct {
	command      string
	flagNames    []string
	selectorKind string
	argCount     int
	recordIDs    []string
	flagValues   map[string]any
	freeTextLens map[string]int
}

// observeInvocation は実行されたコマンドから呼び出しの形を読み取る。
//
// ⚠️ ここは「どう記録するか」を決めない。**どの値がどの分類か**を引き当てるだけで、
// 段による取捨は usage.Records ただ 1 か所にある。
func observeInvocation(executed *cobra.Command) invocationShape {
	shape := invocationShape{
		command:      executed.CommandPath(),
		flagValues:   map[string]any{},
		freeTextLens: map[string]int{},
	}
	selectors := map[string]bool{}

	executed.Flags().Visit(func(f *flag.Flag) {
		shape.flagNames = append(shape.flagNames, f.Name)
		// 真偽・数値は型から分かるので、表を引かずに値を残せる。
		if v, ok := structuralFlagValue(f); ok {
			shape.flagValues[f.Name] = v
			return
		}
		spec, declared := stringFlagSpecs[f.Name]
		if !declared {
			spec = argSpec{class: classFreeText} // 未分類は自由文へ倒す（安全側）
		}
		for _, raw := range flagStrings(f) {
			shape.apply(f.Name, raw, spec, selectors)
		}
	})
	sort.Strings(shape.flagNames)

	args := executed.Flags().Args()
	shape.argCount = len(args)
	specs := positionalSpecs[executed.CommandPath()]
	for i, raw := range args {
		spec, ok := positionalSpecAt(specs, i)
		if !ok {
			continue // 未分類の面は値を記録しない
		}
		shape.apply("arg"+strconv.Itoa(i), raw, spec, selectors)
	}

	var kinds []string
	for k := range selectors {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	shape.selectorKind = strings.Join(kinds, ",")
	return shape
}

// positionalSpecAt は i 番目の位置引数の分類を返す。最後の宣言は残り全部に適用する。
func positionalSpecAt(specs []argSpec, i int) (argSpec, bool) {
	if len(specs) == 0 {
		return argSpec{}, false
	}
	if i >= len(specs) {
		return specs[len(specs)-1], true
	}
	return specs[i], true
}

// apply は 1 つの値を分類に従って観測へ積む。
//
// ⚠️ **値を残す経路はここだけ**である。classFreeText は長さしか積まない。
func (s *invocationShape) apply(name, raw string, spec argSpec, selectors map[string]bool) {
	if spec.selectorFromPrefix {
		if kind, ok := targetPrefixSelectors[strings.SplitN(raw, ":", 2)[0]]; ok {
			selectors[kind] = true
		}
	} else if spec.selector != "" {
		selectors[spec.selector] = true
	}

	switch spec.class {
	case classRecordID:
		s.recordIDs = append(s.recordIDs, raw)
		s.appendFlagValue(name, raw)
	case classToolVocab:
		if inClosedSet(spec.values, raw) {
			s.appendFlagValue(name, raw)
			return
		}
		// 閉じた集合の外にある値は、道具の側の語彙ではない。自由文へ倒す。
		s.addFreeTextLen(name, raw)
	default:
		s.addFreeTextLen(name, raw)
	}
}

func (s *invocationShape) appendFlagValue(name string, raw string) {
	switch prev := s.flagValues[name].(type) {
	case nil:
		s.flagValues[name] = raw
	case string:
		s.flagValues[name] = []string{prev, raw}
	case []string:
		s.flagValues[name] = append(prev, raw)
	}
}

func (s *invocationShape) addFreeTextLen(name, raw string) {
	s.freeTextLens[name] += utf8.RuneCountInString(raw)
}

func inClosedSet(values []string, raw string) bool {
	for _, v := range values {
		if v == raw {
			return true
		}
	}
	return false
}

// structuralFlagValue は、型だけで「値を残してよい」と分かるフラグの値を返す
// （真偽・数値）。表に載せる必要が無いのは、値がプロジェクトを指しようがないため。
func structuralFlagValue(f *flag.Flag) (any, bool) {
	t := f.Value.Type()
	if t == "bool" {
		b, err := strconv.ParseBool(f.Value.String())
		if err != nil {
			return nil, false
		}
		return b, true
	}
	if numericFlagTypes[t] {
		n, err := strconv.ParseFloat(f.Value.String(), 64)
		if err != nil {
			return nil, false
		}
		return n, true
	}
	return nil, false
}

// flagStrings はフラグの値を文字列の並びとして取り出す（スライス系は要素ごと）。
func flagStrings(f *flag.Flag) []string {
	if sv, ok := f.Value.(flag.SliceValue); ok {
		return sv.GetSlice()
	}
	return []string{f.Value.String()}
}

// isStringLikeFlag は「分類の宣言が要るフラグか」を返す（真偽・数値は要らない）。
func isStringLikeFlag(f *flag.Flag) bool {
	_, structural := structuralFlagValue(f)
	return !structural
}
