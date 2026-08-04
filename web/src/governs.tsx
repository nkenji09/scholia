import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';
import { api } from './api';
import type { GovernsRef } from './types';

// 「この記録を支配する規則」の参照を、**app 起動時に1回だけ**取って共有する
// （正本 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1「1画面が投げる要求の本数を、
// レコード件数に比例させない」）。
//
// これは lookups.tsx / pendingDiff.tsx と同じ共有機構である。面ごとに
// 独自機構を作らない（CLAUDE.md「viewer の配線」）——ここを共有にするのは、
// **継承規則の開示（InheritedRules）が3つの面に同じ形で載っている**からで、
// 面ごとに取りに行かせると面を1つ足すたびに N 本が復活する。実測でそうなって
// いた: タグ一覧 82 本・仕様一覧 70 本・語彙一覧 170 本。
//
// ⚠️ **データ源は変えていない。** 返す中身は GET /api/governs（＝ CLI
// `scholia rules` と同じ Go コア index.GovernsFor*）が1件ずつ返すものと
// 同じで、Go 側の一括の口が全レコードぶんをまとめて返しているだけである。
// フロントで実効タグを再計算しない（面間整合 D10b-2）という線はそのまま。

/** 支配される側のレコード（api.ts が組む record ref と同じ 3 種）。 */
export interface GovernsRecordRef {
  kind: 'tag' | 'transition' | 'vocab';
  id: string;
}

/** record ref の綴り。**live の応答・静的の焼き込み・この索引で同一**。 */
export function governsKey(record: GovernsRecordRef): string {
  return `${record.kind}:${record.id}`;
}

interface Governs {
  /** 取得が済んだか（失敗も「済んだ」に含む——開示を永久に保留しない）。 */
  ready: boolean;
  /** そのレコードを支配する規則の参照。未取得・該当なしはどちらも空配列。
      呼び手は `ready` を見てから読む（未取得の空と、本当に0件を取り違えない）。 */
  entriesFor: (record: GovernsRecordRef) => GovernsRef[];
}

const EMPTY: GovernsRef[] = [];
const GovernsContext = createContext<Governs | null>(null);

export function GovernsProvider({ children }: { children: ComponentChildren }) {
  const [byRef, setByRef] = useState<Record<string, GovernsRef[]>>({});
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api
      .getGovernsAll()
      .then((m) => {
        if (cancelled) return;
        setByRef(m || {});
        setReady(true);
      })
      .catch(() => {
        // 取得できなかったときは欄を出さない（従来の1件取得と同じ倒し方）。
        // 開示できないのは望ましくないが、壊れた件数を出すよりはまし。
        if (cancelled) return;
        setByRef({});
        setReady(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const value: Governs = {
    ready,
    entriesFor: (record) => byRef[governsKey(record)] ?? EMPTY,
  };
  return <GovernsContext.Provider value={value}>{children}</GovernsContext.Provider>;
}

export function useGoverns(): Governs {
  const ctx = useContext(GovernsContext);
  if (!ctx) throw new Error('useGoverns() must be called within a GovernsProvider');
  return ctx;
}
