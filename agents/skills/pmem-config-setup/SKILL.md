---
name: pmem-config-setup
description: 新規プロジェクトに pmem を導入した直後、AI エージェントがそのプロジェクト内で「初回 config セットアップ」を対話で進めるときに使う。`.pmem/config.json` がまだ既定値のまま・vocab/tags が空の状態から、プロダクト固有の分類軸（tagKinds/facetKinds/traceabilityKinds 等）と代表的な初期 vocab/tags を対話で仕立てる。「pmem の初期設定をして」「pmem を導入したので config を決めたい」等で起動する。
---

# pmem-config-setup — pmem の初回 config セットアップを対話で仕立てる

## これは何のためか

`pmem init` は汎用的な `config.json`（`DefaultConfig()`）を冪等生成するだけで、
プロダクトのことは何も知らない。このスキルの価値はその先の 2 つ:

1. **既定を対話でプロダクトに仕立てる** — 「このプロダクトにとって tagKinds/facetKinds/traceabilityKinds
   は何が適切か」を聞き出し、`pmem config set` / `pmem kind set` で確定する。
2. **代表的な初期 vocab/tags を撒いて最初の骨格を作る** — 空の `.pmem/` のままでは記録が始まらない。
   数件の vocab・tag をユーザーと一緒に作り、`pmem lint` が通る状態まで導く。

日々の記録の読み書き（`pmem tx add` / `pmem decide` / `pmem lint` を回す通常運用）は
[pmem スキル](../pmem/SKILL.md) の範囲。**このスキルは導入直後の 1 回限りのセットアップだけ**を担う。

## いつ使うか

- 新しいプロジェクトに pmem を導入した直後（`pmem init` 済み、または未実行）。
- `.pmem/config.json` がまだ既定値のまま・`.pmem/vocab/` `.pmem/tags/` が空（またはほぼ空）。
- 「pmem の初回セットアップをして」「このプロジェクト用に config を決めたい」と言われたとき。

セットアップ済みのプロジェクトで tagKinds を 1 つ足したいだけ、のような小さな変更には使わなくてよい
（`pmem config set` を直接叩けば足りる）。

## スコープ外（v1）— ここは対話に含めない

- **`display`（productName/tagline/intro）** — `pmem config set` から扱えず viewer の PUT API 経由でしか
  変更できない（Go 側の変更が要る）。後から viewer 上で設定可能・優先度も低いので v1 では対象外。
- **`roots`** — 追加探索ルート。上級者向けで、既定の `.pmem/` のみで足りるプロジェクトが大半のため対話しない。
  必要になったら `pmem config set roots <path1,path2>` を単独で使えばよい。
- **`idPrefix`** — `cond.`/`act.`/`eff.` の読み取り専用ソフト命名規約。CLI から変更不可なので対話しない。

CLI の穴（対話フロー上どうしても設定したいのに `config set` で扱えないキーが出てきた等）に気づいたら、
**勝手に Go を変更せず** `.concierge/decision.md` に論点を書いて止まる。

## 進め方

### 0. 前提を揃える

`.pmem/` が無ければ先に作る:

```
pmem init [--dir <path>]
```

既に `.pmem/config.json` がある場合は、上書きでなく「今の値を起点に変えるかどうか」を対話する
（`pmem init` 自体が冪等・既存ファイルを壊さない）。

### 1. プロダクトを聞く

- 何を作っているプロダクト/コンポーネントか（1〜2 文でよい）
- 主な利用者・アクター（人間のユーザーか、API 呼び出し元か、システム内部の cron/webhook か等）
- 「要件」「関心事」「主題」のほかに、このプロダクト特有の分類軸（例: リスク種別、対象顧客セグメント）が要るか

この回答が次の knob 選びの材料になる。

### 2. 分類軸を一緒に決める（config）

**各 knob は「意味を平易に説明 → 既定値を見せる → 変えるか聞く」の順で進める。**
変える理由が無ければ既定値のままでよいことも案内する（無理に全部変えさせない）。

| knob | 意味（DESIGN §2/§3.6 準拠） | 既定値 | 効くもの |
| --- | --- | --- | --- |
| `tagKinds` | tag の `kind` に使える値の宣言集合。tag は自由・ネスト可能な横断分類（DESIGN §2.1: "tags classify"）だが、kind 自体は宣言制 | `requirement,concern,subject` | 宣言外の kind で `pmem tag create --kind` すると弾かれる |
| `tagKindLabels` | 上記 tagKind の表示ラベル（ビューア表示用の日本語名など） | `requirement=要件,concern=関心事,subject=主題` | ビューアの表示のみ。挙動には影響しない |
| `facetKinds` | ビューアの既定ナビ軸（派生階層でどの tagKind を軸に一覧・絞り込みするか） | `subject,requirement,concern` | ビューアのナビゲーション。`tagKinds` の部分集合が自然 |
| `traceabilityKinds` | この tagKind のタグが「充足する遷移 0 件」だと `pmem lint` が `requirement-gap`（warn）を出す軸。要件トレーサビリティを保証したい kind | `requirement` | `pmem lint` の警告のみ。error にはならない |
| `kinds`（任意） | vocab の `kind`（condition/action/effect ごとに宣言制）。遷移の主語（action）・結果（effect）の分類 | action=`user,api,lifecycle,system,cron,webhook` / effect=`emit,state,http,storage,log` | `pmem vocab add --kind` で宣言外は弾かれる |

聞き方の例:
- 「要件・関心事・主題のほかに、タグの種類として追加したいものはありますか？（無ければ既定のままで OK です）」
- 「ビューアで絞り込みたい軸はどれですか？（既定 = 主題・要件・関心事のまま で十分なことが多いです）」
- 「トレーサビリティを保証したい（＝抜け漏れを警告してほしい）タグの種類はどれですか？（既定は要件のみ）」

確定したら適用する（値を変える knob だけでよい。既定のままなら実行不要）:

```
pmem config set tagKinds requirement,concern,subject[,追加分]
pmem config set tagKindLabels requirement=要件,concern=関心事,subject=主題[,追加分=ラベル]
pmem config set facetKinds subject,requirement,concern[,追加分]
pmem config set traceabilityKinds requirement[,追加分]
pmem kind set action user,api,lifecycle,system,cron,webhook[,追加分]   # 任意
pmem kind set effect emit,state,http,storage,log[,追加分]              # 任意
```

facetKinds/traceabilityKinds に足す値は tagKinds に宣言済みの kind にする（CLI は部分集合を強制しないので注意）。

### 3. 初期 vocab を撒く

vocab は振る舞いを**構成する部品**（DESIGN §2.1: constitutive）。遷移の `action`/`then` のスロットを
実際に埋めるので、代表的なものを数件作って骨格にする。

- action を 2〜3 件（プロダクトの主要な「きっかけ」。例: ユーザー操作、API 呼び出し、cron 起動）
- effect を 2〜3 件（それに対応する主要な「結果」。例: 状態変更、外部呼び出し、通知）

例を見せつつ、実プロダクトに合わせて対話で決める:

```
pmem vocab add action act.user.<動詞> --label "<日本語ラベル>" --kind user
pmem vocab add effect eff.<領域>.<動詞> --label "<日本語ラベル>" --kind state --owner <主体>
```

（`--owner` は effect のみ・任意。`--kind` は手順 2 で宣言した kind のみ使える）

### 4. 初期 tag 階層を作る

tag は横断分類（DESIGN §2.1: descriptive）。まず「主題（subject）」を数件、次に主要な「要件（requirement）」を
親子関係を持たせて数件作る。関心事（concern）も必要なら加える。

```
pmem tag create subject.<領域> --name "<日本語名>" --kind subject
pmem tag create req.<領域> --name "<日本語名>" --kind requirement
pmem tag create req.<領域>-<詳細> --name "<日本語名>" --kind requirement --parent req.<領域>
pmem tag create concern.<観点> --name "<日本語名>" --kind concern
```

`--parent` は複数指定可（多親 DAG）。循環になる指定は CLI が拒否する。

### 5. lint で仕上げを確認する

```
pmem lint
```

**目標は error 0。** `requirement-gap`（traceabilityKinds のタグを持つ遷移がまだ無いことの warn）は、
このセットアップ段階ではタグを付けた遷移をまだ作っていないため出て当然の**仕様上の警告**であり、
このスキルの完了条件には含めない（error が 0 であればよい）。error が出た場合は原因
（未宣言 kind を使った・親タグが実在しない・循環している 等）を読み解いて直す。

### 6. 反映確認

```
pmem config get
```

で config 全体を出し、手順 2 で決めた内容が反映されているかユーザーと確認する。

## 完了条件

- `.pmem/config.json` がプロダクト固有の分類軸に仕立てられている（既定のままでよいと合意した knob は変更不要）
- 代表的な初期 vocab（action/effect 数件）と tag 階層（subject/requirement を中心に親子関係含む）が `.pmem/` にある
- `pmem lint` が error 0（`requirement-gap` の warn は許容）
- スコープ外（display/roots/idPrefix）は対話に含めていない

## 関連

- 日々の記録（tx/decide/lint の運用）は [pmem スキル](../pmem/SKILL.md) へ。
- config の全フィールドの意味は `DESIGN.md` §3.6、vocab/tag の役割分担は §2.1 が正本。
