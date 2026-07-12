import { useEffect, useState } from 'preact/hooks';
import { api } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import type { Config, Decision, Tag, TraceabilityResponse } from '../../types';
import { Icon } from '../shared/Icon';

interface Props {
  onGoTags: () => void;
  onSelectTag: (tagId: string) => void;
  onSelectTx: (txId: string) => void;
}

export function HomeView({ onGoTags, onSelectTag, onSelectTx }: Props) {
  const t = useT();
  const [tags, setTags] = useState<Tag[] | null>(null);
  const [config, setConfig] = useState<Config | null>(null);
  const [traceability, setTraceability] = useState<TraceabilityResponse | null>(null);
  const [decisions, setDecisions] = useState<Decision[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { tagName, transitionLabel, tagKindLabel, tagline, intro } = useLookups();

  useEffect(() => {
    Promise.all([api.getTags(), api.getConfig(), api.getTraceability(), api.getRules({})])
      .then(([t, cfg, trace, rules]) => {
        setTags(t);
        setConfig(cfg);
        setTraceability(trace);
        setDecisions(rules.decisions);
      })
      .catch((err) => setError(String(err)));
  }, []);

  if (error) return <main class="home-view error">{error}</main>;
  if (!tags || !config || !traceability || !decisions) return <main class="home-view dim">{t.home.loading}</main>;

  const kindCounts = config.tagKinds.map((kind) => ({
    kind,
    count: tags.filter((t) => t.kind === kind).length,
  }));

  const gapEntries = traceability.entries.filter((e) => e.gap);
  const totalEntries = traceability.entries.length;
  const satisfiedCount = totalEntries - gapEntries.length;
  const satisfiedPct = totalEntries > 0 ? Math.round((satisfiedCount / totalEntries) * 100) : 0;

  // index.SortedRulesFor (§F) sorts chronologically ascending; the widget
  // wants newest-first, so take the tail then reverse rather than asking
  // the API for a second, differently-sorted mode.
  const recentDecisions = decisions
    .slice(-5)
    .reverse()
    .map((d) => ({
      id: d.id,
      why: d.why,
      targetLabel: d.target.type === 'tag' ? tagName(d.target.id) : transitionLabel(d.target.id).primary,
      targetKind: d.target.type,
      onClick: () => (d.target.type === 'tag' ? onSelectTag(d.target.id) : onSelectTx(d.target.id)),
    }));

  return (
    <main class="home-view">
      <section class="home-hero">
        <h1>{tagline}</h1>
        <p class="dim">{intro}</p>
      </section>

      <section class="home-kind-cards">
        {kindCounts.map(({ kind, count }) => (
          <button key={kind} type="button" class="home-kind-card" onClick={onGoTags}>
            <span class="home-kind-card-label dim">{tagKindLabel(kind)}</span>
            <span class="home-kind-card-count">{t.home.tagCount(count)}</span>
          </button>
        ))}
      </section>

      <section class="home-grid">
        <div class="home-card">
          <div class="home-card-header">
            <span class="home-card-title">
              <Icon name="radar" size={15} /> {t.home.traceabilityHeading}
            </span>
            <button type="button" onClick={onGoTags}>
              {t.home.goTraceability} <Icon name="arrow-right" size={14} />
            </button>
          </div>
          <div class="home-traceability-stat">
            <span class="home-traceability-ratio">{t.home.satisfiedOf(satisfiedCount, totalEntries)}</span>
            <span class="dim">{t.home.satisfiedSuffix}</span>
          </div>
          <div class="home-traceability-bar">
            <span class="home-traceability-bar-fill" style={{ width: `${satisfiedPct}%` }} />
          </div>
          {gapEntries.length > 0 ? (
            <div class="home-gap">
              <span class="home-gap-heading">
                <Icon name="triangle-alert" size={14} /> {t.home.gapHeading(gapEntries.length)}
              </span>
              <div class="home-gap-chips">
                {gapEntries.map((e) => (
                  <button key={e.tag.id} type="button" class="home-gap-chip" onClick={() => onSelectTag(e.tag.id)}>
                    {e.tag.name || e.tag.id}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <p class="dim">{t.home.noGap}</p>
          )}
        </div>

        <div class="home-card">
          <div class="home-card-header">
            <span class="home-card-title">
              <Icon name="gavel" size={15} /> {t.home.recentDecisionsHeading}
            </span>
          </div>
          {recentDecisions.length === 0 ? (
            <p class="dim">{t.home.noDecisions}</p>
          ) : (
            <ul class="home-recent-list">
              {recentDecisions.map((d) => (
                <li key={d.id}>
                  <button type="button" class="home-recent-item" onClick={d.onClick}>
                    <span class="home-recent-target dim">{d.targetLabel}</span>
                    <span class="home-recent-why">{d.why}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>
    </main>
  );
}
