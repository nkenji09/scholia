---
name: scholia-change
description: scholia の「変更を scholia レコードに着地させる」実行ワークフローを、方針が出た後から decision に着地するまでの手順として実行する。まず scholia-triage で対応方針（A是正/B精緻化/C矛盾/D新規/E却下）を決めた上で、UI コメントドロワーまたは端末で集めたレビューコメント（Tag の要件変更・Transition の実装との食い違い）を貼り付けて起動し、scholia・実装コードを人と読み合わせながら `.scholia/` を変更し、波及検索（Case 1）や兄弟 transition との整合（Case 2）を経て decision に着地させるときに使う。「このコメントを取り込んで」「Tag の要件が変わったので反映して」「Transition が実装とズレている」「方針が決まったので scholia に反映して」等で起動する。
---

# scholia-change — 変更を評価して取り込む（Case 1: Tag / Case 2: Transition / Case 3: 設計・横断 decision）

## これは何のためか

**対応方針の判定は [scholia-triage スキル](../scholia-triage/SKILL.md) が担う**——input（修正指示・要望・レビュー
コメント）を既存 spec に照らして **A是正／B精緻化／C矛盾／D新規／E却下** に分類し、方針＋WHY＋（B/D なら）
decision 下書きを出す判断層。本スキルはその先、**方針が出た後に変更を scholia レコードへ着地させる実行手順そのもの**
を担当する（役割分担: 判断＝scholia-triage、実行＝本スキル。判定基準・分類ロジックは本スキルで繰り返さず triage を
参照するだけに留め、二重記述を避ける）。

**2 つのフローが使える**（landed した評価コックピット・DESIGN §7）:

- **CLI フロー**: 端末でコメントを集め、**コピーして本スキルを AI に貼り付ける**。以降は既存 CLI
  （`diff`/`rules`/`list`/`decide`/`decision add-commit`/`review`）とコメントの copy-paste で完結する。
- **viewer インライン評価フロー**: `scholia view` のコメントドロワーで、pending diff（作業ツリー vs `main`）を
  **差分カード付きの提案**として見ながら評価する。**提案＝変更を持つレコードのコメント・本文＝why**。
  AI は変更本体と対で `scholia review add` で**提案コメントを配送**（`.scholia/reviews/`・read-only オーバーレイ）、
  人は語彙ピッカーで手直し（vocab-only）し、ドロワーの **Adopt** で採用（`POST /api/decision`）＝why を decision へ昇格する。

どちらのフローでも**着地の正本は decision（append-only）**で、判定材料と結着ルールは同じ。以下の手順は
CLI を軸に書くが、viewer を使う場合も同じ順序（提案 → コメント/手直し → 評価 → decide → commit → 結線）で進む。

**提案レビューは必ず `scholia view` で行い、AI（提案者）は対象を開いた状態の深リンク URL をレビュアに渡す。**
チャットに decision/transition 本文を貼るだけの軽い確認や静的 `export --html` はレビュー提示としては
不十分——`export --html` は hash ベースの深リンク自体は file:// でも動くが（DESIGN §7.3）、file:// パスは
共有可能な URL にならず、Adopt/Reject の書込 API（`POST /api/decision` 等）は server-mode 限定で export
では使えない（DESIGN §7.2）。対象タグ・関連 transition・decision 履歴の文脈込みで人が理解してから
adopt/reject できることが目的。深リンクの route 一覧は [scholia スキル](../scholia/SKILL.md) の `scholia view` 節へ。

## いつ使うか

- Tag（要件）の中身が変わった、というレビューコメントを受け取ったとき（**Case 1**）。
- Transition が実装・要件とズレている、というレビューコメントを受け取ったとき（**Case 2**）。
- 設計原理・横断的な選択の why を記録しておきたいとき（**Case 3**）。
- 実装が既に landing 済みで、後から decision だけ記録したいとき（**後追い／retrospective**）。

「この提案を取り込むべきか／どう対応すべきか」の**判断そのもの**は本スキルではなく
[scholia-triage スキル](../scholia-triage/SKILL.md) を先に使う（本スキルは triage で B精緻化／D新規／E却下 の方針が
出たものを scholia レコードに着地させる実行層）。

## task の考え方（1 変更 = 1 task）

1 つの修正は複数レコード（tag／transition／vocab）に跨ることが多い。**1 変更 ＝ 1 task（軽い作業単位）**として
扱い、跨る複数レコードは同一 task に集約する。着地時は **「1 task 1 commit 1 decision」を推奨形**とする
（強制ではない）。

- task はコード上の概念ではなく、**このスキルのセッション内で人と AI が共有する作業のまとまり**。
- 1 decision に複数コミットを許すのは自然（`scholia decision add-commit` で足す）。
- **実装ミス直し（判断は変わらない）は decision を増やさず、既存 decision に commit を足す。**
  別の判断が入ったときだけ新しい decision を足す（`scholia decide`）。decision の無駄な増殖は見づらさに直結するため
  避ける（DESIGN §3.5）。

## 共通の入口

1. **まず [scholia-triage スキル](../scholia-triage/SKILL.md) で対応方針を決める**（A是正／B精緻化／C矛盾／D新規／
   E却下 ＋ WHY ＋ B/D なら decision 下書き）。本スキルに来るのは、triage で **scholia レコードを変える／記録する**
   方針が出たもの——主に **B 精緻化**（既存 decision の敷衍）・**D 新規要件**（spec 追加）・**E 却下**（reject を
   記録）。**A 是正**は実装側の回帰で scholia レコードは変えない（後から decision を残すなら Case 3／retrospective）。
   **C 矛盾**は triage がユーザーに返し、意識的改訂の可否が決まってから来る。
2. UI のコメントドロワーまたは端末で集めたレビューコメント／triage の出力を、この場に貼り付けてもらう。
3. triage の方針を**着地先の Case に割り付ける**。対象（tag/transition の id）を確認し、この変更が
   **Case 1（Tag の要件変更）**・**Case 2（Transition の修正）**・**Case 3（設計/横断 decision）** のどれに
   着地するかを決める（triage の A–E は「どう対応するか」、Case 1/2/3 は「どの record に着地するか」で直交する）。
   複数に跨る場合は 1 task に集約し、該当する手順を順に踏む。
4. この decision が **prospective（変更を先導する）** か **retrospective（既に landing 済みの実装を
   後から記録する）** かを確認する。
   - retrospective なら、Case 1/2 の提案→レビュー→adopt の踊りは不要。
     `scholia decide --on <対象> --why "<why>" --ref <landing commit>`（または `--commit <hash>`）で直行してよい。
   - 完了ゲートも軽量にする: **landing commit を結線**（`decide --commit` または `decision add-commit`）＋
     **`scholia rules` で矛盾する既存 decision が無いか 1 回照合**するだけでよい。波及検索・兄弟ゲートは
     省略できる（後から波及に気づいたら、そのときは改めて Case 1/2 の手順で対応する）。
   - 後追いは特定のプロジェクトや用途に限った例外ではなく、scholia-change の一般的なルートの 1 つ。

**prospective のときの実装前ゲート（必須）** — 振る舞いを変える着手では、実装ワーカー／コード実装に
着手する前に、正本の変更（decision 下書き＋提案）を人が `scholia view` の深リンクでレビュー・承認する
ゲートを必ず挟む。**設計メモ（作業ツリーの散文ドラフト等）のレビューは正本レビューの代替にならない**
——正本＝scholia の decision/transition でレビューする。decision の `ref` は**永続物**（commit・DESIGN 節・
issue）を指し、`.concierge/` 等の揮発物は指さない。

## Case 1: Tag の要件変更

提供体験・要件そのものが変わったとき。

1. **入口理解** — コメントを読み、`scholia show tag <id>` と `scholia rules --tag <id>`（過去 decision＝守る規則）で
   現状を把握する。
2. **Tag に decide** — 人と対話し、要件変更の why を確定してから記録する（cross-cutting 不変条件の更新）:
   ```
   scholia decide --on tag:<id> --why "<要件変更の理由>" --ref <PR/URL>
   ```
3. **波及検索（核心）** — そのタグが波及する範囲を洗う:
   ```
   scholia list --tag <id>          # 実効タグにこの tag を含む transition を列挙（子タグ経由のヒットも拾う）
   scholia rules --tag <id>         # 既存 decision と照合（矛盾は却下寄り・矛盾する decision の id を引用）
   ```
   vocab 側の波及は `vocab` に一覧コマンドが無いため `grep -l '"tags"' .scholia/vocab/*.json` のように直接
   確認して補う。**影響先は全部同じ task に集約する。**
4. **`.scholia/` を編集** — 影響先の transition／vocab をブランチ上で変更する。
   **AI が変更したら、対で提案コメント（why）を配送する**（viewer で見えるようにする・DESIGN §8.4）:
   ```
   scholia review add --on tag:<id> --body "<この変更の why・提案理由>"
   scholia review add --on transition:<id> --body "<why>"   # 影響先ごとに
   ```
5. **提案 diff を出す**:
   ```
   scholia diff                     # 作業ツリー vs base（pending task の diff）
   scholia diff <ref1> <ref2>       # 着地後の landed task を再現したいとき（例: <commit>^ <commit>）
   ```
6. **人がレビュー** — AI は対象タグ／transition を開いた `scholia view` の深リンク URL を渡し、人はそこで
   ドロワー（差分カード＋提案コメント）を見比べる。調整があれば手順 4〜5 に戻り `.scholia/` を編集し直す
   （viewer では語彙ピッカーで vocab-only の手直しも可）。
7. **確定したら decide** — 影響先の transition／tag それぞれに:
   ```
   scholia decide --on transition:<id> --why "<変更の理由>" --changed "<変更内容>" --ref <PR/URL>
   ```
8. **commit（意味単位）** — `.scholia/` の変更を 1 つの意味単位コミットにまとめる。
9. **decision に着地 commit を結ぶ**:
   ```
   scholia decision add-commit <decisionId> <hash>
   ```
   decide 時点で commit のハッシュが既に分かっているなら、手順 7 で `scholia decide --commit <hash>` として
   最初から結んでもよい（9 は省略できる）。
10. **実装/テスト側へ** — 人が task の diff／コメントをコピーし、scholia の外（コード側）の実装・テスト修正を依頼する。

## Case 2: Transition の修正

要件と実装の食い違い・不足が指摘されたとき。

1. **入口** — Transition を指すコメント（要件 vs 実際の transition の齟齬）を読む。
2. **読解** — `scholia show tx <id> --resolve` と実装コードを読み、人と対話して変更提案を固める。
3. **`.scholia/` を編集** — transition（`given`/`then`/`tags` 等）を変更する。
   **AI が変更したら対で提案コメントを配送**: `scholia review add --on transition:<id> --body "<why>"`（DESIGN §8.4）。
4. **提案 diff を出す** — `scholia diff`（作業ツリー vs base）。viewer では当該 transition のドロワーに差分カードが出る。
5. **人がレビュー** — AI は対象 transition を開いた `scholia view` の深リンク URL を渡し、人はそこで
   提案・コメントを見比べる（viewer の語彙ピッカーで vocab-only 手直しも可）。調整があれば手順 3〜4 に戻る。
6. **完了ゲート（必須・DESIGN §8）— 兄弟 transition との整合**:
   同じ主題タグ配下の兄弟 transition を全部洗い、同種の食い違いが残っていないか 1 周確認する:
   ```
   scholia list --tag <subjectTag>   # 同じ主題タグの transition を横断列挙
   scholia spec <subjectTag>         # 主題タグで束ねた仕様レポート（WHEN/GIVEN/THEN を並べて見る）
   ```
   **`scholia lint` が緑でも網羅の証明にはならない**（DESIGN §5・§8）。ここは手動の突合として別に行う。
   **もう一つの観点として、変更が分岐する軸・実装の枝を数え上げる**——「この振る舞いはどの入力／サーフェスで
   発火するか、実装は何分岐しているか」を問い、提案がその全ケースを覆っているか（or 明示的にスコープ外＋
   理由）を確認する。兄弟 transition の整合が「同じ主題タグ配下を横に見る」観点なのに対し、こちらは
   「1 つの操作が実装内で分岐する軸を縦に数える」観点で、互いを代替しない。
   実例（#35）: viewer の「adopt で昇格元コメントを削除」は、採用ボタンの `confirmAdopt` が
   `c.source === 'ai'`（AI review）／それ以外（人コメント）で 2 分岐する
   （`web/src/components/comments/CommentPanel.tsx` の `confirmAdopt`）。初版は人コメントのみモデル化し、
   「AI review を viewer で adopt」ケースが漏れていた——分岐を数えて初めて気づけた。
7. **確定したら decide** — 変更した transition ごとに:
   ```
   scholia decide --on transition:<id> --why "<変更の理由>" --changed "<変更内容>" --ref <PR/URL>
   ```
8. **commit → 結線**:
   ```
   scholia decision add-commit <decisionId> <hash>
   ```
9. **実装/テスト側へ** — 人が task のコンテキストをコピーし、実装・テスト修正を依頼する。

## Case 3: 設計/横断 decision（設計原理の記録）

要件"内容"の変更でも Transition の修正でもなく、**設計原理・横断的な選択（why）を記録したい**とき
（例:「vocab の分類軸は intrinsic category×kind、tag は二次フィルタ」）。手順は軽量:

1. **理解** — `scholia show <対象>` と `scholia rules --tag <対象>` で、矛盾する既存 decision が無いか照合する。
2. **decide** — 対象は多くが `concern.*`（横断関心）や要件 tag:
   ```
   scholia decide --on <tag/concern> --why "<設計原理>" --ref <実装 commit>
   ```
3. 終わり。**波及検索（`scholia list --tag`）・兄弟 transition との整合（Case 2 の完了ゲート）は課さない**
   （設計原理そのものは兄弟に波及しない・実際の影響は実装 commit に `--ref` で結ばれている）。

**Case 1/2 との見分け**: Case 1/2 は「WHAT が要求されるか」を変える（内容が兄弟へ波及するので探索が要る）。
Case 3 は「WHY この設計にしたか」を残す（原理そのものは波及しない・impact は実装 commit 側にある）。

すでに運用に現れている実例: vocab 粒度の meta-decision が `concern.traceability` に置かれているのは
Case 3 パターン（本スキルで名前を付けて正規化する趣旨）。

## adopt / reject の着地

**adopt か reject かの判断は [scholia-triage スキル](../scholia-triage/SKILL.md) で済んでいる**
（B精緻化／D新規＝取り込む・E却下＝取り込まない）。本スキルはその結着を scholia レコードに接続するだけ:

- **adopt** — 変更を採用（git の commit/merge は人が行う）＋「採用」の decision を append。CLI なら `scholia decide`、
  viewer なら提案コメントの **Adopt**（`POST /api/decision`＝コメント本文 why を decision へ昇格）。手順は Case 1/2 の 7〜9 の通り。
- **reject** — 採用しない ＋「取り込まない・理由」の decision を append する。一言で済ませず、
  次回同じ提案が来たときの既決になるよう根拠を why に残す（矛盾する decision があれば id を引用する）。

## 完了条件

- 着手前に [scholia-triage スキル](../scholia-triage/SKILL.md) で対応方針（B精緻化／D新規／E却下 等）を確定している。
- triage の方針を着地先の Case 1／Case 2／Case 3 に割り付け、跨る場合は 1 task に集約している。
- prospective／retrospective のどちらかを確認している。
- prospective の場合、実装ワーカー／コード実装に着手する前に、正本の変更を `scholia view` の深リンクで
  人がレビュー・承認済みである（設計メモのレビューでは代替できない）。
- Case 1: `scholia rules --tag` で守る規則を確認し、`scholia list --tag` の波及検索で影響先を洗い出している。
- Case 2: 完了ゲート（同じ主題タグの兄弟 transition との整合・手順 6）を通している。
- Case 3: 波及検索・兄弟ゲートは課さず、`scholia rules` で矛盾する既存 decision が無いことだけ確認している。
- 後追い（retrospective）: landing commit を結線し、`scholia rules` で矛盾する既存 decision が無いか
  1 回照合している（波及検索・兄弟ゲートは省略可）。
- adopt/reject いずれも decision を why 付きで記録し、着地 commit が `commits[]` に結ばれている
  （`decide --commit` または `decision add-commit`）。
- 実装ミス直しで decision を無駄に増やしていない（`add-commit` で足りるケースは増やさず足りている）。
- 人が task の diff／コメントをコピーし、scholia の外（実装/テスト）の修正依頼まで橋渡ししている。

## 関連

- **対応方針の判定（A是正/B精緻化/C矛盾/D新規/E却下）は本スキルの前段 [scholia-triage スキル](../scholia-triage/SKILL.md)**。
  判定基準の本体は [`../_scholia-shared/references/evaluating-changes.md`](../_scholia-shared/references/evaluating-changes.md)。
- 判定材料・日々の読み書きコマンドは [scholia スキル](../scholia/SKILL.md)（変更評価フロー節）。
- 新規プロジェクトの初回セットアップは [scholia-config-setup スキル](../scholia-config-setup/SKILL.md)（本スキルの範囲外）。
- decision の `commits[]` ／ append-only の精緻化（判断は不変・`commits[]` のみ追記専用）は `DESIGN.md` §3.5 が正本。
- `scholia diff` の ref 対 ref 拡張・`scholia review`・CLI 全体は `DESIGN.md` §6 が正本（`scholia <cmd> --help` も真値）。
- 評価コックピット（viewer のインライン評価・提案＝コメント・語彙ピッカー手直し・Adopt・AI コメント配送）は
  `DESIGN.md` §7・§8.4 が正本。
