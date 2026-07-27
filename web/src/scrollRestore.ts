import { useEffect, useRef } from 'preact/hooks';
import type { RefObject } from 'preact';

// Per-view scroll continuity (req.comfortable-viewer.view-state-continuity):
// each browse view (tags/specs/vocab) remembers where it was scrolled to so
// leaving and coming back — or reloading — lands you back at the same place.
//
// Storage split (spec decision): scroll position lives in sessionStorage, NOT
// the URL. It's a per-tab reading context, not something to share or bookmark:
// sessionStorage survives reload within the same tab and vanishes when the tab
// closes, which is exactly the lifetime we want (search conditions, which ARE
// shareable, go in the URL instead — see router.ts / BrowseView / VocabView).
//
// The scroll root is the *document* (window), not the `.browse-main` element:
// #app is only min-height:100vh, so browse-main grows to its full content
// height and the window is what actually scrolls (its overflow-y:auto never
// engages). Scroll-to-card deep links already rely on this via
// scrollIntoView — this hook saves/restores the same window scrollY.
const KEY_PREFIX = 'scholia-scroll-';

function readSaved(view: string): number | null {
  try {
    const raw = sessionStorage.getItem(KEY_PREFIX + view);
    if (raw === null) return null;
    const n = parseInt(raw, 10);
    return Number.isFinite(n) ? n : null;
  } catch {
    // sessionStorage can throw (private mode / disabled storage). Degrade to
    // "no saved position" rather than breaking the view.
    return null;
  }
}

function writeSaved(view: string, top: number): void {
  try {
    sessionStorage.setItem(KEY_PREFIX + view, String(Math.round(top)));
  } catch {
    // ignore — persistence is best-effort (see readSaved).
  }
}

/**
 * Remembers and restores the window scroll position for one view, keyed per
 * view in sessionStorage.
 *
 * - Save (tx.viewer.scroll-save / act.user.leave-view): every scroll is
 *   persisted (debounced) so a reload keeps the position, and the last known
 *   position is flushed again on unmount (a view switch) — from a tracked ref,
 *   never re-read at teardown.
 * - Restore (tx.viewer.scroll-restore / act.user.enter-view): once `ready`
 *   flips true (the view's content has loaded and laid out) the saved position
 *   is applied. With no saved position the view is reset to the top — since the
 *   window scroll is shared across views, this stops the previous view's scroll
 *   from bleeding into a freshly-entered one. `skipRestore` suppresses both
 *   when the view is instead going to scroll to a focused record (a comment
 *   "位置へ移動" jump, or a #/spec/<id> / #/vocab/<id> deep link) — that focus
 *   scroll wins over the remembered position.
 *
 * `ready` is what makes the restore reliable: the caller only flips it true
 * once the view's cards are in the committed DOM, so the document is already
 * tall enough for the target scrollY to take — no rAF/height-polling needed
 * (and rAF would stall anyway when the tab is backgrounded). A short
 * setTimeout re-apply covers card bodies (markdown/spec detail) that finish
 * laying out a beat later and could otherwise clamp the first attempt.
 *
 * Saving is gated on the restore having run (`restored`). This is essential:
 * while a view is still loading it renders a short placeholder, so the scrollY
 * inherited from the previous view clamps to ~0 and fires a `scroll` event. On
 * a slow view (tags settles 47 spec fetches, well past the 100ms save debounce)
 * that clamp would otherwise persist `0` before the restore — gated on `ready`
 * — ever reads it, destroying the saved position. So pre-restore scroll events
 * (clamp / layout, never the user) are ignored, and the restore reads the
 * position captured at mount rather than a possibly-clobbered sessionStorage.
 */
export function useScrollRestore(view: string, ready: boolean, skipRestore = false): void {
  // Position to restore, captured during the first render — before any effect
  // runs and before the loading-clamp scroll's debounce could overwrite
  // sessionStorage. The restore reads THIS, not a fresh readSaved().
  const targetRef = useRef<number | null>(null);
  const captured = useRef(false);
  if (!captured.current) {
    targetRef.current = readSaved(view);
    captured.current = true;
  }

  // Last observed scrollY (only tracked once saving is enabled).
  const latest = useRef<number>(targetRef.current ?? 0);
  const restored = useRef(false);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const onScroll = () => {
      // Ignore clamp/layout scrolls that fire before the restore has run — only
      // the user's own scrolling (after restore) should be persisted.
      if (!restored.current) return;
      latest.current = window.scrollY;
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => writeSaved(view, latest.current), 100);
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      window.removeEventListener('scroll', onScroll);
      if (timer) clearTimeout(timer);
      // Flush the last user position on leave — but only if the restore ran
      // (saving was enabled). Leaving mid-load must not overwrite the saved
      // value with a pre-restore clamp of 0.
      if (restored.current) writeSaved(view, latest.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view]);

  useEffect(() => {
    if (!ready || restored.current) return;
    if (skipRestore) {
      // A focused record (comment jump / #/spec|vocab/<id>) will scrollIntoView
      // instead — don't fight it. Just enable saving from here so the user's
      // subsequent scrolling is remembered.
      latest.current = window.scrollY;
      restored.current = true;
      return;
    }
    const target = targetRef.current ?? 0;
    latest.current = target;
    // Enable saving as the final act, so the scrollTo below (and any user
    // scroll after) is persisted, but nothing before it was.
    restored.current = true;
    // With no saved position, reset to the top: the window scroll is shared
    // across views, so this stops the previous view's position from bleeding in.
    window.scrollTo(0, target);
    if (target === 0) return;
    // Re-apply once after layout settles: some card bodies grow a beat after
    // `ready`, which can clamp the first scroll to a shorter height. setTimeout
    // (not rAF) so it still fires when the tab is backgrounded.
    const reinforce = setTimeout(() => window.scrollTo(0, target), 120);
    return () => clearTimeout(reinforce);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, ready, skipRestore]);
}

/**
 * 面が本体とは別に持つ「独立してスクロールする領域」の位置を、本体と同じ規律で
 * 保持・復元する（view-state-continuity の適用範囲拡張・01KYH0ESVG1D5NGDH5C4TG920J）。
 * 現在これに当たるのは概要タブの構造ツリーだが、キーを渡すだけで他の領域にも使える。
 *
 * useScrollRestore との違いはスクロールルートだけ（window ではなく要素）で、規律は
 * 同一に保つ:
 * - 保存先も寿命も同じ（sessionStorage・同一タブ・reload を越えて残る）。キー空間だけ
 *   `<view>:<region>` の形で分け、本体のキーと衝突させない。
 * - **復元が済むまで保存しない**（`restored`）。読み込み中は領域の中身がまだ短く、
 *   前の状態から引き継いだ scrollTop が 0 へ clamp されて scroll イベントが飛ぶ。
 *   これを保存すると復元より先に保存値を壊す——本体側で差し戻し2回を要した罠と同型
 *   なので、対象が増えてもここは緩めない（01KXFEJ8HZWGTE6D7FHA5W9PS0）。
 *
 * 本体側と1点だけ違うのは**復元値をいつ読むか**。本体側は最初の描画時点で捕まえて以後
 * 読み直さない——window は常に存在し、読み込み中の clamp が復元より先に保存値を壊し
 * うるからで、その競合を避けるための措置だった。要素側にはその競合が無い（要素は
 * `ready` が立つまで存在せず、保存を始めるのも下の `restored` ガードの後）。逆に要素は
 * 出入りのたびに作り直されるので、**現れたその時点の保存値を読む**必要がある。1度だけ
 * 捕まえる形にすると、いったん消えて再び現れた領域（狭い画面のドロワー等）が二度と
 * 復元されない。規律（復元前は保存しない）はそのままに、読むタイミングだけを合わせる。
 *
 * `ready` は「要素が DOM にあり、中身が並び終わった」ことを呼び出し側が伝えるフラグ。
 * 要素は条件付きで描かれること（狭い画面でドロワーが閉じている等）があるので、その
 * 場合は ready を false にして呼ぶ——本フックは要素が無い間は何もしない。
 */
export function useElementScrollRestore(key: string, ref: RefObject<HTMLElement | null>, ready: boolean, skipRestore = false): void {
  const latest = useRef<number>(0);
  const restored = useRef(false);

  // 要素は ready が立ってから初めて存在するので、監視と復元を1つの effect にまとめる
  // （本体側が2つに分かれているのは、window が最初から存在して ready 前の scroll も
  // 届いてしまうため。要素側にはその前段が無い）。
  useEffect(() => {
    if (!ready) return;
    const el = ref.current;
    if (!el) return;

    let timer: ReturnType<typeof setTimeout> | undefined;
    const onScroll = () => {
      if (!restored.current) return;
      latest.current = el.scrollTop;
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => writeSaved(key, latest.current), 100);
    };
    el.addEventListener('scroll', onScroll, { passive: true });

    let reinforce: ReturnType<typeof setTimeout> | undefined;
    if (skipRestore) {
      // 呼び出し側が領域内の別の位置へ寄せる場合は争わない。以降の利用者スクロールだけ
      // 覚える（本体側 skipRestore と同じ扱い）。
      latest.current = el.scrollTop;
      restored.current = true;
    } else {
      // 要素が現れたこの時点の保存値を読む（上の注記参照）。保存を有効にするのは復元を
      // 当ててから＝ここより前の scroll は利用者のものではないので残さない。
      const target = readSaved(key) ?? 0;
      latest.current = target;
      restored.current = true;
      el.scrollTop = target;
      // 中身が一拍遅れて伸びる場合に備えて一度だけ復元し直す（本体側と同じ理由・
      // rAF ではなく setTimeout なのはタブが背面でも動かすため）。
      if (target !== 0) reinforce = setTimeout(() => (el.scrollTop = target), 120);
    }

    return () => {
      el.removeEventListener('scroll', onScroll);
      if (timer) clearTimeout(timer);
      if (reinforce) clearTimeout(reinforce);
      if (restored.current) writeSaved(key, latest.current);
      // 要素が DOM から出た（ready が落ちた）ら、次に現れたときに改めて復元する。ここを
      // 立てたままにすると、いったん消えた領域が二度と復元されない。以降 scroll が来ても
      // 上のリスナは外れており、次の復元まで保存は起きない＝規律は保たれる。
      restored.current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, ready, skipRestore]);
}
