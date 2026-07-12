import { useEffect, useRef, useState } from 'preact/hooks';
import { Icon } from '../shared/Icon';

export interface VocabPickerOption {
  id: string;
  label: string;
}

interface Props {
  options: VocabPickerOption[];
  onSelect: (id: string) => void;
  triggerLabel: string;
  searchPlaceholder: string;
  emptyLabel: string;
}

// 語彙ピッカー（change-cockpit-design-v3.md §3/M-3・#27 P3）— 既存 vocab/tag
// の中から選ぶだけの小さなポップオーバー。テキスト入力は渡された options を
// 絞り込む（narrow）だけで、入力値そのものを選択肢として確定する経路が無い
// ＝自由記述不可の構造ガードを UI 側でも徹底する（API 側の型ガードは
// internal/viewer/transition_write.go 参照）。options は呼び出し側が既に
// カテゴリ/未選択で絞ってから渡す（このコンポーネント自身は vocab/tag の
// 種類を知らない）。
export function VocabPicker({ options, onSelect, triggerLabel, searchPlaceholder, emptyLabel }: Props) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDocMouseDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', onDocMouseDown);
    return () => document.removeEventListener('mousedown', onDocMouseDown);
  }, [open]);

  const q = query.trim().toLowerCase();
  const matches = q ? options.filter((o) => (o.id + ' ' + o.label).toLowerCase().includes(q)) : options;

  const select = (id: string) => {
    onSelect(id);
    setOpen(false);
    setQuery('');
  };

  return (
    <div class="vocab-picker" ref={rootRef}>
      <button
        type="button"
        class="vocab-picker-trigger"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        <Icon name="plus" size={11} /> {triggerLabel} <Icon name="chevron-down" size={11} />
      </button>
      {open && (
        <div class="vocab-picker-popover">
          <input
            autoFocus
            class="vocab-picker-search"
            type="text"
            placeholder={searchPlaceholder}
            value={query}
            onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
          />
          <ul class="vocab-picker-list">
            {matches.map((o) => (
              <li key={o.id}>
                <button type="button" class="vocab-picker-option" onMouseDown={(e) => e.preventDefault()} onClick={() => select(o.id)}>
                  {o.label}
                </button>
              </li>
            ))}
            {matches.length === 0 && <li class="vocab-picker-empty dim">{emptyLabel}</li>}
          </ul>
        </div>
      )}
    </div>
  );
}
