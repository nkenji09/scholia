package usage

// Field は行に載りうる記録項目。
//
// ⚠️ **項目を足すときは AllFields にも足すこと。** 行は AllFields を回して組み立てるので、
// ここに無い項目は行に載らない（＝表を通らない経路が作れない）。逆に、ここに足しただけで
// Records に置き場所を決めなければ「記録しない」に倒れる（安全側）。
type Field int

const (
	// --- マスク以上（道具の側の語彙・数・時刻・実行環境の名乗り） ---

	// FieldLevel はその行を書いた段の名前。
	// 「その行が何を記録していない行か」を行だけで判定できるように、段によらず必ず載る。
	FieldLevel Field = iota
	// FieldTimestamp は起動を記録した時刻（RFC3339・UTC）。
	FieldTimestamp
	// FieldCommand は実行されたコマンドのパス（"scholia rules" 等）。道具の側の語彙。
	FieldCommand
	// FieldFlagNames は実際に指定されたフラグの名前。⚠️ 名前だけで、値は含まない。
	FieldFlagNames
	// FieldSelectorKind はどの種類の選択子で引いたか（tag / transition / vocab / decision / facet）。
	// 組み込みの列挙値なので、マスクでも残せる。
	FieldSelectorKind
	// FieldArgCount は位置引数の個数。値ではなく数。
	FieldArgCount
	// FieldExitCode は終了コード。
	FieldExitCode
	// FieldStdoutBytes は標準出力へ渡したバイト数——この計測の主題。
	FieldStdoutBytes
	// FieldDurationUs は所要（マイクロ秒）。
	FieldDurationUs
	// FieldCaller は呼び出し元の名乗り。実行環境が置いた値を写すだけで、AI か人かの判定はしない。
	FieldCaller
	// FieldSessionID はセッション識別子。プロジェクトを指さないので、マスクでも残す
	// （これがあるので 1 セッションあたりの総量はマスクでも出る）。
	FieldSessionID
	// FieldToolVersion は scholia 自身の版。道具の側の語彙。
	FieldToolVersion

	// --- 通常以上（プロジェクトが名付けたものを指す値） ---

	// FieldRecordIDs は呼び出しが名指ししたレコード id。
	FieldRecordIDs
	// FieldProjectRoot はプロジェクトルート。
	FieldProjectRoot

	// --- 詳細のみ（その 1 回の呼び出しの形と、量の内訳） ---

	// FieldFlagValues は列挙値・数値・真偽のフラグの値。⚠️ 自由文の値はここに入らない。
	FieldFlagValues
	// FieldFreeTextLens は自由文の引数の**長さ**（値ではない）。
	FieldFreeTextLens
	// FieldStderrBytes は標準エラーへ渡したバイト数（量が本文側か注記側かを分ける）。
	FieldStderrBytes
	// FieldDurationParts は所要の内訳（マイクロ秒）。
	FieldDurationParts
)

// AllFields は行に載りうる項目すべてを、行に並ぶ順で返す。
//
// ⚠️ 検査（4 段 × 全項目の対）はこれを回す。ここから漏れた項目は検査も素通りする。
func AllFields() []Field {
	return []Field{
		FieldLevel,
		FieldTimestamp,
		FieldCommand,
		FieldFlagNames,
		FieldSelectorKind,
		FieldArgCount,
		FieldExitCode,
		FieldStdoutBytes,
		FieldDurationUs,
		FieldCaller,
		FieldSessionID,
		FieldToolVersion,
		FieldRecordIDs,
		FieldProjectRoot,
		FieldFlagValues,
		FieldFreeTextLens,
		FieldStderrBytes,
		FieldDurationParts,
	}
}

// fieldKeys は項目の JSON キー。キーの集合は段によって変わらない。
var fieldKeys = map[Field]string{
	FieldLevel:         "level",
	FieldTimestamp:     "ts",
	FieldCommand:       "command",
	FieldFlagNames:     "flagNames",
	FieldSelectorKind:  "selectorKind",
	FieldArgCount:      "argCount",
	FieldExitCode:      "exitCode",
	FieldStdoutBytes:   "stdoutBytes",
	FieldDurationUs:    "durationUs",
	FieldCaller:        "caller",
	FieldSessionID:     "sessionId",
	FieldToolVersion:   "toolVersion",
	FieldRecordIDs:     "recordIds",
	FieldProjectRoot:   "projectRoot",
	FieldFlagValues:    "flagValues",
	FieldFreeTextLens:  "freeTextLens",
	FieldStderrBytes:   "stderrBytes",
	FieldDurationParts: "durationPartsUs",
}

// Key は項目の JSON キーを返す。未知の項目は空文字（Line が組み立てを拒む）。
func (f Field) Key() string { return fieldKeys[f] }

// namesProject は「その項目の値が、プロジェクトが名付けたものを指しうるか」の宣言。
//
// ⚠️ これは Records とは独立に書いてある。マスクが漏らさないことは
// 「Records(Masked, f) ならば !namesProject[f]」という性質で検査でき、
// 片方だけを書き換えれば検査が落ちる。
var namesProject = map[Field]bool{
	FieldRecordIDs:   true, // レコード id はプロジェクトが名付けたもの
	FieldProjectRoot: true, // パスにプロジェクト名が入る

	// 詳細の 4 項目は、通常（＝プロジェクトを名指しする段）の上にしか立たない。
	// flagValues は選択子の値（レコード id）を含みうるし、freeTextLens は
	// 「プロジェクトが名付けたものの長さ」を含みうる——長さは名前を指す。
	FieldFlagValues:   true,
	FieldFreeTextLens: true,
}

// NamesProject は、その項目の値がプロジェクトが名付けたものを指しうるかを返す。
func (f Field) NamesProject() bool { return namesProject[f] }

// Records は「その段で、その項目を記録するか」を返す**唯一の判断**である。
//
// ⚠️ この関数は出力の書き方から切り離してある（正本の歯止め 1）。
// 行の組み立て（Line）はここを通してしか値を置かない。段を足すときも項目を足すときも、
// ここに載せない限り記録されない——未分類は「記録しない」に倒れる（安全側）。
//
// 検査は 4 段 × 全項目の対で行う（歯止め 2・fields_test.go）。
func Records(l Level, f Field) bool {
	if l <= Off {
		// オフは観測しない・書かない。行そのものが出ないので、全項目が false。
		return false
	}
	switch f {
	case FieldLevel,
		FieldTimestamp,
		FieldCommand,
		FieldFlagNames,
		FieldSelectorKind,
		FieldArgCount,
		FieldExitCode,
		FieldStdoutBytes,
		FieldDurationUs,
		FieldCaller,
		FieldSessionID,
		FieldToolVersion:
		return l >= Masked
	case FieldRecordIDs,
		FieldProjectRoot:
		return l >= Normal
	case FieldFlagValues,
		FieldFreeTextLens,
		FieldStderrBytes,
		FieldDurationParts:
		return l >= Detailed
	}
	// 表に載っていない項目は記録しない。
	return false
}
