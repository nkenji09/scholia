import { strings } from '../strings';

interface Props {
  actionLabel: string;
  givenLabels?: string[];
  thenLabels?: string[];
  /** compact: single line for list rows. full: labeled trigger/given/result rows for detail/spec. */
  mode?: 'compact' | 'full';
}

// The record model's grammar is fixed (action/given/then — DESIGN §2), but
// "WHEN action GIVEN given THEN then" reads like a spec-language token
// dump, not prose a project owner would read comfortably. This renders the
// same three slots as きっかけ(trigger)/前提(given)/結果(result) — each
// slot's color is the one signal this page uses for structure: it always
// means "which grammatical role", never decoration (調整4).
export function TransitionFlow({ actionLabel, givenLabels, thenLabels, mode = 'full' }: Props) {
  if (mode === 'compact') {
    return (
      <span class="tx-flow-compact">
        <span class="tx-flow-compact-trigger">{actionLabel}</span>
        {thenLabels && thenLabels.length > 0 && (
          <span class="tx-flow-compact-result dim">→ {thenLabels.join('、')}</span>
        )}
      </span>
    );
  }

  return (
    <dl class="tx-flow">
      <div class="tx-flow-row tx-flow-trigger">
        <dt>{strings.flow.trigger}</dt>
        <dd>{actionLabel}</dd>
      </div>
      {givenLabels && givenLabels.length > 0 && (
        <div class="tx-flow-row tx-flow-given">
          <dt>{strings.flow.given}</dt>
          <dd>{givenLabels.join('、')}</dd>
        </div>
      )}
      <div class="tx-flow-row tx-flow-result">
        <dt>{strings.flow.result}</dt>
        <dd>{thenLabels && thenLabels.length > 0 ? thenLabels.join(' → ') : strings.flow.noResult}</dd>
      </div>
    </dl>
  );
}
