import { render } from 'preact';
import { AppRoot } from '../root';
import type { Decision, GovernsRef, Tag, Transition, VocabEntry } from '../types';

// 描画を1回起こして「URL に書かれた条件 → 一覧に出た行」まで通すための足場。
//
// **ここは検査ではない。** 検査（何に落ちるか・落ちないか）は renderWiring.test.tsx
// が持つ。この file が持つのは3つだけ:
//
//   1. 製品が実際に叩く HTTP の口を、手元の corpus で答える偽サーバ
//   2. 製品の合成ルート（AppRoot）を happy-dom の上に起こす mount
//   3. 出た DOM から**値**（行の並び・見出しの件数・点灯タブ）を読み出す関数
//
// ⚠️ **偽サーバは Go 側の選択規則を再実装しない。** `/api/governs` が返す集合は
// corpus に**そのまま書いてある**ものを返すだけで、「この記録を支配する規則は何か」
// を viewer 側で導出しない——導出を2箇所に持たないことは 01KYKS4Y56FAHRVCWKMQJK4RT6
// 条項5 が定めた線で、テスト側にその3本目を書けば同じ線を跨ぐことになる。
// よってこの harness が守れるのは「Go が返した答えが画面の行まで届くか」であって、
// 「Go の答えが正しいか」ではない（後者は Go 側のテストが持つ）。

// ---------------------------------------------------------------------------
// corpus（この harness が答える唯一のデータ）
// ---------------------------------------------------------------------------

/** 意思決定の id。生成 id は画面に出ないので、行の同定は why 冒頭の marker
    `[D1]` で行う（=画面に出ている文字列だけで同定する）。 */
export const DEC = {
  D1: '01HARNESSD1000000000000001',
  D2: '01HARNESSD2000000000000002',
  D3: '01HARNESSD3000000000000003',
  D4: '01HARNESSD4000000000000004',
  D5: '01HARNESSD5000000000000005',
  D6: '01HARNESSD6000000000000006',
  D7: '01HARNESSD7000000000000007',
  D8: '01HARNESSD8000000000000008',
  D9: '01HARNESSD9000000000000009',
  /** **入れ子の構成要素**を対象にした規則。⚠️ これが無いと「見出しの『現行ルール N』が
      入れ子の欄の中まで数える」を落とす変異が、数える対象がゼロなので素通りする。 */
  D10: '01HARNESSD10000000000000A',
} as const;

export const TAGS: Tag[] = [
  { id: 'req.viewer', name: 'ビューア', kind: 'requirement' },
  { id: 'req.viewer.filter', name: '絞り込み', kind: 'requirement', parentIds: ['req.viewer'] },
  { id: 'req.viewer.nav', name: 'ナビ', kind: 'requirement', parentIds: ['req.viewer'] },
  { id: 'req.cli', name: 'コマンドライン', kind: 'requirement' },
  // 概要（仕様シート）の面を起こすための最小の骨格。この repo の実データには
  // 役割 kind（component/part）を持つタグが1件も無く、概要タブは**実機では空**に
  // なる——だからこそ、この面は corpus 側で起こさないと1行も検査できない。
  // ⚠️ **コンポーネントは束ねる段の下に置く。** 実データはこの形（親を持たない役割タグが
  // **束ねる段だけ**）で、corpus がコンポーネントを起点に持つ形をしていると、
  // **起点の側から束ねる段の資格だけを外す変異が素通りする**——`comp.*` が残るので
  // 「1行以上ある」を満たしてしまう。実データではその変異でツリーが**丸ごと空**になる。
  { id: 'comp.viewer', name: 'ビューア画面', kind: 'component', parentIds: ['grp.main'] },
  { id: 'part.list', name: '意思決定の一覧', kind: 'part', parentIds: ['comp.viewer'] },
  // ⚠️ **配下に構成要素を持つ構成要素（入れ子・3段）。** これが無いと、入れ子の欄を
  // 1段で止める変異・入れ子の行の指し先を落とす変異・「現行ルール N」が入れ子の中を
  // 数えない変異が、**すべて素通りする**（数える対象も描く対象も無いため）。
  // 段は3段（`part.list > part.list.row > part.list.row.cell`）——2段だと
  // 「1段だけ降りる」実装で答えが変わらない形が残る。
  { id: 'part.list.row', name: '一覧の行', kind: 'part', parentIds: ['part.list'] },
  { id: 'part.list.row.cell', name: '行の欄', kind: 'part', parentIds: ['part.list.row'] },
  // comp.viewer のもう1つの構成要素（多親「同じシートの兄弟」の相手として要る）。
  { id: 'part.detail', name: '意思決定の単票', kind: 'part', parentIds: ['comp.viewer'] },
  // 構成要素を持たず、遷移が**直接**付いているコンポーネント。実データはこの形が
  // 多数派（この repo は 58 遷移中 57 が主題タグ直付け）で、かつては仕様シートが
  // この形の遷移を1本も描かなかった。corpus に無ければその欠陥は再発しても緑のまま。
  { id: 'comp.cli', name: '端末', kind: 'component', parentIds: ['grp.main'] },
  // 入れ子のコンポーネント。⚠️ これが無いと「祖先展開込みの索引を使う」変異が
  // 素通りする——親も子も遷移1本ずつなら、祖先展開しても答えが変わらないため。
  // 親のシートに子の振る舞いが再掲される形を落とすには、この形が corpus に要る。
  { id: 'comp.cli.sub', name: '端末: 下位', kind: 'component', parentIds: ['comp.cli'] },
  // ⚠️ **コンポーネントの下のコンポーネントの、構成要素。** これが無いと「上へ辿って
  // **いちばん近い**コンポーネントを答えにする」を「**最初に見つかった**（＝いちばん
  // 外側の）コンポーネント」へ変える変異が**素通りする**（実見）——他の構成要素は
  // 上に1つしかコンポーネントが無いので、どちらでも同じ答えになるため。
  { id: 'part.sub.pane', name: '下位の面', kind: 'part', parentIds: ['comp.cli.sub'] },
  // 3枚目のコンポーネント。多親の相手として要る。
  // ⚠️ **`comp.cli` を相手に使わないこと。** `comp.cli` は「構成要素を持たない
  // コンポーネント」として直下の振る舞いの欄を守っている corpus であり、そこへ
  // 構成要素を1つ足すと**その欄が消えて、既存のガードが空振りに変わる**。
  // ⚠️ **`parentIds[0]` に役割を持たないタグを置いてある**（実データで作れた形・`lint` は通る）。
  // これが無いと、シートのパンくずを「`parentIds[0]` を素通しで遡る」形へ戻す変異が
  // **素通りする**——他のコンポーネントは `parentIds[0]` がそのまま束ねる段なので、
  // 素通しでも同じ答えになるため。実測ではこの形で、同じ画面のツリーとパンくずが
  // 違う答えを出していた。
  { id: 'comp.export', name: '書き出し', kind: 'component', parentIds: ['req.tools', 'grp.main'] },
  { id: 'part.export.job', name: '書き出しのジョブ', kind: 'part', parentIds: ['comp.export'] },
  // ---------------------------------------------------------------------------
  // 多親の4形（③B′ が守るもの）。⚠️ **corpus にこの形が無いと、③のガードは全部空振りする。**
  // ---------------------------------------------------------------------------
  // 形1: 親が**別々のコンポーネント**。→ 両方のシートに1回ずつ出る。
  //   ⚠️ この形は**是正前の実装でも既に両方から読めていた**ので、案A（記録の1つ目の親に
  //   固定）へ戻す変異は「いま読めているものを失う」。それを落とすのがこの1件の役目。
  { id: 'part.shared.probe', name: '共有: 走査', kind: 'part', parentIds: ['comp.viewer', 'comp.export'] },
  // 形2: 親が**別コンポーネントの構成要素**。→ 是正前はどのシートにも出なかった形（C2）。
  { id: 'part.shared.index', name: '共有: 索引の構築', kind: 'part', parentIds: ['part.list', 'part.export.job'] },
  // 形3: 2つの親が**同じシートの中で親子**（コンポーネントとその構成要素）。
  { id: 'part.shared.mixed', name: '共有: 混在', kind: 'part', parentIds: ['comp.viewer', 'part.list'] },
  // 形4: 2つの親が**同じシートの中で兄弟**（同じコンポーネントの2つの構成要素）。
  { id: 'part.shared.pair', name: '共有: 位置の復元', kind: 'part', parentIds: ['part.list', 'part.detail'] },
  // ⚠️ **役割を持たない種類が「子」の位置に居る形。** これが無いと、構造ツリーの
  // 子の絞りを「役割で絞る」から「要件系を除く」へ戻す変異が**素通りする**——
  // 上の要件タグはどちらの綴りでも除かれるので、答えが変わらないため。
  // 軸はどちらでもない（要件系ではないが役割も持たない）ので、綴りの違いが出る。
  //
  // ⚠️ **親は「既定で開いているノード」でなければならない。** 最初はこれを `comp.cli`
  // の子に置いたが、`comp.cli` は既定で畳まれているので子まで降りず、**変異が素通りした**
  // （実見）。既定の現在地は最初のコンポーネント＝`comp.viewer` なので、そこに置く。
  { id: 'axis.mode', name: 'モード軸', kind: 'axis', parentIds: ['comp.viewer'] },
  // ⚠️ **「子は居るが、役割を持つ子は1件も居ない構造ノード」**。これが無いと、
  // 行の指し先を決める材料を「役割を持つ子の数」から「全部の子の数」へ戻す変異が
  // 素通りする——その変異は**三角も出ず・リンクでもない行**（＝押しても何も起きない行）を
  // 作るが、それが起きるのはこの形のノードだけである。実データではこの形が4件あった
  // （要件の子だけを持つ要件タグ）。
  //
  // ⚠️ **`roots` にも入れること。** corpus の `roots` は非空なので、そこに無いタグは
  // 起点にならず**1行も描かれない**。最初はこれを入れ忘れ、変異が素通りした（実見）。
  { id: 'grp.tools', name: '道具のまとまり', kind: 'group' },
  { id: 'req.tools', name: '道具の要件', kind: 'requirement', parentIds: ['grp.tools'] },
  // コンポーネントを実際に束ねる段（実データと同じ形）。上の `grp.tools` は
  // 「子は居るが役割を持つ子は居ない」形を保つため、こちらとは別に置いてある。
  { id: 'grp.main', name: '主要なまとまり', kind: 'group' },
];

export const VOCAB: VocabEntry[] = [
  { id: 'v.open-list', category: 'action', label: '一覧をひらく', tags: ['req.viewer.filter'] },
  { id: 'v.rows-shown', category: 'effect', label: '行がならぶ', tags: [] },
  { id: 'v.run', category: 'action', label: 'コマンドを実行する', tags: [] },
  { id: 'v.printed', category: 'effect', label: '結果が印字される', tags: [] },
  { id: 'v.sub', category: 'action', label: '下位を呼ぶ', tags: [] },
  { id: 'v.boot', category: 'action', label: '画面を起こす', tags: [] },
  // ⚠️ **タグが vocab 側にしか付いていない**きっかけ。遷移の実効タグは
  // `tx.tags ∪ 参照 vocab の tags` なので、これも comp.cli の振る舞いである。
  // これが無いと「合成に vocab を渡し忘れる」変異が描画側で素通りする
  // （純関数の検査は落ちるのに配線は落ちない＝この repo が繰り返している型）。
  { id: 'v.cli-only', category: 'action', label: '語彙経由で束ねる', tags: ['comp.cli'] },
  // 入れ子・多親の欄が「直接付いた分」を出していることを、カードの中身で見分けるための語彙。
  { id: 'v.list', category: 'action', label: '一覧をならべる', tags: [] },
  { id: 'v.row', category: 'action', label: '行をえがく', tags: [] },
  { id: 'v.cell', category: 'action', label: '欄をえがく', tags: [] },
  { id: 'v.index', category: 'action', label: '索引を組む', tags: [] },
  { id: 'v.probe', category: 'action', label: '走査する', tags: [] },
];

export const TRANSITIONS: Transition[] = [
  { id: 'T-open-list', action: 'v.open-list', given: [], then: ['v.rows-shown'], tags: ['req.viewer.filter'] },
  // comp.cli に**直接**付いた遷移（構成要素を経由しない）。
  { id: 'T-run-cli', action: 'v.run', given: [], then: ['v.printed'], tags: ['comp.cli'] },
  // 子コンポーネントに付いた遷移。親のシートにこれが出たら祖先展開が混ざっている。
  { id: 'T-sub', action: 'v.sub', given: [], then: ['v.printed'], tags: ['comp.cli.sub'] },
  // ⚠️ **構成要素を持つコンポーネントに、直接付いた遷移。** これが無いと
  // 「構成要素があっても直下の欄を出す」変異が描画側では素通りする（構成要素持ちの
  // コンポーネントに直下の遷移が1本も無ければ、欄を出そうとしても空になるため）。
  { id: 'T-boot', action: 'v.boot', given: [], then: ['v.rows-shown'], tags: ['comp.viewer'] },
  // 自身にタグを持たず、参照する vocab のタグだけで comp.cli に属す遷移。
  { id: 'T-cli-vocab', action: 'v.cli-only', given: [], then: ['v.printed'] },
  // ⚠️ **入れ子の各段に、直接付いた遷移を1本ずつ置く。** これが無いと「欄に祖先展開込みの
  // 索引を使う」変異が描画側で素通りする——親の欄に配下の分が持ち上がっても、配下に
  // 遷移が無ければカードの枚数が変わらないため。段ごとに別の語彙を使うので、
  // **どの欄にどのカードが出たか**を中身で見分けられる。
  { id: 'T-list', action: 'v.list', given: [], then: ['v.rows-shown'], tags: ['part.list'] },
  { id: 'T-row', action: 'v.row', given: [], then: ['v.rows-shown'], tags: ['part.list.row'] },
  { id: 'T-cell', action: 'v.cell', given: [], then: ['v.rows-shown'], tags: ['part.list.row.cell'] },
  // 多親の構成要素に直接付いた遷移（欄が出ていることを中身で確かめるため）。
  { id: 'T-index', action: 'v.index', given: [], then: ['v.rows-shown'], tags: ['part.shared.index'] },
  { id: 'T-probe', action: 'v.probe', given: [], then: ['v.rows-shown'], tags: ['part.shared.probe'] },
];

/** getRules は時系列**昇順**で返す（一覧側が反転して新しい順に並べる）。 */
export const DECISIONS: Decision[] = [
  // ⚠️ **直下の振る舞いカードに紐づく規則。** これが無いと「見出しの『現行ルール N』に
  // 直下の分を算入する」（01KYNV5PYT6A659K8Q3X0NZ9J1 変更4・01KYHW54B8ZXH0NEPH2J7N1X39
  // 条項5）を落とす変異が、数える対象がゼロなので素通りする。
  //
  // ⚠️ 日付も並びも**既存のどれよりも古く／前に**置いてある。期間の絞り込みを見る検査
  // （「30日以内は D7 だけ」）の境界を動かさないため——corpus を足した都合で既存の
  // 検査の期待値を書き換えると、そこで何を守っていたのかが薄まる。一覧は昇順で受けて
  // 反転するので、先頭に置いた2件は並びの末尾に出る。
  { id: DEC.D8, target: { type: 'transition', id: 'T-run-cli' }, at: '2026-01-02T00:00:00Z', why: '[D8] 端末の実行は結果を必ず印字する' },
  { id: DEC.D9, target: { type: 'transition', id: 'T-cli-vocab' }, at: '2026-01-03T00:00:00Z', why: '[D9] 語彙側のタグでも同じ主題に属す' },
  // ⚠️ **入れ子の構成要素（3段目の親）を対象にした規則。** 日付は上2件と同じ理由で
  // 既存のどれよりも古く置く（期間の絞り込みを見る検査の境界を動かさない）。
  { id: DEC.D10, target: { type: 'tag', id: 'part.list.row' }, at: '2026-01-04T00:00:00Z', why: '[D10] 行は単票の中身を全部持つ' },
  { id: DEC.D1, target: { type: 'tag', id: 'req.viewer' }, at: '2026-01-05T00:00:00Z', why: '[D1] 一覧は URL に書かれた条件から起こす' },
  { id: DEC.D2, target: { type: 'tag', id: 'req.viewer.filter' }, at: '2026-02-05T00:00:00Z', why: '[D2] タグの絞り込みは AND で重ねる' },
  { id: DEC.D3, target: { type: 'transition', id: 'T-open-list' }, at: '2026-03-05T00:00:00Z', why: '[D3] 折りたたみは保存値が既定より勝つ' },
  { id: DEC.D4, target: { type: 'vocab', id: 'v.open-list' }, at: '2026-04-05T00:00:00Z', why: '[D4] 語彙はラベルで示す' },
  { id: DEC.D5, target: { type: 'tag', id: 'req.cli' }, at: '2026-05-05T00:00:00Z', why: '[D5] 端末で読む手段を残す' },
  {
    id: DEC.D6,
    target: { type: 'tag', id: 'req.cli' },
    at: '2026-06-05T00:00:00Z',
    why: '[D6] 端末の出力は現行だけに畳む',
    supersedes: [{ id: DEC.D5, mode: 'supersede' }],
  },
  // 概要の構成要素（part）に紐づく規則。これが無いと概要側の「一覧で開く」入口が
  // そもそも描かれない（renderRules は entries 空なら null を返す）。
  { id: DEC.D7, target: { type: 'tag', id: 'part.list' }, at: '2026-07-05T00:00:00Z', why: '[D7] 一覧の行が単票の中身を全部持つ' },
];

/** `/api/governs` の答え。**Go 側 GovernsFor* が返すもののスタブ**で、ここで
    導出はしない（上の ⚠️ を参照）。key は api.ts が組む record ref と同じ綴り。 */
export const GOVERNS: Record<string, GovernsRef[]> = {
  'tag:req.viewer.filter': [
    { decisionId: DEC.D1, provenance: 'parent', viaTag: 'req.viewer' },
    { decisionId: DEC.D2, provenance: 'own' },
  ],
  'tag:req.viewer': [{ decisionId: DEC.D1, provenance: 'own' }],
  'transition:T-open-list': [
    { decisionId: DEC.D1, provenance: 'parent', viaTag: 'req.viewer' },
    { decisionId: DEC.D3, provenance: 'own' },
  ],
};

/** 既定 config。**役割 behaviors を宣言していない**（＝リテラル kind id への
    フォールバック経路）ままにしてある——宣言していないプロジェクトが従来どおり
    動くことが、この corpus で常時踏まれている状態を保つため。宣言した側の経路は
    `installFakeServer({ config: ... })` で差し替えて検査する。 */
const CONFIG = {
  schemaVersion: 1,
  kinds: { action: ['action'], condition: ['condition'], effect: ['effect'] },
  tagKinds: ['requirement', 'component', 'part'] as unknown[],
  facetKinds: ['requirement'],
  // 「要件系＝葉」の宣言（概要のカバレッジがここを見る）。
  traceabilityKinds: ['requirement'],
  idPrefix: {},
  // ⚠️ 役割を持たない `req.*` を意図的に混ぜてある——**設定が起点に指定していても
  // 資格判定は効く**ことを、この corpus が常時踏む状態にするため。
  //
  // ⚠️ **役割を持つ起点は束ねる段だけにしてある。** コンポーネントを起点に置くと、
  // **起点の側から束ねる段の資格だけを外す変異**が「まだ行がある」で素通りする。
  //
  // ⚠️ **これは「実データと同じ形」ではない。** 実データは `roots` を**宣言していない**
  // （`[]`）ので、起点は「親を持たないタグ」のフォールバックで決まる。この既定 config は
  // **宣言している側の分岐しか踏まない**——フォールバック側は `NO_ROOTS_CONFIG` で別に踏む。
  roots: ['grp.main', 'grp.tools', 'req.viewer', 'req.cli'],
  viewer: {},
  tagKindLabels: { requirement: '要件', component: 'コンポーネント', part: '構成要素' } as Record<string, string>,
  display: { productName: 'scholia' },
  branch: 'harness',
  localOverride: {},
  effectiveTimezone: 'UTC',
};

export type HarnessConfig = typeof CONFIG;
export const DEFAULT_CONFIG: HarnessConfig = CONFIG;

/** `config.roots` を**宣言していない**世界。⚠️ **この repo の実データがこちら側である。**
 *
 *  構造ツリーの起点は「設定が指定した集合」と「親を持たないタグ（フォールバック）」の
 *  2分岐で決まるが、既定 corpus は `roots` が非空なので**宣言している側しか踏まない**。
 *  そのため、**フォールバック側の起点候補を痩せさせる変異が素通りする**——実測では、
 *  候補をコンポーネントだけに絞る変異で 288/288 が緑のままだった。実データでは
 *  コンポーネントは全件が束ねる段の子なので、その変異で**ツリーは0行になる**。
 *
 *  綴りを1つ塞ぐ話ではなく、**分岐の片側を1度も踏んでいない**という話なので、
 *  corpus を1つ増やして踏ませる。 */
export const NO_ROOTS_CONFIG: HarnessConfig = { ...CONFIG, roots: [] };

// ---------------------------------------------------------------------------
// 役割 component を、**フォールバックと違う id** が担う世界
// ---------------------------------------------------------------------------
//
// ⚠️ **これが無いと、シート単位の kind をリテラル `'component'` に固定する変異が
// 素通りする。** 上の既定 corpus は役割 kind の id が慣用 id とたまたま一致して
// いるので、「宣言を読んでいるか」と「リテラルを書いているか」を区別できない。
//
// `01KYCC2THS5RX3HB27SQGFWSA5` の眼目は「component 概念を**別 id で表す**
// プロジェクトでも仕様シートが出ること」であり、**この repo 自身がその経路に
// 乗っている**（`.scholia/config.json` は主題種別 `subject` に役割を宣言する）。
// つまりここは、製品の成立条件そのものを踏む corpus である。

/** 役割 component を `subject` が担うタグ集合（id は既定 corpus と同じ・kind だけ変える）。 */
export const ALIAS_TAGS: Tag[] = TAGS.map((t) => (t.kind === 'component' ? { ...t, kind: 'subject' } : t));

/** その世界の config。`component` という id はどこにも無い。 */
export const ALIAS_CONFIG: HarnessConfig = {
  ...CONFIG,
  tagKinds: ['requirement', 'part', { id: 'subject', behaviors: ['component'] }],
  tagKindLabels: { requirement: '要件', subject: 'ZZ主題ZZ', part: '構成要素' },
};

const FACETS = {
  facetKinds: ['requirement'],
  roots: [
    {
      tag: TAGS[0],
      children: [{ tag: TAGS[1] }, { tag: TAGS[2] }],
    },
    { tag: TAGS[3] },
    { tag: TAGS[4], children: [{ tag: TAGS[5] }] },
  ],
};

const EMPTY_DIFF = {
  transitions: { added: [], changed: [], removed: [] },
  vocab: { added: [], changed: [], removed: [] },
  tags: { added: [], changed: [], removed: [] },
  decisions: { added: [], changed: [], removed: [] },
};

// ---------------------------------------------------------------------------
// 偽サーバ（`scholia view` が立てている HTTP の口だけを埋める）
// ---------------------------------------------------------------------------

export interface FakeServer {
  /** 答えを持っていなかった要求。**空でなければ harness が製品の経路を1つ
      取りこぼしている**（新しい fetch が足されたのに harness が追随していない）。 */
  unhandled: string[];
  /** 実際に来た要求（順序つき）。 */
  requests: string[];
  /** `hold` で止めてある応答を、テストが選んだ時点で返す。 */
  release: () => void;
  restore: () => void;
}

function body(path: string, params: URLSearchParams, opts: ServerOptions): unknown | undefined {
  switch (path) {
    case '/api/config':
      return opts.config ?? CONFIG;
    case '/api/facets':
      return FACETS;
    case '/api/tags':
      return opts.tags ?? TAGS;
    case '/api/vocab':
      return VOCAB;
    case '/api/transitions':
      return { transitions: TRANSITIONS };
    case '/api/rules': {
      // selector 付き（コメント欄がレコード単位で引く形）も、この corpus では
      // 対象一致で答えられる。
      const tag = params.get('tag');
      const tx = params.get('tx');
      if (tag) return { decisions: DECISIONS.filter((d) => d.target.type === 'tag' && d.target.id === tag) };
      if (tx) return { decisions: DECISIONS.filter((d) => d.target.type === 'transition' && d.target.id === tx) };
      return { decisions: DECISIONS };
    }
    case '/api/governs': {
      const key = params.get('tag')
        ? `tag:${params.get('tag')}`
        : params.get('tx')
          ? `transition:${params.get('tx')}`
          : params.get('vocab')
            ? `vocab:${params.get('vocab')}`
            : '';
      return { entries: GOVERNS[key] ?? [] };
    }
    case '/api/traceability':
      return { kinds: ['requirement'], entries: [] };
    case '/api/reviews':
      return [];
    case '/api/diff':
      return EMPTY_DIFF;
    case '/api/lint':
      return { findings: [] };
    case '/api/search':
      return { transitions: [], matchedOn: {}, records: [] };
  }
  if (path.startsWith('/api/transitions/')) {
    const id = decodeURIComponent(path.slice('/api/transitions/'.length));
    const tx = TRANSITIONS.find((t) => t.id === id);
    return tx ? { ...tx, rules: DECISIONS.filter((d) => d.target.type === 'transition' && d.target.id === id) } : undefined;
  }
  return undefined;
}

/**
 * `hold` に挙げた path の応答を、`release()` が呼ばれるまで**返さずに握る**。
 *
 * 実サーバは即答しない。「まだ飛んでいる要求がある最中に利用者が widget を触り、
 * その後に応答が返る」——実運用では当たり前に起きるこの順序を、**待ち時間ではなく
 * 順序として**再現するための口である（時間で待つ形にすると、速い機械では窓が
 * 閉じてしまい、遅い機械でだけ落ちる検査になる＝この単位で実際に起きたこと）。
 */
export interface ServerOptions {
  hold?: string[];
  /** `/api/config` の答えを差し替える（役割 behaviors を宣言した／していない
      プロジェクトの両方を、同じ製品コードに食わせるため）。 */
  config?: HarnessConfig;
  /** `/api/tags` の答えを差し替える（「役割は宣言したが、その種別のタグがまだ
      1件も無い」状態を作るため）。 */
  tags?: Tag[];
}

export function installFakeServer(opts: ServerOptions = {}): FakeServer {
  const original = globalThis.fetch;
  const held: Array<() => void> = [];
  const server: FakeServer = {
    unhandled: [],
    requests: [],
    release: () => {
      while (held.length) held.shift()!();
    },
    restore: () => {
      globalThis.fetch = original;
    },
  };
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
    const url = new URL(raw, 'http://harness.local');
    server.requests.push(url.pathname + url.search);
    if (opts.hold?.includes(url.pathname)) await new Promise<void>((r) => held.push(r));
    const payload = body(url.pathname, url.searchParams, opts);
    if (payload === undefined) {
      server.unhandled.push(url.pathname + url.search);
      return { ok: false, status: 404, statusText: `harness has no answer for ${url.pathname}`, json: async () => ({}) } as Response;
    }
    return { ok: true, status: 200, statusText: 'OK', json: async () => payload } as Response;
  }) as typeof fetch;
  return server;
}

// ---------------------------------------------------------------------------
// mount（製品の合成ルートを1つ起こす）
// ---------------------------------------------------------------------------

export interface Mounted {
  host: HTMLElement;
  unmount: () => void;
}

/** URL（hash）を先に立ててから、**製品と同じ合成ルート**（root.tsx の AppRoot）を
    起こす。順序が逆だと useHashRoute の初期値が既定ルートになり、「URL に書かれた
    条件から起こす」という検査の前提そのものが消える。 */
export function mountApp(hash: string): Mounted {
  window.location.hash = hash;
  const host = document.createElement('div');
  document.body.appendChild(host);
  render(<AppRoot />, host);
  return {
    host,
    unmount: () => {
      render(null, host);
      host.remove();
    },
  };
}

/** テスト間で持ち越すと結果が順序依存になるもの（保存された折りたたみ・言語・
    スクロール位置・hash）を落とす。 */
export function resetBrowserState(): void {
  try {
    localStorage.clear();
    sessionStorage.clear();
  } catch {
    // happy-dom では起きないが、実装に合わせて握る（settings.ts 等と同じ流儀）。
  }
  window.location.hash = '';
}

// ---------------------------------------------------------------------------
// 出た DOM から値を読む
// ---------------------------------------------------------------------------

export async function waitFor(check: () => boolean, what: string, timeoutMs = 3000): Promise<void> {
  // ⚠️ 時計は `performance.now()` を使う。`Date` は期間の絞り込みを検査するときに
  // 固定する（vi.setSystemTime）ので、`Date.now()` で測ると**待ち時間が永遠に
  // 経過しない**ことになる。
  const started = performance.now();
  for (;;) {
    if (check()) return;
    if (performance.now() - started > timeoutMs) throw new Error(`timed out waiting for: ${what}`);
    await new Promise((r) => setTimeout(r, 10));
  }
}

/** 一覧に出ている行を、上から順に marker（`[D3]` → `D3`）で返す。 */
export function rowMarkers(host: HTMLElement): string[] {
  return Array.from(host.querySelectorAll('.decisions-list > li')).map((li) => {
    const text = li.textContent || '';
    const m = /\[(D\d+)\]/.exec(text);
    return m ? m[1] : `?<${text.slice(0, 24)}>`;
  });
}

/** 見出しに出ている件数（`3 件` → 3）。文言に依らず数だけ読む。 */
export function headingCount(host: HTMLElement): number | null {
  const el = host.querySelector('.decisions-count');
  const m = /\d+/.exec(el?.textContent || '');
  return m ? Number(m[0]) : null;
}

/** 点灯しているナビタブのラベル（複数点いていたら複数返る＝検査で落ちる）。 */
export function activeNavLabels(host: HTMLElement): string[] {
  return Array.from(host.querySelectorAll('.topbar-nav-btn.active')).map((b) => (b.textContent || '').trim());
}

/** 左レールの絞り込み `<select>`（対象種別・現行性・期間）。 */
export function railSelects(host: HTMLElement): HTMLSelectElement[] {
  return Array.from(host.querySelectorAll<HTMLSelectElement>('.decisions-rail-filters select'));
}

/** `<select>` を人が選んだときと同じ形で変える（値を入れて change を配る）。 */
export function selectValue(el: HTMLSelectElement, value: string): void {
  el.value = value;
  el.dispatchEvent(new Event('change', { bubbles: true }));
}

/** 検索欄に打つ（focus してから input を配る＝候補が開く条件を満たす）。 */
export function typeQuery(host: HTMLElement, text: string): void {
  const input = host.querySelector<HTMLInputElement>('.browse-rail-search');
  if (!input) throw new Error('検索欄が見つからない');
  input.dispatchEvent(new Event('focus', { bubbles: true }));
  input.value = text;
  input.dispatchEvent(new Event('input', { bubbles: true }));
}
