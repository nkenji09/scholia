import { useT } from '../../i18n';
import { Chip } from '../shared/Chip';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';

interface Props {
  label: string;
  mono: string;
  icon: IconName;
  description: string;
  values: string[];
  editable: boolean;
  /** Field name this set is conventionally a subset of (e.g. facetKinds ⊆ tagKinds) — hint only, not enforced. */
  subsetOf?: string;
  isSubsetMember?: (value: string) => boolean;
  onAdd?: (value: string) => void;
  onRemove?: (value: string) => void;
}

// Renders one config.json field as a chip/token set (claude-design-request-
// settings.md's core ask: a collection reads as a collection, not a comma
// string) with its own inline description and, for fields conventionally a
// subset of another (facetKinds/traceabilityKinds ⊆ tagKinds), a gentle
// non-blocking deviation hint rather than hard validation.
export function TokenSetField({ label, mono, icon, description, values, editable, subsetOf, isSubsetMember, onAdd, onRemove }: Props) {
  const t = useT();
  const deviates = subsetOf && isSubsetMember ? values.some((v) => !isSubsetMember(v)) : false;

  return (
    <div class="config-field">
      <div class="config-field-head">
        <span class="config-field-icon">
          <Icon name={icon} size={14} />
        </span>
        <span class="config-field-label">{label}</span>
        <span class="config-field-mono">{mono}</span>
        {subsetOf && <span class="config-field-subset dim">⊆ {subsetOf}</span>}
      </div>
      <p class="config-field-desc dim">{description}</p>
      <div class="config-field-chips">
        {values.length === 0 && <span class="dim config-field-empty">{t.config.unsetPlaceholder}</span>}
        {values.map((v) => {
          const warn = subsetOf && isSubsetMember ? !isSubsetMember(v) : false;
          return (
            <Chip key={v} color={warn ? 'var(--lm-warning-ic)' : 'var(--lm-text-dim)'} onRemove={editable && onRemove ? () => onRemove(v) : undefined}>
              {v}
            </Chip>
          );
        })}
        {editable && onAdd && (
          <input
            class="config-field-input"
            placeholder={t.config.addPlaceholder}
            onKeyDown={(e) => {
              if (e.key !== 'Enter') return;
              e.preventDefault();
              const input = e.target as HTMLInputElement;
              const value = input.value.trim();
              if (value) onAdd(value);
              input.value = '';
            }}
          />
        )}
      </div>
      {deviates && subsetOf && (
        <span class="config-field-warning">
          {t.config.subsetWarningBefore}
          <span class="config-field-mono">{subsetOf}</span>
          {t.config.subsetWarningAfter}
        </span>
      )}
    </div>
  );
}
