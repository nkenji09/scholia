# scholia モデリング原則（粒度・同一性・命名・記録）

scholia で「良い記録」を書くための、ドメイン非依存の判断規則。複数スキル（`scholia` / `scholia-config-setup`
ほか）から参照される共有リファレンス。CLI の使い方は各スキル本体、ここは**何をどの粒度で作るか**の原則。

---

## 1. 粒度は「同一性の種類」で決める（vocab と tag は直交する）

vocab と tag は形が近い（id・label・kind）が役割が直交する。**この違いが粒度を決める**:

| 軸 | 意味 | 逆引き（使用箇所）の意味 |
| --- | --- | --- |
| **vocab**（構成部品・constitutive） | この挙動は**同じコード/機構か** | **本物の影響集合**（同じ実装を共有＝そこを直すと全使用箇所に効く） |
| **tag**（記述分類・descriptive） | この挙動は**同じ型/概念か** | **族として一覧されるだけ**・依存は主張しない |

規律:

- **vocab を共有する ⟺ 実装が同一**（同じ component / composable / 関数を使う）。だから vocab の逆引き＝真の影響。
- **概念が同じだけ**（型は同じ `v-model` でも A と B が独立実装）→ **vocab は分ける**。束ねるのは **tag（概念の族）**。共有にすると A を直しても B に影響しないのに**偽の影響**が逆引きに出る。
- **逆に、usage が片コンポだけでも実装が共有なら共有を維持する**（usage で false-split しない）。範例: ある状態軸が複数コンポで共有される composable の実装なら真の共有なので、片コンポでしか使われないように見えても owner 名を付けて切ると false-split になる。**判定根拠は常に実装同一性——usage でも概念でもない**（概念で false-share せず・usage で false-split せず、を両方向で締める）。
- **「この概念はこう振る舞うべき」という共有ルール**は、その概念 tag への **decision**。編集時に surface するのは誤爆ではなく「契約が変わるなら関係箇所を全部見直せ」という**正当な cross-cutting**。
- **横断（concept）タグは「横断の観点＝軸」で作る**（重要）。`v-model` のような **generic な共通点**でグルーピングしない — v-model は全主題が持つので、プロジェクト全体では無関係な主題まで巻き込み、軸として意味を成さない。まず **「何の観点でこれらは横断されるのか」= 軸を特定**し、その軸で概念タグを作る。派生する個別ルールはその軸タグへの decision に置く。
  - 例: WidgetA と WidgetB を横断させている軸は **「下限〜上限のペア（range）を扱う」**（`concept.range`）。v-model が範囲タプル型であること・境界（lower/upper）・lower≤upper 不変条件は、**すべて「range である」ことから派生する aspect** で、`concept.range` への decision（共有ルール）として置く。`concept.v-model` のような generic 名で切らない。
  - 見分け方: 「これらが共通なのは、**何の性質を共有しているからか**？」を問う。その性質＝軸。軸が generic（どの主題にもある）なら、それは横断タグの単位として弱い。

### phase-boundary（重要）

**「実装が同一になるべきか」は実装フェーズの関心**であって、設計・型合意の段階では詰めない。この段階で分かる/決めるのは:

- 各主題の **IF サーフェス**（＝ per-component が既定。各コンポの IF は各自のもの）
- **概念の族**（＝ tag）
- **契約に書いてある自明な合成再利用**（wrapper が inner を embed して同じロジックを呼ぶ）だけ共有 vocab

内部実装の共有（composable 化）は実装フェーズで決めて **vocab を統合し decision に残す**（共有は後からの意図的決定・append-only で辿れる）。→ **この段階で「共有か独立か」を悩まない**（既定は分ける・概念は tag）。

---

## 2. 三つの軸の粒度

- **vocab**：実装の同一性（§1）。独立実装なら分ける。
- **tag**：1 要件 = 1 tag（要件は verbatim）。**出力が排他分岐する要件は子に割る**（例 `@invalid` / `@out-of-range` を `3-6-1` / `3-6-2` に）→ 各子を単一の action→effect にできる（「分岐だから保留」ではなく「割って解消」）。**kind で軸を分ける**（`requirement`=トレーサビリティ〔gap 判定に載る〕/ `concept`=族〔載せない〕/ …）。
- **transition**：原子的 `action ＋ [given] ＋ then`（then は順序保存）。**排他的な原因群は畳まず別遷移に**。効果が一様なら今引く／出力が実行時状態で分岐するなら分割 or given。
  **裏返し（束ねる）**: 逆に、**常に共起する効果は 1 つの意図を運ぶ transition に束ねる**。関連する効果を
  低レベルプリミティブの手動連鎖にモデル化すると「やり忘れ」が生まれる——`then` は複数効果を順序保存で
  持てるので、意図の単位はそこで束ね、低レベル操作は escape hatch として別に残す（分けるのは実行時に
  排他分岐するときだけ・常に一緒に起きる効果を分けない）。
  実例（#35）：「提案を adopt する」は「why を decision へ昇格 ＋ 昇格元コメントを削除」が常に共起する 1 つの
  意図。この操作を「裸のコメント削除（意思決定を伴わない `review rm` 相当）」と「decide」の 2 手順に分けて
  モデル化する案が初版で出たが、それだと decide 後の削除をやり忘れる余地が残る——正しい粒度は「adopt」
  「reject」という意図単位の 1 transition に束ね、裸の削除は escape hatch として別に残すこと。

---

## 3. 軸の見つけ方 — 型でなく action から／投影と遷移の境界

「どの状態を axis（状態次元・`kind="axis"`）にするか」の線引き。過剰に軸を切ると分析が薄まり、切り足りないと gap を見逃す。

### concept と axis の関係

- **concept（横断タグ）= 複数 subject が共有する横断的な族/trait**。共有不変条件(decision)と member の家であり、本質は *横断* と *ルールの家*（フィールドの袋ではない）。どの観点で横断するかは §1 の「横断の観点」で決める。
- **axis（`kind="axis"`）= 状態次元**。互いに排他な condition 値（enum 的・ちょうど 1 つが真）が **transition の `given`** に載り、action の結果(`then`)を分岐させる。`scholia flow`/`gaps` の網羅検査の単位（DESIGN §3.4）。
- **関係**: 1 concept は **0..N 個の axis** を持つ。**結果(effect)は axis に直結せず、transition が `given`→`then` で繋ぐ**（前提=condition／結果=effect の分離＝§4 命名の原則と接続）。
- ※ §1 で言う「横断の観点＝軸」は concept タグを束ねる *観点* のこと。本節の axis（`kind="axis"`・状態次元）とは別語義（「軸」の多義は DESIGN §3.4 の用語注も参照）。

### 軸は「型」でなく action から見つける

- ❌ 誤り: 「enum 型フィールド＝全部 axis」。全 `status`/`loading` を軸化すると過剰粒度になる。
- ✅ 正: axis は「その値で **action の観測可能な結果が変わる** 状態」。各 action で「結果(`then`)が排他状態で分岐するか？」を問うて探す。prop 名（`status`/`loading` 等）で pre-classify しない（分岐するかは次節の判定次第）。

### 投影(projection) と 遷移(transition) の境界（最重要）

「状態→見た目」を細かく効果化すると（例 `status=error → apply-error-class`）どの rendering も軸候補に見えてしまう。線引き:

- **transition が扱うのは「action が起こす、忘れうる・順序がある・副作用(emit)を持つ状態変化」だけ。**
- **rendering／導出（view = f(state)）は transition に載せない**——状態の投影であり、現在状態からいつでも再生成できる。

**判定（1 問）**: *その効果は「今の状態だけからいつでも再生成できるか」？*

- **YES → 投影**。軸にしない。「全 enum 値に対応する見た目があるか」という完全性は**型の exhaustiveness（型検査）の仕事**であって、軸の網羅検査の仕事ではない。
- **NO → 遷移**（"emit が発火した" は現在状態から再生成できない）。その状態を gate する condition は軸候補。

**この線引きが恣意的でない理由**: `scholia flow`/`gaps` が出す signal（coverage の抜け・subset-shadow・順序）は「忘れうる・順序がある・副作用のある結果」にしか働かない。投影は total-by-enum・無順序・無副作用なので、軸化しても cell が増えるだけで signal はゼロ——分析を薄めるだけになる。

- 例: `status=error → 赤スタイルを当てる` は**投影**（軸にしない）。`error 状態で入力 → 値をリセットして `change` を emit する` は**遷移**（軸候補）。

### 軸化前の罠 2 つ

1. **冪等 no-op**: ある値で観測可能な変化が起きない no-op なら、それは条件で分岐しない無条件効果であって軸ではない。
2. **所有/因果**: 状態が消費側（別コンポ）の所有で、subject は validity を emit するだけなら、その状態は subject の軸ではない（軸は状態を所有して分岐する owner のもの）。

### 既存規律との接続

- **既に別 action に割れている排他値**（`open`/`close` 等）は軸にしない——偽の L-total（抜け）を生む。同じ outcome に落ちる値は束ねる。
- **独立実装を跨ぐ軸は per-component に分割する（軸 gaps 健全性）**——`scholia gaps` は軸値を**ストア全体からグローバルに列挙**して網羅検査するので、共有した 1 つの axis が独立実装の複数コンポを跨ぐと、各コンポの action に**他コンポ専用の軸値が偽の L-total（抜け）**として混入する（実証: WidgetA の action gaps に WidgetB 専用の軸値が抜けとして出た）。**ここで vocab と axis は非対称**: vocab は per-component が既定（§4）だが、**axis は単一 consumer なら概念scope維持でよく、独立実装が複数コンポを跨ぐときだけ per-component 分割**する（範例: ある軸が WidgetA/WidgetB で独立実装なら分割・単一コンポだけが consumer の軸は概念scope維持）。vocab だけ分割して軸を共有のまま残すと、この偽 L-total が残って壊れる。
- **no-op 側を持つ 2 値軸は `total=false`**（DESIGN §8 lint の `complement-missing`／既存の axis-gaps 系 decision と整合。詳細は DESIGN §3.4 の #40）。
- **重なり（overlap）は given を完全修飾で割るより `priority`（評価順）で解決する方が安い**（#45 D8）。同じ cell を取り合う遷移が実装で決定的な if/else 順を持つなら、given を disjoint な完全修飾に書き換える（authoring 負担が遷移×軸に爆発する）代わりに `tx edit --priority <n>` で評価順を宣言する。畳めるのは**全遷移が相異なる priority を持つときだけ**（部分宣言・同 priority は未解決のまま）。全遷移に振ると最後尾が宣言的残余になり L-total を免除する。番号は実装の分岐順を読んで振る（desc の散文を鵜呑みにしない）。詳細は SKILL の「評価順」節・DESIGN §3.4。

---

## 4. 命名（衝突回避と可読性）

- **desc / label は markdown で書ける**：prop 名やコマンドは `` `code` ``、強調は `**bold**` を使って読みやすくする。ただし長文をベタ書きしない（短さは §5 の原則）。
- **transition id の prefix は `config.idPolicy` が正本**（宣言があると新規 id は保存時に強制される・P3 の reject 経由。既存 id と rename は対象外）。ポリシーを決めるときの指針：`tx.<Component>.<name>`（例 `tx.WidgetA.clear`）のような**主題名入りの prefix はファイル名衝突を避ける**（`tx.input-*` のような総称は他コンポと同名になりやすい／意図的な共有だけ `tx.shared.*`）。一方で総称 prefix（`T-` 等）を採る store もある。どちらが正かは store ごとに違うので、**迷ったら既存レコードの並びでなく `config.idPolicy` を見る**（他プロジェクトは自 config で別 prefix を宣言してよい）。
- **vocab id は実装同一性で粒度を決める**：**独立実装は最初から `<eff|act|cond>.<Owner>.<name>`（＝主題名で命名）が既定**。**effect / action だけでなく condition vocab も等しく owner-scope**——概念名で `cond.<共有概念>.<name>` のように切ると「概念scope＝共有」に見えて false-share になる（実装は各コンポ独立なのに逆引きが偽の影響を出す）。plain / 総称名（`eff.self.apply-size` 等）にすると、**プロジェクト全体 store では別主題が同名の独立実装を足したとき id 衝突→意図せず共有→主題横断の false-impact** を生む（size/blur/focus/status/loading 等の汎用挙動は必ず被る）。だから片主題専用でも owner 名で作る。plain id にしてよいのは**実装が共通と判明した共有**だけ（例: wrapper が inner を embed して同一コードを呼ぶ → owner=inner 名にして wrapper がそれを参照する）。「per-component で作り、共通と分かったら共通化」がルール。
  - ※ **軸(`kind="axis"` タグ)の scope は vocab と非対称**——vocab は owner-scope が既定だが、axis は単一 consumer なら概念scope維持でよく、独立実装が複数コンポを跨ぐときだけ per-component 分割する（§3「独立実装を跨ぐ軸」）。命名規則を axis に機械適用しない。
- **label**：action（きっかけ）は **「〜したとき」のトリガー表現**（例 `API setValue() を実行したとき`）。メソッドシグネチャの羅列にしない — `spec` の `WHEN 〜 THEN 〜` が読めなくなる。effect（結果）は**起きる事実**（`終了入力へフォーカスを送る`）。
  - ※ label / owner を変える CLI は無い（`vocab edit` は description のみ）。後から直すなら JSON の当該フィールドを直接編集。
- **きっかけ・前提・結果を書き分ける（action / condition / effect のカテゴリ分離）**：一つの事柄を書く前に「これは *きっかけ*（action・WHEN で発火するトリガー）／*前提*（condition・GIVEN で真の状態）／*結果*（effect・THEN で起きること）のどれか」を決め、その一つの欄にだけ書く。とくに **condition の label には「そのとき成り立っている事実・状態」だけ**を書き、**結果（何が起きるか）は transition の `then`（effect）の責務**なので condition に埋め込まない。同じ 1 つの事柄を condition と effect の両方に書かない。
  - ❌ `--force が指定されている（既存を上書き）` — 「既存を上書き」は結果（effect）であり、前提（condition）に混ぜてはいけない。
  - ✅ `--force が指定されている` — 結果は `then` 側の effect vocab（例 `既存を上書きする`）が表す。
  - **WHY**：condition は **`scholia flow` が読む状態次元の軸**として扱われる。結果を混ぜると given 集合が汚れ、subset-shadow（given 集合の包含関係）・L-total（軸の抜け）の読みが濁って分析が嘘の given を見る。しかも結果は transition の `then`／effect に既にあるので**二重書き**になり、**`then` を変えたときに condition の label／desc だけが古い結果を語り続けて嘘になる（drift）**。
- **label は観測可能に書く（ファジー語回避）**：特に effect の label は、**曖昧語を避け何が起きるかを具体的に**書く。読み手（人・AI・実装者）が実装を推測できる粒度にする。「温存する」「よしなに」「適切に処理する」「ハンドリングする」「ガード」等の**観測できない/解釈が揺れる**表現は避け、**何を出力/変更/返すか**を述べる。spec は人と AI が読み合わせる契約であり、曖昧語は解釈が分かれて実装とレビューがすれ違う。観測可能な記述なら実装の合否が spec だけで判定できる。
  - ❌ `既存の利用者ファイルを上書きせず温存する（ガード）`
  - ✅ `既存の利用者ファイルを上書きせず、スキップした旨の警告を出力する`

---

## 5. desc と decision の役割分担

- **desc = 決定を反映した「最新の状態」を分かりやすく**。使い方コード例・図・文脈は持ち込むが、採用判断が決着した箇所は現在形に書く（`[ ]` を残さない）。
- **decision = 意思決定・修正履歴（append-only）**。「何が・なぜ・いつ変わったか」はここ。**desc に決定メモをベタ書きしない**（仕様が更新されたら desc を最新形に直し、履歴は decision に append）。
  - 例: axis の desc に「なぜこの値数か／なぜ `total=false` か／owner は誰か」を詰めるのは、全部 decision の二重書き。desc は「**これは何か**」（＝次元名）だけを書く。
- **「〜を参照」というメタ指示を desc に書かない**：「根拠は decision を参照」「詳細は別タグを見よ」のような**メタ指示**を desc に埋めない。decision・vocab は構造的にタグへ紐づいており、viewer が対象カードの文脈として surface する——読者は散文の道案内ではなく**構造〔axis → 親 concept → decision〕を辿って**根拠に着く。散文の「参照」は *どの* decision かを指さないので **解決不能**になりがちだ（例: axis の desc に「decision を参照」と書いても、その decision は親 concept に target していて axis カードには出ず、読者は辿れない）。
  - ※ 子カード（axis 等）に親の decision を surface し、カード間を相互リンクする viewer の traversability は現在整備中。この原則は**その到達点（構造で根拠に辿り着ける）を前提**に書いている——根拠は構造に委ね、desc をメタ指示で膨らませない。

---

## 6. 派生ビューで見る（保存しない・全部 query）

分類のために vocab へタグを撒かない。**コンポ別の語彙一覧は「コンポ →（その遷移）→ vocab を `kind` で束ねる」導出**で見る（vocab に requirement/component タグを付けると遷移が誤継承し、requirement-gap も masking する）。共有 vocab は該当する全コンポの導出ビューに正しく現れる。vocab を分類したいだけなら軸は `category × kind`（全 vocab が必ず持つ）。

- **派生できる情報を desc / name にハードコードしない**：例えば**軸の値一覧を desc や name に列挙しない**。軸の値は「condition が axis タグを貼る」構造から派生表示される——列挙すると値を追加するたびに desc の保守が要り（メンテコスト）、構造と二重になる。desc は**次元名だけ**にして、値は構造に委ねる。同じ理由で、逆引き・一覧・`scholia spec` で導出できるものを desc へ書き写さない。

---

## 7. 完了ゲート（lint の読み方・typed 容認前提・#45 D6）

`scholia lint` の `requirement-gap` を潰し切る。**残ってよい gap は「当該タグ宛てで `acknowledges:[requirement-gap]` を含む decision が存在する」ものだけ**（typed 容認）。従来の「祖先に decision があれば緑」（untyped）は採らない——無関係な decision による偽陰性を作らないため。残す gap の類型と、それぞれ decision に書く内容:

1. **外部に実仕様がある**（「定義されたらリンクする」旨）
2. **不採用**（タイトルに `【不採用】` を明記＋不採用理由）
3. **構造制約 = 性質型要件**（action→effect でない・「〜しない」系・単一バイナリ等の非機能要件）。これは `scholia tag edit <id> --fulfillment property` で性質型と宣言し、**かつ** `scholia decide --on tag:<id> --acknowledges requirement-gap --why "…"` を足す。**property 宣言だけでは畳まない**（宣言のみ・decision 無しは warn のまま＝怠慢な宣言を許さない）。

flow の finding（`subset-shadow`・`total-gap`・`overlap`）も同様に typed 容認する: 対象（total-gap は軸タグ/欠落値 condition・shadow/overlap は関与 transition）宛てで `--acknowledges <rule>` を付けた decision があれば畳む。**同じ穴が複数 rule で出る場合は出る rule を全列挙して acknowledges に書く**。

`unused-vocab`(info) は「まだ遷移に出ていない語彙」＝保留中の挙動語彙で正常。`decision-stale`(info・git 導出) は「既存レコードを変更した commit に decision を結ばなかった」警告——正当なら対象レコード宛て `acknowledges:[decision-stale]` で容認。**エラー 0** を保つ。
