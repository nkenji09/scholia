import type { TagSource } from './types';

// UI chrome copy (headings, buttons, empty states, labels) for BOTH
// languages the viewer supports. Data values (vocab label / tag name·
// description / decision text / requirement prose) never come from here —
// they're rendered as stored in `.scholia/`, unmodified regardless of
// language. `ja`/`en` below must share the exact same shape (`en` is typed
// as `Strings = typeof ja`) so a missing translation is a compile error,
// not a silent fallback. i18n.tsx's `useT()` picks the active one; `DICTS`
// is also read directly (via `loadLang()`) from non-component code (api.ts)
// that can't call a hook.

// action/condition/effect are a fixed 3-axis schema (not user-configurable
// like tagKinds), so their display labels are plain frontend constants —
// no config plumbing (2026-07-11 tweaks3 §1). Shared between flow.* (a
// transition's きっかけ/前提/結果 sections) and vocab.categoryLabel (Vocab's
// category facet/badges) so both read the same vocabulary rather than
// drifting into two translations of the same three concepts.
const FLOW_TRIGGER_JA = 'きっかけ';
const FLOW_GIVEN_JA = '前提';
const FLOW_RESULT_JA = '結果';
const FLOW_TRIGGER_EN = 'Trigger';
const FLOW_GIVEN_EN = 'Given';
const FLOW_RESULT_EN = 'Result';

const ja = {
  // レビュー差し戻し MAJOR-1: ナビは概要/タグ/仕様(デザイン正本 navItems)＋
  // 外挿1画面(語彙)＋設定、全て日本語ラベルに統一。'spec'（旧・独立タブ）は
  // ここに含めない — router.ts の #/spec/<tag> は引き続き解決するが、
  // 'tags' タブと同一facetのため独立ボタンにはしない（重複ナビの解消）。
  // トレーサビリティ/比較はデザイン未対応のため2026-07-11にナビから削除
  // （ユーザー指示、Header.tsx参照）。表示順は 概要/語彙/タグ/仕様
  // （2026-07-11 tweaks2 のユーザー視覚FBで語彙を概要の直後へ移動、
  // Header.tsx の NAV 配列参照）。
  nav: {
    // IA-rework (viewer-overview-browser): ナビは「概要」と「ブラウザ」の2つに
    // 畳む。旧タブ（home/tags/specs/vocab/flow）のラベルは他コードが参照しうる
    // ため残すが、Header はもう overview/browse/config しか描画しない。
    overview: '概要',
    browse: 'ブラウザ',
    home: '概要',
    tags: 'タグ',
    specs: '仕様',
    vocab: '語彙',
    flow: 'フロー',
    config: '設定',
  },
  // 概要ビュー（viewer-overview-browser）: 左=構造ツリー、右=コンポーネント仕様
  // シート。データは実 api（tags/transitions/vocab/traceability/rules/config）
  // から導出。ラベルは lookups（vocabLabel/tagName/tagKindLabel）経由で、ここは
  // chrome 文言のみ。
  overview: {
    loading: '読み込み中…',
    treeHeading: '構造',
    // シートに出すコンポーネントが選ばれていない（＝プロジェクトに component
    // 種別のタグが無い旧スキーマ等）ときの空状態。
    selectPrompt: '左の構造ツリーからコンポーネントを選ぶと、仕様シートが表示されます。',
    noComponents: 'このプロジェクトには「コンポーネント」種別のタグがありません。ブラウザから全レコードを横断できます。',
    openInBrowser: 'ブラウザで開く',
    coverageHeading: 'カバレッジ',
    coverageSuffix: 'の要件が仕様で充足',
    coverageNone: '対象要件なし',
    partCount: (n: number) => `構成要素 ${n}`,
    ruleCount: (n: number) => `現行ルール ${n}`,
    gapCount: (n: number) => `未充足 ${n}`,
    responsibilityHeading: '責務',
    behaviorsHeading: '構成要素ごとの振る舞い',
    behaviorsHint: 'きっかけ → 前提 → 結果',
    unconditional: '無条件',
    satisfiesReqs: '満たす要件',
    txCount: (n: number) => `${n} 遷移`,
    constraintsHeading: '「〜しない」制約',
    constraintsHint: '性質型要件 property',
    readFull: '全文を読む',
    backToSummary: '要約に戻す',
    // decision を関連レコードの文脈にインライン展開するトグル（⑤）。N は当該レコード
    // を target とする decision 総数（現行＋失効）。
    rulesToggle: (n: number) => `規則 (${n})`,
    componentRulesToggle: (n: number) => `このコンポーネントの規則 (${n})`,
    // decision の出自ラベル（決定がこのコンポーネントに効く経路）。component 本体の
    // 規則展開でのみ使う（tx/part/制約の展開は target 直下なので via を出さない）。
    // viaSpec は part 配下に現れない直属 transition の decision を本体へ回収する際の
    // ラベル（取りこぼし防止）。
    viaComponent: 'このコンポーネントに直接',
    viaSpec: 'この仕様に直接',
    viaTag: (name: string) => `タグ〈${name}〉経由`,
  },
  header: {
    fontDec: '文字を小さく',
    fontInc: '文字を大きく',
    themeToggle: 'テーマ切替',
    // narrow viewport only, on screens with a BrowseRail (2026-07-11 レスポンシブ対応)。
    filterToggle: '絞り込み',
    fontScaleGroupLabel: '文字サイズ',
    commentList: 'コメント一覧',
    langToggle: '表示言語を切り替え',
  },
  home: {
    tagline: '記録を、読みたくなる形で。',
    intro:
      'scholia は、プロダクトの意思決定・要件・振る舞いを原子（遷移）として記録し、構造は派生（query）で見るためのツールです。',
    tagCount: (n: number) => `${n} 件`,
    traceabilityHeading: '要件トレーサビリティ',
    goTraceability: '要件を読む',
    satisfiedOf: (satisfied: number, total: number) => `${satisfied} / ${total}`,
    satisfiedSuffix: 'の要件が仕様で充足',
    gapHeading: (n: number) => `未充足（gap）${n} 件 — まだ仕様が紐づかない要件`,
    noGap: 'すべての要件が仕様で充足しています',
    recentDecisionsHeading: '直近の意思決定',
    noDecisions: 'まだ意思決定が記録されていません',
    loading: 'loading…',
  },
  vocab: {
    heading: '語彙',
    intro: '仕様を形作る、言葉の定義',
    owner: 'owner',
    usageCount: (n: number) => `${n} 件の遷移で使用`,
    usageHeading: '使用箇所',
    noUsage: 'どの遷移からも参照されていません',
    empty: '該当する語彙はありません',
    loading: 'loading…',
    // #45 D5 の関係スロット表示。
    refHeading: '外部契約',
    altLabelsHeading: '別表記',
    establishesHeading: '成立させる条件',
    establishedByHeading: '成立させる効果',
    decisionsHeading: '意思決定',
    // 2026-07-11 tweaks3 §1: 遷移の きっかけ/前提/結果 と同じ語彙に統一
    // （grammar色 --t-act/--t-giv/--t-then とも対応）。VocabEntry.category は
    // Go側では string（3軸固定の想定値だが型では絞られていない）なので、
    // 未知の値は素の文字列にフォールバックする。
    categoryLabel: (c: string): string =>
      ({ action: FLOW_TRIGGER_JA, condition: FLOW_GIVEN_JA, effect: FLOW_RESULT_JA } as Record<string, string>)[c] || c,
    // 語彙ドロワーの索引ツリー（vocab-view-p1）: category→kind の第2階層で
    // kind 未設定の vocab を落とす末尾バケットのラベル。
    otherKind: 'その他',
    // コンポ別モード（vocab-view-p2 → combobox-unify）: コンボボックスで
    // コンポーネント（facetKinds タグ）を選ぶと、その subject に属す遷移が参照する
    // 語彙だけを category→kind ツリーで表示する。サジェストの種別バッジ／絞り込み
    // チップの接頭ラベルは、選択タグの kind を tagKindLabels で解決する
    // （lookups.tagKindLabel）— 単一の「コンポーネント」ハードコードは廃止済み。
    subjectEmpty: (name: string) => `${name} に属す遷移が参照する語彙はありません`,
    // 索引の表示モード切替（vocab-tree-mode）: モードA=category×kind の分類ツリー、
    // モードB=消費 transition 文脈ツリー。索引ヘッダ右端のトグルのラベル。
    treeModeCategory: '分類',
    treeModeTransition: '文脈',
    // モードB でどの遷移にも消費されない vocab を集約する末尾バケットのラベル。
    unusedBucket: '未使用',
  },
  // WHEN/GIVEN/THEN の言い換え（調整4）。遷移カード全般（一覧・詳細・spec）で共通利用。
  flow: {
    trigger: FLOW_TRIGGER_JA,
    given: FLOW_GIVEN_JA,
    result: FLOW_RESULT_JA,
    noResult: '（結果なし）',
    noGiven: '無条件（前提なし）',
    // action の ⋮ メニュー項目（tx.viewer.action-flow-link）。別タブでフロー図を開く。
    menuShowFlow: 'フロー図を表示',
    // #/flow/<action> ビュー（tx.viewer.action-flow-render）。マトリクス／
    // scope-disclosure のテキスト節は viewer から削除済み（decision
    // 01KXN6G0R4DSXEVV86K8W0CZYW・#39 フォローアップ）— 同じ情報は
    // `scholia flow`/`scholia gaps` で引き続き見られる。図がこのビューの唯一の
    // コンテンツ。
    viewTitle: (label: string) => `${label} のフロー`,
    loading: '読み込み中…',
    emptyAction: 'この action を持つ遷移はありません。',
    // #/flow インデックス（tx.viewer.flow-nav-tab）: nav の「フロー」タブから
    // action 一覧を出し、選ぶと #/flow/<action> へ。
    indexTitle: 'フロー',
    indexIntro: 'action を選ぶと、その分岐（条件×遷移）のフロー図が開きます。',
    indexEmpty: 'フロー図を持つ action がありません。',
    // 絞り込み結果が空（viewer-search-consistency）。action は在るが条件に一致しない。
    indexNoMatch: '条件に一致する action がありません。',
    // 見出し脇の絞り込み後 action 件数（viewer-search-consistency）。
    indexCount: (n: number) => `${n} 件`,
    indexSearchPlaceholder: 'action を絞り込む…',
    indexTxCount: (n: number) => `${n} 遷移`,
    diagramError: '図の描画に失敗しました。',
    // 凡例。矢印は同じ意味の繰り返しラベルを持たせず、ここで一括して説明する。
    legendClickable: '結果（クリックで遷移詳細へ）',
    zoomIn: '拡大',
    zoomOut: '縮小',
    zoomReset: 'リセット',
    // 点線の辺自体に付くラベル（buildDiagram 参照）。結果ノードは色を持たず
    // 辺のラベルのみで関係を表現する（両方の結果とも正常な遷移で、赤枠に
    // すると「壊れている」に見えるとの指摘を反映）。優先順位を定義する
    // 仕組みは .scholia に存在しないため「優先順位未定義」という、決められるのに
    // 決めていないかのような表現は使わない。
    coOccur: '同時に発生',
    legendSubsetShadow: '点線(片矢印)＝同時に発生する組み合わせ',
    // 抜け（total-gap）マーカー。この前提を明示的に持つ遷移が1つもない、
    // 決定木の本来あるべき枝に配置し、該当する条件の語彙ページへリンクする
    // （遷移が存在しないので遷移詳細へはリンクできない）。
    gapLabel: '未定義',
    legendGap: '赤＝未定義（この前提を明示的に持つ遷移がない）',
    // scope-honesty（req.action-flow.scope-honesty）を viewer 側で果たす
    // 一行 caveat（レビュー MAJOR-A 対応）。CLI（`scholia flow`/`scholia gaps`）の
    // フル scope-disclosure ほど詳しくする必要はなく、「宣言軸＝完全な区別
    // 集合」と読者に誤読させない最小限の注意書きで足りる、というのが
    // decision の骨子（why は tx.viewer.action-flow-render 側に記録）。
    scopeCaveat: '※ この図は宣言された状態次元（axis kind の軸）のみに基づく整理です。評価順は宣言された priority によるもので実装との一致は未検証。網羅は保証しません（全量は scholia flow）',
  },
  // BROWSE(タグ/仕様) — 旧 Browse(3ペイン)/TagsView(ツリー)/SpecView を検索
  // レール＋カード一覧の1つの型に統合した画面（.concierge/decision.md A-2）。
  browse: {
    searchPlaceholder: 'フリーワード・タグ検索',
    kindAll: 'すべて',
    conditionsHeading: '絞り込み条件',
    and: 'AND',
    clear: 'クリア',
    indexHeading: '見出し',
    indexEmpty: '該当なし',
    indexExpand: '展開',
    indexCollapse: '折りたたむ',
    uncategorized: '未分類',
    tagsTitle: 'タグ',
    tagsSubtitle: 'どの観点でまとめるか',
    specsTitle: '仕様',
    specsSubtitle: '意思決定の上にある、正しい動作の拠り所',
    empty: '条件に一致する項目がありません',
    loading: 'loading…',
    satisfiedSpecs: '関連仕様',
    relatedVocab: '関連語彙',
    relatedDecisions: '意思決定',
    childTags: '下位のタグ',
    gapBadge: '未充足',
    satBadge: (n: number) => `${n} 仕様`,
    showDetail: '詳細を見る',
    hideDetail: '詳細を閉じる',
    rulesHeading: '意思決定',
    tagsHeading: 'タグ',
    derivedHeading: '継承タグ',
    derivedHint: 'vocab継承＋親タグ展開の実効タグ',
    clickToFilter: 'クリックで検索条件に追加',
    // ↗ 詳細リンク（card-detail-link）。filter（⊕）とは別の専用アンカーで、
    // 平打ち=SPA遷移／Cmd・Ctrl・中クリック=別タブ／右クリック=リンクコピー。
    openDetail: '詳細を開く（Cmd/Ctrl+クリックで別タブ）',
    // ⋮ アフォーダンスメニュー（card-affordance-menu）: 各タグ/語彙の末尾の
    // 3点メニュー。トリガの aria-label と2項目（フィルタ追加／別タブで詳細）。
    menuTrigger: 'この項目の操作',
    menuAddFilter: '検索条件に追加',
    menuOpenLink: 'リンク先を開く（別タブ）',
    // 実効タグの由来ラベル（gap G11）。own/vocab/ancestor は複数同時成立しうる
    // ので順に連結する — バックエンドの EffectiveTag.sources をそのまま表示
    // するだけで、フロントは由来を再計算しない（§9）。
    provenanceSourceLabel: { own: '直接付与', vocab: 'vocab由来', ancestor: '祖先由来' } as Record<TagSource, string>,
    provenanceLabel: (sources: TagSource[]): string => sources.map((s) => ja.browse.provenanceSourceLabel[s]).join(' + '),
    fetchWarning: (n: number) => `${n} 件の読み込みに失敗しました（表示されているカードは正常です。再読み込みで再試行できます）`,
    parentLinkTitle: '親タグのカードへ移動',
    childLinkTitle: 'このカードへ移動',
    kindHeading: '種別',
    // 継承した規則の開示（01KYHW4NBNVN9BFXYZMBX8MPF8 条項3）。「この記録を
    // 支配する規則」欄（本文を全文で再掲していた）を廃止した代わりに、件数と
    // 継承元と導線だけを出す。件数は**効いている規則の数**。
    inheritedFromAncestors: (n: number) => `上位から継承した規則 ${n}件`,
    inheritedFromTags: (n: number) => `タグから継承した規則 ${n}件`,
    inheritedSourceTitle: '継承元の記録を開く',
    // 条項5 の入口。exact はタグのカード（一覧の絞り込み単位と一致する）、
    // scoped は transition / vocab（絞り込みに使うタグ名を必ず名乗る）。
    rulesListLinkExact: 'この記録に効く規則を一覧で見る',
    rulesListLinkScoped: (tag: string) => `〈${tag}〉の規則を一覧で見る`,
    rulesListLinkTitle: '意思決定の一覧を対象で絞って開く',
    // 軸カードの構造表示（#45 D10b-6）。状態次元・total（宣言由来・非検証）・値・効く action。
    axisStructureHeading: '軸の構造',
    // 「軸」= 状態次元（分岐を束ねる分類軸）。kind バッジの「軸」とは別義（D9 の
    // kind description で説明）だが、この見出しは分類軸そのものを指す。
    axisDimensionBadge: '状態次元',
    axisTotalTrue: '網羅宣言あり（宣言による・非検証）',
    axisTotalFalse: '網羅宣言なし',
    axisValuesHeading: '値',
    axisValueActions: '効いている action',
    axisNoValues: '軸タグ付きの値（condition）がまだありません',
  },
  // 複数画面で同じ語を使う汎用ボタン/操作ラベル（保存・キャンセル等）。
  common: {
    save: '保存',
    cancel: 'キャンセル',
    delete: '削除',
    close: '閉じる',
    remove: '除去',
    edit: '編集',
  },
  // コメント機能（#18・2026-07-11 コメント拡張4件）全体のchrome文言。
  // ユーザーが書いたコメント本文/返信本文自体はデータなので、ここには
  // 含めない（copy*系はテンプレ文言のみ、text/{...}は呼び出し側が埋める）。
  comments: {
    cardAnchorLabel: 'カード全体',
    descriptionAnchorLabel: '説明',
    pageAnchorLabel: 'ページ全体',
    addHere: 'この箇所にコメント',
    recordType: { tag: 'タグ', transition: '仕様', vocab: '語彙', page: 'ページ' },
    panelTitle: 'コメント',
    copied: 'コピーしました',
    copyAllTitle: 'AI が修正するための情報をまとめてコピー',
    copyAll: 'コピー',
    composerPlaceholder: 'コメントを入力…（このカードのこの箇所について）',
    submitHintMac: '⌘+Enter で投稿',
    submitHintOther: 'Ctrl+Enter で投稿',
    emptyLine1: 'まだコメントはありません。',
    emptyLine2Before: '各カードの見出し横の',
    emptyLine2After: 'から追加できます。',
    replyPlaceholder: '返信を追加…',
    replyDelete: '返信を削除',
    replyAdd: '返信',
    gotoLocation: '位置へ移動',
    copyDocTitle: '# scholia ビューア — レビューコメント',
    copyTaskLine: (title: string) => `タスク: ${title}`,
    copyIntro: (n: number) =>
      `以下の ${n} 件のコメントに基づき、該当箇所を修正してください（[ページ] は特定のカードに紐づかない、そのビュー全体への指摘です）。`,
    copyItemHeader: (i: number, typeLabel: string, recordId: string, title: string) => `${i}. [${typeLabel}] ${recordId} 「${title}」`,
    copyLocationLine: (anchorLabel: string) => `   箇所: ${anchorLabel}`,
    copyCommentLine: (text: string) => `   コメント: ${text}`,
    copyReplyHeading: '   返信スレッド:',
    // #27 Phase2-2b: task セレクタ（コメントを task 単位で束ねる・設計 §0/§3）。
    taskDefaultTitle: '未整理',
    taskLabel: 'タスク',
    taskNew: '新規タスク',
    taskNewTitle: '新しいタスクを作成',
    taskNewPlaceholder: 'タスク名を入力…',
    // #27 P2: 現状 vs 提案（pending diff）の read-only カード。
    proposalHeading: '提案',
    proposalUncommitted: '未コミット',
    proposalUnavailableError: '提案の差分を取得できませんでした',
    // #27 P2′-rework（change-cockpit-design-v3.md §8）: 提案＝変更を持つ
    // レコードのコメント（別 why 欄は無い）。SpecCard のこのフラグは
    // 「変更はあるがまだコメントが無い」状態だけに出る控えめな pending 表示。
    proposalCleanFlag: '変更あり（未コメント・→ドロワー）',
    proposalWhatLabel: '提案の差分表示',
    // #27 P5a（change-cockpit-design-v3.md §3/§8.8「3種別の表し方」）: 追加
    // ＝subject の仕様一覧に出る緑カードのバッジ／削除＝メイン一覧に残る
    // tombstone カードの文言。
    proposalAddedBadge: '新規 仕様（提案）',
    tombstoneBadge: '削除（提案）',
    tombstoneRestoreButton: '削除を取り消す（再作成）',
    tombstoneRestoring: '取り消し中…',
    tombstoneRestoreError: (msg: string) => `取り消せませんでした: ${msg}`,
    newTransitionButton: '新規 仕様を提案',
    newTransitionActionUnset: '（未選択）',
    newTransitionIdLabel: 'id（新規識別子）',
    newTransitionIdPlaceholder: '例: tx.lint.check',
    newTransitionIdDuplicate: (id: string) => `id "${id}" は既に存在します。別の id を指定してください`,
    newTransitionCreateButton: '作成',
    newTransitionCreating: '作成中…',
    newTransitionCancel: '閉じる',
    newTransitionCreateError: (msg: string) => `作成できませんでした: ${msg}`,
    deleteProposalButton: 'この 仕様を削除（提案）',
    deleteProposalConfirmLabel: '本当に削除しますか？（作業ツリーから未コミットで除去します）',
    deleteProposalConfirmButton: '削除する',
    deleteProposalDeleting: '削除中…',
    deleteProposalCancel: 'キャンセル',
    deleteProposalError: (msg: string) => `削除できませんでした: ${msg}`,
    // AI配送（change-cockpit-design-v3.md §8.4）: GET /api/reviews から合流
    // した AI コメント（read-only・編集/削除/返信不可）。
    aiBadge: 'AI',
    aiReadonlyNote: 'AI が書いたコメントです（編集・削除・返信はできません）',
    // 採用（change-cockpit-design-v3.md §8.5・P4）: 提案コメント→
    // decision.why への昇格（POST /api/decision・server-mode 限定）。
    adoptButton: '採用',
    adoptWhyLabel: '確定する why（decision として記録されます）',
    adoptConfirm: '採用を確定',
    adoptedBadge: '採用済み',
    adoptedWhyHeading: '採用された why（decision）',
    adoptedNote: 'この提案は decision として記録されました（commits[] は空）。commit 後は `scholia decision add-commit <id> <hash>` で結線してください。',
    // 結線の確認（adopt が supersedes まで束ねる要件・01KYHE08WNA8H1Q1DM2H45Y4TK）:
    // 採用と同時に張る現行性リンクを、Adopt を押す前に見せて一緒に承認させる。
    supersedeHeading: '採用と同時に張る現行性リンク',
    // 戻り値を string に固定する（リテラル union に推論されると en 辞書の
    // 同名関数が代入不能になる — 辞書は ja を型の正本にしている）。
    supersedeModeLabel: (mode: string): string =>
      mode === 'supersede' ? '全文置換（旧を失効させる）' : mode === 'exception' ? '意識的例外（旧は生きたまま）' : '部分改訂（旧は生きたまま）',
    supersedeMissing: 'この意思決定は見つかりません（採用時にエラーになります）',
    supersedeUnknownTarget: '対象不明の意思決定',
    supersedeNoneWithPrior: (n: number) =>
      `置き換えの宣言はありません。このレコードには既に意思決定が ${n} 件あります——旧を改訂する提案なら、結線しないと現行規則の導出が旧のまま残ります。`,
    // 採用が結線の検証で失敗したときの文言（01KYCC2TF3NW3JRSSRK9ZHN078:
    // viewer は生レコード id を表示しない）。サーバは code だけを返し、
    // 読ませる文言はここが持つ——上の確認ブロックと同じ語彙に揃える。
    supersedeErrorMissingTarget: 'この意思決定は見つかりません。提案が宣言した置き換え先が既に消えています。',
    supersedeErrorInvalidMode: '置き換えの種別が不正です（全文置換・部分改訂・意識的例外のいずれかである必要があります）。',
    supersedeErrorDuplicate: '同じ意思決定が二重に指定されています。',
    supersedeErrorSelfReference: '意思決定は自分自身を置き換えられません。',
    supersedeErrorEmptyId: '置き換え対象が指定されていません。',
    supersedeErrorModeRewrite: '宣言済みの置き換えの種別は変更できません（リンクは追記のみ）。',
    // 昇格元コメントの掃除（DELETE /api/reviews/{id}）の失敗。review の id も
    // ULID なので、サーバの文言をそのまま出さずここが持つ。
    reviewErrorNotFound: 'この提案コメントは見つかりません（既に削除されています）。',
    reviewErrorInvalidId: '提案コメントの指定が不正です。',
    // 却下（#35・tx.review.reject/tx.comment.reject）— 採用と対称の束ね操作。
    // decision として記録した上で昇格元コメントを削除する点は採用と同じ。
    rejectButton: '却下',
    rejectWhyLabel: '却下理由（decision として記録されます）',
    rejectConfirm: '却下を確定',
    rejectWhyDraft: (text: string) => `却下: ${text}`,
    // #27 P3: 語彙ピッカー（change-cockpit-design-v3.md §3/§1 (Wp)・G-1′）。
    // 既存 vocab/tag から選ぶだけ — 自由記述の入力欄は無い（構造ガード）。
    pickerAddButton: '語彙を選ぶ',
    pickerSearchPlaceholder: '語彙を検索…',
    pickerEmpty: '候補がありません',
    pickerRemoveTitle: '除去',
    pickerMoveUpTitle: '上へ',
    pickerMoveDownTitle: '下へ',
    reflectButton: 'この手直しを提案に反映',
    reflecting: '反映中…',
    reflectError: (msg: string) => `反映できませんでした: ${msg}`,
    // #27 §8.8 P5 vocab/tag: transition の ProposalCard は語彙ピッカーで
    // 手直しできる（G-1′）が、vocab/tag は書込エンドポイントが無い
    // read-only の before/after 表示のみ（RecordDiffCard.tsx）。
    recordDiffLabelField: 'ラベル',
    recordDiffKindField: '種別',
    recordDiffDescriptionField: '説明',
    recordDiffNameField: '名前',
    recordDiffParentsField: '親タグ',
    recordDiffNoParents: '（親タグなし）',
  },
  // #/decisions 一覧・#/decision/<ulid> 詳細（D10a・tx.viewer.decision-list /
  // tx.viewer.decision-detail）。意思決定を独立の read 面として通覧・パーマ
  // リンクで共有する。ラベル解決（target/tagName 等）は lookups 経由、
  // why/changed 本文はデータなのでここには含めない。
  decisions: {
    heading: '意思決定',
    intro: 'いつ・何を・なぜ変えたかの記録。既定では効いている規則だけを出し、置き換え済みは絞り込みで切り替えます。',
    loading: 'loading…',
    empty: 'まだ意思決定が記録されていません',
    noMatch: '条件に一致する意思決定がありません',
    backToList: '一覧へ戻る',
    // フリーワード検索（why/changed/target/acknowledges を対象）。
    searchPlaceholder: 'why・変更内容で検索',
    // フィルタ群。
    filterTargetKind: '対象種別',
    filterTag: 'タグ',
    filterCurrency: '現行性',
    filterAll: 'すべて',
    targetKindTransition: '仕様（遷移）',
    targetKindTag: 'タグ',
    targetKindVocab: '語彙',
    // 期間（簡易・直近フィルタ）。
    filterPeriod: '期間',
    periodAll: '全期間',
    period30d: '直近30日',
    period90d: '直近90日',
    period1y: '直近1年',
    // 現行性バッジ／フィルタ値。「失効」= 他 decision が supersede でこれを
    // 置き換えた。「改訂」= 他 decision が amend/exception で参照（置換はしない）。
    currencyCurrent: '現行',
    currencySuperseded: '失効',
    // 画面に出す効力は2値（01KYHW54B8ZXH0NEPH2J7N1X39 条項1・3）。記録の3値
    // （supersede/amend/exception）は不変で、状態列だけを2値にする。
    // 「失効」ではなく「置き換え済み」なのは、効いていない**理由**が語から
    // 読めるようにするため（条項3）——「失効」は期限切れに読める。
    effectInForce: '現行',
    effectReplaced: '置き換え済み',
    // 付帯情報（条項2）: 後続に部分改訂・例外が付いている。読み飛ばしてよいと
    // 解釈できる語（補足・関連）は使わず、とるべき行動を述べる。
    readTogether: (n: number) => `併せて読む意思決定 ${n}件`,
    openReplacement: '置き換えた規則を見る',
    // 置き換え済みを畳む口（条項4）。中身が「置き換えられたもの」だと分かる語。
    replacedHeading: (n: number) => `置き換えられた規則 (${n})`,
    // 詳細ページの節見出し。
    whyHeading: 'なぜ',
    changedHeading: '変更内容',
    targetHeading: '対象',
    commitsHeading: 'commit',
    refHeading: '参照',
    acknowledgesHeading: '容認する finding',
    supersedesHeading: '置き換え/改訂する意思決定',
    supersededByHeading: 'この意思決定を置き換え/改訂',
    atHeading: '記録日時',
    // supersede モード表示（省略時は amend）。
    modeSupersede: '置換',
    modeAmend: '改訂',
    modeException: '例外',
    // target 種別のプレフィクス（一覧行・詳細の対象表示）。
    targetPrefixTransition: '仕様',
    targetPrefixTag: 'タグ',
    targetPrefixVocab: '語彙',
    countLabel: (n: number) => `${n} 件`,
    // HOME の「直近の意思決定」カードから一覧へ飛ぶリンク。
    viewAll: 'すべて見る',
    notFound: 'この意思決定は見つかりませんでした',
  },
  // lookups.tsx の describeMatch()（検索結果の一致理由テキスト）。
  lookups: {
    searchById: '遷移 id',
    tagPrefix: 'タグ: ',
    vocabPrefix: '語彙: ',
    kindPrefix: '種別: ',
  },
  config: {
    loading: 'loading…',
    heading: '設定',
    introBefore: 'プロジェクト設定 ',
    introAfter: '。語彙とタグの分類軸・派生の定義です。変更頻度は低いですが、lint・要件トレーサビリティ・facet ナビ全体に波及します。',
    serverModeBefore: 'サーバモード — 変更は ',
    serverModeAfter: ' に書き込まれます',
    dirtyBadge: '未保存の変更',
    discard: '破棄',
    readonlyTitle: '閲覧専用（静的版）',
    readonlyBannerMid: ' で書き出した1ファイル版です。編集・保存するには ',
    readonlyBannerSuffix: ' でサーバを起動してください。',
    savedMessage: '保存しました — .scholia/config.json に書き込みました',
    savedLocalMessage: '保存しました — .scholia/config.local.json（この端末のみ・gitignore 対象）に書き込みました',
    portInvalid: (current: string) => `ポートは 1〜65535 の整数で入力してください（現在: ${current}）`,
    portEmptyWord: '空',
    timezoneInvalid: (current: string) => `"${current}" は IANA タイムゾーン名として解決できません（例: Asia/Tokyo）`,
    localBadge: 'この端末のみ',
    sections: {
      // 「分類軸」= タグをどう分類し facet ナビでどう束ねるか（グルーピングの軸）。
      // kind バッジの「軸」（axis kind＝状態次元）とは別義（D10b-6 文言是正）。
      classification: { title: 'タグの分類', desc: 'タグをどの種類（kind）で分類し、どの種類で facet ナビに束ねて見せるか' },
      traceability: { title: 'トレーサビリティ', desc: '要件↔実装（仕様）の対応を追跡する対象' },
      viewer: { title: 'ビューア', desc: 'ローカルサーバの設定' },
      display: { title: '表示', desc: 'ヘッダーの製品名・概要画面の見出し文・意思決定日時のタイムゾーン。空欄は既定にフォールバックします。' },
      local: {
        title: 'この端末だけの設定',
        desc: '.scholia/config.local.json（gitignore 対象）に保存します。プロジェクト設定より優先されますが、共有はされません。空欄なら上のプロジェクト既定を使います。',
      },
      readonlyMeta: {
        title: '読み取り専用メタ',
        descBefore: '語彙(vocab)の種別・接頭辞・スキーマ版。変更は CLI（',
        descMid: ' / ',
        descAfter: '）で行います。',
      },
    },
    fields: {
      tagKinds: { label: 'タグ種別', description: 'タグに付けられる分類の種類。タグの「役割」を定義します。' },
      // 「facet ナビの種類」= サイドバーの分類ナビに出す tag kind（グルーピングの
      // 軸）。axis kind（状態次元）とは無関係（D10b-6 文言是正）。
      facetKinds: { label: 'facet ナビの種類', description: 'Browse 画面のサイドバー facet ナビに束ねて出す tag kind。通常 tagKinds の部分集合です。' },
      roots: { label: 'ルートタグ', description: 'タグ階層のルートに置くタグ。空でも構いません。' },
      traceabilityKinds: {
        label: 'トレーサビリティ対象',
        description: '要件トレーサビリティ（充足 gap 検出）の対象にする種類。通常 requirement のみ。',
      },
      port: { label: '待受ポート', descriptionBefore: 'ローカルサーバ（', descriptionAfter: '）が待ち受けるポート。1〜65535 の整数。' },
      productName: { label: '製品名', description: 'ヘッダー左上に表示する製品名。空欄なら既定の「scholia」を使います。' },
      tagline: { label: 'タグライン', description: '概要（HOME）画面の見出し。空欄なら既定文言を使います。' },
      intro: { label: 'イントロ文', description: '概要（HOME）画面の説明文。空欄なら既定文言を使います。' },
      timezone: {
        label: 'タイムゾーン',
        description: '意思決定の日時（記録はUTC・常に不変）を表示する際のIANAタイムゾーン名（例: Asia/Tokyo）。空欄ならUTCのまま表示します。',
      },
      localPort: { label: '待受ポート（この端末だけ）', description: 'この端末で scholia view を起動するときのポート。空欄なら上のプロジェクト既定を使います。' },
      localTimezone: {
        label: 'タイムゾーン（この端末だけ）',
        description: 'この端末での意思決定の日時表示だけをプロジェクト既定と違うタイムゾーンにしたいときに設定します。空欄なら上のプロジェクト既定（またはUTC）を使います。',
      },
    },
    tagKindLabelsField: { label: 'タグ種別の表示ラベル', description: '各タグ種別の画面表示名。未設定のままなら id をそのまま表示します。' },
    tagKindsUnset: '（タグ種別が未設定です）',
    addPlaceholder: '追加して Enter',
    subsetWarningBefore: '一部が ',
    subsetWarningAfter: ' に含まれていません（通常は部分集合として運用します）',
    unsetPlaceholder: '（未設定）',
    schemaVersionLabel: 'スキーマ版',
    vocabKindsHeading: '語彙の種別（category ごと）',
    undefinedMarker: '（未定義）',
  },
  // api.ts の静的版（scholia export --html）フォールバックエラー文言。
  api: {
    unavailable: (what: string) => `${what}は静的版（scholia export --html）では利用できません`,
    configEdit: 'config の編集',
    transitionsByFacetKind: 'facet/kind での遷移一覧',
    transitionsForTag: (tag: string) => `tag ${tag} の遷移一覧`,
    transition: (id: string) => `遷移 ${id}`,
    spec: (tagId: string) => `spec ${tagId}`,
    flow: (actionId: string) => `フロー図 ${actionId}`,
    rulesWithSelectors: 'rules (tag/tx/facet 指定)',
    diff: '差分（diff）',
    reviews: 'AI コメント',
    decisionAdopt: '採用（decision の記録）',
    reviewDelete: '提案コメントの削除（採用/却下の掃除）',
    transitionEdit: '提案の手直し（語彙ピッカー）',
    transitionCreate: '新規 Transition の提案',
    transitionDelete: 'Transition の削除提案',
  },
};

// NOT `as const` — every string field must widen to plain `string` (not a
// literal type) so `en` below, typed as this same `Strings`, can hold
// different literal text for each key. Only the innermost Record<K, string>
// casts (provenanceSourceLabel below) narrow their *keys*, not the values.
export type Strings = typeof ja;
export type Lang = 'ja' | 'en';

const en: Strings = {
  nav: {
    overview: 'Overview',
    browse: 'Browse',
    home: 'Home',
    tags: 'Tags',
    specs: 'Specs',
    vocab: 'Vocab',
    flow: 'Flow',
    config: 'Settings',
  },
  overview: {
    loading: 'Loading…',
    treeHeading: 'Structure',
    selectPrompt: 'Pick a component from the structure tree on the left to see its spec sheet.',
    noComponents: 'This project has no tags of kind "component". You can still browse every record from Browse.',
    openInBrowser: 'Open in Browse',
    coverageHeading: 'Coverage',
    coverageSuffix: 'requirements satisfied by specs',
    coverageNone: 'no scoped requirements',
    partCount: (n) => `${n} parts`,
    ruleCount: (n) => `${n} current rules`,
    gapCount: (n) => `${n} unsatisfied`,
    responsibilityHeading: 'Responsibility',
    behaviorsHeading: 'Behavior by part',
    behaviorsHint: 'trigger → given → result',
    unconditional: 'Unconditional',
    satisfiesReqs: 'Satisfies',
    txCount: (n) => `${n} transitions`,
    constraintsHeading: '"Never" constraints',
    constraintsHint: 'property-kind requirements',
    readFull: 'Read full text',
    backToSummary: 'Back to summary',
    rulesToggle: (n) => `Rules (${n})`,
    componentRulesToggle: (n) => `Rules for this component (${n})`,
    viaComponent: 'directly on this component',
    viaSpec: 'directly on this spec',
    viaTag: (name) => `via tag <${name}>`,
  },
  header: {
    fontDec: 'Decrease font size',
    fontInc: 'Increase font size',
    themeToggle: 'Toggle theme',
    filterToggle: 'Filters',
    fontScaleGroupLabel: 'Font size',
    commentList: 'Comment list',
    langToggle: 'Switch language',
  },
  home: {
    tagline: 'Records, in a form worth reading.',
    intro:
      'scholia records product decisions, requirements, and behavior as atoms (transitions), and lets you view structure as derived queries.',
    tagCount: (n) => `${n} tags`,
    traceabilityHeading: 'Requirement traceability',
    goTraceability: 'View requirements',
    satisfiedOf: (satisfied, total) => `${satisfied} / ${total}`,
    satisfiedSuffix: 'requirements satisfied by specs',
    gapHeading: (n) => `${n} unsatisfied (gap) — requirements with no linked spec yet`,
    noGap: 'All requirements are satisfied by specs',
    recentDecisionsHeading: 'Recent decisions',
    noDecisions: 'No decisions recorded yet',
    loading: 'loading…',
  },
  vocab: {
    heading: 'Vocab',
    intro: 'The words that shape specs, defined',
    owner: 'owner',
    usageCount: (n) => `Used in ${n} transitions`,
    usageHeading: 'Usage',
    noUsage: 'Not referenced by any transition',
    empty: 'No matching vocab entries',
    loading: 'loading…',
    // #45 D5 relationship slots.
    refHeading: 'External contract',
    altLabelsHeading: 'Alternate labels',
    establishesHeading: 'Establishes conditions',
    establishedByHeading: 'Established by effects',
    decisionsHeading: 'Decisions',
    categoryLabel: (c) => ({ action: FLOW_TRIGGER_EN, condition: FLOW_GIVEN_EN, effect: FLOW_RESULT_EN } as Record<string, string>)[c] || c,
    otherKind: 'Other',
    subjectEmpty: (name) => `No vocab referenced by transitions under ${name}`,
    treeModeCategory: 'Category',
    treeModeTransition: 'Context',
    unusedBucket: 'Unused',
  },
  flow: {
    trigger: FLOW_TRIGGER_EN,
    given: FLOW_GIVEN_EN,
    result: FLOW_RESULT_EN,
    noResult: '(no result)',
    noGiven: 'Unconditional (no given)',
    menuShowFlow: 'Show flow diagram',
    viewTitle: (label) => `${label} flow`,
    loading: 'Loading…',
    emptyAction: 'No transitions carry this action.',
    indexTitle: 'Flow',
    indexIntro: 'Pick an action to open its branching (conditions × transitions) flow diagram.',
    indexEmpty: 'No actions have a flow diagram.',
    indexNoMatch: 'No actions match the current conditions.',
    indexCount: (n) => `${n} actions`,
    indexSearchPlaceholder: 'Filter actions…',
    indexTxCount: (n) => `${n} transitions`,
    diagramError: 'Failed to render the diagram.',
    legendClickable: 'Result (click for the transition detail)',
    zoomIn: 'Zoom in',
    zoomOut: 'Zoom out',
    zoomReset: 'Reset',
    coOccur: 'Occurs together',
    legendSubsetShadow: 'Dotted line (one-way) = fires together',
    gapLabel: 'Undefined',
    legendGap: 'Red = undefined (no transition explicitly requires this)',
    scopeCaveat: '※ This diagram reflects only declared state dimensions (axis-kind axes). Evaluation order follows declared priority and is not verified against the implementation. Coverage is not guaranteed (see `scholia flow` for the full picture).',
  },
  browse: {
    searchPlaceholder: 'Search by keyword or tag',
    kindAll: 'All',
    conditionsHeading: 'Filter conditions',
    and: 'AND',
    clear: 'Clear',
    indexHeading: 'Index',
    indexEmpty: 'No matches',
    indexExpand: 'Expand',
    indexCollapse: 'Collapse',
    uncategorized: 'Uncategorized',
    tagsTitle: 'Tags',
    tagsSubtitle: 'How to group by perspective',
    specsTitle: 'Specs',
    specsSubtitle: 'The grounds for correct behavior, built on decisions',
    empty: 'No items match the current conditions',
    loading: 'loading…',
    satisfiedSpecs: 'Related specs',
    relatedVocab: 'Related vocabulary',
    relatedDecisions: 'Decisions',
    childTags: 'Child tags',
    gapBadge: 'Gap',
    satBadge: (n) => `${n} specs`,
    showDetail: 'Show details',
    hideDetail: 'Hide details',
    rulesHeading: 'Decisions',
    tagsHeading: 'Tags',
    derivedHeading: 'Inherited tags',
    derivedHint: 'Effective tags from vocab inheritance + parent tag expansion',
    clickToFilter: 'Click to add as a search condition',
    openDetail: 'Open details (⌘/Ctrl-click for a new tab)',
    menuTrigger: 'Actions for this item',
    menuAddFilter: 'Add to search conditions',
    menuOpenLink: 'Open link (new tab)',
    provenanceSourceLabel: { own: 'direct', vocab: 'via vocab', ancestor: 'via ancestor' } as Record<TagSource, string>,
    provenanceLabel: (sources) => sources.map((s) => en.browse.provenanceSourceLabel[s]).join(' + '),
    fetchWarning: (n) => `${n} item(s) failed to load (the cards shown are fine — reload to retry)`,
    parentLinkTitle: 'Go to parent tag card',
    childLinkTitle: 'Go to this card',
    kindHeading: 'Kind',
    inheritedFromAncestors: (n: number) => `${n} rule(s) inherited from above`,
    inheritedFromTags: (n: number) => `${n} rule(s) inherited from tags`,
    inheritedSourceTitle: 'Open the record it comes from',
    rulesListLinkExact: 'See all rules for this record',
    rulesListLinkScoped: (tag: string) => `See rules for ⟨${tag}⟩`,
    rulesListLinkTitle: 'Open the decisions list scoped to this target',
    axisStructureHeading: 'Axis structure',
    axisDimensionBadge: 'state dimension',
    axisTotalTrue: 'total declared (by declaration, unverified)',
    axisTotalFalse: 'no total declaration',
    axisValuesHeading: 'Values',
    axisValueActions: 'Actions affected',
    axisNoValues: 'No axis-tagged values (conditions) yet',
  },
  common: {
    save: 'Save',
    cancel: 'Cancel',
    delete: 'Delete',
    close: 'Close',
    remove: 'Remove',
    edit: 'Edit',
  },
  comments: {
    cardAnchorLabel: 'Whole card',
    descriptionAnchorLabel: 'Description',
    pageAnchorLabel: 'Whole page',
    addHere: 'Comment on this section',
    recordType: { tag: 'Tag', transition: 'Spec', vocab: 'Vocab', page: 'Page' },
    panelTitle: 'Comments',
    copied: 'Copied',
    copyAllTitle: 'Copy a summary for an AI to use when making fixes',
    copyAll: 'Copy',
    composerPlaceholder: 'Enter a comment… (about this part of this card)',
    submitHintMac: '⌘+Enter to post',
    submitHintOther: 'Ctrl+Enter to post',
    emptyLine1: 'No comments yet.',
    emptyLine2Before: 'Add one from the',
    emptyLine2After: 'next to any card heading.',
    replyPlaceholder: 'Add a reply…',
    replyDelete: 'Delete reply',
    replyAdd: 'Reply',
    gotoLocation: 'Go to location',
    copyDocTitle: '# scholia viewer — review comments',
    copyTaskLine: (title) => `Task: ${title}`,
    copyIntro: (n) =>
      `Please fix the following ${n} comment(s) at their respective locations ([Page] items aren't tied to a specific card — they're feedback on the whole view).`,
    copyItemHeader: (i, typeLabel, recordId, title) => `${i}. [${typeLabel}] ${recordId} "${title}"`,
    copyLocationLine: (anchorLabel) => `   Location: ${anchorLabel}`,
    copyCommentLine: (text) => `   Comment: ${text}`,
    copyReplyHeading: '   Reply thread:',
    taskDefaultTitle: 'Uncategorized',
    taskLabel: 'Task',
    taskNew: '+ New task',
    taskNewTitle: 'Create a new task',
    taskNewPlaceholder: 'Enter a task name…',
    proposalHeading: 'Proposal',
    proposalUncommitted: 'Uncommitted',
    proposalUnavailableError: 'Could not load the proposal diff',
    proposalCleanFlag: 'Change pending (no comment yet → drawer)',
    proposalWhatLabel: 'the proposal diff view',
    aiBadge: 'AI',
    aiReadonlyNote: "Written by AI — can't be edited, deleted, or replied to.",
    adoptButton: 'Adopt',
    adoptWhyLabel: 'Final why (will be recorded as a decision)',
    adoptConfirm: 'Confirm adoption',
    adoptedBadge: 'Adopted',
    adoptedWhyHeading: 'Adopted why (decision)',
    adoptedNote: 'This proposal was recorded as a decision (commits[] is empty). After committing, link it with `scholia decision add-commit <id> <hash>`.',
    supersedeHeading: 'Currency links applied on adoption',
    supersedeModeLabel: (mode: string) =>
      mode === 'supersede'
        ? 'Full replacement (retires the older decision)'
        : mode === 'exception'
          ? 'Deliberate exception (older stays current)'
          : 'Partial amendment (older stays current)',
    supersedeMissing: 'This decision no longer exists (adoption will fail)',
    supersedeUnknownTarget: 'Decision with unresolved target',
    supersedeNoneWithPrior: (n: number) =>
      `No replacement declared. This record already has ${n} decision(s) — if this proposal revises one of them, leaving it unlinked keeps the old rule showing as current.`,
    supersedeErrorMissingTarget: 'That decision no longer exists — the replacement this proposal declared is gone.',
    supersedeErrorInvalidMode: 'Invalid replacement kind (must be full replacement, partial amendment, or deliberate exception).',
    supersedeErrorDuplicate: 'The same decision is listed twice.',
    supersedeErrorSelfReference: 'A decision cannot replace itself.',
    supersedeErrorEmptyId: 'No replacement target was given.',
    supersedeErrorModeRewrite: 'A declared replacement kind cannot be changed (links are append-only).',
    reviewErrorNotFound: 'That proposal comment no longer exists (it has already been deleted).',
    reviewErrorInvalidId: 'The proposal comment reference is invalid.',
    rejectButton: 'Reject',
    rejectWhyLabel: 'Rejection reason (will be recorded as a decision)',
    rejectConfirm: 'Confirm rejection',
    rejectWhyDraft: (text: string) => `Rejected: ${text}`,
    pickerAddButton: 'Add vocab',
    pickerSearchPlaceholder: 'Search vocab…',
    pickerEmpty: 'No matches',
    pickerRemoveTitle: 'Remove',
    pickerMoveUpTitle: 'Move up',
    pickerMoveDownTitle: 'Move down',
    reflectButton: 'Apply this edit to the proposal',
    reflecting: 'Applying…',
    reflectError: (msg) => `Could not apply the edit: ${msg}`,
    proposalAddedBadge: 'New transition (proposed)',
    tombstoneBadge: 'Deletion (proposed)',
    tombstoneRestoreButton: 'Undo deletion (re-create)',
    tombstoneRestoring: 'Undoing…',
    tombstoneRestoreError: (msg) => `Could not undo: ${msg}`,
    newTransitionButton: '+ Propose a new transition',
    newTransitionActionUnset: '(not selected)',
    newTransitionIdLabel: 'id (new identifier)',
    newTransitionIdPlaceholder: 'e.g. tx.lint.check',
    newTransitionIdDuplicate: (id) => `id "${id}" already exists — pick a different id`,
    newTransitionCreateButton: 'Create',
    newTransitionCreating: 'Creating…',
    newTransitionCancel: 'Close',
    newTransitionCreateError: (msg) => `Could not create: ${msg}`,
    deleteProposalButton: 'Delete this transition (proposed)',
    deleteProposalConfirmLabel: 'Delete for real? (removes it from the working tree, uncommitted)',
    deleteProposalConfirmButton: 'Delete',
    deleteProposalDeleting: 'Deleting…',
    deleteProposalCancel: 'Cancel',
    deleteProposalError: (msg) => `Could not delete: ${msg}`,
    recordDiffLabelField: 'Label',
    recordDiffKindField: 'Kind',
    recordDiffDescriptionField: 'Description',
    recordDiffNameField: 'Name',
    recordDiffParentsField: 'Parent tags',
    recordDiffNoParents: '(no parent tags)',
  },
  decisions: {
    heading: 'Decisions',
    intro: 'A record of when, what, and why things changed. Browse them with current vs. superseded distinguished.',
    loading: 'loading…',
    empty: 'No decisions recorded yet',
    noMatch: 'No decisions match the current conditions',
    backToList: 'Back to list',
    searchPlaceholder: 'Search why / changed',
    filterTargetKind: 'Target kind',
    filterTag: 'Tag',
    filterCurrency: 'Currency',
    filterAll: 'All',
    targetKindTransition: 'Spec (transition)',
    targetKindTag: 'Tag',
    targetKindVocab: 'Vocab',
    filterPeriod: 'Period',
    periodAll: 'All time',
    period30d: 'Last 30 days',
    period90d: 'Last 90 days',
    period1y: 'Last year',
    currencyCurrent: 'Current',
    currencySuperseded: 'Superseded',
    effectInForce: 'In force',
    effectReplaced: 'Replaced',
    readTogether: (n: number) => `${n} decision(s) to read with this`,
    openReplacement: 'Open the replacement',
    replacedHeading: (n: number) => `Replaced rules (${n})`,
    whyHeading: 'Why',
    changedHeading: 'Changed',
    targetHeading: 'Target',
    commitsHeading: 'Commits',
    refHeading: 'Ref',
    acknowledgesHeading: 'Acknowledged findings',
    supersedesHeading: 'Supersedes / amends',
    supersededByHeading: 'Superseded / amended by',
    atHeading: 'Recorded',
    modeSupersede: 'Supersede',
    modeAmend: 'Amend',
    modeException: 'Exception',
    targetPrefixTransition: 'Spec',
    targetPrefixTag: 'Tag',
    targetPrefixVocab: 'Vocab',
    countLabel: (n) => `${n}`,
    viewAll: 'View all',
    notFound: 'That decision could not be found',
  },
  lookups: {
    searchById: 'transition id',
    tagPrefix: 'Tag: ',
    vocabPrefix: 'Vocab: ',
    kindPrefix: 'Kind: ',
  },
  config: {
    loading: 'loading…',
    heading: 'Settings',
    introBefore: 'Project configuration ',
    introAfter:
      '. Defines the classification axes and derivations for vocab and tags. Changes are infrequent, but affect lint, requirement traceability, and the facet nav throughout.',
    serverModeBefore: 'Server mode — changes are written to ',
    serverModeAfter: '.',
    dirtyBadge: 'Unsaved changes',
    discard: 'Discard',
    readonlyTitle: 'Read-only (static export)',
    readonlyBannerMid: ' is a single-file export. To edit and save, start the server with ',
    readonlyBannerSuffix: '.',
    savedMessage: 'Saved — written to .scholia/config.json',
    savedLocalMessage: 'Saved — written to .scholia/config.local.json (this machine only, gitignored)',
    portInvalid: (current) => `Port must be an integer between 1 and 65535 (current: ${current})`,
    portEmptyWord: 'empty',
    timezoneInvalid: (current) => `"${current}" doesn't resolve as an IANA timezone name (e.g. Asia/Tokyo)`,
    localBadge: 'This machine only',
    sections: {
      // "Tag classification" = how tags are grouped in the facet nav — the
      // grouping axis, distinct from the axis *kind* (a state dimension).
      classification: { title: 'Tag classification', desc: 'Which kind classifies tags, and which kind groups them in the facet nav' },
      traceability: { title: 'Traceability', desc: 'What requirement↔implementation (spec) traceability tracks' },
      viewer: { title: 'Viewer', desc: 'Local server settings' },
      display: {
        title: 'Display',
        desc: 'The header product name, HOME headline text, and the timezone decision timestamps render in. Blank falls back to the built-in default.',
      },
      local: {
        title: 'This machine only',
        desc: "Saved to .scholia/config.local.json (gitignored). Wins over the project settings above, but isn't shared. Blank uses the project default above.",
      },
      readonlyMeta: {
        title: 'Read-only metadata',
        descBefore: 'Vocab category/idPrefix/schema version. Changed via the CLI (',
        descMid: ' / ',
        descAfter: ').',
      },
    },
    fields: {
      tagKinds: { label: 'Tag kinds', description: 'The classification kinds a tag can carry. Defines a tag\'s "role".' },
      facetKinds: { label: 'Facet nav kinds', description: "The tag kinds grouped in the Browse screen's sidebar facet nav. Usually a subset of tagKinds. Unrelated to the axis kind (a state dimension)." },
      roots: { label: 'Root tags', description: 'Tags placed at the root of the tag hierarchy. May be empty.' },
      traceabilityKinds: {
        label: 'Traceability targets',
        description: 'The kinds tracked for requirement traceability (satisfied/gap detection). Usually just requirement.',
      },
      port: { label: 'Listen port', descriptionBefore: 'The port the local server (', descriptionAfter: ') listens on. An integer from 1–65535.' },
      productName: { label: 'Product name', description: 'The product name shown at the top-left of the header. Blank uses the built-in "scholia".' },
      tagline: { label: 'Tagline', description: "The HOME screen's headline. Blank uses the built-in copy." },
      intro: { label: 'Intro text', description: "The HOME screen's description text. Blank uses the built-in copy." },
      timezone: {
        label: 'Timezone',
        description: 'The IANA timezone name (e.g. Asia/Tokyo) decision timestamps render in — storage stays UTC always. Blank shows UTC.',
      },
      localPort: { label: 'Listen port (this machine only)', description: 'The port this machine uses for scholia view. Blank uses the project default above.' },
      localTimezone: {
        label: 'Timezone (this machine only)',
        description: "Set this only if you want decision timestamps shown in a different timezone than the project default, on this machine. Blank uses the project default (or UTC) above.",
      },
    },
    tagKindLabelsField: { label: 'Tag kind display labels', description: 'The display name for each tag kind. Left unset, the id is shown as-is.' },
    tagKindsUnset: '(no tag kinds set)',
    addPlaceholder: 'Add and press Enter',
    subsetWarningBefore: 'Some values are not included in ',
    subsetWarningAfter: ' (this is normally operated as a subset)',
    unsetPlaceholder: '(unset)',
    schemaVersionLabel: 'Schema version',
    vocabKindsHeading: 'Vocab kinds (per category)',
    undefinedMarker: '(undefined)',
  },
  api: {
    unavailable: (what) => `${what} is not available in the static export (scholia export --html)`,
    configEdit: 'editing config',
    transitionsByFacetKind: 'transition list by facet/kind',
    transitionsForTag: (tag) => `transition list for tag ${tag}`,
    transition: (id) => `transition ${id}`,
    spec: (tagId) => `spec ${tagId}`,
    flow: (actionId) => `flow diagram ${actionId}`,
    rulesWithSelectors: 'rules (tag/tx/facet selectors)',
    diff: 'the diff',
    reviews: 'AI comments',
    decisionAdopt: 'adopting (recording a decision)',
    reviewDelete: 'deleting a review (adopt/reject cleanup)',
    transitionEdit: 'editing the proposal (vocab picker)',
    transitionCreate: 'proposing a new transition',
    transitionDelete: 'proposing a transition deletion',
  },
};

export const DICTS: Record<Lang, Strings> = { ja, en };
