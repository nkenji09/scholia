// Package skills embeds the Claude Code skill tree under agents/skills/ into
// the scholia binary so `scholia skills install` can materialize it without
// requiring the caller's cwd to contain this repo (e.g. after `go install`).
//
// 各スキルは相対パスで参照し合う（例: scholia-change → ../scholia/SKILL.md、
// 各 SKILL → ../_scholia-shared/references/modeling-principles.md）ため、
// FS のルートには scholia/scholia-change/scholia-triage/scholia-config-setup/_scholia-shared が
// 同階層で並ぶ（展開先でもこの構造をそのまま保つ）。
//
// 落とし穴: //go:embed はデフォルトで "_" や "." で始まる名前を除外する。
// _scholia-shared を含めるには all: プレフィックスが必須。
package skills

import "embed"

//go:embed all:scholia all:scholia-change all:scholia-triage all:scholia-config-setup all:_scholia-shared
var FS embed.FS
