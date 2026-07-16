---
name: scholia-triage
description: 既存 spec（scholia の decision／transition）に照らして、input（修正指示・実現したいこと・要望・レビューコメント）の対応方針を A是正／B精緻化／C矛盾／D新規／E却下 の 5 つに選別し、方針＋WHY（＋ B/D なら decision 下書き）を出す判断プリミティブ。修正指示や要望やレビュー指摘を受け取って「そもそもどう対応すべきか」を過去の意思決定と突き合わせて決めたいとき、実装や decision 記録に着手する前の判断に必ず使う。「この指摘どう対応する」「この要望を spec に照らして」「これ取り込むべき？」「言われた通り直していいか」等で起動する。何も変更しない判断層で、実行は scholia-change。
---

# scholia-triage — input を spec に照らして対応方針を選別する（判断層）

## これは何のためか

修正指示・要望・レビューコメント等の **input を、既存の scholia 記録（decision の「なぜ」＋ transition の契約）に
照らして「どう対応すべきか」を決める**判断プリミティブ。もっともらしい指摘に**言われるがまま reactive patch**
すると、「指摘 → 修正 → 別のエッジケースを壊す → 指摘 …」の**モグラ叩き**に陥る。その手前で一度必ず spec に
照らす関門がこれ——scholia の存在意義そのもの。

**位置づけ: triage＝判断層（何もしない・方針だけ出す）／ [scholia-change](../scholia-change/SKILL.md)＝実行層。**
判断と実行を分けるのは、直す前に必ず「あるべき対応は何か」を過去の意思決定に照らすため。triage は `.scholia/` も
実装も変更しない。

## いつ使うか

- 修正指示・要望・「実現したいこと」を受け取り、**着手前に対応方針を決めたい**とき。
- レビューコメント／バグ報告を受け、**言われるがまま直してよいか**を過去の decision に照らして判断したいとき。
- 「この提案を取り込むべきか」を、実装や decision 記録の**前に**方針として確定したいとき。
- scope はコード修正でもデータでも機能でもよい（scholia-change が scholia レコードの変更に閉じるのと対照的に、triage は
  「そもそもどう対応するか」の一段手前）。

## 手順（詳細は共有リファレンスに従う）

> **判定基準・各分類の進め方・decision 下書きの書き方・「C 矛盾は黙って上書きせずユーザーへ」の徹底は、共有
> リファレンス [`../_scholia-shared/references/evaluating-changes.md`](../_scholia-shared/references/evaluating-changes.md)
> に従う**（スキルのベースディレクトリ起点）。以下は薄い呼び口。

1. **該当 spec を特定する**——input が触る挙動を支配している decision と transition を引く。方針を根拠づける材料。
   id や領域がまだ分からないときは、まず `scholia search` で keyword から候補レコードを逆引きし、そこから rules/show/decision list で正確な集合を詰める。
   ```
   scholia search <keyword> [--type tag|transition|vocab|decision] [--json]  # id 未確定なら逆引きの入口（read-only）
   scholia rules --tag <領域>          # 自身＋祖先タグへの decisions＝守るべき規則の全集合
   scholia show tag <id> / scholia show tx <id> --resolve    # 現状の契約
   scholia decision list --on tag:<id>|transition:<id>    # その対象ちょうどの決定（完全一致）
   scholia list --tag <領域>           # input が跨る transition を把握
   ```
   （`rules` は祖先展開で cross-cutting を漏らさない／`decision list --on` は完全一致。用途で使い分ける。）

2. **input を 5 分類する**——引いた記録に照らして、次のどれかに割り当てる。**分類が方針を決める。**

   | 分類 | 意味 | 方針 |
   |---|---|---|
   | **A 是正** | 実装が spec を満たしていない（バグ／gap） | 実装を spec に適合させる（**spec 変更でない**） |
   | **B 精緻化** | 過去 decision と consistent・詳細／レンズを足す | 実装＋**精緻化 decision を記録** |
   | **C 矛盾** | 過去 decision と衝突する | **止めてユーザーへ**（意識的改訂・黙って上書きしない） |
   | **D 新規要件** | 既存 spec が未カバー（net-new） | spec を追加（transition／tag／decision） |
   | **E 却下** | 検討済で却下／中核原理を壊す | 根拠 decision を挙げて却下 |

3. **方針を出力して手を止める**——次の 3 点を出し、**何も変更しない**:
   - **対応方針**（A〜E のどれ・具体的に何をするか）
   - **WHY**（根拠の decision／transition id を必ず引用。C は衝突 decision、E は却下根拠 decision）
   - **decision の下書き**（**B・D のときだけ**。`--on` 対象と `--why` 文面を実データ由来で下書き）

   渡し先: **A**→実装セッション／**B・D**→[scholia-change](../scholia-change/SKILL.md)（下書き decision を着地）／
   **C**→ユーザー／**E**→却下 decision を残す（scholia-change の reject 手順）。

## 完了条件

- 該当 spec（支配する decision／transition）を引いてから判定している（勘で分類していない）。
- input を A／B／C／D／E に分類し、**方針 ＋ WHY（decision id 引用）**を出している。
- B・D のときは **decision の下書き**を添えている（A は増やさない／C は止める／E は却下を残す）。
- **`.scholia/` も実装も変更していない**（triage は判断層・実行は渡し先）。
- C（矛盾）を独断で上書きせず、ユーザーへ判断を返している。

## 実例

「タグ往復でスクロールが戻らない」というレビュー指摘 → `scholia rules --tag <スクロール保存の領域>` で
「利用者スクロールのみ永続する」旨の decision を確認 → 実装がローディング中の clamp まで保存していた
＝ spec は正しく実装がズレている → **A 是正**（spec 変更でなく、実装を「利用者スクロールのみ保存」へ回帰）。
仕様の意図に沿った方針＋根拠 decision id を出し、実装セッションへ渡す。

## 関連

- 判定基準の本体（5 分類・進め方・下書き・大原則）は
  [`../_scholia-shared/references/evaluating-changes.md`](../_scholia-shared/references/evaluating-changes.md)。
- 方針が出た後の**実行**（`.scholia/` 変更・提案→レビュー→adopt・decision の着地）は
  [scholia-change スキル](../scholia-change/SKILL.md)。
- 日々の読み書きコマンド・判定材料の説明は [scholia スキル](../scholia/SKILL.md)（変更評価フロー節）。
- 粒度・同一性・命名の原則は [`../_scholia-shared/references/modeling-principles.md`](../_scholia-shared/references/modeling-principles.md)。
