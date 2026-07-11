import type { TagSource } from './types';

// Centralized UI copy for the views added in the "記録ブラウザ" unit
// (Vocab / Spec / Tags). Kept in one file — rather than inline JSX strings
// like the older views use — so a future i18n pass has a single place to
// swap in a translation table instead of hunting through components (data
// values such as tag names / vocab labels / decision text stay untouched:
// only chrome copy — headings, buttons, empty states — lives here).

// action/condition/effect are a fixed 3-axis schema (not user-configurable
// like tagKinds), so their display labels are plain frontend constants —
// no config plumbing (2026-07-11 tweaks3 §1). Shared between flow.* (a
// transition's きっかけ/前提/結果 sections) and vocab.categoryLabel (Vocab's
// category facet/badges) so both read the same vocabulary rather than
// drifting into two translations of the same three concepts.
const FLOW_TRIGGER = 'きっかけ';
const FLOW_GIVEN = '前提';
const FLOW_RESULT = '結果';

export const strings = {
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
    categoryLabel: (c: string): string => ({ action: FLOW_TRIGGER, condition: FLOW_GIVEN, effect: FLOW_RESULT } as Record<string, string>)[c] || c,
  },
  // WHEN/GIVEN/THEN の言い換え（調整4）。遷移カード全般（一覧・詳細・spec）で共通利用。
  flow: {
    trigger: FLOW_TRIGGER,
    given: FLOW_GIVEN,
    result: FLOW_RESULT,
    noResult: '（結果なし）',
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
    tagsTitle: 'タグ',
    tagsSubtitle: 'どの観点でまとめるか',
    specsTitle: '仕様',
    specsSubtitle: '意思決定の上にある、正しい動作の拠り所',
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
    // 実効タグの由来ラベル（gap G11）。own/vocab/ancestor は複数同時成立しうる
    // ので順に連結する — バックエンドの EffectiveTag.sources をそのまま表示
    // するだけで、フロントは由来を再計算しない（§9）。
    provenanceSourceLabel: { own: '直接付与', vocab: 'vocab由来', ancestor: '祖先由来' } as Record<TagSource, string>,
    provenanceLabel: (sources: TagSource[]): string =>
      sources.map((s) => strings.browse.provenanceSourceLabel[s]).join(' + '),
  },
} as const;
