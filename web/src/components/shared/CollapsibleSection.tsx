import { useState } from 'preact/hooks';
import type { ComponentChildren } from 'preact';
import { Icon } from './Icon';
import type { IconName } from './Icon';
import { loadCardSectionOpen, saveCardSectionOpen, defaultCardSectionOpen } from '../../collapseState';

interface Props {
  recordId: string;
  section: string;
  count: number;
  icon: IconName;
  label: string;
  extra?: ComponentChildren;
  // SpecCard の既存 isOpen（フォーカス時の初期展開）を初期値に取り込むための
  // 追加シード。マウント後は内部 state が開閉を持つ（loadCardSectionOpen 優先）。
  initialOpen?: boolean;
  onToggle?: () => void;
  children: ComponentChildren;
}

// F2: カード内セクション（意思決定/関連仕様/使用箇所）の開閉可能ヘッダ。
// ヘッダに保有件数を表示し、5件以上は既定で折りたたむ（TagCard/SpecCard/
// VocabCard の3箇所で共通利用）。開閉状態はレコード×セクション単位で
// localStorage に永続化（rail の折りたたみと同じパターン、キーは別名前空間）。
export function CollapsibleSection({ recordId, section, count, icon, label, extra, initialOpen, onToggle, children }: Props) {
  const [open, setOpen] = useState<boolean>(() => loadCardSectionOpen(recordId, section) ?? (initialOpen || defaultCardSectionOpen(count)));

  function toggle() {
    setOpen((prev) => {
      const next = !prev;
      saveCardSectionOpen(recordId, section, next);
      return next;
    });
    onToggle?.();
  }

  return (
    <div class="card-section">
      <div class="card-section-heading-row">
        <button type="button" class="card-section-toggle" onClick={toggle} aria-expanded={open}>
          <Icon name={open ? 'chevron-down' : 'chevron-right'} size={13} />
          <span class="card-section-heading">
            <Icon name={icon} size={14} /> {label} <span class="card-section-count dim">({count})</span>
          </span>
        </button>
        {extra}
      </div>
      {open && children}
    </div>
  );
}
