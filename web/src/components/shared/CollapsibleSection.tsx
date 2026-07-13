import { useEffect, useRef, useState } from 'preact/hooks';
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
  // SpecCard の既存 isOpen（フォーカス時に true になる外部シグナル）を渡す
  // 口。マウント時の初期状態（localStorage 優先・無ければ件数しきい値）に
  // 織り込むほか、マウント後に false→true へ変化した場合（同一 SpecCard
  // インスタンスが後から focus される・#27 差し戻しレビュー1-3）も展開に
  // 同期する。true→false 方向へは同期しない（ユーザーの明示的な開閉・
  // localStorage 済みの状態を上書きしないため、一方向のみ）。
  focusOpen?: boolean;
  // マウント時の既定開閉を件数しきい値ではなく明示 bool で決める口（H1・
  // SpecCard「継承タグ」は件数に関わらず常に既定閉じ）。未指定なら従来通り
  // defaultCardSectionOpen(count)（5件以上で折りたたみ）。いずれの場合も
  // localStorage 済みのユーザー操作が最優先（＝一度開けば次回復元）。
  defaultOpen?: boolean;
  onToggle?: () => void;
  children: ComponentChildren;
}

// F2: カード内セクション（意思決定/関連仕様/使用箇所）の開閉可能ヘッダ。
// ヘッダに保有件数を表示し、5件以上は既定で折りたたむ（TagCard/SpecCard/
// VocabCard の3箇所で共通利用）。開閉状態はレコード×セクション単位で
// localStorage に永続化（rail の折りたたみと同じパターン、キーは別名前空間）。
export function CollapsibleSection({ recordId, section, count, icon, label, extra, focusOpen, defaultOpen, onToggle, children }: Props) {
  const [open, setOpen] = useState<boolean>(() => loadCardSectionOpen(recordId, section) ?? (focusOpen || (defaultOpen ?? defaultCardSectionOpen(count))));

  // focusOpen は「一方向の開シグナル」— マウント*後*に true へ変化した瞬間
  // だけ open へ反映する（同一ビュー内で別 spec へフォーカス移動し、既存
  // カードの isOpen が false→true になる経路・#27 差し戻しレビュー1-3）。
  // 初回マウントは意図的にスキップする: マウント時の focusOpen 値は上の
  // useState 初期化子が既に `localStorage ?? (focusOpen || threshold)` で
  // 織り込み済みなので、ここで二重に触ると、ユーザーが明示的に閉じて
  // localStorage に「閉じ」を保存済みのセクションを、focus 付き URL で
  // リロードした瞬間に開き直してしまう（＝閉じ状態の永続復元が壊れる・
  // 差し戻しレビュー2）。true→false 方向へも同期しない（一方向）。
  const didMountRef = useRef(false);
  useEffect(() => {
    if (!didMountRef.current) {
      didMountRef.current = true;
      return;
    }
    if (focusOpen) setOpen(true);
  }, [focusOpen]);

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
