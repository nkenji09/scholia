# product-memory — 設計（v1）

AI 向けの「コンテキスト保存支援」ツール。コンポーネントやフローの**詳細な振る舞い**を、
**語彙の組み合わせ**（自由記述でない構造）として蓄積し、**意思決定の履歴**とともに残す。
CLI（単一バイナリ）＋ ビューア ＋ AI エージェント用スキルの 3 点セットで提供する。

> 本文の散文は日本語、スキーマ／CLI 識別子は英語。OSS 公開時に散文を英訳できるよう識別子は最初から英語で固定する。

---

## 0. このツールは何のためか（先に読む）

主目的は**バグの早期検知ではない**。次の 2 つ:

1. **意思決定の記録を蓄積する** — 「この振る舞いは既に検討済みで、これが正しい」という判断を、
   なぜそうしたか（why）とともに残す。AI が次の作業で読み込む「守るべき規則」になる。
2. **変更を評価する土台にする** — 後から来る修正依頼・仕様変更を「本当に取り込むべきか」を、
   過去の意図と突き合わせて判断する基準にする。

だから `lint` や機械検査も「早期バグ検知」ではなく、**記録が自己矛盾していないことの整合性チェック**として扱う。

### ゴール
- **スタンドアロン**。任意の repo・任意の言語・任意の AI エージェントで動く（loom 非依存）。
- **単一バイナリ**（Go）。ランタイム前提ゼロ。CI・コンテナでもそのまま動く。
- 記録は repo 内の**素の JSON**。コードと同じ版で版管理される。
- **利用者に難しい分割・整理の判断を強いない**（§1）。
- 人間が快適に閲覧できる**ビューア**を同梱。

### 非ゴール（v1）
- 汎用グラフ DB 化（＝文法を可変にすること。§2）。
- 実行時シーケンス（フローチャート）を一級市民にすること（§2「辺を持たない」）。
- DB を真実の源にすること（§1「なぜ git-as-DB か」）。SQLite は派生インデックスとしてのみ持つ（§3.7）。
- 正本↔テストの照合の**自動化**（手動の突合は完了ゲートとして残す・§8）。

---

## 1. 中核原理 — 原子を保存し、構造は全部 derive する

このツールの背骨となる一つの決断:

> **原子（transition）だけをファイルに保存する。unit・"仕様"・階層・フロー図・グルーピングは、
> すべてタグ／メタデータからの派生ビュー（query）にする。手で確定しない。**

理由は「**利用者に難しい判断を強いない**」。従来型（loom を含む）は「この振る舞いはどのファイル／どのクラスタ／どのノードに属すか」という
**分割・整理の判断**を人に負わせ、それが難しすぎた。原子だけ保存して構造を派生にすると、その判断が丸ごと消える:

- 「この spec をどう分ける?」→ **分けない**（spec ファイルが無い）。
- 「このノードのタイトルは?」→ **無い**（ノードが無い。見出しはタグ名から derive）。
- 「どの unit に入れる?」→ unit も**タグ**。付けるだけ（複数可）。

### なぜ git-as-DB か（DB の利点を、git のまま得る）

「全部 1 つの DB に入れて分割しない」は魅力的だが、SQLite 等は git にはバイナリで、**conflict を人が直せない**。
そこで **git 自体を DB にする**:

> **1 レコード = 1 テキストファイル。** 行を足す＝ファイルを足す（**衝突しない**）。行を直す＝小さなテキスト衝突（**直せる**）。

これで「分割不要（DB の利点）」と「テキストでマージ可能（git の利点）」を両取りする。
先行例: `git-bug`（バグを git に構造化レコードとして分散保存）。真実の源は常にファイル、DB（SQLite）は**捨てても再生成できる派生インデックス**に限る（§3.7）。

### 手書き禁止・CLI 経由（鉄則）
レコードは直接エディタで書き換えない。CLI が read→build→write を一貫して行い、正規化・不変条件チェック・
`decisions` の append-only 保証を担う。手で書くとこの保証が崩れ、記録の信頼性が失われる。

---

## 2. 三つの軸 — どこを固定し、どこを可変にするか

分類には 3 軸があり、可変性の線引きが本ツールの意味論を決める。

| 軸 | 中身 | 可変性 | 役割 |
| --- | --- | --- | --- |
| **カテゴリ** | `condition` / `action` / `effect` | **固定（設定不可）** | 遷移の**文法**。これがあるから「組み合わせであって自由記述でない」が成立する |
| **kind** | 例: action の `user`/`api`/`lifecycle` | **プロジェクトが宣言（可変）** | カテゴリ内の**分類**。単一値。§3.6 |
| **tag** | 例: `req.*`／`concern.*`／`subject.*`／任意 | **自由・ネスト可能・多値** | **横断分類**。派生階層・検索・要件トレーサビリティの軸。§3.4 |

- **カテゴリを可変にしない**: 文法を可変にすると単なるグラフ DB に退化し、「構造化された振る舞い」という価値が消える。
  1 つの遷移 = `(action, given[], then[])` ＝「きっかけ WHEN 条件 THEN 結果」。これが唯一固定の形。
- **kind を可変にする**: `user/api/prop/lifecycle` は Vue 由来。任意環境（CLI／バックエンド／ワークフロー…）で意味を持つ分類は
  プロジェクトごとに違う。だから宣言制（`kind 追加＝スキーマ変更`なので diff に出る＝複雑性シグナル）。
- **遷移は 1 つで完結する。辺（transition→transition）は持たない。** シーケンスが要るなら、それは
  **共有状態の語彙**に暗黙に入る（遷移 A の `then` が状態 S を立て、遷移 B の `given` が状態 S を読む）。
  フロー図が要るときはそこから**derive**する（後フェーズ）。明示的な辺を一級市民にはしない。

### 2.1 vocab と tag の違い（constitutive vs descriptive）

形は近い（id・ラベル・kind を持つ）が、**役割は直交する**:

- **vocab は振る舞いを"構成する部品"（constitutive）** — 遷移の `action`/`given`/`then` の**スロットを埋める**。消すと振る舞いが壊れる。
- **tag は振る舞いに"貼るラベル"（descriptive）** — 分類・検索・入れ子・要件トレーサビリティ・cross-cutting 不変条件のための横断メタデータ。消しても振る舞いは同じ（見つけにくくなるだけ）。

帰結:
- **入れ子（`parentIds`）は tag だけが持つ。** vocab は `parentIds` を持たない。vocab のグルーピングは
  `category`（固定）＋ `kind`（1 段）＋ **tag 参照**（深い入れ子は tag に委ねる）で表す。**階層システムは 1 つ（tag）に統一**する。
- 標語: **tags classify; vocab composes.**（tag は分類する／vocab は組み立てる。tag は vocab すらも分類する万能ファブリック）

---

## 3. データモデル（レコード＝ファイル）

### 3.1 ファイル配置（対象 repo 内）

**真実の源はテキストファイル（git ネイティブ）。** 1 レコード 1 ファイル。

```
.pmem/
  config.json                      # プロジェクト設定（kind 宣言・facet 軸など）※唯一の singleton
  vocab/        <id>.json          # 語彙（condition/action/effect）1 件 1 ファイル
  tags/         <id>.json          # タグ（ネスト可能）1 件 1 ファイル
  transitions/  <id>.json          # ★原子。遷移 1 件 1 ファイル
  decisions/    <ulid>.json        # 意思決定 1 件 1 ファイル（append-only・transition か tag を指す）
  index.db                         # 派生インデックス（gitignore・任意・SQLite）→ §3.7
.gitignore                         # .pmem/index.db を無視
```

- **探索は `.pmem/` を再帰 walk**（`config.roots` で追加ルート可・既定は `.pmem/` のみ）。id がファイル名＝プロジェクト内で一意。
- 書き込みは atomic（tmp→rename）。派生物（`index.db`）は git 管理しない。
- **未設定フィールドは省略（omitempty）に正規化**する（`null` を書かない。diff の安定のため）。
- `pmem init --no-gitignore` で `.gitignore` への追記をオプトアウトできる（既定はこれまで通り追記する）。

### 3.2 遷移（transition）— 原子

```jsonc
// .pmem/transitions/T-login-submit-valid.json
{
  "id":     "T-login-submit-valid",
  "action": "act.user.submit-login",                 // actionId 参照（1 つ）
  "given":  ["cond.credentials-valid"],              // conditionId の集合（AND・順不同）
  "then":   ["eff.session.issue-token",              // effectId の順序リスト（発火順）
             "eff.nav.redirect-home"],
  "tags":   ["req.auth-happy-path", "subject.auth", "concern.security"]
}
```

- `Transition { id, action, given[], then[], tags?[] }` — **これで全部**。
  - `given` は**集合**（diff は順不同として正規化）。`then` は**順序リスト**（並び替えも変更として検出）。
  - **来歴（誰が・いつ作ったか）は git 履歴が持つ**ので専用フィールドは持たない。意図は `decisions` に（§3.5）。
  - **実テストとの結び付けは `tests` フィールドではなく `decision.commits[]` が指す commit 経由で辿る**（§8・M-4）。commit 履歴が結べば専用フィールドは不要というユーザー判断。
- unit も "spec" もここには無い。それらは**タグと query から derive**（§3.8）。

### 3.3 語彙（vocab）

```jsonc
// .pmem/vocab/act.user.submit-login.json
{ "id": "act.user.submit-login", "category": "action", "label": "ログイン送信", "kind": "user", "tags": [] }
// .pmem/vocab/eff.session.issue-token.json
{ "id": "eff.session.issue-token", "category": "effect", "label": "セッショントークン発行",
  "kind": "state", "owner": "server", "tags": [] }
// .pmem/vocab/cond.credentials-valid.json
{ "id": "cond.credentials-valid", "category": "condition", "label": "資格情報が正当", "tags": [] }
```

- `VocabEntry { id, category, label, kind?, owner?(effectのみ), tags?[] }`
- **kind は 3 カテゴリすべてで任意**。設定値は config の宣言集合に含まれる必要がある（§5 `kind-valid`）。
- `owner` は effect の任意の自由文（「その効果を起こす主体」＝レイヤ／サービス名。特定値を強制しない）。
- **★`tags`（tagId 参照）を持つ**。遷移は参照する語彙のタグを継承する（§3.7 実効タグ）。

### 3.4 タグ（tag）— ネスト可能・横断分類の要

```jsonc
// .pmem/tags/req.auth-happy-path.json
{ "id": "req.auth-happy-path", "name": "正常系ログイン", "kind": "requirement",
  "parentIds": ["req.auth"], "description": "正しい資格情報でセッションが張れること",
  "ref": "https://…", "color": "#3b82f6" }
// .pmem/tags/subject.auth.json
{ "id": "subject.auth", "name": "認証", "kind": "subject", "parentIds": [] }
```

- `Tag { id, name, kind?, parentIds?[], description?, color?, ref? }`
- **ネスト = `parentIds`（多親 DAG）**。タグ自身に複数の親タグを付けてグルーピングできる。循環禁止（§5 `tag-ref`）。
- タグの `kind` も**宣言制**（`config.tagKinds`）。「要件」「関心」「主題(unit)」はいずれも tagKind の一種にすぎない。
- **`subject.*`（＝旧 unit）もタグ**。「どの主題に属すか」は付けるだけ・複数可。専用の unit フィールドは持たない。
- **スコープ**（プロジェクト共通／コンポーネント共通）も専用概念を持たず**タグのネストで表現**する。

### 3.5 意思決定（decision）— transition と tag のどちらにも付く

```jsonc
// .pmem/decisions/01J8XR....json  （遷移への意思決定）
{ "id": "01J8XR...", "target": { "type": "transition", "id": "T-login-submit-valid" },
  "why": "トークンは httpOnly cookie で発行（XSS 対策）", "changed": "add eff.session.issue-token",
  "ref": "PR#42", "commits": ["a1b2c3d", "e4f5g6h"], "at": "2026-07-10T00:00:00Z" }

// .pmem/decisions/01K2AF....json  （タグへの意思決定＝cross-cutting 不変条件）
{ "id": "01K2AF...", "target": { "type": "tag", "id": "req.auth-symmetry" },
  "why": "null と空文字は同じ『未入力』として同一に扱う。この要件を持つ全遷移が守る規則。",
  "ref": "PR#57", "at": "2026-07-11T00:00:00Z" }
```

- `Decision { id, target: {type: "transition"|"tag", id}, why, changed?, ref?, commits?[], at }` — **append-only**（消さない・直さない。訂正は新しい 1 件を足す）。
- **1 件 1 ファイル**にすることで、append が**衝突しない**（配列 JSON だと末尾追記が git で衝突する）。
- **`commits[]`（git hash の集合・任意）は実装来歴**。`ref`（外部 URL/PR）とは別軸で、**repo 内の正確な着地点**を持つ
  （推奨 1 decision : 1 commit だが、実装ミス直しなどで 1 decision に複数 commit を許容）。
  - `pmem decide --commit <hash>`（繰り返し可）で decide 時に結べる。
  - 着地後に結ぶ／追加する場合は **`pmem decision add-commit <decisionId> <hash>...`**（追加専用）を使う。
  - **append-only の精緻化**: decision の**判断**（`target` / `why` / `changed` / `ref`）は immutable。
    `commits[]` だけが `add-commit` により**追記専用・単調増加**で変わりうる（判断は凍結されたまま）。
    「decision ファイル完全不変」から一段緩めた運用だが、CLI は commit の追加しかできず、
    過去の判断は消えない・書き換わらないため、append-only の狙い（監査可能性）は保たれる。
    別の判断が入った場合は decision を新しく足す（`pmem decide`）— decision の無駄な増殖を避けるため、
    実装ミス直しのような「判断は同じだが着地コミットが増える」ケースにだけ `add-commit` を使う。
- **★タグに付けられる**のが要。spec という"容れ物"を無くした代わりに、
  **複数遷移をまたぐ不変条件はタグに刻む**。A と B が同じタグを持てば、A の変更時に**そのタグの decision が surface** する
  ＝「片方の変更がもう片方との矛盾に気づける」という変更評価の核を、**明示的な共有タグ**で実現する
  （従来の「たまたま同じファイル」より強い）。

### 3.6 config（singleton）

```jsonc
// .pmem/config.json
{
  "pmemVersion": 1,
  "kinds": {
    "condition": [],
    "action":    ["user", "api", "lifecycle", "system", "cron", "webhook"],
    "effect":    ["emit", "state", "http", "storage", "log"]
  },
  "tagKinds":          ["requirement", "concern", "subject"],  // タグ kind の宣言
  "facetKinds":        ["subject", "requirement", "concern"],  // ビューアの既定ナビ軸（派生階層の軸）
  "traceabilityKinds": ["requirement"],                        // 0 充足を gap 警告する tag kind
  "idPrefix": { "condition": "cond.", "action": "act.", "effect": "eff." }, // 命名規約（ソフト・grep 用）
  "roots":  [],                                                 // 追加探索ルート（co-location 任意・既定は .pmem のみ）
  "viewer": { "port": 4577 }
}
```

- **「生成結果の出力先などの設定」= config**。ビューアが**唯一 CRUD できる**のもここ（§7）。
- `idPrefix` は**慣例のみ**（強制は kind フィールドが担う）。タグは prefix でなく `kind` で分類する。

### 3.7 実効タグ（検索・派生階層の要）

遷移を検索・グルーピングするときの**実効タグ**を read-time join で計算する:

```
effective(transition) = 展開祖先( transition.tags ∪ ⋃ tags(参照している vocab) )
```

- **遷移自身のタグ** ∪ **参照する語彙語のタグ**（`vocab tag` で付与）を合わせ、**`parentIds` を辿って祖先まで展開**。
- これで「要件 `req.auth` で検索 → 子 `req.auth-happy-path` を持つ遷移も全部ヒット」が成立する。

### 3.8 派生ビュー（保存しない・全部 query）

以下は**どれもファイルに持たない**。インデックス（§3.9）に対する query として計算する:

| 派生 | 定義 |
| --- | --- |
| **unit / "spec"** | `subject.*` タグ（または任意のタグ）で遷移を束ねた**レポート**。`pmem spec <subjectTag>` で描画 |
| **階層（faceted）** | facet 軸（tagKind）を選ぶと、その軸のタグ入れ子がそのまま木になり、遷移が葉に並ぶ。多軸・多重所属可 |
| **rules（守る規則）** | 対象（tag／transition／facet）に関わる `decisions` を横断集約（cross-cutting 不変条件を含む） |
| **フロー図** | （後フェーズ）共有状態の語彙から derive。手書きしない |
| **検索** | 実効タグ・kind・語彙 label 横断 |

### 3.9 派生インデックス（クエリ層）

- **既定は in-memory** — CLI 起動時／ビューア読込時に `.pmem/` を walk して索引を建てる。小〜中規模はこれで即応。
- **規模が要求したら SQLite に昇格** — `.pmem/index.db`（gitignore）に永続化し、`pmem index --rebuild` で再構築。
- インデックスは**捨てても再生成できる**。壊れても真実は失われない。SQLite は「読み取り」の正しい位置に置く。

---

## 4. 変更の評価（git ネイティブ）

spec ファイルも proposal ファイルも無い。**提案 = git ブランチ**、**評価結果 = decision** に落ちる。

```
# 提案する側: ブランチで transition / decision を編集して PR
pmem diff [<ref1> [<ref2>]]  # 現在 vs <ref1>（既定 HEAD）、または <ref1> vs <ref2>（両方 git ref）の semantic diff（語彙± / 遷移± / then 順序 / decisions±）
pmem rules --tag <id>        # その提案が触るタグの過去 decisions（守る規則）と照合

# 評価する側の結着（どちらも decision を残す）
#   adopt  = ブランチを merge ＋「採用」decision を append
#   reject = merge しない ＋「取り込まない・理由」decision を append（次回同じ提案が来ても即・既決）
pmem decide --on transition:<id> --why "評価: 取り込まない。<理由>" --ref <PR/URL>
```

- 判定材料: **(a) 複雑性 diff**（語彙±・遷移±・then 順序）／ **(b) 既存 decision と矛盾するか**（衝突＝却下寄り・id 引用）／
  **(c) 既に検討済みか**。**decision は append-only**（過去を消す提案＝取り込み拒否の最有力根拠）。
- 「却下」を一言で済ませない——**なぜ取り込まないかを decision に根拠つきで残す**＝守る規則を厚くする。
- ビューアの**比較ビュー**は「ブランチ vs main の `pmem diff`」を描画する（§7）。

---

## 5. 不変条件 / lint

`lint` は「記録が自己矛盾していない」ことを守る（早期バグ検知ではない）。error があれば exit 1。

| rule | 重大度 | 何を守るか |
| --- | --- | --- |
| `vocab-ref` | error | 遷移の `action/given/then` が実在する語彙を解決する（宙ぶらりん参照なし） |
| `kind-valid` | error | 語彙の kind／タグの kind が config の宣言集合に含まれる（可変な不変条件） |
| `tag-ref` | error | 遷移・語彙の tagId、タグの parentIds が解決する＋**タグに循環がない** |
| `decision-target` | error | `decision.target` が実在する transition／tag を指す |
| `empty-then` | error | `then` 空の遷移は作れない。意図的 no-op は**ガード効果**（kind=state 等）で表す |
| `id-unique` | error | 各レコード種別で id 一意（ファイル名と一致） |
| `requirement-gap` | warn | `traceabilityKinds` のタグで、充足する遷移が 0 件（未充足要件の表面化） |
| `ref-freshness` | warn | `decision.ref` が `file:line`（腐る参照）でなく URL/commit |
| `decision-coverage` | info | 挙動を持つ遷移に `decisions` がある（why が記録されている） |
| `unused-vocab` | info | どの遷移からも参照されない語彙（`vocab rm` の発見手段） |

**lint 緑＝網羅完了ではない**。別途、完了ゲート（§8）を通す。

---

## 6. CLI コマンド表

バイナリ名は暫定 `pmem`。真値は `pmem <cmd> --help`。全書き込み系に `--json`（エージェント駆動用）。

```
# セットアップ / 設定
pmem init [--dir <path>]                                   # .pmem/ を作成
pmem kind set <condition|action|effect> <k1,k2,...>        # kind 宣言（list/get も）
pmem config get|set <key> [<value>]                        # facetKinds / tagKinds / roots など

# 語彙
pmem vocab add <condition|action|effect> <id> --label <l> [--kind <k>] [--owner <l>]
pmem vocab rm <id> [--category <c>]                        # 未参照限定
pmem vocab tag <id> --add <tagId>… [--rm <tagId>…]        # 語彙にタグ（遷移が継承）
pmem vocab rename <id> --to <newId>                        # 参照も一括更新

# タグ（ネスト対応）
pmem tag create <id> --name <n> [--kind <k>] [--parent <tagId>…] [--desc <t>] [--color <c>] [--ref <url>]
pmem tag list [--kind <k>] [--tree] [--json]              # --tree でネスト表示
pmem tag edit <id> [--name][--kind][--parent…][--desc][--color][--ref]
pmem tag rm <id> [--force]                                # 未参照のみ・--force で detach cascade

# 遷移（原子）
pmem tx add <id> --action <a> [--given <c,…>] --then <e,…> [--tags <t,…>]
pmem tx edit <id> [--action][--given][--then][--tags]
pmem tx tag <id> --add <tagId>… [--rm <tagId>…] | --set <ids>
pmem tx rename <id> --to <newId>                          # decisions の target も一括更新
pmem tx rm <id> --why <理由> --force                      # 破壊的（decisions も道連れ）

# 意思決定（transition か tag に付く）
pmem decide --on <transition|tag>:<id> --why <t> [--changed <s>] [--ref <s>] [--commit <hash>…]
pmem decision add-commit <decisionId> <hash> [<hash>...] [--json]  # 既存 decision の commits[] に追記専用（§3.5）

# 読み取り / 派生ビュー
pmem show tx <id> [--resolve] [--json]                    # 遷移 1 件（語彙 label 解決）
pmem spec <subjectTag> [--json]                           # 主題タグで束ねた"仕様"レポート（派生）
pmem list [--facet <tagKind>] [--tag <id>] [--kind <k>] [--json]   # faceted 一覧・グルーピング
pmem rules [--tag <id> | --tx <id> | --facet <k>] [--sort chrono|target] [--json]
pmem lint [--json]
pmem diff [<ref1> [<ref2>]] [--json]                      # 現在 vs ref1、または ref1 vs ref2 の semantic diff（変更評価。ref1 vs ref2 で `<commit>^ <commit>` = 1コミット分の変更）

# インデックス / ビューア
pmem index [--rebuild]                                     # 派生インデックス（SQLite）再構築（任意）
pmem view [--port <p>]                                     # ローカルビューア（埋め込み SPA）
pmem export --html <dir>                                   # 静的 HTML 書き出し
```

---

## 7. ビューア

`pmem view` が**ローカル HTTP サーバ**を起動し、`//go:embed` で焼き込んだ SPA を配信する（同じ 1 バイナリ）。
フロントは**バイナリの API を叩く薄いクライアント**＝ lint/diff/search のロジックは Go に 1 つだけ持ち、検索は派生インデックス（§3.9）を叩く。

- **プロジェクト設定 CRUD**（config）— ビューアで書けるのはここだけ。
- **faceted ナビ** — facet 軸（主題／要件／関心…）を選んで遷移を辿る → **詳細表示**（遷移・実効タグ・rules）。
- **要件トレーサビリティ** — `requirement` タグ → 充足遷移の逆引き＋ 0 充足 gap。
- **比較ビュー** — ブランチ vs main の `pmem diff`（変更評価）。
- **`pmem export --html`** — 自己完結の静的 HTML（共有・CI 成果物・GitHub Pages 用・サーバ不要）。

---

## 8. 良い記録の書き方（what — 中身で価値が決まる）

CLI の how とは別に、**何を登録するか**で価値が決まる（詳細は別途 authoring ドキュメント化）:

1. **アクションの網羅** — 対象の外部 IF（入口）から機械的に洗い出す。ここで漏らすと以降すべて漏れる。
2. **条件(given)の網羅** — 決定表で。排他的な原因群は畳まず別遷移に。
3. **効果(then)を妄想で書かない** — 実際に起きることだけ。全 emit/効果が then に現れるか逆引き。
4. **完了ゲート（必須）** — 主題タグ単位で: マトリクス空白ゼロ／`decision.commits[]` が指す commit 経由でテスト/実装を辿る／穴探し 1 周。**lint 緑は網羅の証明ではない**。網羅チェックは `pmem spec <subjectTag>` の兄弟整合（同じ主題タグ配下の遷移群を横に並べて漏れを見る）で継続する。
5. **decision の質** — 後から矛盾に気づける why を書く（不変参照 `ref`・`file:line` は避ける）。cross-cutting な規則はタグに刻む。

---

## 9. 言語・リポジトリ構成（Go）

```
product-memory/
  cmd/pmem/main.go            # CLI エントリ（cobra）
  internal/
    model/                    # レコード型（Transition/VocabEntry/Tag/Decision/Config）
    store/                    # 1 レコード 1 ファイルの探索・atomic read/write・id↔ファイル名
    index/                    # 派生インデックス（in-memory→SQLite）・faceted query・rules・search
    lint/                     # lint ルール
    diff/                     # semantic diff（git ref / 作業ツリー間）
    render/                   # 派生"仕様"ビュー・export
    cli/                      # cobra コマンド
  internal/viewer/            # HTTP サーバ ＋ //go:embed
  web/                        # ビューア SPA のソース（ビルド→ go:embed）
  npm/                        # npm ランチャ（postinstall で binary 取得）
  packaging/                  # goreleaser / homebrew / scoop / install.sh
  agents/skills/pmem/         # .agents/skills（＋ claude plugin）
  docs/  .goreleaser.yaml  go.mod
```

- **module path**: `github.com/nkenji09/product-memory`（`go install …/cmd/pmem@latest` を成立させるため実 URL と一致）。
- CLI は **cobra**。JSON は標準 `encoding/json`。ロジックは `internal/*` に集約し CLI とビューアが同じコアを呼ぶ（単一の真値）。

---

## 10. 配布（みんなが使える形）

**プレビルド・バイナリを軸に多チャネル**。特に ④ が AI エージェント界隈への到達に効く。

1. **GoReleaser** → GitHub Releases に darwin/linux/windows × amd64/arm64
2. **Homebrew tap**（`brew install`）／ **Scoop・winget**（Windows）
3. **`curl … | sh`** ワンライナ（install.sh）
4. **⭐ npm ランチャ** — postinstall で対応バイナリを取得。**`npx product-memory …` がゼロインストールで動く**（esbuild=Go / Biome=Rust の実績）
5. **`go install`**
6. **Claude plugin ＋ `.agents/skills`**（＋ 必要なら `.cursor/rules` 等）— 無ければ検出してインストールを促し、あとはバイナリに委譲

---

## 11. 仮の既定値（要確認）

- **バイナリ名**: `pmem`（npm/repo 名は `product-memory`）
- **ストレージ**: `.pmem/`
- **module path**: `github.com/nkenji09/product-memory`
- **ビューア既定ポート**: 4577

---

## 12. loom との差分（先行実装との関係）

loom の `loom spec` は概念的な先行実装（**発展途上・「正解」として扱わない**）。依存はしないが参照はする。

- **引き継ぐ**: append-only decisions / vocab 先行 / semantic diff / 変更評価フロー / 要件トレーサビリティ / kind の複雑性シグナル。
- **汎用化**: 語彙 kind を設定可能に。owner は自由文（`w`/`p` 慣例を捨てる）。
- **足す**: タグのネスト／**decision を tag にも付ける（cross-cutting 不変条件の家）**／`rename`／`requirement-gap`。
- **捨てる（今回の軌道修正）**: **spec ファイル／クラスタ分割の判断**・**node ツリー／ノードタイトル**・**手書き overview 図**・
  Vue 固有 lint（clear-symmetry）。→ すべて「原子＋派生」で不要になった。

---

## 13. ロードマップ

- **Phase 0（骨格）**: `model` + `store`（1 レコード 1 ファイル）+ `pmem init` + `vocab add` + `tag create` + `tx add` + `show` + `lint`。
  素の JSON が生まれて `lint` が緑になるまで。
- **Phase 1（記録の核）**: `decide`（transition/tag）+ `rules` + 実効タグ + `requirement-gap` + `rename`。
- **Phase 2（派生・評価）**: in-memory インデックス + `list --facet` + `spec <subjectTag>` + `diff`（git ref 間）。
- **Phase 3（閲覧）**: `pmem view`（埋め込みビューア・faceted・比較）+ `export --html`（+ 規模次第で SQLite）。
- **Phase 4（配布）**: GoReleaser + npm ランチャ + Homebrew + skill/plugin パッケージ。
- **後フェーズ**: フロー図の derive（共有状態）・正本↔テスト照合の自動化・pluggable lint。
```
