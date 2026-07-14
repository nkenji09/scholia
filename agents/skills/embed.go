// Package skills embeds the Claude Code skill tree under agents/skills/ into
// the pmem binary so `pmem skills install` can materialize it without
// requiring the caller's cwd to contain this repo (e.g. after `go install`).
//
// 各スキルは相対パスで参照し合う（例: pmem-change → ../pmem/SKILL.md、
// 各 SKILL → ../_pmem-shared/references/modeling-principles.md）ため、
// FS のルートには pmem/pmem-change/pmem-triage/pmem-config-setup/_pmem-shared が
// 同階層で並ぶ（展開先でもこの構造をそのまま保つ）。
//
// 落とし穴: //go:embed はデフォルトで "_" や "." で始まる名前を除外する。
// _pmem-shared を含めるには all: プレフィックスが必須。
package skills

import "embed"

//go:embed all:pmem all:pmem-change all:pmem-triage all:pmem-config-setup all:_pmem-shared
var FS embed.FS
