import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';
import { api, isStaticMode } from './api';
import type { Review } from './types';

// AI コメント配送（change-cockpit-design-v3.md §8.4） — `GET /api/reviews`
// が返す read-only サイドカー（.pmem/reviews/）。pendingDiff.tsx と同型の
// Provider: mount 時に一度 fetch し、static export では常に unavailable
// （§8.4 「本単位ではやらない」＝焼き込み未実施）。useComments.tsx がこの
// Provider の下で reviews を読み、人コメント（localStorage）と合流する。
interface Reviews {
  ready: boolean;
  /** 'static' = pmem export --html（サイドカー未焼き込み・§8.4）。'error' =
      server mode だがフェッチ失敗。いずれも人コメント/task 機能をブロック
      しない（この Provider に依存しないため）。 */
  unavailable: 'static' | 'error' | null;
  reviews: Review[];
  // 昇格元コメント掃除（#35・T-review-adopt/-reject）: DELETE /api/reviews/
  // {id} が成功した直後、再フェッチを待たずローカル state から即座に
  // 落とす — useComments.tsx の merged comment list からその場で消える
  // （pendingDiff.tsx の refresh 系と違い、削除は id 単位で分かっているので
  // 全件再取得は不要）。
  removeReview: (id: string) => void;
}

const ReviewsContext = createContext<Reviews | null>(null);

export function ReviewsProvider({ children }: { children: ComponentChildren }) {
  const [reviews, setReviews] = useState<Review[]>([]);
  const [ready, setReady] = useState(false);
  const [unavailable, setUnavailable] = useState<'static' | 'error' | null>(isStaticMode ? 'static' : null);

  useEffect(() => {
    if (isStaticMode) {
      setUnavailable('static');
      setReady(true);
      return;
    }
    api
      .getReviews()
      .then((r) => {
        setReviews(r);
        setUnavailable(null);
        setReady(true);
      })
      .catch(() => {
        setUnavailable('error');
        setReady(true);
      });
  }, []);

  const removeReview = (id: string) => setReviews((prev) => prev.filter((r) => r.id !== id));

  const value: Reviews = { ready, unavailable, reviews, removeReview };
  return <ReviewsContext.Provider value={value}>{children}</ReviewsContext.Provider>;
}

export function useReviews(): Reviews {
  const ctx = useContext(ReviewsContext);
  if (!ctx) throw new Error('useReviews() must be called within a ReviewsProvider');
  return ctx;
}
