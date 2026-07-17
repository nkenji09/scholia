---
name: scholia
description: scholia で意思決定の記録を読み書きする。コンポーネント/フローの振る舞いを語彙の組み合わせとして蓄積し、意思決定の履歴を残す／過去の意図と突き合わせて変更を評価するときに使う。
---

# scholia — 意思決定の記録を蓄積・評価する CLI

## これは何のためか（DESIGN §0）

scholia の主目的は**バグの早期検知ではない**。次の 2 つ:

1. **意思決定の記録を蓄積する** — 「この振る舞いは既に検討済みで、これが正しい」という判断を、
   なぜそうしたか（why）とともに残す。記録は AI が次の作業で読み込む「守るべき規則」になる。
2. **変更を評価する土台にする** — 後から来る修正依頼・仕様変更を「本当に取り込むべきか」を、
   過去の意図と突き合わせて判断する基準にする。

`scholia lint` も「早期バグ検知」ではなく、**記録が自己矛盾していないことの整合性チェック**として扱う。

## いつ scholia を使うか

- コンポーネントやフローの**詳細な振る舞い**（条件・アクション・効果）を記録したいとき。
- 「これは既に検討済みで、この形が正しい」という**意思決定**を残したいとき。
- 修正依頼・仕様変更の提案を受け取り、**過去の decision と矛盾しないか**を評価したいとき。
- 記録の整合性（網羅漏れ・矛盾）を機械的に確認したいとき（`scholia lint`）。

scholia は任意の repo・任意の言語・任意の AI エージェントで動くスタンドアロン CLI（単一バイナリ、ランタイム前提ゼロ）。
記録は repo 内の素の JSON としてコードと同じ版で版管理される。

新規プロジェクトへの導入直後の「初回 config セットアップ」（tagKinds/facetKinds/traceabilityKinds を
プロダクトに仕立て、初期 vocab/tags を撒く）は [scholia-config-setup スキル](../scholia-config-setup/SKILL.md) へ。
このスキルは日々の読み書きの範囲。

## 基本フロー（記録する）

```
scholia init                                    # .scholia/ を作成
scholia vocab add <condition|action|effect> <id> --label <l>   # 語彙を先に登録
scholia tag create <id> --name <n> [--parent <tagId>…]         # 主題・要件などのタグ
scholia tx add <id> --action <a> [--given <c,…>] --then <e,…> [--tags <t,…>]
scholia decide --on <transition|tag>:<id> --why <t> [--commit <hash>…]   # なぜそうしたかを残す（実装 commit も結べる）
scholia lint                                     # 記録の自己矛盾チェック（緑＝網羅の証明ではない）
```

CLI 全体は `scholia <cmd> --help` が真値。全書き込み系コマンドに `--json` あり（エージェント駆動用）。
書き込み系の `--json` は応答封筒 `{ record, advisories }` を返す（保存したレコード＋同一ターンの書き方規律警告。
`--allow` 使用時は `allowed` に規則・理由が記録される）。

## 変更評価フロー（提案を評価する）

spec ファイルも proposal ファイルも無い。**提案 = 作業ツリーの pending 変更＋それを説明するコメント**、
**評価結果 = decision**に落ちる（DESIGN §4）。commit は評価の結果であって前提ではない。

```
scholia diff [<ref1> [<ref2>]]  # 現在 vs <ref1>（既定 HEAD）＝pending diff（主線）
                              # <ref1> vs <ref2>（両方 git ref）は landed 監査用（例: <commit>^ <commit>）
                              # semantic diff（語彙± / 遷移± / then 順序 / decisions±）
scholia rules --tag <id>        # その提案が触るタグの過去 decisions（守る規則）と照合
scholia decide --on transition:<id> --why "評価: 取り込まない。<理由>" --ref <PR/URL> [--commit <hash>…]
                              # adopt = 変更を採用 ＋「採用」decision を append
                              # reject = 採用しない ＋「取り込まない・理由」decision を append
```

変更評価はビューアのコメントドロワーにインラインで行える（提案＝変更を持つレコードのコメント・本文＝why・
採用でその why を decision へ昇格）。UI 評価と CLI 両フローの具体手順は下記 scholia-change スキルへ。

判定材料:
- **(a) 複雑性 diff**（語彙±・遷移±・then 順序）
- **(b) 既存 decision と矛盾するか**（衝突＝却下寄り。矛盾する decision の id を引用する）
- **(c) 既に検討済みか**（`scholia rules` で過去の結着を確認）

decision は **append-only**（過去を消す提案＝取り込み拒否の最有力根拠）。「却下」を一言で済ませず、
**なぜ取り込まないかを decision に根拠つきで残す**——それが次回同じ提案が来たときの既決になる。

上記の判定材料をもとに、input（修正指示・要望・レビューコメント）を spec に照らして**対応方針を選別する**
（A是正／B精緻化／C矛盾／D新規／E却下 に分類し、方針＋WHY を出す判断層）は
[scholia-triage スキル](../scholia-triage/SKILL.md) へ。方針が出た後、decision に着地するまでの具体的な手順
（波及検索・兄弟 transition との整合・`commits[]` の結線）は [scholia-change スキル](../scholia-change/SKILL.md) へ。

## 良い記録の書き方（DESIGN §8）— CLI の how より what で価値が決まる

> **粒度・同一性・命名・記録の役割分担は共有リファレンス `../_scholia-shared/references/modeling-principles.md` に従う**
> （スキルのベースディレクトリ起点）。要点: **vocab=実装の同一性**（逆引き＝真の影響集合）／**tag=概念の族**
> （逆引き＝族の一覧・依存を主張しない）／横断ルールは概念 tag への decision／transition は `tx.<Component>.<name>`／
> desc=最新形・decision=履歴／コンポ別語彙は「遷移から kind で束ねる」派生ビュー／実装の同一性は実装フェーズの関心で
> 設計段階は per-component 既定。

**記録を書く前のチェック（意味判断3点のみ・機械判定できる規律は保存時の advisory が担う・正典 → `../_scholia-shared/references/modeling-principles.md`）**:

- [ ] **軸は action から見つけたか（投影 vs 遷移）** — enum 型フィールドを一律 axis にしない。判定は「その効果は今の状態からいつでも再生成できるか？」＝ **YES は投影**（型検査の仕事・軸にしない）／**NO は遷移**（軸候補）（§3）。
- [ ] **粒度・実装同一性** — vocab を共有するのは実装が同一のときだけ。概念が同じだけなら vocab は分けて tag（概念の族）で束ねる（§1・§2）。
- [ ] **きっかけ・前提・結果を書き分けたか** — condition に結果を混ぜない（結果は `then`／effect の責務・§4）。

機械判定できる規律（派生値の列挙・時点依存語・「〜を参照」メタ指示・腐る file:line・消えた文書参照・複製原子・desc 長・矛盾 given・--total の kind 不一致・idPolicy 命名）は保存時に機械が検査する。**書き込み後の手順は機械ループ**:

1. 書き込みコマンドの出力（`--json` は応答封筒 `{ record, advisories }`）を読む。
2. **advisory がゼロになるまで是正する**（是正が正。容認は理由必須で稀な例外）。
3. `decide` は**必ず `--dry-run` を先に打つ**（decision は append-only・保存後の why は直せない）。
4. 保存拒否（reject: `exclusive-violation`／`total-kind-mismatch`／`id-policy`）を `--allow <rule> --reason <理由>` で破るのは、自己矛盾でないと確信できるときだけ。**--allow の使用例を手本（few-shot）として他レコードに写さない**。

以下はその上での網羅の勘所:

1. **アクションの網羅** — 対象の外部 IF（入口）から機械的に洗い出す。ここで漏らすと以降すべて漏れる。
2. **条件(given)の網羅** — 決定表で。排他的な原因群は畳まず別遷移に。
3. **効果(then)を妄想で書かない** — 実際に起きることだけ。全 emit/効果が then に現れるか逆引きする。
4. **完了ゲート（必須）** — 主題タグ単位で: マトリクス空白ゼロ／`decision.commits[]` が指す commit 経由でテスト・実装を辿る／
   穴探し 1 周（`scholia spec <subjectTag>` の兄弟整合）。**`scholia lint` が緑でも網羅の証明にはならない** — 完了ゲートは手動の突合として別に行う。
   （`tests` フィールドは廃止済み。実テストとの結び付けは commit 履歴が担う・DESIGN §8・§3.2）
5. **decision の質** — 後から矛盾に気づける why を書く（`file:line` のような不変でない参照は避ける）。
   コンポーネント横断の規則（cross-cutting invariant）はタグに刻む。

## 規約を変えたら手本も掃除する（retrofit）

ガイダンス・規約（本スキル・modeling-principles・DESIGN の書き方規律）を改定する PR には、
`scholia retrofit` の差分レポート（改定で新たに violation になる既存レコードの一覧）を添付する。
AI は既存 store を few-shot の手本として読むため、古い違反レコードを掃除しない限り違反様式が再生産される。
是正は提案フロー（pending 変更＋review コメント→adopt）に乗せる（`scholia retrofit` は read-only・`--fix` を持たない・exit 0）。
decision の判断欄位（why/changed/ref）由来の finding は append-only により是正不能なので acknowledge-only として別掲される。

## CLI コマンド表（DESIGN §6 要約。真値は `scholia <cmd> --help`）

```
# セットアップ
scholia init [--dir <path>]
scholia kind set <condition|action|effect> <k1,k2,...>
scholia config get|set <key> [<value>]

# 語彙・タグ
scholia vocab add <condition|action|effect> <id> --label <l> [--kind <k>] [--owner <l>（effect のみ）]
scholia vocab rm <id> [--category <c>]
scholia vocab tag <id> --add <tagId>… [--rm <tagId>…]
scholia vocab rename <id> --to <newId>
scholia tag create <id> --name <n> [--kind <k>] [--parent <tagId>…] [--desc <t>] [--ref <url>]
scholia tag list [--kind <k>] [--tree] [--json]
scholia tag edit <id> [--name][--kind][--parent…][--desc]
scholia tag rm <id> [--force]

# 遷移（原子）— tests フィールドは廃止済み（--test/--clear-tests は無い）
scholia tx add <id> --action <a> [--given <c,…>] --then <e,…> [--tags <t,…>]
scholia tx edit <id> [--action][--given][--then][--tags]
scholia tx tag <id> --add <tagId>… [--rm <tagId>…] | --set <ids>
scholia tx rename <id> --to <newId>
scholia tx rm <id> --why <理由> --force

# 意思決定
scholia decide --on <transition|tag>:<id> --why <t> --dry-run          # 保存せず advisory をプレビュー（decide の前に必ず打つ）
scholia decide --on <transition|tag>:<id> --why <t> [--changed <s>] [--ref <s>] [--commit <hash>…]
scholia decision add-commit <decisionId> <hash> [<hash>...] [--json]   # 既存 decision の commits[] に追記専用

# 提案コメント（レビュー）— AI コメント配送のサイドカー（DESIGN §8.4・変更評価は scholia-change へ）
scholia review add --on <transition|vocab|tag>:<id> --body <why> [--source ai] [--json]
scholia review list [--on <transition|vocab|tag>:<id>] [--json]

# 読み取り / 派生ビュー
scholia show tx <id> [--resolve] [--json]
scholia spec <subjectTag> [--json]
scholia list [--facet <tagKind>] [--tag <id>] [--kind <k>] [--json]
scholia rules [--tag <id> | --tx <id> | --facet <k>] [--sort chrono|target] [--json]
scholia search <keyword> [--type tag|transition|vocab|decision] [--json]   # keyword で横断逆引き（id 未確定な入口）
scholia lint [--json]
scholia retrofit [--rule <id>] [--json]                       # advisory 規則で store を read-only 走査し是正候補を棚卸し（--fix なし・exit 0）
scholia diff [<ref1> [<ref2>]] [--json]                       # 現在 vs ref1、または ref1 vs ref2（landed 監査）

# ビューア
scholia view [--port <p>] [--host <h>]
scholia export --html <dir>
```

`scholia view` は既定 `127.0.0.1`（ローカル専用）。LAN 公開（スマホ等で見る）は `--host 0.0.0.0` 等の明示指定が
要る opt-in（`scholia view --help` が真値）。レビュー提示（deep-link route。DESIGN §7・hash ルーティング）:
タグ spec=`#/spec/<tagId>`、transition=`#/browse/tx/<txId>`（tag と組み合わせ可: `#/browse/tag/<tagId>/tx/<txId>`）、
vocab=`#/vocab/<id>`（実ルートの正本は `web/src/router.ts`）。
