import { useEffect, useMemo, useState } from 'preact/hooks';
import { api } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import type { Decision, SupersedeLink } from '../../types';
import { Icon } from '../shared/Icon';
import { Markdown } from '../Markdown';
import { DecisionIdReveal } from './DecisionIdReveal';
import { buildCurrencyIndex, currencyOf, linkMode, type Currency } from './decisionModel';

interface Props {
  decisionId?: string;
  onBack: () => void;
  onOpenDecision: (id: string) => void;
}

function isUrl(s: string): boolean {
  return /^https?:\/\//i.test(s.trim());
}

export function DecisionDetailView({ decisionId, onBack, onOpenDecision }: Props) {
  const t = useT();
  const { tagName, vocabLabel, transitionLabel, formatDecisionAt } = useLookups();
  const [decisions, setDecisions] = useState<Decision[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getRules({})
      .then((r) => setDecisions(r.decisions))
      .catch((err) => setError(String(err)));
  }, []);

  const byId = useMemo(() => new Map((decisions || []).map((d) => [d.id, d])), [decisions]);
  const currencyIndex = useMemo(() => buildCurrencyIndex(decisions || []), [decisions]);

  const targetLabel = (d: Decision): string => {
    if (d.target.type === 'tag') return tagName(d.target.id);
    if (d.target.type === 'vocab') return vocabLabel(d.target.id);
    return transitionLabel(d.target.id).primary;
  };
  const targetPrefix = (type: Decision['target']['type']): string =>
    type === 'tag' ? t.decisions.targetPrefixTag : type === 'vocab' ? t.decisions.targetPrefixVocab : t.decisions.targetPrefixTransition;
  const modeLabel = (mode: string | undefined): string => {
    const m = linkMode(mode);
    return m === 'supersede' ? t.decisions.modeSupersede : m === 'exception' ? t.decisions.modeException : t.decisions.modeAmend;
  };
  // 効力は2値（条項1）。「改訂」を状態として並べない——amend を付けられた
  // decision はまだ効いており、下の「この意思決定を置き換え/改訂」欄が
  // その関係（付帯情報・条項2/7）を明示的な導線として持っている。
  const currencyLabel = (c: Currency): string =>
    c === 'superseded' ? t.decisions.effectReplaced : t.decisions.effectInForce;
  const currencyCls = (c: Currency): string =>
    c === 'superseded' ? 'decision-badge-superseded' : 'decision-badge-current';

  if (error) return <main class="decision-detail-view error">{error}</main>;
  if (!decisions) return <main class="decision-detail-view dim">{t.decisions.loading}</main>;

  const decision = decisionId ? byId.get(decisionId) : undefined;
  if (!decision) {
    return (
      <main class="decision-detail-view">
        <button type="button" class="decision-back" onClick={onBack}>
          <Icon name="arrow-right" size={14} /> {t.decisions.backToList}
        </button>
        <p class="dim decisions-empty">{t.decisions.notFound}</p>
      </main>
    );
  }

  const cur = currencyOf(decision.id, currencyIndex);
  // Superseded/amended-by is DERIVED: decisions whose supersedes[] links here.
  const supersededBy = currencyIndex.supersededByMap.get(decision.id) || [];
  const supersedes = decision.supersedes || [];

  // A supersedes/superseded-by link chip: label resolves to the linked
  // decision's own target when we can find it, else the bare id.
  const linkChipLabel = (id: string): string => {
    const linked = byId.get(id);
    return linked ? `${targetPrefix(linked.target.type)} ${targetLabel(linked)}` : id;
  };

  return (
    <main class="decision-detail-view">
      <button type="button" class="decision-back" onClick={onBack}>
        <Icon name="arrow-right" size={14} /> {t.decisions.backToList}
      </button>

      <header class="decision-detail-header">
        <div class="decision-detail-title-row">
          <span class="decision-detail-target">
            <span class="decision-detail-target-kind dim">{targetPrefix(decision.target.type)}</span>
            {targetLabel(decision)}
          </span>
          <span class={'decision-badge ' + currencyCls(cur)}>{currencyLabel(cur)}</span>
        </div>
        <div class="decision-detail-meta dim">
          <span>{formatDecisionAt(decision.at)}</span>
        </div>
        <DecisionIdReveal id={decision.id} />
      </header>

      <section class="decision-detail-section">
        <h2 class="decision-detail-heading">{t.decisions.whyHeading}</h2>
        <Markdown text={decision.why} class="decision-detail-why" />
      </section>

      {decision.changed && decision.changed.trim() && (
        <section class="decision-detail-section">
          <h2 class="decision-detail-heading">{t.decisions.changedHeading}</h2>
          <Markdown text={decision.changed} class="decision-detail-changed" />
        </section>
      )}

      {decision.ref && (
        <section class="decision-detail-section">
          <h2 class="decision-detail-heading">{t.decisions.refHeading}</h2>
          {isUrl(decision.ref) ? (
            <a class="decision-detail-ref-link" href={decision.ref} target="_blank" rel="noopener noreferrer">
              {decision.ref} <Icon name="external-link" size={12} />
            </a>
          ) : (
            <p class="decision-detail-ref">{decision.ref}</p>
          )}
        </section>
      )}

      {(decision.commits || []).length > 0 && (
        <section class="decision-detail-section">
          <h2 class="decision-detail-heading">{t.decisions.commitsHeading}</h2>
          <div class="decision-chip-row">
            {(decision.commits || []).map((c) => (
              <span key={c} class="decision-commit-chip" title={c}>
                {c.slice(0, 8)}
              </span>
            ))}
          </div>
        </section>
      )}

      {(decision.acknowledges || []).length > 0 && (
        <section class="decision-detail-section">
          <h2 class="decision-detail-heading">{t.decisions.acknowledgesHeading}</h2>
          <div class="decision-chip-row">
            {(decision.acknowledges || []).map((a) => (
              <span key={a} class="decision-ack-chip">
                {a}
              </span>
            ))}
          </div>
        </section>
      )}

      {supersedes.length > 0 && (
        <section class="decision-detail-section">
          <h2 class="decision-detail-heading">{t.decisions.supersedesHeading}</h2>
          <div class="decision-chip-row">
            {supersedes.map((link: SupersedeLink) => (
              <button key={link.id} type="button" class="decision-link-chip" onClick={() => onOpenDecision(link.id)}>
                <span class="decision-link-mode">{modeLabel(link.mode)}</span>
                {linkChipLabel(link.id)}
              </button>
            ))}
          </div>
        </section>
      )}

      {supersededBy.length > 0 && (
        <section class="decision-detail-section">
          <h2 class="decision-detail-heading">{t.decisions.supersededByHeading}</h2>
          <div class="decision-chip-row">
            {supersededBy.map((d) => {
              const link = (d.supersedes || []).find((l) => l.id === decision.id);
              return (
                <button key={d.id} type="button" class="decision-link-chip" onClick={() => onOpenDecision(d.id)}>
                  <span class="decision-link-mode">{modeLabel(link?.mode)}</span>
                  {`${targetPrefix(d.target.type)} ${targetLabel(d)}`}
                </button>
              );
            })}
          </div>
        </section>
      )}
    </main>
  );
}
