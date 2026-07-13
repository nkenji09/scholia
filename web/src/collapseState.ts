// カードセクション（意思決定/関連仕様/使用箇所 等）の開閉状態の永続化。
// BrowseView の見出しリール折りたたみ（COLLAPSE_KEY_PREFIX='pmem-browse-collapse-'）
// と同じ「localStorage・private-mode/quota失敗は黙って諦める」パターンを
// レコード×セクション単位に踏襲する（BrowseView.tsx 自体は変更しない）。
const CARD_COLLAPSE_KEY_PREFIX = 'pmem-card-collapse-';

// F2要件: 保有件数が5件以上のとき既定で閉じる（4件以下は既定で開く）。
export const CARD_SECTION_COLLAPSE_THRESHOLD = 5;

function storageKey(recordId: string, section: string): string {
  return `${CARD_COLLAPSE_KEY_PREFIX}${recordId}-${section}`;
}

/** ユーザーが明示的に開閉した保存値。未保存なら null（呼び出し側が件数から既定値を決める）。 */
export function loadCardSectionOpen(recordId: string, section: string): boolean | null {
  try {
    const raw = localStorage.getItem(storageKey(recordId, section));
    if (raw === '1') return true;
    if (raw === '0') return false;
    return null;
  } catch {
    return null;
  }
}

export function saveCardSectionOpen(recordId: string, section: string, open: boolean): void {
  try {
    localStorage.setItem(storageKey(recordId, section), open ? '1' : '0');
  } catch {
    // Private-mode/quota failures still work for this session — just don't persist.
  }
}

export function defaultCardSectionOpen(count: number): boolean {
  return count < CARD_SECTION_COLLAPSE_THRESHOLD;
}
