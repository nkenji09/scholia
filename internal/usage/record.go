package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Observation は 1 回の起動について観測した素材。
//
// ⚠️ ここは「観測できたもの」を素直に持つだけで、段による取捨はしない。
// 取捨は Records ただ 1 か所で行う（歯止め 1）。
type Observation struct {
	Timestamp     time.Time
	Command       string
	FlagNames     []string
	SelectorKind  string
	ArgCount      int
	ExitCode      int
	StdoutBytes   int64
	Duration      time.Duration
	Caller        string
	SessionID     string
	ToolVersion   string
	RecordIDs     []string
	ProjectRoot   string
	FlagValues    map[string]any
	FreeTextLens  map[string]int
	StderrBytes   int64
	DurationParts map[string]int64
}

// value は項目に対応する観測値を返す。
//
// ⚠️ 「記録しない」を表すのは null ただ 1 つなので、**記録すると決めた項目は決して nil を返さない**
// （空のスライス・空のマップは空のまま返す——「記録したが空だった」と「記録していない」は別の事実である）。
// 未知の項目に対しては nil を返し、Line がそれを組み立ての失敗として扱う。
func (o Observation) value(l Level, f Field) any {
	switch f {
	case FieldLevel:
		return l.String()
	case FieldTimestamp:
		return o.Timestamp.UTC().Format(time.RFC3339Nano)
	case FieldCommand:
		return o.Command
	case FieldFlagNames:
		return nonNilStrings(o.FlagNames)
	case FieldSelectorKind:
		return o.SelectorKind
	case FieldArgCount:
		return o.ArgCount
	case FieldExitCode:
		return o.ExitCode
	case FieldStdoutBytes:
		return o.StdoutBytes
	case FieldDurationUs:
		return o.Duration.Microseconds()
	case FieldCaller:
		return o.Caller
	case FieldSessionID:
		return o.SessionID
	case FieldToolVersion:
		return o.ToolVersion
	case FieldRecordIDs:
		return nonNilStrings(o.RecordIDs)
	case FieldProjectRoot:
		return o.ProjectRoot
	case FieldFlagValues:
		if o.FlagValues == nil {
			return map[string]any{}
		}
		return o.FlagValues
	case FieldFreeTextLens:
		if o.FreeTextLens == nil {
			return map[string]int{}
		}
		return o.FreeTextLens
	case FieldStderrBytes:
		return o.StderrBytes
	case FieldDurationParts:
		if o.DurationParts == nil {
			return map[string]int64{}
		}
		return o.DurationParts
	}
	return nil
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Line は 1 起動 1 行の JSON（末尾に改行）を組み立てる。
//
// **キーの集合は段によって変わらない。** AllFields を順に回し、Records が false の項目は
// null で埋める——集計する側が段ごとに別の読み方をしなくて済むように、欠落させない。
// null はただ 1 つの「記録していない」の形である。
func Line(l Level, obs Observation) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, f := range AllFields() {
		key := f.Key()
		if key == "" {
			return nil, fmt.Errorf("usage: 項目 %d に JSON キーがありません", int(f))
		}
		if i > 0 {
			buf.WriteByte(',')
		}
		encoded, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(encoded)
		buf.WriteByte(':')

		var v any
		if Records(l, f) {
			v = obs.value(l, f)
			if v == nil {
				// 記録すると決めた項目が値を持たないのは、value の場合分けが
				// 項目に追いついていないということ。null で黙って埋めると
				// 「記録していない」と読めてしまうので、行ごと諦める。
				return nil, fmt.Errorf("usage: 項目 %q は %s で記録すると決まっているのに値がありません", key, l)
			}
		}
		encoded, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
		buf.Write(encoded)
	}
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}
