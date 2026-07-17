# scholia — 設計（v1）

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
lint は二層で、**error＝記録の自己矛盾**（保存拒否・CI fail の根拠——これが従来の定義そのまま）／
**advisory＝authoring 規律の改善提案**（派生値の二重書き・時点依存語・消えた文書参照など。保存は拒まず・info/warn・書いた同じターンに警告する）。
「早期バグ検知ではない」という位置づけは両層で不変。

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
.scholia/
  config.json                      # プロジェクト設定（kind 宣言・facet 軸など）※唯一の singleton
  vocab/        <id>.json          # 語彙（condition/action/effect）1 件 1 ファイル
  tags/         <id>.json          # タグ（ネスト可能）1 件 1 ファイル
  transitions/  <id>.json          # ★原子。遷移 1 件 1 ファイル
  decisions/    <ulid>.json        # 意思決定 1 件 1 ファイル（append-only・transition か tag を指す）
  reviews/      <ulid>.json        # AI 提案コメント（read-only オーバーレイ・gitignore・揮発層）→ §7・§8
  index.db                         # 派生インデックス（gitignore・任意・SQLite）→ §3.9
.gitignore                         # .scholia/index.db と .scholia/reviews/ を無視
```

- **レコードの探索は `.scholia/` 下の 4 ディレクトリ（`vocab/` `tags/` `transitions/` `decisions/`）を開く**。id がファイル名＝プロジェクト内で一意。`reviews/`（§8 の AI 提案コメント）と `index.db` は**レコードではない**ので読み込まれず、`scholia lint` にも影響しない。
- 書き込みは atomic（tmp→rename）。派生物（`index.db`）と揮発物（`reviews/`）は git 管理しない。
- **未設定フィールドは省略（omitempty）に正規化**する（`null` を書かない。diff の安定のため）。
- `scholia init --no-gitignore` で `.gitignore` への追記をオプトアウトできる（既定はこれまで通り追記する）。

### 3.2 遷移（transition）— 原子

```jsonc
// .scholia/transitions/T-login-submit-valid.json
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
// .scholia/vocab/act.user.submit-login.json
{ "id": "act.user.submit-login", "category": "action", "label": "ログイン送信", "kind": "user", "tags": [] }
// .scholia/vocab/eff.session.issue-token.json
{ "id": "eff.session.issue-token", "category": "effect", "label": "セッショントークン発行",
  "kind": "state", "owner": "server", "tags": [] }
// .scholia/vocab/cond.credentials-valid.json
{ "id": "cond.credentials-valid", "category": "condition", "label": "資格情報が正当", "tags": [] }
```

- `VocabEntry { id, category, label, kind?, owner?(effectのみ), tags?[] }`
- **kind は 3 カテゴリすべてで任意**。設定値は config の宣言集合に含まれる必要がある（§5 `kind-valid`）。
- `owner` は effect の任意の自由文（「その効果を起こす主体」＝レイヤ／サービス名。特定値を強制しない）。
- **★`tags`（tagId 参照）を持つ**。遷移は参照する語彙のタグを継承する（§3.7 実効タグ）。

### 3.4 タグ（tag）— ネスト可能・横断分類の要

```jsonc
// .scholia/tags/req.auth-happy-path.json
{ "id": "req.auth-happy-path", "name": "正常系ログイン", "kind": "requirement",
  "parentIds": ["req.auth"], "description": "正しい資格情報でセッションが張れること",
  "ref": "https://…", "color": "#3b82f6" }
// .scholia/tags/subject.auth.json
{ "id": "subject.auth", "name": "認証", "kind": "subject", "parentIds": [] }
```

- `Tag { id, name, kind?, parentIds?[], description?, color?, ref?, total? }`
- **ネスト = `parentIds`（多親 DAG）**。タグ自身に複数の親タグを付けてグルーピングできる。循環禁止（§5 `tag-ref`）。
- タグの `kind` も**宣言制**（`config.tagKinds`）。「要件」「関心」「主題(unit)」はいずれも tagKind の一種にすぎない。
- **`subject.*`（＝旧 unit）もタグ**。「どの主題に属すか」は付けるだけ・複数可。専用の unit フィールドは持たない。
- **スコープ**（プロジェクト共通／コンポーネント共通）も専用概念を持たず**タグのネストで表現**する。
- **`axis.*`（kind="axis"）は「きっかけの gap 検出」の軸宣言タグ**（#39 action-flow）。1 軸＝1 枚のタグレコード。値＝その軸タグを貼られた condition 群（`VocabEntry.Tags` 経由・多重所属可）。`total`（bool・additive・omitempty）は「軸の値のうち必ず1つが真であるべきか」を軸タグ自身が持つ——true の軸で値の condition がどの transition の given にも現れなければ、`scholia flow` が抜け(L-total)として検出する（§8 lint も参照）。
- **軸タグを貼るだけでは効かない**（#40）: `scholia flow`/`scholia gaps` が拾う軸は、対象 action の transition の `given` に実際に現れる condition が持つ axis タグだけ（`relevantAxes`）。condition に axis タグを貼っても、その condition をどの transition の given にも書かなければ軸は解析対象に入らない。軸を効かせるには、複数条件を畳んだ transition を条件別に割り、given へ materialize する構造変更が要る。
- **軸分析は単一 owner の相互排他分岐を前提にする**（#40）: per-action 解析（`scholia flow`/`scholia gaps`）は transition の owner を区別しない（`Transition` に owner フィールドは無く、owner は effect の `VocabEntry.Owner` のみ）。複数 owner の transition が同じ action を共有すると、同一 cell を別々の owner の transition が覆って偽の重なり（Overlap）が出うる。軸を張る action は単一 owner の相互排他分岐であることを前提とし、複数 owner に共有される action は軸分析の対象外として扱う。
- **no-op 側を持つ2値は total にしない**（#40）: 片方の値が本質的に no-op（対応する transition が無いのが正しい）な2値軸を `total=true` にすると、no-op 側が偽の抜け（L-total）として出る。そのような軸は非 total にするか、値を分割して表す。
- **どの状態を軸にするか（型でなく action から／投影と遷移の境界）** の判断は authoring 正典 `agents/skills/_scholia-shared/references/modeling-principles.md` の「軸の見つけ方」節に従う。要点: enum 型フィールドを一律 axis にせず、「その効果は今の状態からいつでも再生成できるか？」で **YES=投影（軸にしない・型検査の仕事）／NO=遷移（軸候補）** を切り分ける。

> **用語注（「軸」の 3 義）**: 本ドキュメントの「軸」は文脈で別物を指す。混同を避けるため:
> 1. **宣言軸（`kind="axis"`）** — 本節の `axis.*` タグ。状態次元で、`scholia flow`/`gaps` の網羅検査の単位。
> 2. **分類軸（カテゴリ × kind）** — §2「三つの軸」やタグ分類で言う「軸」。`condition`/`action`/`effect` のカテゴリと kind による振る舞いの分類次元。
> 3. **facet 軸（`facetKinds`）** — ビューア Browse の絞り込みナビ次元（§3.6/§3.8）。
>
> 加えて §2 の見出し「三つの軸」（カテゴリ/kind/tag）、`modeling-principles.md` §2「三つの軸の粒度」（vocab/tag/transition）でも "軸" を一般名詞として使う（いずれも上の 1 とは別語義）。

### 3.5 意思決定（decision）— transition と tag のどちらにも付く

```jsonc
// .scholia/decisions/01J8XR....json  （遷移への意思決定）
{ "id": "01J8XR...", "target": { "type": "transition", "id": "T-login-submit-valid" },
  "why": "トークンは httpOnly cookie で発行（XSS 対策）", "changed": "add eff.session.issue-token",
  "ref": "PR#42", "commits": ["a1b2c3d", "e4f5g6h"], "at": "2026-07-10T00:00:00Z" }

// .scholia/decisions/01K2AF....json  （タグへの意思決定＝cross-cutting 不変条件）
{ "id": "01K2AF...", "target": { "type": "tag", "id": "req.auth-symmetry" },
  "why": "null と空文字は同じ『未入力』として同一に扱う。この要件を持つ全遷移が守る規則。",
  "ref": "PR#57", "at": "2026-07-11T00:00:00Z" }
```

- `Decision { id, target: {type: "transition"|"tag", id}, why, changed?, ref?, commits?[], at }` — **append-only**（消さない・直さない。訂正は新しい 1 件を足す）。
- **1 件 1 ファイル**にすることで、append が**衝突しない**（配列 JSON だと末尾追記が git で衝突する）。
- **`commits[]`（git hash の集合・任意）は実装来歴**。`ref`（外部 URL/PR）とは別軸で、**repo 内の正確な着地点**を持つ
  （推奨 1 decision : 1 commit だが、実装ミス直しなどで 1 decision に複数 commit を許容）。
  - `scholia decide --commit <hash>`（繰り返し可）で decide 時に結べる。
  - 着地後に結ぶ／追加する場合は **`scholia decision add-commit <decisionId> <hash>...`**（追加専用）を使う。
  - **append-only の精緻化（欄位単位・#45 U4）**: decision の append-only とは「ファイル不変」ではなく
    「**判断欄位の不変＋来歴欄位の単調追記**」である。判断（`why` / `changed` / `ref` / `at`・`target.type`）は
    凍結され、来歴（`commits[]`）は `add-commit` により**追記専用・単調増加**で変わりうる（判断は凍結されたまま）。
    `target.id` は正本レコード側 rename／merge の機械追随でのみ張替わる（`scholia diff` は同一 diff 内の
    rename／merge ペア照合で判定し、`diff --check` が CI でこの不変条件を守る。#42 型の全店 retrofit は
    明示の例外承認——理由必須・出力記録——の逃し弁で通す）。
    「decision ファイル完全不変」から一段緩めた運用だが、CLI は commit の追加しかできず、
    過去の判断は消えない・書き換わらないため、append-only の狙い（監査可能性）は保たれる。
    別の判断が入った場合は decision を新しく足す（`scholia decide`）— decision の無駄な増殖を避けるため、
    実装ミス直しのような「判断は同じだが着地コミットが増える」ケースにだけ `add-commit` を使う。
  - **ビューアからの採用（adopt）も同じ append-only 経路**: `POST /api/decision`（§7）は毎回新しい ULID の
    decision を 1 件生成するだけ（既存 decision には触れない）。採用時点では `commits[]` は空で、
    人が commit した後に `scholia decision add-commit` で結ぶ。採用前に why を練り直すのは**未コミット下書きの合成**で
    あって過去判断の書き換えではない（commit 済み decision は凍結・監査可能性は保たれる）。
- **★タグに付けられる**のが要。spec という"容れ物"を無くした代わりに、
  **複数遷移をまたぐ不変条件はタグに刻む**。A と B が同じタグを持てば、A の変更時に**そのタグの decision が surface** する
  ＝「片方の変更がもう片方との矛盾に気づける」という変更評価の核を、**明示的な共有タグ**で実現する
  （従来の「たまたま同じファイル」より強い）。

### 3.6 config（singleton）

```jsonc
// .scholia/config.json
{
  "schemaVersion": 1,
  "kinds": {
    "condition": [],
    "action":    ["user", "api", "lifecycle", "system", "cron", "webhook"],
    "effect":    ["emit", "state", "http", "storage", "log"]
  },
  "tagKinds":          ["requirement", "concern", "subject"],  // タグ kind の宣言
  "facetKinds":        ["subject", "requirement", "concern"],  // ビューアの既定ナビ軸（派生階層の軸）
  "traceabilityKinds": ["requirement"],                        // 0 充足を gap 警告する tag kind
  "idPrefix": { "condition": "cond.", "action": "act.", "effect": "eff." }, // 命名規約（ソフト・grep 用）
  "roots":  [],                                                 // 追加探索ルート（構想）。現状**未配線**＝探索（LoadAll/index）には効かない。config get/set・PUT /api/config の読み書きのみ（§3.1）
  "sourceRefs": { "scan": [], "exclude": [] },                   // additive・任意（省略可）。rename の source-ref 走査／`scholia refs scan|rewrite` の対象範囲を絞る（§8.5）。scan 省略時はプロジェクトルート全体、exclude は常時除外（.scholia/.git/_workspace/.concierge・.gitignore）に加える追加除外。roots とは異なりこちらは**実配線済み**（EnumerateFiles の結果を境界安全なパス prefix でフィルタする）
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
| **unit / "spec"** | `subject.*` タグ（または任意のタグ）で遷移を束ねた**レポート**。`scholia spec <subjectTag>` で描画 |
| **タグ階層（統一ツリー）** | browse ナビ（viewer）は全タグを `parentIds` で入れ子にした**1 本の木**（kind 非依存・`GET /api/facets` の `roots`・`index.TagForest`）。**kind は木を分割する軸ではなくノードの属性**（バッジ/色）＋「その kind だけ表示」フィルタ。cross-kind の入れ子（例: `subject` 配下に `requirement`）や `kind` 未設定のタグも `parentIds` 通りに出る＝ CLI `tag list --tree`（無フィルタ）と木形が一致。多親は複数箇所に出現・循環はパス単位ガード |
| **facet 別グルーピング（kind スコープ）** | `scholia list --facet <tagKind>` は**その kind だけ**のタグ入れ子を木にして遷移を葉に並べる（`index.FacetTree`・kind スコープ）。要件トレーサビリティ（§7）も同じ per-kind 木を使う。統一ツリーとは別経路で、kind を横断しない |
| **rules（守る規則）** | 対象（tag／transition／facet）に関わる `decisions` を横断集約（cross-cutting 不変条件を含む） |
| **フロー図** | （後フェーズ）共有状態の語彙から derive。手書きしない |
| **検索** | 実効タグ・kind・語彙 label 横断 |

### 3.9 派生インデックス（クエリ層）

- **現状は in-memory のみ** — CLI 起動時／ビューア読込時に `.scholia/` のレコードを読んで索引を建てる。小〜中規模はこれで即応。
- **規模が要求したら SQLite に昇格（後フェーズ・未実装）** — `.scholia/index.db`（gitignore）に永続化する構想。再構築コマンドは Phase 昇格時に追加する（現状 `scholia index` コマンドは存在しない）。
- インデックスは**捨てても再生成できる**。壊れても真実は失われない。SQLite は「読み取り」の正しい位置に置く。

---

## 4. 変更の評価（提案起点・インライン評価）

spec ファイルも proposal ファイルも無い。**提案 = 作業ツリーの未コミット `.scholia` 変更（pending diff）＋それを説明するコメント**、
**評価結果 = decision** に落ちる。commit は評価の**結果**であって前提ではない（順序は一方向）:

> **① 提案（AI/人が作業ツリーへ pending 変更を書く）→ ② その変更に必ずコメントを付ける（本文＝why・＝「提案」）
> → ③ 人が viewer のドロワーで pending diff・コメント・守る規則を突き合わせて評価 → ④ 採用＝コメント本文を
> decision.why へ昇格（`POST /api/decision`）→ ⑤ 人が commit → `decision.commits[]` を後付けで結線。**

- **提案＝変更を伴うコメント。** あるレコード（transition/vocab/tag）が pending diff に変更を持つとき、そのレコードに
  付いたコメントが「提案」として差分カード付きで表示される。**コメント本文＝why**（別 why 欄を持たない）。
  変更だけでは提案にならず、**コメントが付いて初めて提案化**する（AI は変更時に必ずコメントを付ける・§8）。

```
# 提案する側（AI/人）: 作業ツリーで transition/vocab/tag を編集し、対でコメント（why）を配送
scholia review add --on transition:<id> --body "<why・提案コメント>"   # AI コメントのサイドカー配送（§8.4）
scholia diff [<ref1> [<ref2>]]  # 現在 vs <ref1>（既定 HEAD）＝pending diff（主線）、または <ref1> vs <ref2>（両方 git ref・landed 監査用）
scholia rules --tag <id>        # その提案が触るタグの過去 decisions（守る規則）と照合

# 評価する側の結着（どちらも decision を残す）
#   adopt  = 変更を採用 ＋「採用」decision を append（viewer のドロワーから `POST /api/decision`・§7）
#   reject = 採用しない ＋「取り込まない・理由」decision を append（次回同じ提案が来ても即・既決）
scholia decide --on transition:<id> --why "評価: 取り込まない。<理由>" --ref <PR/URL>
```

- 判定材料: **(a) 複雑性 diff**（語彙±・遷移±・then 順序）／ **(b) 既存 decision と矛盾するか**（衝突＝却下寄り・id 引用）／
  **(c) 既に検討済みか**。**decision は append-only**（過去を消す提案＝取り込み拒否の最有力根拠）。
- 「却下」を一言で済ませない——**なぜ取り込まないかを decision に根拠つきで残す**＝守る規則を厚くする。
- **評価はビューアの「見ているレコードのコメントドロワー」にインライン**（独立の比較ビューは持たない・§7）。
  主線 diff は pending diff（作業ツリー vs `main`）。ref 対 ref（`scholia diff <ref1> <ref2>`・`/api/diff?ref=&head=`）は
  landed した変更の**事後監査用**に温存する。

---

## 5. 不変条件 / lint

`lint` は「記録が自己矛盾していない」ことを守る（早期バグ検知ではない）。error があれば exit 1。

lint は**二層**。**error/warn＝記録の自己矛盾**の検査（従来定義そのまま・不変。error は保存拒否/exit 1・warn は CI ratchet の対象）。
**advisory＝authoring 規律（書き方規律）の改善提案**（`tier=advisory`・severity=info・保存も CI も止めない）。
advisory は「自己矛盾」ではなく「読みが重い・腐りやすい書き方」を検出し、書いた同じターンに警告して是正コストが最小の時点で直せるようにする。
decision の判断欄位（why/changed/ref/at）由来の advisory は append-only により是正が原理的に不能なので **acknowledge-only** 区分とし、是正リスト・残件バッジの分母から別掲する。

| rule | 重大度 | 何を守るか |
| --- | --- | --- |
| `vocab-ref` | error | 遷移の `action/given/then` が実在する語彙を解決する（宙ぶらりん参照なし） |
| `kind-valid` | error | 語彙の kind／タグの kind が config の宣言集合に含まれる（可変な不変条件） |
| `tag-ref` | error | 遷移・語彙の tagId、タグの parentIds が解決する＋**タグに循環がない** |
| `decision-target` | error | `decision.target` が実在する transition／tag を指す |
| `empty-then` | error | `then` 空の遷移は作れない。意図的 no-op は**ガード効果**（kind=state 等）で表す |
| `id-unique` | error | 各レコード種別で id 一意（ファイル名と一致） |
| `requirement-gap` | warn | `traceabilityKinds` のタグで、充足する遷移が 0 件（未充足要件の表面化）。warn 行に対象タグの direct decision 件数を併記（判断材料の提示であり、decision の存在だけで沈黙させない） |
| `kind-missing` | warn | タグの `kind` が未設定（null-kind）。どの facet/traceability にも属さず階層・要件追跡から漏れる |
| `ref-freshness` | warn | `decision.ref` が `file:line`（腐る参照）でなく URL/commit |
| `decision-coverage` | info | 遷移の why 到達性を direct（own decision）／via-tag（実効タグ＝own∪参照 vocab∪祖先閉包の経由 decision）／none の3段で判定。info に列挙するのは none のみで、3段の件数はサマリ行に畳む（via-tag の内訳は `--verbose`・`--json` は全件＋coverage） |
| `unused-vocab` | info | どの遷移からも参照されない語彙（`vocab rm` の発見手段）。axis kind タグ付き condition には削除助言を出さず「軸の値（given 未出現・placeholder/remainder 候補）」として軸 id・軸 decision 件数つきの文脈で報告 |
| `exclusive-violation` | warn | 同一 `given` が同一 axis タグの複数値を同時に持つ（軸排他の不変条件破れ。#39・`scholia flow` の gap 検出の健全性の前提） |
| `complement-missing` | warn | `total=true` の軸で、値(condition)が 2 件未満しか materialize されていない（相補条件の欠落。#39） |

**advisory 層（authoring 規律・`tier=advisory`・severity=info）**。導入時は全規則 info（CI ratchet の対象外）。warn 昇格は当該規則の違反ゼロ達成後に別 decision で行う（既存レコードが一斉に赤くなる移行断絶を作らない）。検出仕様・除外パターンの正本は実装（`internal/lint/rules_authoring.go`）とする。

| rule | 走査対象 | 検出・除外 |
| --- | --- | --- |
| `derived-value-in-desc` | axis タグの description | 自軸に貼られた condition id・`値={…}` 列挙・`total=` の書き写し（構造から派生できる情報の二重書き） |
| `stale-tense` | tag/vocab の description | 時点依存語（`現状`/`今回`/`未実装`/`#N`/`Level N` 等。除外は `config.lint.stalePatternExcludes`）。label・decision.why は対象外（runtime 状態欄・時点判断の履歴欄で時点依存が正） |
| `prose-ref` | tag/vocab の description | 「〜を参照」型のメタ指示（根拠は構造〔タグ→decision〕で辿る）。動詞活用の domain 用法（参照する/参照先 等）は構造的に除外 |
| `why-file-line` | decision の why/changed（acknowledge-only）・tag/vocab description | `path.ext:line` の腐る file:line 参照（decision.ref は `ref-freshness`(warn) の領分で重複検査しない） |
| `axis-without-decision` | kind=axis のタグ | own decision が 0 件（軸導入の根拠が未記録。write-time には含めない——tag create 直後は正規フロー上常に未充足） |
| `duplicate-atom` | transition 全量 | action＋given 集合＋then 列が一致する複製グループ（正規形「1 原子＋複数 subject タグ」への統合候補。decision 付き遷移を含む場合は張替えが要る旨を併記） |
| `dangling-id` | decision why/changed/ref・tag/vocab description・label | id 様トークンが現存レコードに解決しない。検出のみ（append-only 尊重）。除外3種——族 glob（`〜-*`）・プレースホルダ語彙（xxx/foo 等・config 追加可）・category+kind 族参照（`eff.log` 等） |
| `dead-doc-ref` | decision why/changed/ref・tag/vocab description | `*.md`/`docname §…` の文書参照が repo の versioned ファイルに解決しない・`/tmp/`・gitignored（`.concierge/` 等）パス（既決「ref は永続物・.concierge 禁止」の機械化）。http(s) URL は検査しない |
| `desc-length` | vocab の description | 閾値超（既定 300 字）——長文契約 desc は versioned 文書へ外部化して参照する（tag.desc はユーザーストーリー形式が正当で対象外・write-time 専用検査） |

**lint 緑＝網羅完了ではない**。別途、完了ゲート（§8）を通す。

---

## 6. CLI コマンド表

バイナリ名は暫定 `scholia`。真値は `scholia <cmd> --help`。全書き込み系に `--json`（エージェント駆動用）。

```
# セットアップ / 設定
scholia init [--dir <path>]                                   # .scholia/ を作成
scholia kind set <condition|action|effect> <k1,k2,...>        # kind 宣言（list/get も）
scholia config get|set <key> [<value>]                        # facetKinds / tagKinds / roots など

# 語彙
scholia vocab add <condition|action|effect> <id> --label <l> [--kind <k>] [--owner <l>]
scholia vocab edit <id> [--label <l>] [--description <d>|--desc-file <f>|--edit]  # label/description のみ更新（--label は空不可）
scholia vocab rm <id> [--category <c>]                        # 未参照限定
scholia vocab tag <id> --add <tagId>… [--rm <tagId>…]        # 語彙にタグ（遷移が継承）
scholia vocab rename <id> --to <newId> [--rewrite-refs] [--no-refs]   # 参照も一括更新。ソースの旧 id 残存は既定で dry-run 表示（§8.5）

# タグ（ネスト対応）
# --kind 省略時: tagKinds が 1 種類ならそれを既定採用／2 種類以上なら必須エラー／0 種類（退化 config）なら空許容（lint kind-missing が警告）
scholia tag create <id> --name <n> [--kind <k>] [--parent <tagId>…] [--desc <t>] [--color <c>] [--ref <url>] [--total]
scholia tag list [--kind <k>] [--tree] [--json]              # --tree でネスト表示
scholia tag edit <id> [--name][--kind][--parent…][--desc][--color][--ref][--total]  # --total は kind="axis" タグ向け（#39・§3.4）
scholia tag rename <old-id> <new-id> [--cascade] [--rewrite-refs] [--no-refs]  # 全参照を張替。--cascade でサブツリーごと。ソースの旧 id 残存は既定で dry-run 表示（§8.5）
scholia tag rm <id> [--force]                                # 未参照のみ・--force で detach cascade

# 遷移（原子）
scholia tx add <id> --action <a> [--given <c,…>] --then <e,…> [--tags <t,…>]
scholia tx edit <id> [--action][--given][--then][--tags]
scholia tx tag <id> --add <tagId>… [--rm <tagId>…] | --set <ids>
scholia tx rename <id> --to <newId> [--rewrite-refs] [--no-refs]     # decisions の target も一括更新。ソースの旧 id 残存は既定で dry-run 表示（§8.5）
scholia tx merge <dupId> --into <survivorId> [--rewrite-refs] [--no-refs]  # 重複遷移の統合（同一原子のみ・decision 追随・タグ union・baseline 追随・#45 U4）
scholia tx rm <id> --why <理由> --force                      # 破壊的（decisions も道連れ）

# 意思決定（transition か tag に付く）
scholia decide --on <transition|tag>:<id> --why <t> [--changed <s>] [--ref <s>] [--commit <hash>…]
scholia decision add-commit <decisionId> <hash> [<hash>...] [--json]  # 既存 decision の commits[] に追記専用（§3.5）
scholia decision list [--on <transition|tag>:<id>] [--json]           # decision をフラット一覧（--on は完全一致・祖先展開なし。rules=対象別集約とは別）

# 提案コメント（レビュー）— AI コメント配送のサイドカー（§8.4・read-only オーバーレイ）
scholia review add --on <transition|vocab|tag>:<id> --body <why> [--source ai] [--json]  # .scholia/reviews/<ulid>.json を書く
scholia review list [--on <transition|vocab|tag>:<id>] [--json]                          # 提案コメントを一覧

# 読み取り / 派生ビュー
scholia show tx <id> [--resolve] [--json]                    # 遷移 1 件（語彙 label 解決）
scholia show vocab <id> [--json]                             # 語彙 1 件 + 使用箇所（参照 transition の逆引き＝真の影響集合・§3.3）
scholia spec <subjectTag> [--json]                           # 主題タグで束ねた"仕様"レポート（派生）
scholia list [--facet <tagKind>] [--tag <id>] [--kind <k>] [--json]   # faceted 一覧・グルーピング
scholia rules [--tag <id> | --tx <id> | --facet <k>] [--sort chrono|target] [--json]
scholia flow <action> [--json]                                # きっかけ(action)の給条件×遷移マトリクス＋証明可能な gap 検出（honesty-first・派生・§3.4・#39）
scholia lint [--json] [--ci]                                 # --ci は歯止め（ratchet）: error 常時 exit 1・baseline に無い新規 warn のみ exit 1（不在は非活性・#45 U4）
scholia lint baseline update [--json]                        # .scholia/lint-baseline.json（rule+target キー）を warn 集合で全置換（この経路以外で書かない）
scholia retrofit [--rule <id>] [--json]                      # advisory 規則で store を read-only 走査し「record×rule×該当引用×修正候補」を棚卸し（--fix なし・exit 0・acknowledge-only を別掲・#45 U2）
scholia diff [<ref1> [<ref2>]] [--json] [--check] [--allow-decision-retrofit <理由>]  # 現在 vs ref1、または ref1 vs ref2 の semantic diff（変更評価。ref1 vs ref2 で `<commit>^ <commit>` = 1コミット分の変更）。--check は CI ゲート（decision の欄位単位 append-only 判定のみ・違反 exit 1・#45 U4）

# ソース参照（rename の source-ref 機能の単体版・.scholia には触れない・§8.5）
scholia refs scan [--id <id>] [--json]                        # ソース中の scholia id 出現を一覧（健全性・棚卸し用）
scholia refs rewrite <old-id> <new-id> [--apply] [--json]     # ソースの旧 id を境界安全に置換。既定 dry-run・--apply で実施・冪等

# ビューア
scholia view [--port <p>]                                     # ローカルビューア（埋め込み SPA）
scholia export --html <dir>                                   # 静的 HTML 書き出し
```

> **注**: SQLite 索引の再構築コマンド（旧 `scholia index`）は後フェーズ・未実装（§3.9）。現状のインデックスは in-memory のみ。
>
> **注**: `scholia flow`/`scholia gaps` の軸解析は、対象 action の transition の `given` に現れる condition が持つ axis タグしか拾わない。axis タグを condition に貼るだけでは軸は効かず、その条件を given へ materialize する（畳んだ transition を条件別に割る）必要がある（§3.4）。

---

## 7. ビューア ＝ 評価コックピット

`scholia view` が**ローカル HTTP サーバ**を起動し、`//go:embed` で焼き込んだ SPA を配信する（同じ 1 バイナリ）。
フロントは**バイナリの API を叩く薄いクライアント**＝ lint/diff/search のロジックは Go に 1 つだけ持ち、検索は派生インデックス（§3.9）を叩く。

### 7.1 評価コックピットのモデル（インライン評価）

**評価は「見ているレコードのコメントドロワー」にインラインで行う**（独立の比較ビューは持たない）:

- **提案＝変更を伴うコメント。** あるレコード（transition/vocab/tag）が pending diff（作業ツリー vs `main`）に変更を持つとき、
  そのレコードに付いたコメントが「提案」として**差分カード付き**で表示される。**コメント本文＝why**（別 why 欄を持たない）。
- **AI は提案時に必ずコメントを付ける。** AI/変更スキルは変更本体（作業ツリー `.scholia`）と対で、**提案コメント**を
  `scholia review add`（§8.4）で `.scholia/reviews/` へ書く。ビューアは `GET /api/reviews` で読み、**人コメントと合流表示**する
  （AI コメントは read-only・`source='ai'`）。**人コメントは localStorage 維持**（`scholia-comments-v1`・G-3・ビューアはコメントをサーバへ書かない）。
- **語彙ピッカーで人が手直し。** ドロワーの提案カードで action/given/then/tags を編集できるが、**既存 vocab/tag の追加/削除/入替のみ**
  （自由記述は構造的に不可能＝vocab-only 構造ガード・§1 の atoms＋宣言制を UI/API の型で強制）。
- **採用＝コメント本文を decision.why へ昇格**（`POST /api/decision`）。append-only（毎回新しい ULID・既存 decision に触れない）。
  採用後 why の正本は decision（凍結）。commit は後付けで `commits[]` に結線（§3.5）。
- **追加/削除**も提案として表現する（追加＝subject 仕様一覧に緑カード／削除＝取消線で採用まで残す）。

### 7.2 ビューアが書ける 4 つ（他は全て read-only）

従来「ビューアが書けるのは config だけ」だったが、評価コックピットは以下の**書込 4 経路**を持つ。いずれも**未コミットの作業ツリー
`.scholia/` に書くだけ**で、**git は常に人**（ビューアは commit/branch/merge を一切しない・G-2）。

| API | 何を書くか | ゲート |
| --- | --- | --- |
| `PUT /api/config` | プロジェクト設定（config） | 従来から許可 |
| `POST /api/decision` | 採用の decision（`{on, why, changed?, ref?, commits?[]}` → 201・target は transition\|tag・why 必須・`commits[]` は採用時点で空） | **G-1（承認済）**。append-only は構造保証（新規 ULID のみ生成） |
| `POST /api/transition` | 提案の手直し／新規作成（`{id, action, given[], then[], tags[]}`・**vocab-id/tag-id スロットのみ**・自由記述不可）。未知 id→201 作成／既存 id→200 編集 | **G-1′（承認済・narrow）**。vocab-only 構造ガード＋lint（`vocab-ref`/`kind-valid`）＋git は人で封じ込め |
| `DELETE /api/transitions/{id}` | 提案の削除（作業ツリーの transition ファイルのみ削除） | **G-1′**。**参照整合ポリシー**（下記） |

- **G-1/G-1′ は承認済み。narrow に留める** — ビューアが書くのは decision（派生レコード）と transition（原子）のみ。
  transition 書込は自由記述を**型で受けない**（`transitionPostBody` は vocab-id/tag-id スロットしか持たない）＝機械的に atoms＋宣言制を強制する。
  vocab/tag の**原子そのものを書く**エンドポイントは持たない（vocab/tag は提案コメント・diff カードの対象にはなるが、ビューアからの書込対象ではない）。
- **削除の参照整合（P5a）**: `DELETE /api/transitions/{id}` は **`RemoveTransitionUnlinked`** を呼び、**その transition ファイルだけ**を消す
  （**cascade しない**）。**その transition を対象にする decision が 1 件でも残っていれば `409 Conflict` で拒否**し、何も消さない
  （decision は append-only ＝ ビューアからの削除で他人の判断記録を巻き込まない。`scholia lint` の `decision-target`（error）を
  宙ぶらりんにしない）。decision ごと道連れにする破壊的削除は `scholia tx rm --force`（人/CLI の明示操作）だけに残す。

### 7.3 主な閲覧・派生ビュー

- **タグ階層ナビ（統一ツリー）** — `GET /api/facets` が返す**1 本の統一フォレスト**（`{facetKinds, roots}`・`roots` は kind 非依存に `parentIds` で入れ子にした単一の木・各ノードは `tag`（`id/name/kind/color`）＋`children`）を辿る → **詳細表示**（遷移・実効タグ・rules）。
  **kind は木を分割せず**ノードのバッジ/色で示し、`facetKinds` は「その kind だけ表示」フィルタ（chips）として残す。見出し（サイドの索引）は `parentIds` の親ノードごとに**折りたたみ可能**（状態は facet ごとに localStorage 保持）。cross-kind の入れ子や `kind` 未設定タグも脱落せず、CLI `tag list --tree` と木形が一致する。同レスポンスは live handler と `scholia export --html` の静的焼き込みで共有（§9 単一の真実）。
  検索状態（検索語・kind facet・AND フィルタ chips）は URL（hash）に同期する（deep-linking）。リロード・ブラウザバック/フォワードで
  同じ検索条件・同じ位置を復元でき、既存のカード focus 用ルート（`#/browse/tag/<id>` 等）とはクエリ文字列（`?q=&k=&f=`）で共存する
  （`web/src/router.ts`）。見出しの折りたたみ状態は URL 化せず localStorage のまま（上記）。`scholia export --html` の file:// でもサーバ round-trip なしの hash 方式のまま動く。
- **要件トレーサビリティ** — `requirement` タグ → 充足遷移の逆引き＋ 0 充足 gap。kind スコープの per-kind 木（`index.FacetTree`）で独立に集計し、統一ツリー化の影響を受けない。
- **提案・守る規則の突き合わせ** — ドロワーで pending diff（`GET /api/diff?ref=main`）＋コメント＋対象タグの rules（`GET /api/rules`）を並べる。
  ref 対 ref（`?ref=&head=`）は landed した変更の事後監査用に温存（§4）。
- **`scholia export --html`** — 自己完結の静的 HTML（共有・CI 成果物・GitHub Pages 用・サーバ不要）。書込系（decision/transition 提案）は
  server-mode 限定で縮退。人コメントは localStorage で一様、AI コメント（サイドカー）は焼き込めば静的でも read-only 表示できる。
  Tag/VocabEntry の description は markdown-it（`html:false`）で描画し、highlight.js でコードハイライト、mermaid で図を描く。
  mermaid は動的 import で遅延ロードする重い依存だが、export はサイドカーファイルを持たない単一 HTML なので、実行時に動的
  import されるチャンク群は `internal/render/export_bundle.go` がエントリから再帰的に集めて Blob URL 経由で解決し直す
  （file:// でもネットワーク取得ゼロで動く）。

---

## 8. 良い記録の書き方（what — 中身で価値が決まる）

CLI の how とは別に、**何を登録するか**で価値が決まる。粒度・同一性・命名・desc/decision の役割分担・派生ビュー・**軸の見つけ方（型でなく action から／投影と遷移の境界）**・記録を書く前のチェックは authoring の正典 `agents/skills/_scholia-shared/references/modeling-principles.md`（各スキルから参照）に従う（用語「軸」の 3 義整理は §3.4 の用語注）。以下はその上での網羅の勘所:

1. **アクションの網羅** — 対象の外部 IF（入口）から機械的に洗い出す。ここで漏らすと以降すべて漏れる。
2. **条件(given)の網羅** — 決定表で。排他的な原因群は畳まず別遷移に。
3. **効果(then)を妄想で書かない** — 実際に起きることだけ。全 emit/効果が then に現れるか逆引き。
4. **完了ゲート（必須）** — 主題タグ単位で: マトリクス空白ゼロ／`decision.commits[]` が指す commit 経由でテスト/実装を辿る／穴探し 1 周。**lint 緑は網羅の証明ではない**。網羅チェックは `scholia spec <subjectTag>` の兄弟整合（同じ主題タグ配下の遷移群を横に並べて漏れを見る）で継続する。
5. **decision の質** — 後から矛盾に気づける why を書く（不変参照 `ref`・`file:line` は避ける）。cross-cutting な規則はタグに刻む。

### 8.5 ソース→scholia の id 参照（rename が追随する残存参照）

ソースコード中のコメント等に書かれた scholia id（`req.foo` 等）は、上の 5 と向きが逆の参照であり、避けるべき `file:line`
とは性質が違う——**`decision.ref` が外向きに `file:line` を指すのが問題**（コードが動くたびに腐る）だったのに対し、
ここは**ソースが安定 id で scholia を指す**。参照キーが行番号でなく id なので、コードやコメントの位置が変わっても腐らず、
**id が変わる唯一の操作（`rename`）が同時に追随を申し出る**。だから `ref-freshness`（§5）とは矛盾しない。

具体的には、`scholia {tag|vocab|tx} rename` は `.scholia/` の参照を張り替えたあと既定でソースを走査し、旧 id の残存箇所を
dry-run 表示する（ソースは変えない）。`--rewrite-refs` を付けるとその場で境界安全に old→new を置換する。単体で
使いたいときや部分失敗の再実行には `scholia refs rewrite <old-id> <new-id> [--apply]`（冪等）・棚卸しには
`scholia refs scan [--id <id>]` を使う。境界安全性は「直前直後が id 継続文字（`[A-Za-z0-9._-]`）でない」ことを条件に、
文末句読点（`req.foo.`）だけを例外的に許容し、`req.foo-bar`／`req.foobar` のような別 id・別語との衝突は起こさない
（詳細は `internal/refs` のコメントを正本とする）。真実の源は依然 `.scholia/` — このマーカー無しの走査・置換は
あくまでソース側の利便であり、scholia の記録そのものを増やすものではない。

走査対象は `git ls-files`（`.gitignore` 尊重）が既定経路。**git が使えない場合（git 未導入・非 git リポジトリ）は
ディレクトリ walk にフォールバックし、その場合は `.gitignore` を尊重しない**（常時除外の `.scholia/.git/_workspace/
.concierge` のみ適用）。git 管理下の通常利用では影響しない非対称だが、黙って挙動が変わらないようここに明記する。
走査範囲は additive な `config.sourceRefs`（`scan`/`exclude`・§3.6）で絞れる。

---

## 9. 言語・リポジトリ構成（Go）

```
scholia/
  cmd/scholia/main.go            # CLI エントリ（cobra）
  internal/
    model/                    # レコード型（Transition/VocabEntry/Tag/Decision/Config）
    store/                    # 1 レコード 1 ファイルの探索・atomic read/write・id↔ファイル名
    index/                    # 派生インデックス（in-memory→SQLite）・faceted query・rules・search
    lint/                     # lint ルール
    diff/                     # semantic diff（git ref / 作業ツリー間）
    render/                   # 派生"仕様"ビュー・export
    review/                   # AI 提案コメントのサイドカー（.scholia/reviews/・§8.4）
    cli/                      # cobra コマンド
  internal/viewer/            # HTTP サーバ ＋ //go:embed（config/decision/transition 書込・reviews/diff/rules 読取）
  web/                        # ビューア SPA のソース（ビルド→ go:embed）
  npm/                        # npm ランチャ（postinstall で binary 取得）
  packaging/                  # goreleaser / homebrew / scoop / install.sh
  agents/skills/scholia/         # .agents/skills（＋ claude plugin）
  docs/  .goreleaser.yaml  go.mod
```

- **module path**: `github.com/nkenji09/scholia`（`go install …/cmd/scholia@latest` を成立させるため実 URL と一致）。
- CLI は **cobra**。JSON は標準 `encoding/json`。ロジックは `internal/*` に集約し CLI とビューアが同じコアを呼ぶ（単一の真値）。

---

## 10. 配布（みんなが使える形）

**プレビルド・バイナリを軸に多チャネル**。特に ④ が AI エージェント界隈への到達に効く。

1. **GoReleaser** → GitHub Releases に darwin/linux/windows × amd64/arm64
2. **Homebrew tap**（`brew install`）／ **Scoop・winget**（Windows）
3. **`curl … | sh`** ワンライナ（install.sh）
4. **⭐ npm ランチャ** — postinstall で対応バイナリを取得。**`npx scholia …` がゼロインストールで動く**（esbuild=Go / Biome=Rust の実績）
5. **`go install`**
6. **Claude plugin ＋ `.agents/skills`**（＋ 必要なら `.cursor/rules` 等）— 無ければ検出してインストールを促し、あとはバイナリに委譲

---

## 11. 仮の既定値（要確認）

- **バイナリ名**: `scholia`（npm/repo 名も `scholia` で統一）
- **ストレージ**: `.scholia/`
- **module path**: `github.com/nkenji09/scholia`
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

- **Phase 0（骨格）**: `model` + `store`（1 レコード 1 ファイル）+ `scholia init` + `vocab add` + `tag create` + `tx add` + `show` + `lint`。
  素の JSON が生まれて `lint` が緑になるまで。
- **Phase 1（記録の核）**: `decide`（transition/tag）+ `rules` + 実効タグ + `requirement-gap` + `rename`。
- **Phase 2（派生・評価）**: in-memory インデックス + `list --facet` + `spec <subjectTag>` + `diff`（git ref 間）。
- **Phase 3（閲覧）**: `scholia view`（埋め込みビューア・faceted・比較）+ `export --html`（+ 規模次第で SQLite）。
- **Phase 4（配布）**: GoReleaser + npm ランチャ + Homebrew + skill/plugin パッケージ。
- **後フェーズ**: フロー図の derive（共有状態）・正本↔テスト照合の自動化・pluggable lint。
```
