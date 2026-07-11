// Centralized UI copy for the views added in the "記録ブラウザ" unit
// (Vocab / Spec / Tags). Kept in one file — rather than inline JSX strings
// like the older views use — so a future i18n pass has a single place to
// swap in a translation table instead of hunting through components (data
// values such as tag names / vocab labels / decision text stay untouched:
// only chrome copy — headings, buttons, empty states — lives here).
export const strings = {
  nav: {
    home: '概要',
    vocab: 'Vocab',
    spec: 'Spec',
    tags: 'Tags',
  },
  header: {
    fontDec: '文字を小さく',
    fontInc: '文字を大きく',
    themeToggle: 'テーマ切替',
    density: {
      compact: 'コンパクト',
      normal: '標準',
      comfortable: 'ゆったり',
    },
    accent: 'アクセント色',
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
    heading: '語彙 (Vocab)',
    intro: 'condition / action / effect の語彙一覧。action と effect は kind 別にまとまります。',
    kindUnset: '(kind 未設定)',
    owner: 'owner',
    usageCount: (n: number) => `${n} 件の遷移で使用`,
    noUsage: 'どの遷移からも参照されていません',
    showTransitions: '使用箇所を表示',
    hideTransitions: '閉じる',
    empty: '該当する語彙はありません',
    loading: 'loading…',
  },
  spec: {
    heading: '仕様 (Spec)',
    intro: 'タグを選ぶと、そのタグに束ねた遷移をまとめた仕様レポートを表示します。',
    searchPlaceholder: 'タグを検索…',
    pickTag: '左のリストからタグを選択してください',
    noEntries: 'このタグに該当する遷移はありません',
    tagRules: 'このタグの決定（cross-cutting rules）',
    txRules: 'この遷移固有の決定',
    tests: 'tests',
    openInBrowse: 'Browse で開く',
    loading: 'loading…',
  },
  // WHEN/GIVEN/THEN の言い換え（調整4）。遷移カード全般（一覧・詳細・spec）で共通利用。
  flow: {
    trigger: 'きっかけ',
    given: '前提',
    result: '結果',
    noResult: '（結果なし）',
  },
  tags: {
    heading: 'タグ階層',
    intro: 'facet 軸ごとにタグの入れ子構造を俯瞰し、各タグから Browse / Spec / Traceability へ辿れます。',
    expandAll: 'すべて展開',
    collapseAll: 'すべて折りたたむ',
    browse: 'Browse',
    specLink: 'Spec',
    traceability: 'Traceability',
    description: '説明',
    txCount: (n: number) => `${n} 件`,
    stats: (tags: number, depth: number) => `${tags} タグ・最大深さ ${depth}`,
    empty: '該当するタグはありません',
    loading: 'loading…',
  },
} as const;
