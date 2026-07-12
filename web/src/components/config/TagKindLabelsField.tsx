import { useT } from '../../i18n';
import { Icon } from '../shared/Icon';

interface Props {
  tagKinds: string[];
  labels: Record<string, string>;
  editable: boolean;
  onChange: (kind: string, label: string) => void;
}

// Additive display-label editor for tagKinds (2026-07-11 tweaks3 §2):
// tagKinds itself (TokenSetField above this in ConfigView) still owns which
// kinds exist — this only lets each one carry an optional Japanese display
// name. An empty input is a real "unset" (not a validation error): the
// central resolver (lookups.tsx's tagKindLabel) falls back to the bare kind
// id, and this field's own placeholder previews that fallback live.
export function TagKindLabelsField({ tagKinds, labels, editable, onChange }: Props) {
  const t = useT();
  return (
    <div class="config-field">
      <div class="config-field-head">
        <span class="config-field-icon">
          <Icon name="pencil" size={14} />
        </span>
        <span class="config-field-label">{t.config.tagKindLabelsField.label}</span>
        <span class="config-field-mono">tagKindLabels</span>
      </div>
      <p class="config-field-desc dim">{t.config.tagKindLabelsField.description}</p>
      {tagKinds.length === 0 ? (
        <span class="dim config-field-empty">{t.config.tagKindsUnset}</span>
      ) : (
        <div class="config-label-map">
          {tagKinds.map((kind) => (
            <div key={kind} class="config-label-map-row">
              <span class="config-field-mono config-label-map-kind">{kind}</span>
              {editable ? (
                <input
                  class="config-label-map-input"
                  value={labels[kind] || ''}
                  placeholder={kind}
                  onInput={(e) => onChange(kind, (e.target as HTMLInputElement).value)}
                />
              ) : (
                <span class="config-label-map-readonly">{labels[kind] || kind}</span>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
