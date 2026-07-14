import type { ComponentChildren, JSX } from 'preact';
import { useT } from '../../i18n';
import { Icon } from './Icon';

// Kind color mapping (design tokens --k-req/--k-sub/--k-con for tag kinds,
// --t-act/--t-giv/--t-then for vocab categories). Any kind/category not in
// this map falls back to --lm-text-dim rather than guessing a color, since
// tagKinds/facetKinds are project-configurable (§2 constitutive vs
// descriptive) — there is no fixed universal set to enumerate.
const KIND_COLOR: Record<string, string> = {
  requirement: 'var(--k-req)',
  subject: 'var(--k-sub)',
  concern: 'var(--k-con)',
  action: 'var(--t-act)',
  condition: 'var(--t-giv)',
  effect: 'var(--t-then)',
};

export function kindColor(kind: string | undefined): string {
  return (kind && KIND_COLOR[kind]) || 'var(--lm-text-dim)';
}

/** VocabEntry.owner pill color — not a tag kind, so it lives outside
    KIND_COLOR's kind→color map, on its own reserved token (tokens.css). */
export const OWNER_COLOR = 'var(--k-owner)';

interface Props {
  color?: string;
  onClick?: () => void;
  onRemove?: () => void;
  /** Design's + mark for chips whose click adds an AND search condition
      (distinct from a chip that merely navigates, e.g. TagCard's kind
      badge — the design reference doesn't mark those). */
  filterable?: boolean;
  title?: string;
  /** Opt-in slot rendered inside the pill, after children (e.g. SpecCard's ⋮
      affordance menu) — kept out of `children` so the "tag の外側にボタンを
      置かない" 原則 is the default for any future trailing content, not
      something each caller has to remember to nest correctly. Only wired up
      on the non-clickable (span) chip; no caller needs it on the button
      variant yet. */
  trailing?: ComponentChildren;
  children: ComponentChildren;
}

export function Chip({ color = 'var(--lm-text-dim)', onClick, onRemove, filterable, title, trailing, children }: Props) {
  const t = useT();
  const style = { '--chip-color': color } as JSX.CSSProperties;
  if (onClick) {
    return (
      <button type="button" class="chip chip-clickable" style={style} onClick={onClick} title={title}>
        {children}
        {filterable && <Icon name="plus" size={11} class="filter-plus-icon" />}
      </button>
    );
  }
  return (
    <span class="chip" style={style} title={title}>
      {children}
      {onRemove && (
        <button type="button" class="chip-remove" aria-label={t.common.remove} onClick={onRemove}>
          <Icon name="x" size={11} />
        </button>
      )}
      {trailing}
    </span>
  );
}
