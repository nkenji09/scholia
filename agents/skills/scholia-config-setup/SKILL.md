---
name: scholia-config-setup
description: 新規プロジェクトに scholia を導入した直後、AI エージェントがそのプロジェクト内で「初回 config セットアップ」を対話で進めるときに使う。`.scholia/config.json` がまだ既定値のまま・vocab/tags が空の状態から、プロダクト固有の分類軸（tagKinds/facetKinds/traceabilityKinds 等）と代表的な初期 vocab/tags を対話で仕立てる。「scholia の初期設定をして」「scholia を導入したので config を決めたい」等で起動する。
---

# scholia-config-setup — scholia の初回 config セットアップを対話で仕立てる

## これは何のためか

`scholia init` は汎用的な `config.json`（`DefaultConfig()`）を冪等生成するだけで、
プロダクトのことは何も知らない。このスキルの価値はその先の 2 つ:

1. **既定を対話でプロダクトに仕立てる** — 「このプロダクトにとって tagKinds/facetKinds/traceabilityKinds
   は何が適切か」を聞き出し、`scholia config set` / `scholia kind set` で確定する。宣言したのに結局使わなかった
   kind が残らないよう、最後に掃除するところまで面倒を見る。
2. **代表的な初期 vocab/tags を撒いて最初の骨格を作る** — 空の `.scholia/` のままでは記録が始まらない。
   数件の vocab・tag をユーザーと一緒に作り、`scholia lint` が通る状態まで導く。
3. **display（productName/tagline/intro）の叩き台を作る** — `scholia init` 直後の `config.json` には
   scholia 自身のブランディング文言がそのまま入っている。`config set` は display 非対応なので、
   プロダクト理解から下書きを起こし `.scholia/config.json` を直接編集して差し替える。

日々の記録の読み書き（`scholia tx add` / `scholia decide` / `scholia lint` を回す通常運用）は
[scholia スキル](../scholia/SKILL.md) の範囲。**このスキルは導入直後の 1 回限りのセットアップだけ**を担う。

## いつ使うか

- 新しいプロジェクトに scholia を導入した直後（`scholia init` 済み、または未実行）。
- `.scholia/config.json` がまだ既定値のまま・`.scholia/vocab/` `.scholia/tags/` が空（またはほぼ空）。
- 「scholia の初回セットアップをして」「このプロジェクト用に config を決めたい」と言われたとき。

セットアップ済みのプロジェクトで tagKinds を 1 つ足したいだけ、のような小さな変更には使わなくてよい
（`scholia config set` を直接叩けば足りる）。

## スコープ外（v1）— ここは対話に含めない

- **`roots`** — 追加探索ルート。上級者向けで、既定の `.scholia/` のみで足りるプロジェクトが大半のため対話しない。
  必要になったら `scholia config set roots <path1,path2>` を単独で使えばよい。
- **`idPrefix`** — `cond.`/`act.`/`eff.` の読み取り専用ソフト命名規約。CLI から変更不可なので対話しない。

`display`（productName/tagline/intro）は `scholia config set` が非対応（対応キーは
tagKinds/facetKinds/traceabilityKinds/tagKindLabels/viewer.port/roots のみ）だが、**スコープ外ではない**。
手順 8 で `.scholia/config.json` を直接編集して叩き台を書く（詳細は手順 8 を参照）。

CLI の穴（対話フロー上どうしても設定したいのに `config set` で扱えないキーが出てきた等）に気づいたら、
**勝手に Go を変更せず** `.concierge/decision.md` に論点を書いて止まる。

## 進め方

### 0. 前提を揃える

`.scholia/` が無ければ先に作る:

```
scholia init [--dir <path>]
```

既に `.scholia/config.json` がある場合は、上書きでなく「今の値を起点に変えるかどうか」を対話する
（`scholia init` 自体が冪等・既存ファイルを壊さない）。

### 1. プロダクトを聞く

- 何を作っているプロダクト/コンポーネントか（1〜2 文でよい）
- 主な利用者・アクター（人間のユーザーか、API 呼び出し元か、システム内部の cron/webhook か等）
- 「要件」「関心事」「主題」のほかに、このプロダクト特有の分類軸（例: リスク種別、対象顧客セグメント）が要るか

この回答が次の knob 選びの材料になる。

### 2. 分類軸を一緒に決める（config）

> **進め方の全体像**: 手順 2〜4 でここからの内容（config の knob・初期 vocab・初期 tag 階層）を対話で決める。
> **この時点ではまだ `scholia config set` 等を実行しない** — 手順 5 で一度 ASCII 構造プレビューとして
> 可視化し、ユーザーと確認してから、決まった分をまとめて実行する。一般利用者はテキストの対話だけでは
> 最終的なタグ構成を想像しにくいため、実行前に一度見せて手戻りを防ぐのが狙い。

**各 knob は「意味を平易に説明 → 既定値を見せる → 変えるか聞く」の順で進める。**
変える理由が無ければ既定値のままでよいことも案内する（無理に全部変えさせない）。

| knob | 意味（DESIGN §2/§3.6 準拠） | 既定値 | 効くもの |
| --- | --- | --- | --- |
| `tagKinds` | tag の `kind` に使える値の宣言集合。tag は自由・ネスト可能な横断分類（DESIGN §2.1: "tags classify"）だが、kind 自体は宣言制 | `requirement,concern,subject` | 宣言外の kind で `scholia tag create --kind` すると弾かれる |
| `tagKindLabels` | 上記 tagKind の表示ラベル（ビューア表示用の日本語名など） | `requirement=要件,concern=関心事,subject=主題` | ビューアの表示のみ。挙動には影響しない |
| `facetKinds` | ビューアの既定ナビ軸（派生階層でどの tagKind を軸に一覧・絞り込みするか） | `subject,requirement,concern` | ビューアのナビゲーション。`tagKinds` の部分集合が自然 |
| `traceabilityKinds` | この tagKind のタグが「充足する遷移 0 件」だと `scholia lint` が `requirement-gap`（warn）を出す軸。要件トレーサビリティを保証したい kind | `requirement` | `scholia lint` の警告のみ。error にはならない |
| `kinds`（任意） | vocab の `kind`（condition/action/effect ごとに宣言制）。遷移の主語（action）・結果（effect）の分類 | action=`user,api,lifecycle,system,cron,webhook` / effect=`emit,state,http,storage,log` | `scholia vocab add --kind` で宣言外は弾かれる |
| `ownerKind`（任意・#45 D9） | effect の `owner` を「どの tagKind のタグ id で指すか」。宣言すると owner が subject タグ id 参照になり write-time で実在検証される（未宣言なら owner は自由文字列のまま） | 未宣言（空） | `scholia vocab add/edit --owner` の実在検証・viewer の owner 正準ルート |

聞き方の例:
- 「要件・関心事・主題のほかに、タグの種類として追加したいものはありますか？（無ければ既定のままで OK です）」
- 「ビューアで絞り込みたい軸はどれですか？（既定 = 主題・要件・関心事のまま で十分なことが多いです）」
- 「トレーサビリティを保証したい（＝抜け漏れを警告してほしい）タグの種類はどれですか？（既定は要件のみ）」

内容が決まったら、実行コマンドは次の形になる（値を変える knob だけでよい。既定のままなら実行不要。
**実行は手順 5 のプレビュー確認を経てから**）:

```
scholia config set tagKinds requirement,concern,subject[,追加分]
scholia config set tagKindLabels requirement=要件,concern=関心事,subject=主題[,追加分=ラベル]
scholia config set facetKinds subject,requirement,concern[,追加分]
scholia config set traceabilityKinds requirement[,追加分]
scholia kind set action user,api,lifecycle,system,cron,webhook[,追加分]   # 任意
scholia kind set effect emit,state,http,storage,log[,追加分]              # 任意
```

facetKinds/traceabilityKinds に足す値は tagKinds に宣言済みの kind にする（CLI は部分集合を強制しないので注意）。

#### kind の object 宣言・condition kind プリセット・ownerKind（#45 D9）

kind 宣言（`tagKinds`・`kinds.condition`）は **string（id のみ）でも object（`{id, label, description, behaviors}`）でも書ける** union 型。`scholia config set tagKinds`／`scholia kind set condition` は CSV を id 集合として解釈し、**既存 object 宣言の label/description/behaviors は id が残る限り保持する**（CSV で id を消さない限りメタは消えない）。description/behaviors を新規に持たせたいときは `.scholia/config.json` を直接 object 形で編集してよい（縮退マーシャルで既存 string 宣言は string のまま・git diff を汚さない）:

```jsonc
// tagKinds に軸 kind を behaviors 付きで宣言する例（別名 kind を軸化できる）
"tagKinds": ["requirement", "concern", "subject",
             { "id": "axis", "description": "網羅検査の軸", "behaviors": ["axis"] }]
```

- **`behaviors`（現状 `"axis"` のみ）**: kind に軸性を与える。flow・lint はこの behaviors 宣言を読んで軸判定するので、`axis` 以外の名前の kind でも `behaviors:["axis"]` を宣言すれば網羅検査の軸として振る舞う（旧 string `"axis"` 宣言は互換で軸挙動）。
- **condition kind プリセット**: `condition` の kind は「前提の出どころ」で分類すると読みやすい。典型は5 kind（description 付き object 形で宣言）:

```jsonc
"kinds": { "condition": [
  { "id": "input",   "label": "入力",     "description": "呼び出しの形。フラグ・引数・UI 操作のパラメータ" },
  { "id": "env",     "label": "環境",     "description": "プロセス外の実行環境。OS・導入方式・ポート・外部サービス状態" },
  { "id": "store",   "label": "ストア",   "description": "永続ストアの状態。実在・参照整合・pending 差分" },
  { "id": "derived", "label": "導出",     "description": "自身の導出状態。解析・index・ゲート判定" },
  { "id": "view",    "label": "ビュー状態", "description": "クライアントの一時状態。session/URL/表示モード" }
], "action": ["user","api","lifecycle","system","cron","webhook"], "effect": ["emit","state","http","storage","log"] }
```

- **`ownerKind`**: effect の owner を構造化したいプロジェクトは `.scholia/config.json` に `"ownerKind": "subject"`（等）を足す。宣言後は `scholia vocab add/edit --owner <tag-id>` が「`kind==ownerKind` の実在タグ id か」を write-time 検証し、未解決なら候補を提示して弾く。既存の自由文字列 owner の棚卸しは `scholia vocab owner-migrate` が対応案を出す（書き込みはしない・実適用は `vocab edit --owner`）。**前方非互換注記**: object 宣言を含む config は union を知らない旧バイナリで読めないので「バイナリ更新が先」。

### 3. 初期 vocab を撒く

vocab は振る舞いを**構成する部品**（DESIGN §2.1: constitutive）。遷移の `action`/`then` のスロットを
実際に埋めるので、代表的なものを数件作って骨格にする。

- action を 2〜3 件（プロダクトの主要な「きっかけ」。例: ユーザー操作、API 呼び出し、cron 起動）
- effect を 2〜3 件（それに対応する主要な「結果」。例: 状態変更、外部呼び出し、通知）

例を見せつつ、実プロダクトに合わせて対話で決める（**実行は手順 5 のプレビュー確認後**）:

```
scholia vocab add action act.user.<動詞> --label "<日本語ラベル>" --kind user
scholia vocab add effect eff.<領域>.<動詞> --label "<日本語ラベル>" --kind state --owner <主体>
```

（`--owner` は effect のみ・任意。`--kind` は手順 2 で宣言した kind のみ使える）

### 4. 初期 tag 階層を作る

tag は横断分類（DESIGN §2.1: descriptive）。まず「主題（subject）」を数件、次に主要な「要件（requirement）」を
親子関係を持たせて数件作る。関心事（concern）も必要なら加える。

```
scholia tag create subject.<領域> --name "<日本語名>" --kind subject
scholia tag create req.<領域> --name "<日本語名>" --kind requirement
scholia tag create req.<領域>-<詳細> --name "<日本語名>" --kind requirement --parent req.<領域>
scholia tag create concern.<観点> --name "<日本語名>" --kind concern
```

`--parent` は複数指定可（多親 DAG）。循環になる指定は CLI が拒否する。

ここまでで tagKinds/facetKinds/vocab kinds の値、初期 vocab、初期タグ案が対話の中で固まった。
**上記の `scholia config set` / `scholia kind set` / `scholia vocab add` / `scholia tag create` はまだ実行しない。**
次の手順 5 で一度プレビューとして可視化し、ユーザーと確認してから、決まった分をまとめて実行する。

### 5. 確定前に ASCII 構造プレビューで確認する

手順 2〜4 で決めた内容を実行する前に、**対話で出てきた実プロダクトの具体例を使って**タグ階層を
ASCII ツリーで描き、ユーザーに確認してもらう。一般利用者はテキストの対話だけでは最終的なタグ構成を
想像しにくいので、コマンドを実行する前に一度目に見える形で見せて、手戻りを防ぐのが狙い。

プレビューに含めるもの:

1. **タグ階層ツリー** — 手順 4 で話した具体例（主題→要件のネストなど）を、抽象的な「subject/requirement」
   のままでなく、**そのプロダクトの実際の名前**で描く（利用者が自分のプロダクトに当てはめて想像できるように）。
2. **vocab kinds のサマリ** — 手順 2 で決めた action/effect の kind 採用一覧。

書式例（この体裁に寄せて描く）:

```
提案した config だと、こう構造化されます:

components/
├─ Form inputs/
│   ├─ UISampleCombobox
│   │   └─[req] 複数選択モード対応
│   └─ UISampleInput
│       └─[req] open中の編集を反映
└─ Data display/
    └─ UISampleTable
        └─[req] Figmaプロト整合

vocab: action=user,system
       effect=emit,state,a11y,style

この構成で進めますか？調整したい点は？
```

「この構成で進めますか？調整したい点は？」と確認し:

- 調整が無ければ → 手順 2〜4 で示したコマンドをまとめて実行し、手順 6（lint）へ進む。
- 調整があれば → 手順 2〜4 の該当箇所を対話でやり直し、プレビューを描き直して再確認する
  （ユーザーが納得するまでこのループを繰り返す。作る前に見せる＝手戻り削減が目的）。

### 6. lint で仕上げを確認する

```
scholia lint
```

**目標は error 0。** `requirement-gap`（traceabilityKinds のタグを持つ遷移がまだ無いことの warn）は、
このセットアップ段階ではタグを付けた遷移をまだ作っていないため出て当然の**仕様上の警告**であり、
このスキルの完了条件には含めない（error が 0 であればよい）。error が出た場合は原因
（未宣言 kind を使った・親タグが実在しない・循環している 等）を読み解いて直す。

### 7. 未使用の宣言を掃除する

手順 2 の「変える理由が無ければ既定値のままでよい」は knob の**値**の話であり、
**宣言だけあって中身（実タグ／実 vocab）が 0 件の kind を放置してよい、という意味ではない**。
使わない設定は消してあげるところまでがこのスキルの対応範囲。手順 3〜4 で実際に何を作ったかを
振り返り、宣言した tagKinds/facetKinds/vocab kind のうち使われなかったものが無いか最終チェックする。

チェック方法:

```
scholia tag list --kind <tagKind>                        # 「(該当するタグはありません)」なら未使用
grep -l '"kind": "<vocabKind>"' .scholia/vocab/*.json     # ヒットが無ければ未使用（vocab list コマンドは無い）
```

未使用が見つかったら、その kind を tagKinds/facetKinds/traceabilityKinds や kind action/effect から
外してよいかユーザーに確認し、合意が得られたら反映する（合意が得られなければ残したままでよい）:

```
scholia config set tagKinds <残す kind の一覧>
scholia config set facetKinds <残す kind の一覧>
scholia config set traceabilityKinds <残す kind の一覧>
scholia kind set action <残す kind の一覧>   # 未使用の action kind があった場合のみ
scholia kind set effect <残す kind の一覧>   # 未使用の effect kind があった場合のみ
```

**注意**: `tagKinds`（`scholia config set tagKinds`）と `kind set`（vocab の action/effect）は、
実タグ／実 vocab が残っている kind を外そうとすると CLI が使用中エラーで弾いてくれるが、
`facetKinds`/`traceabilityKinds` にはこの使用中チェックが無い。実タグが残っている kind を
`facetKinds` からだけ外す、といった操作もエラーなく通ってしまうので、**上のチェック方法で
0 件と確認できた kind だけを慎重に外す**（tagKinds からも同時に外れて 0 件になった kind に限る）。

（`scholia lint` の `unused-vocab` info は個々の vocab エントリ単位の警告で、この kind バケツ単位の
チェックとは粒度が異なる。両方見るのが望ましい。）

### 8. display の叩き台を作る

`display`（productName/tagline/intro）は `scholia config set` の対象外なので `.scholia/config.json` を
直接編集する。`scholia init` 直後の `config.json` には scholia 自身のブランディング文言
（`productName: "scholia"` など）がそのまま入っており、他プロダクトのために放置すると viewer に
「scholia」表記が残ってしまう。

**これは決定稿ではなく叩き台**: AI が完璧に仕上げるのではなく、手順 1 で聞いたプロダクト理解から
素早く下書きを提示し、「後で viewer の設定画面や `config.json` から仕上げてください」と伝える
位置づけであることを対話で明示する。

- `productName` — プロダクト名（手順 1 の回答から）
- `tagline` — 一言キャッチ（既定の「記録を、読みたくなる形で。」に類する簡潔な一文）
- `intro` — 2〜3 文の紹介文（何を記録するツールか）

進め方:

1. 叩き台を対話で提示し、「これは叩き台です。後で調整できます」と断った上で確認する。
2. 合意が得られたら `.scholia/config.json` を読み込み、`display` オブジェクトの 3 フィールドだけを
   書き換えて保存する（`display` 以外のキーには触れない・有効な JSON を維持する）。
3. 空文字にした場合は `omitempty` で JSON キーごと省略され、viewer 側の組み込み既定（"scholia" 表記）に
   フォールバックする（`internal/model/model.go` の `DisplayConfig` 参照）。叩き台としてはプロダクト名を
   埋める方が実用的なので、基本は空文字にせず下書きの文言を入れる。
4. `scholia lint` が変わらず error 0 であることと、`scholia view` で反映されることに軽く触れる。

### 9. 反映確認

```
scholia config get
```

で config 全体（`display` を含む）を出し、手順 2・7・8 で決めた内容が反映されているか
ユーザーと確認する。

## 完了条件

- `.scholia/config.json` がプロダクト固有の分類軸に仕立てられている（既定のままでよいと合意した knob は変更不要）
- 代表的な初期 vocab（action/effect 数件）と tag 階層（subject/requirement を中心に親子関係含む）が `.scholia/` にある
- コマンド実行前に ASCII 構造プレビュー（タグ階層ツリー＋vocab kinds サマリ）をユーザーに見せ、
  「この構成で進めますか？」の合意を得ている
- 宣言した tagKinds/facetKinds/vocab kind のうち実際に使われなかったものを最終チェックし、
  あれば削除を提案している（手順 7）
- display（productName/tagline/intro）の叩き台を `.scholia/config.json` に直接書き込み、
  「後で仕上げる下書き」であることをユーザーに伝えている（手順 8）
- `scholia lint` が error 0（`requirement-gap` の warn は許容）
- スコープ外（roots/idPrefix）は対話に含めていない

## 関連

- 日々の記録（tx/decide/lint の運用）は [scholia スキル](../scholia/SKILL.md) へ。
- **粒度・同一性・命名・記録の原則は共有リファレンス `../_scholia-shared/references/modeling-principles.md`**（vocab=実装の同一性／tag=概念の族／概念には `concept` のような facet kind を足す／per-component 既定 等）。分類軸（手順 2 の tagKinds/facetKinds/traceabilityKinds）を決めるときの拠り所。
- config の全フィールドの意味は `DESIGN.md` §3.6（内部設計の背景）。
