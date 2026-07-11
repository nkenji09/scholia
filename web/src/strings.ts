// Centralized UI copy for the views added in the "記録ブラウザ" unit
// (Vocab / Spec / Tags). Kept in one file — rather than inline JSX strings
// like the older views use — so a future i18n pass has a single place to
// swap in a translation table instead of hunting through components (data
// values such as tag names / vocab labels / decision text stay untouched:
// only chrome copy — headings, buttons, empty states — lives here).
export const strings = {
  // レビュー差し戻し MAJOR-1: ナビは概要/タグ/仕様(デザイン正本 navItems)＋
  // 外挿1画面(語彙)＋設定、全て日本語ラベルに統一。'spec'（旧・独立タブ）は
  // ここに含めない — router.ts の #/spec/<tag> は引き続き解決するが、
  // 'tags' タブと同一facetのため独立ボタンにはしない（重複ナビの解消）。
  // トレーサビリティ/比較はデザイン未対応のため2026-07-11にナビから削除
  // （ユーザー指示、Header.tsx参照）。
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
  // WHEN/GIVEN/THEN の言い換え（調整4）。遷移カード全般（一覧・詳細・spec）で共通利用。
  flow: {
    trigger: 'きっかけ',
    given: '前提',
    result: '結果',
    noResult: '（結果なし）',
  },
  // BROWSE(タグ/仕様) — 旧 Browse(3ペイン)/TagsView(ツリー)/SpecView を検索
  // レール＋カード一覧の1つの型に統合した画面（.concierge/decision.md A-2）。
  browse: {
    searchPlaceholder: '名前・ID・本文で絞り込み',
    kindAll: 'すべて',
    conditionsHeading: '絞り込み条件',
    and: 'AND',
    clear: 'クリア',
    indexHeading: '見出し',
    indexEmpty: '該当なし',
    tagsTitle: 'タグ',
    tagsSubtitle: '説明（markdown）と関連仕様・意思決定を読む',
    specsTitle: '仕様',
    specsSubtitle: 'きっかけ → 前提 → 結果。タグ・Vocab で絞り込み',
    empty: '条件に一致する項目がありません',
    loading: 'loading…',
    satisfiedSpecs: '関連仕様',
    relatedDecisions: '関連する意思決定',
    childTags: '下位のタグ',
    gapBadge: '未充足',
    satBadge: (n: number) => `${n} 仕様`,
    showDetail: '詳細を見る',
    hideDetail: '詳細を閉じる',
    tests: 'tests',
    rulesHeading: '守る規則（意思決定）',
    tagsHeading: 'タグ',
    derivedHeading: '継承・祖先展開で効くタグ',
    derivedHint: 'vocab継承＋親タグ展開の実効タグ',
    clickToFilter: 'クリックで検索条件に追加',
  },
} as const;
