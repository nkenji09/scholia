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

// lint の抑止キー（--allow）も道具の側の閉じた集合。
var lintAllowValues = []string{"exclusive-violation", "total-kind-mismatch", "id-policy"}

// stringFlagSpecs は**文字列を取るフラグ**の分類。
//
// ⚠️ **キーは「そのフラグを宣言したコマンド」と「フラグ名」の組**である（flagSpecKey）。
// フラグ名だけで引いてはいけない。**名前だけで引くと、新しいコマンドが `--to` `--id` のような
// 一般的な名前を使ったときに、誰も何も宣言しないまま既存の分類を継承する。**
// これは「分類し忘れ」と違って**検査を通ってしまう**——実際に `scholia export` へ自由文のパスを取る
// `--to` を足すだけで、通常の段の recordIds に自由文のパスが出た（正本 01KYSKM4T0RWRY1N7407KZSZ17
// 条項 3 違反）。組で引く限り、新しい (コマンド, フラグ) は必ず未宣言＝自由文から始まる。
//
// ⚠️ 継承した永続フラグ（`--dir`）は**宣言元**（`scholia`）の 1 行で引く。宣言が 1 つなら分類も 1 つで、
// 子コマンドが増えるたびに同じ宣言を書き足させる必要は無い（flagDeclarationPath）。
//
// ⚠️ 真偽・数値のフラグはここに載せない——型から分かるので構造的に扱う（下の structuralFlagValue）。
// ⚠️ ここに無い組は classFreeText に倒れる。**倒れること自体は安全**だが、
// 新しいフラグが黙って値を落とすのは分かりにくいので、
// TestUsage_EveryStringFlagIsClassified が未宣言の組を落とす。
var stringFlagSpecs = map[string]argSpec{
	// --- レコードを指す（通常以上で値を残す） ---
	"scholia decide --on":                {class: classRecordID, selectorFromPrefix: true},
	"scholia decide --supersedes":        {class: classRecordID, selector: selDecision},
	"scholia decision link --supersedes": {class: classRecordID, selector: selDecision},
	"scholia decision list --on":         {class: classRecordID, selectorFromPrefix: true},
	"scholia list --tag":                 {class: classRecordID, selector: selTag},
	"scholia refs scan --id":             {class: classRecordID, selector: selRecord},
	"scholia review add --on":            {class: classRecordID, selectorFromPrefix: true},
	"scholia review add --supersedes":    {class: classRecordID, selector: selDecision},
	"scholia review adopt --supersedes":  {class: classRecordID, selector: selDecision},
	"scholia review list --on":           {class: classRecordID, selectorFromPrefix: true},
	"scholia rules --tag":                {class: classRecordID, selector: selTag},
	"scholia rules --tx":                 {class: classRecordID, selector: selTransition},
	"scholia rules --vocab":              {class: classRecordID, selector: selVocab},
	"scholia search --tag":               {class: classRecordID, selector: selTag},
	"scholia tag create --parent":        {class: classRecordID, selector: selTag},
	"scholia tag edit --parent":          {class: classRecordID, selector: selTag},
	"scholia tx add --action":            {class: classRecordID, selector: selVocab},
	"scholia tx add --given":             {class: classRecordID, selector: selVocab},
	"scholia tx add --tags":              {class: classRecordID, selector: selTag},
	"scholia tx add --then":              {class: classRecordID, selector: selVocab},
	"scholia tx edit --action":           {class: classRecordID, selector: selVocab},
	"scholia tx edit --given":            {class: classRecordID, selector: selVocab},
	"scholia tx edit --tags":             {class: classRecordID, selector: selTag},
	"scholia tx edit --then":             {class: classRecordID, selector: selVocab},
	"scholia tx merge --into":            {class: classRecordID, selector: selTransition},
	"scholia tx rename --to":             {class: classRecordID, selector: selRecord},
	"scholia tx tag --add":               {class: classRecordID, selector: selTag},
	"scholia tx tag --rm":                {class: classRecordID, selector: selTag},
	"scholia tx tag --set":               {class: classRecordID, selector: selTag},
	"scholia vocab add --establishes":    {class: classRecordID, selector: selVocab},
	"scholia vocab edit --establishes":   {class: classRecordID, selector: selVocab},
	"scholia vocab rename --to":          {class: classRecordID, selector: selRecord},
	"scholia vocab tag --add":            {class: classRecordID, selector: selTag},
	"scholia vocab tag --rm":             {class: classRecordID, selector: selTag},

	// --- 道具の側の閉じた集合（詳細で値を残す） ---
	"scholia rules --sort":           {class: classToolVocab, values: []string{"chrono", "target"}},
	"scholia search --type":          {class: classToolVocab, values: []string{"tag", "transition", "vocab", "decision"}},
	"scholia tag create --allow":     {class: classToolVocab, values: lintAllowValues},
	"scholia tag edit --allow":       {class: classToolVocab, values: lintAllowValues},
	"scholia tag edit --fulfillment": {class: classToolVocab, values: []string{model.FulfillmentTransitions, model.FulfillmentProperty}},
	"scholia tx add --allow":         {class: classToolVocab, values: lintAllowValues},
	"scholia tx edit --allow":        {class: classToolVocab, values: lintAllowValues},
	"scholia vocab add --allow":      {class: classToolVocab, values: lintAllowValues},
	"scholia vocab rm --category":    {class: classToolVocab, values: categoryValues},

	// --- 自由文（詳細で長さだけ） ---
	// ⚠️ kind / facet / owner は config が宣言する集合＝**プロジェクトが名付けたもの**なので、
	// 道具の側の語彙ではない。値は残さず、選択子の種類だけ名乗る。
	"scholia --dir":                          {class: classFreeText},
	"scholia decide --acknowledges":          {class: classFreeText},
	"scholia decide --changed":               {class: classFreeText},
	"scholia decide --commit":                {class: classFreeText},
	"scholia decide --ref":                   {class: classFreeText},
	"scholia decide --why":                   {class: classFreeText},
	"scholia diff --allow-decision-retrofit": {class: classFreeText},
	"scholia export --html":                  {class: classFreeText},
	"scholia list --facet":                   {class: classFreeText, selector: "facet"},
	"scholia list --kind":                    {class: classFreeText},
	"scholia retrofit --rule":                {class: classFreeText},
	"scholia review add --body":              {class: classFreeText},
	"scholia review add --source":            {class: classFreeText},
	"scholia review adopt --changed":         {class: classFreeText},
	"scholia review adopt --ref":             {class: classFreeText},
	"scholia review adopt --why":             {class: classFreeText},
	"scholia review reject --changed":        {class: classFreeText},
	"scholia review reject --ref":            {class: classFreeText},
	"scholia review reject --why":            {class: classFreeText},
	"scholia rules --facet":                  {class: classFreeText, selector: "facet"},
	"scholia tag create --color":             {class: classFreeText},
	"scholia tag create --desc":              {class: classFreeText},
	"scholia tag create --desc-file":         {class: classFreeText},
	"scholia tag create --kind":              {class: classFreeText},
	"scholia tag create --name":              {class: classFreeText},
	"scholia tag create --reason":            {class: classFreeText},
	"scholia tag create --ref":               {class: classFreeText},
	"scholia tag edit --color":               {class: classFreeText},
	"scholia tag edit --desc":                {class: classFreeText},
	"scholia tag edit --desc-file":           {class: classFreeText},
	"scholia tag edit --kind":                {class: classFreeText},
	"scholia tag edit --name":                {class: classFreeText},
	"scholia tag edit --reason":              {class: classFreeText},
	"scholia tag edit --ref":                 {class: classFreeText},
	"scholia tag list --kind":                {class: classFreeText},
	"scholia tx add --reason":                {class: classFreeText},
	"scholia tx edit --reason":               {class: classFreeText},
	"scholia tx rm --why":                    {class: classFreeText},
	"scholia view --host":                    {class: classFreeText},
	"scholia vocab add --alt-label":          {class: classFreeText},
	"scholia vocab add --desc-file":          {class: classFreeText},
	"scholia vocab add --description":        {class: classFreeText},
	"scholia vocab add --kind":               {class: classFreeText},
	"scholia vocab add --label":              {class: classFreeText},
	"scholia vocab add --owner":              {class: classFreeText},
	"scholia vocab add --reason":             {class: classFreeText},
	"scholia vocab add --ref":                {class: classFreeText},
	"scholia vocab edit --alt-label":         {class: classFreeText},
	"scholia vocab edit --desc-file":         {class: classFreeText},
	"scholia vocab edit --description":       {class: classFreeText},
	"scholia vocab edit --kind":              {class: classFreeText},
	"scholia vocab edit --label":             {class: classFreeText},
	"scholia vocab edit --owner":             {class: classFreeText},
	"scholia vocab edit --ref":               {class: classFreeText},
}

// positionalSpec は 1 つのコマンドの**位置引数**の宣言。
type positionalSpec struct {
	// at は位置ごとの宣言（0 番から順に）。空なら位置引数の値を記録しない。
	at []argSpec
	// variadic は「at の最後の宣言が、それより後ろの位置すべてに適用される」ことを示す。
	//
	// ⚠️ **可変長のコマンドだけが宣言する。** ここを既定（暗黙に最後の宣言を延ばす）にすると、
	// 既存のコマンドに位置が 1 つ増えたときに、**誰も何も宣言しないまま最後の分類を継承する**
	// ——フラグの表がフラグ名だけで引いていたのと同じ形の穴になる。
	// 宣言していないコマンドでは、宣言を超えた位置は「記録しない」（安全側）へ倒れる。
	variadic bool
}

// positionalSpecs はコマンドごとの位置引数の分類。キーは cobra の CommandPath()。
//
// ⚠️ **キーは初めからコマンドパスなので、フラグの表にあった「名前だけで引く」穴はここには無い。**
// 新しいコマンドは必ず未宣言から始まる。
//
// ⚠️ ここに無いコマンドは位置引数の値を一切記録しない（安全側）。
// TestUsage_EveryRunnableSurfaceIsClassified が未分類の面を落とす
// （CLAUDE.md 5: 新しく作った面には、ガードを置き忘れる）。
var positionalSpecs = map[string]positionalSpec{
	"scholia config get":             {at: []argSpec{{class: classFreeText}}},
	"scholia config infer-id-policy": {},
	"scholia config set":             {at: []argSpec{{class: classFreeText}, {class: classFreeText}}},
	"scholia decide":                 {},
	// 2 つ目以降は commit hash の並び（cobra.MinimumNArgs(2)）。
	"scholia decision add-commit": {at: []argSpec{{class: classRecordID, selector: selDecision}, {class: classFreeText}}, variadic: true},
	"scholia decision link":       {at: []argSpec{{class: classRecordID, selector: selDecision}}},
	"scholia decision list":       {},
	"scholia decision show":       {at: []argSpec{{class: classRecordID, selector: selDecision}}},
	// git の ref を 2 つまで（cobra.MaximumNArgs(2)）。**位置ごとに書く**——可変長ではない。
	"scholia diff":                 {at: []argSpec{{class: classFreeText}, {class: classFreeText}}},
	"scholia export":               {},
	"scholia flow":                 {at: []argSpec{{class: classRecordID, selector: selVocab}}},
	"scholia gaps":                 {at: []argSpec{{class: classRecordID, selector: selVocab}}},
	"scholia init":                 {},
	"scholia kind get":             {at: []argSpec{{class: classToolVocab, values: categoryValues}}},
	"scholia kind list":            {},
	"scholia kind set":             {at: []argSpec{{class: classToolVocab, values: categoryValues}, {class: classFreeText}}},
	"scholia lint":                 {},
	"scholia lint baseline update": {},
	"scholia list":                 {},
	"scholia refs rewrite":         {at: []argSpec{{class: classRecordID, selector: selRecord}, {class: classRecordID, selector: selRecord}}},
	"scholia refs scan":            {},
	"scholia retrofit":             {},
	"scholia review add":           {},
	"scholia review adopt":         {at: []argSpec{{class: classRecordID, selector: selReview}}},
	"scholia review list":          {},
	"scholia review reject":        {at: []argSpec{{class: classRecordID, selector: selReview}}},
	"scholia review rm":            {at: []argSpec{{class: classRecordID, selector: selReview}}},
	"scholia rules":                {},
	// 検索語は自由文で、いくつでも取る（cobra.MinimumNArgs(1)）。
	"scholia search":              {at: []argSpec{{class: classFreeText}}, variadic: true},
	"scholia show decision":       {at: []argSpec{{class: classRecordID, selector: selDecision}}},
	"scholia show tag":            {at: []argSpec{{class: classRecordID, selector: selTag}}},
	"scholia show tx":             {at: []argSpec{{class: classRecordID, selector: selTransition}}},
	"scholia show vocab":          {at: []argSpec{{class: classRecordID, selector: selVocab}}},
	"scholia skills install":      {},
	"scholia skills ls":           {},
	"scholia skills show":         {at: []argSpec{{class: classFreeText}}},
	"scholia spec":                {at: []argSpec{{class: classRecordID, selector: selTag}}},
	"scholia tag create":          {at: []argSpec{{class: classRecordID, selector: selTag}}},
	"scholia tag edit":            {at: []argSpec{{class: classRecordID, selector: selTag}}},
	"scholia tag list":            {},
	"scholia tag rename":          {at: []argSpec{{class: classRecordID, selector: selTag}, {class: classRecordID, selector: selTag}}},
	"scholia tag rm":              {at: []argSpec{{class: classRecordID, selector: selTag}}},
	"scholia tx add":              {at: []argSpec{{class: classRecordID, selector: selTransition}}},
	"scholia tx edit":             {at: []argSpec{{class: classRecordID, selector: selTransition}}},
	"scholia tx merge":            {at: []argSpec{{class: classRecordID, selector: selTransition}}},
	"scholia tx rename":           {at: []argSpec{{class: classRecordID, selector: selTransition}}},
	"scholia tx rm":               {at: []argSpec{{class: classRecordID, selector: selTransition}}},
	"scholia tx tag":              {at: []argSpec{{class: classRecordID, selector: selTransition}}},
	"scholia update":              {},
	"scholia version":             {},
	"scholia view":                {},
	"scholia vocab add":           {at: []argSpec{{class: classToolVocab, values: categoryValues}, {class: classRecordID, selector: selVocab}}},
	"scholia vocab edit":          {at: []argSpec{{class: classRecordID, selector: selVocab}}},
	"scholia vocab owner-migrate": {},
	"scholia vocab rename":        {at: []argSpec{{class: classRecordID, selector: selVocab}}},
	"scholia vocab rm":            {at: []argSpec{{class: classRecordID, selector: selVocab}}},
	"scholia vocab tag":           {at: []argSpec{{class: classRecordID, selector: selVocab}}},
}

// flagSpecKey は分類表のキー。**(コマンド, フラグ名) の組**で引く。
//
// 検査（TestUsage_EveryStringFlagIsClassified）も実行時の引き当ても、この 1 つの関数を通る。
// 検査が別の綴りでキーを組むと、検査が通るのに実行時は未宣言、という食い違いが起きる。
func flagSpecKey(commandPath, flagName string) string {
	return commandPath + " --" + flagName
}

// lookupStringFlagSpec は、実行されたコマンドとフラグ名から分類を引き当てる。
//
// ⚠️ **フラグ名だけでは引かない。** 引き当てるのは (宣言したコマンド, フラグ名) の組で、
// 表に無い組は未宣言＝自由文へ倒れる（呼び出し側で classFreeText を与える）。
func lookupStringFlagSpec(executed *cobra.Command, name string) (argSpec, bool) {
	spec, ok := stringFlagSpecs[flagSpecKey(flagDeclarationPath(executed, name), name)]
	return spec, ok
}

// flagDeclarationPath は、そのフラグを**宣言したコマンド**の CommandPath を返す。
//
// 継承した永続フラグは、実行されたコマンドではなく**宣言元**の名前で引く。
// 宣言が 1 つなら分類も 1 つであるべきで、子コマンドが増えるたびに同じ宣言を
// 書き足させるのは表を無意味に太らせるだけだからである。
func flagDeclarationPath(executed *cobra.Command, name string) string {
	// 実行されたコマンド自身の宣言（局所・永続とも）が最優先。親の同名を隠す場合もここで決まる。
	if executed.LocalFlags().Lookup(name) != nil {
		return executed.CommandPath()
	}
	for c := executed.Parent(); c != nil; c = c.Parent() {
		if c.PersistentFlags().Lookup(name) != nil {
			return c.CommandPath()
		}
	}
	// どこにも宣言が見つからないときは実行されたコマンドの名前で引く（＝未宣言＝自由文へ倒れる）。
	return executed.CommandPath()
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
		spec, declared := lookupStringFlagSpec(executed, f.Name)
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

// positionalSpecAt は i 番目の位置引数の分類を返す。
//
// ⚠️ 宣言を超えた位置は、**可変長と名乗ったコマンドでだけ**最後の宣言を延ばす。
// 名乗っていなければ「記録しない」へ倒れる——位置が 1 つ増えたときに黙って分類を継承させないため。
func positionalSpecAt(spec positionalSpec, i int) (argSpec, bool) {
	if i < len(spec.at) {
		return spec.at[i], true
	}
	if spec.variadic && len(spec.at) > 0 {
		return spec.at[len(spec.at)-1], true
	}
	return argSpec{}, false
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
