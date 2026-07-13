import type { TagSource } from './types';

// UI chrome copy (headings, buttons, empty states, labels) for BOTH
// languages the viewer supports. Data values (vocab label / tag name·
// description / decision text / requirement prose) never come from here —
// they're rendered as stored in `.pmem/`, unmodified regardless of
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
    home: '概要',
    tags: 'タグ',
    specs: '仕様',
    vocab: '語彙',
    config: '設定',
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
      'product-memory は、プロダクトの意思決定・要件・振る舞いを原子（遷移）として記録し、構造は派生（query）で見るためのツールです。',
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
    noUsage: 'どの遷移からも参照されていません',
    empty: '該当する語彙はありません',
    loading: 'loading…',
    // 2026-07-11 tweaks3 §1: 遷移の きっかけ/前提/結果 と同じ語彙に統一
    // （grammar色 --t-act/--t-giv/--t-then とも対応）。VocabEntry.category は
    // Go側では string（3軸固定の想定値だが型では絞られていない）なので、
    // 未知の値は素の文字列にフォールバックする。
    categoryLabel: (c: string): string =>
      ({ action: FLOW_TRIGGER_JA, condition: FLOW_GIVEN_JA, effect: FLOW_RESULT_JA } as Record<string, string>)[c] || c,
  },
  // WHEN/GIVEN/THEN の言い換え（調整4）。遷移カード全般（一覧・詳細・spec）で共通利用。
  flow: {
    trigger: FLOW_TRIGGER_JA,
    given: FLOW_GIVEN_JA,
    result: FLOW_RESULT_JA,
    noResult: '（結果なし）',
    noGiven: '無条件（前提なし）',
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
    relatedDecisions: '意思決定',
    childTags: '下位のタグ',
    gapBadge: '未充足',
    satBadge: (n: number) => `${n} 仕様`,
    showDetail: '詳細を見る',
    hideDetail: '詳細を閉じる',
    rulesHeading: '意思決定',
    tagsHeading: 'タグ',
    derivedHeading: '継承・祖先展開で効くタグ',
    derivedHint: 'vocab継承＋親タグ展開の実効タグ',
    clickToFilter: 'クリックで検索条件に追加',
    // 実効タグの由来ラベル（gap G11）。own/vocab/ancestor は複数同時成立しうる
    // ので順に連結する — バックエンドの EffectiveTag.sources をそのまま表示
    // するだけで、フロントは由来を再計算しない（§9）。
    provenanceSourceLabel: { own: '直接付与', vocab: 'vocab由来', ancestor: '祖先由来' } as Record<TagSource, string>,
    provenanceLabel: (sources: TagSource[]): string => sources.map((s) => ja.browse.provenanceSourceLabel[s]).join(' + '),
    fetchWarning: (n: number) => `${n} 件の読み込みに失敗しました（表示されているカードは正常です。再読み込みで再試行できます）`,
    parentLinkTitle: '親タグのカードへ移動',
    childLinkTitle: 'このカードへ移動',
    kindHeading: '種別',
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
    copyDocTitle: '# product-memory ビューア — レビューコメント',
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
    taskNew: '＋ 新規タスク',
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
    proposalAddedBadge: '新規 Transition（提案）',
    tombstoneBadge: '削除（提案）',
    tombstoneRestoreButton: '削除を取り消す（再作成）',
    tombstoneRestoring: '取り消し中…',
    tombstoneRestoreError: (msg: string) => `取り消せませんでした: ${msg}`,
    newTransitionButton: '＋ 新規 Transition を提案',
    newTransitionActionUnset: '（未選択）',
    newTransitionIdLabel: 'id（新規識別子）',
    newTransitionIdPlaceholder: '例: T-lint-check',
    newTransitionIdDuplicate: (id: string) => `id "${id}" は既に存在します。別の id を指定してください`,
    newTransitionCreateButton: '作成',
    newTransitionCreating: '作成中…',
    newTransitionCancel: '閉じる',
    newTransitionCreateError: (msg: string) => `作成できませんでした: ${msg}`,
    deleteProposalButton: 'この Transition を削除（提案）',
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
    adoptedNote: 'この提案は decision として記録されました（commits[] は空）。commit 後は `pmem decision add-commit <id> <hash>` で結線してください。',
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
    savedMessage: '保存しました — .pmem/config.json に書き込みました',
    portInvalid: (current: string) => `ポートは 1〜65535 の整数で入力してください（現在: ${current}）`,
    portEmptyWord: '空',
    sections: {
      classification: { title: '分類軸', desc: 'タグをどう分類し、どの軸で見せるか' },
      traceability: { title: 'トレーサビリティ', desc: '要件↔実装（仕様）の対応を追跡する対象' },
      viewer: { title: 'ビューア', desc: 'ローカルサーバの設定' },
      display: { title: '表示', desc: 'ヘッダーの製品名と概要画面の見出し文。空欄は既定文言にフォールバックします。' },
      readonlyMeta: {
        title: '読み取り専用メタ',
        descBefore: '語彙(vocab)の種別・接頭辞・スキーマ版。変更は CLI（',
        descMid: ' / ',
        descAfter: '）で行います。',
      },
    },
    fields: {
      tagKinds: { label: 'タグ種別', description: 'タグに付けられる分類の種類。タグの「役割」を定義します。' },
      facetKinds: { label: 'facet 軸', description: 'Browse 画面のサイドバー facet ナビに出す種類。通常 tagKinds の部分集合です。' },
      roots: { label: 'ルートタグ', description: 'タグ階層のルートに置くタグ。空でも構いません。' },
      traceabilityKinds: {
        label: 'トレーサビリティ対象',
        description: '要件トレーサビリティ（充足 gap 検出）の対象にする種類。通常 requirement のみ。',
      },
      port: { label: '待受ポート', descriptionBefore: 'ローカルサーバ（', descriptionAfter: '）が待ち受けるポート。1〜65535 の整数。' },
      productName: { label: '製品名', description: 'ヘッダー左上に表示する製品名。空欄なら既定の「pmem」を使います。' },
      tagline: { label: 'タグライン', description: '概要（HOME）画面の見出し。空欄なら既定文言を使います。' },
      intro: { label: 'イントロ文', description: '概要（HOME）画面の説明文。空欄なら既定文言を使います。' },
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
  // api.ts の静的版（pmem export --html）フォールバックエラー文言。
  api: {
    unavailable: (what: string) => `${what}は静的版（pmem export --html）では利用できません`,
    configEdit: 'config の編集',
    transitionsByFacetKind: 'facet/kind での遷移一覧',
    transitionsForTag: (tag: string) => `tag ${tag} の遷移一覧`,
    transition: (id: string) => `遷移 ${id}`,
    spec: (tagId: string) => `spec ${tagId}`,
    rulesWithSelectors: 'rules (tag/tx/facet 指定)',
    diff: '差分（diff）',
    reviews: 'AI コメント',
    decisionAdopt: '採用（decision の記録）',
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
    home: 'Home',
    tags: 'Tags',
    specs: 'Specs',
    vocab: 'Vocab',
    config: 'Settings',
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
      'product-memory records product decisions, requirements, and behavior as atoms (transitions), and lets you view structure as derived queries.',
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
    noUsage: 'Not referenced by any transition',
    empty: 'No matching vocab entries',
    loading: 'loading…',
    categoryLabel: (c) => ({ action: FLOW_TRIGGER_EN, condition: FLOW_GIVEN_EN, effect: FLOW_RESULT_EN } as Record<string, string>)[c] || c,
  },
  flow: {
    trigger: FLOW_TRIGGER_EN,
    given: FLOW_GIVEN_EN,
    result: FLOW_RESULT_EN,
    noResult: '(no result)',
    noGiven: 'Unconditional (no given)',
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
    relatedDecisions: 'Decisions',
    childTags: 'Child tags',
    gapBadge: 'Gap',
    satBadge: (n) => `${n} specs`,
    showDetail: 'Show details',
    hideDetail: 'Hide details',
    rulesHeading: 'Decisions',
    tagsHeading: 'Tags',
    derivedHeading: 'Tags in effect via inheritance / ancestor expansion',
    derivedHint: 'Effective tags from vocab inheritance + parent tag expansion',
    clickToFilter: 'Click to add as a search condition',
    provenanceSourceLabel: { own: 'direct', vocab: 'via vocab', ancestor: 'via ancestor' } as Record<TagSource, string>,
    provenanceLabel: (sources) => sources.map((s) => en.browse.provenanceSourceLabel[s]).join(' + '),
    fetchWarning: (n) => `${n} item(s) failed to load (the cards shown are fine — reload to retry)`,
    parentLinkTitle: 'Go to parent tag card',
    childLinkTitle: 'Go to this card',
    kindHeading: 'Kind',
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
    copyDocTitle: '# product-memory viewer — review comments',
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
    adoptedNote: 'This proposal was recorded as a decision (commits[] is empty). After committing, link it with `pmem decision add-commit <id> <hash>`.',
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
    newTransitionIdPlaceholder: 'e.g. T-lint-check',
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
    savedMessage: 'Saved — written to .pmem/config.json',
    portInvalid: (current) => `Port must be an integer between 1 and 65535 (current: ${current})`,
    portEmptyWord: 'empty',
    sections: {
      classification: { title: 'Classification axes', desc: 'How to classify tags, and which axes to show' },
      traceability: { title: 'Traceability', desc: 'What requirement↔implementation (spec) traceability tracks' },
      viewer: { title: 'Viewer', desc: 'Local server settings' },
      display: { title: 'Display', desc: "The header product name and HOME headline text. Blank falls back to the built-in copy." },
      readonlyMeta: {
        title: 'Read-only metadata',
        descBefore: 'Vocab category/idPrefix/schema version. Changed via the CLI (',
        descMid: ' / ',
        descAfter: ').',
      },
    },
    fields: {
      tagKinds: { label: 'Tag kinds', description: 'The classification kinds a tag can carry. Defines a tag\'s "role".' },
      facetKinds: { label: 'Facet axes', description: "The kinds shown in the Browse screen's sidebar facet nav. Usually a subset of tagKinds." },
      roots: { label: 'Root tags', description: 'Tags placed at the root of the tag hierarchy. May be empty.' },
      traceabilityKinds: {
        label: 'Traceability targets',
        description: 'The kinds tracked for requirement traceability (satisfied/gap detection). Usually just requirement.',
      },
      port: { label: 'Listen port', descriptionBefore: 'The port the local server (', descriptionAfter: ') listens on. An integer from 1–65535.' },
      productName: { label: 'Product name', description: 'The product name shown at the top-left of the header. Blank uses the built-in "pmem".' },
      tagline: { label: 'Tagline', description: "The HOME screen's headline. Blank uses the built-in copy." },
      intro: { label: 'Intro text', description: "The HOME screen's description text. Blank uses the built-in copy." },
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
    unavailable: (what) => `${what} is not available in the static export (pmem export --html)`,
    configEdit: 'editing config',
    transitionsByFacetKind: 'transition list by facet/kind',
    transitionsForTag: (tag) => `transition list for tag ${tag}`,
    transition: (id) => `transition ${id}`,
    spec: (tagId) => `spec ${tagId}`,
    rulesWithSelectors: 'rules (tag/tx/facet selectors)',
    diff: 'the diff',
    reviews: 'AI comments',
    decisionAdopt: 'adopting (recording a decision)',
    transitionEdit: 'editing the proposal (vocab picker)',
    transitionCreate: 'proposing a new transition',
    transitionDelete: 'proposing a transition deletion',
  },
};

export const DICTS: Record<Lang, Strings> = { ja, en };
