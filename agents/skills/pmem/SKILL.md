<!-- draft: Phase 4 で正式化 -->
---
name: pmem
description: product-memory (pmem) で意思決定の記録を読み書きする。コンポーネント/フローの振る舞いを語彙の組み合わせとして蓄積し、意思決定の履歴を残す／過去の意図と突き合わせて変更を評価するときに使う。
---

# pmem — 意思決定の記録を蓄積・評価する CLI

## これは何のためか（DESIGN §0）

pmem の主目的は**バグの早期検知ではない**。次の 2 つ:

1. **意思決定の記録を蓄積する** — 「この振る舞いは既に検討済みで、これが正しい」という判断を、
   なぜそうしたか（why）とともに残す。記録は AI が次の作業で読み込む「守るべき規則」になる。
2. **変更を評価する土台にする** — 後から来る修正依頼・仕様変更を「本当に取り込むべきか」を、
   過去の意図と突き合わせて判断する基準にする。

`pmem lint` も「早期バグ検知」ではなく、**記録が自己矛盾していないことの整合性チェック**として扱う。

## いつ pmem を使うか

- コンポーネントやフローの**詳細な振る舞い**（条件・アクション・効果）を記録したいとき。
- 「これは既に検討済みで、この形が正しい」という**意思決定**を残したいとき。
- 修正依頼・仕様変更の提案を受け取り、**過去の decision と矛盾しないか**を評価したいとき。
- 記録の整合性（網羅漏れ・矛盾）を機械的に確認したいとき（`pmem lint`）。

pmem は任意の repo・任意の言語・任意の AI エージェントで動くスタンドアロン CLI（単一バイナリ、ランタイム前提ゼロ）。
記録は repo 内の素の JSON としてコードと同じ版で版管理される。

## 基本フロー（記録する）

```
pmem init                                    # .pmem/ を作成
pmem vocab add <condition|action|effect> <id> --label <l>   # 語彙を先に登録
pmem tag create <id> --name <n> [--parent <tagId>…]         # 主題・要件などのタグ
pmem tx add <id> --action <a> [--given <c,…>] --then <e,…> [--tags <t,…>] [--test <s>…]
pmem decide --on <transition|tag>:<id> --why <t>             # なぜそうしたかを残す
pmem lint                                     # 記録の自己矛盾チェック（緑＝網羅の証明ではない）
```

CLI 全体は `pmem <cmd> --help` が真値。全書き込み系コマンドに `--json` あり（エージェント駆動用）。

## 変更評価フロー（提案を評価する）

spec ファイルも proposal ファイルも無い。**提案 = git ブランチ**、**評価結果 = decision**に落ちる（DESIGN §4）。

```
pmem diff [<gitref>]         # 現在 vs <gitref>（既定 HEAD）の semantic diff
                              # （語彙± / 遷移± / then 順序 / decisions±）
pmem rules --tag <id>        # その提案が触るタグの過去 decisions（守る規則）と照合
pmem decide --on transition:<id> --why "評価: 取り込まない。<理由>" --ref <PR/URL>
                              # adopt = ブランチを merge ＋「採用」decision を append
                              # reject = merge しない ＋「取り込まない・理由」decision を append
```

判定材料:
- **(a) 複雑性 diff**（語彙±・遷移±・then 順序）
- **(b) 既存 decision と矛盾するか**（衝突＝却下寄り。矛盾する decision の id を引用する）
- **(c) 既に検討済みか**（`pmem rules` で過去の結着を確認）

decision は **append-only**（過去を消す提案＝取り込み拒否の最有力根拠）。「却下」を一言で済ませず、
**なぜ取り込まないかを decision に根拠つきで残す**——それが次回同じ提案が来たときの既決になる。

## 良い記録の書き方（DESIGN §8）— CLI の how より what で価値が決まる

1. **アクションの網羅** — 対象の外部 IF（入口）から機械的に洗い出す。ここで漏らすと以降すべて漏れる。
2. **条件(given)の網羅** — 決定表で。排他的な原因群は畳まず別遷移に。
3. **効果(then)を妄想で書かない** — 実際に起きることだけ。全 emit/効果が then に現れるか逆引きする。
4. **完了ゲート（必須）** — 主題タグ単位で: マトリクス空白ゼロ／`tests` と実テストの双方向突合／穴探し 1 周。
   **`pmem lint` が緑でも網羅の証明にはならない** — 完了ゲートは手動の突合として別に行う。
5. **decision の質** — 後から矛盾に気づける why を書く（`file:line` のような不変でない参照は避ける）。
   コンポーネント横断の規則（cross-cutting invariant）はタグに刻む。

## CLI コマンド表（DESIGN §6 要約。真値は `pmem <cmd> --help`）

```
# セットアップ
pmem init [--dir <path>]
pmem kind set <condition|action|effect> <k1,k2,...>
pmem config get|set <key> [<value>]

# 語彙・タグ
pmem vocab add <condition|action|effect> <id> --label <l> [--kind <k>] [--owner <l>]
pmem vocab rm <id> [--category <c>]
pmem vocab tag <id> --add <tagId>… [--rm <tagId>…]
pmem vocab rename <id> --to <newId>
pmem tag create <id> --name <n> [--kind <k>] [--parent <tagId>…] [--desc <t>] [--ref <url>]
pmem tag list [--kind <k>] [--tree] [--json]
pmem tag edit <id> [--name][--kind][--parent…][--desc]
pmem tag rm <id> [--force]

# 遷移（原子）
pmem tx add <id> --action <a> [--given <c,…>] --then <e,…> [--tags <t,…>] [--test <s>…]
pmem tx edit <id> [--action][--given][--then][--tags][--test][--clear-tests]
pmem tx tag <id> --add <tagId>… [--rm <tagId>…] | --set <ids>
pmem tx rename <id> --to <newId>
pmem tx rm <id> --why <理由> --force

# 意思決定
pmem decide --on <transition|tag>:<id> --why <t> [--changed <s>] [--ref <s>]

# 読み取り / 派生ビュー
pmem show tx <id> [--resolve] [--json]
pmem spec <subjectTag> [--json]
pmem list [--facet <tagKind>] [--tag <id>] [--kind <k>] [--json]
pmem rules [--tag <id> | --tx <id> | --facet <k>] [--sort chrono|target] [--json]
pmem lint [--json]
pmem diff [<gitref>] [--json]

# インデックス / ビューア
pmem index [--rebuild]
pmem view [--port <p>]
pmem export --html <dir>
```
