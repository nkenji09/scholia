import { useEffect, useMemo, useState } from 'preact/hooks';
import { api } from '../api';
import { strings } from '../strings';
import type { Decision, SpecReport, Tag } from '../types';
import { Markdown } from './Markdown';
import { TransitionFlow } from './TransitionFlow';

interface Props {
  selectedTagId?: string;
  onSelectTag: (id: string | undefined) => void;
  onSelectTx: (id: string) => void;
}

function dedupeDecisions(decisions: Decision[]): Decision[] {
  const seen = new Set<string>();
  const out: Decision[] = [];
  for (const d of decisions) {
    if (seen.has(d.id)) continue;
    seen.add(d.id);
    out.push(d);
  }
  return out;
}

function DecisionList({ decisions }: { decisions: Decision[] }) {
  if (decisions.length === 0) return null;
  return (
    <ul class="spec-decision-list">
      {decisions.map((d) => (
        <li key={d.id}>
          {d.why}
          {d.ref && ` (${d.ref})`}
        </li>
      ))}
    </ul>
  );
}

export function SpecView({ selectedTagId, onSelectTag, onSelectTx }: Props) {
  const [tags, setTags] = useState<Tag[] | null>(null);
  const [filter, setFilter] = useState('');
  const [report, setReport] = useState<SpecReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reportError, setReportError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getTags()
      .then(setTags)
      .catch((err) => setError(String(err)));
  }, []);

  useEffect(() => {
    if (!selectedTagId) {
      setReport(null);
      return;
    }
    setReportError(null);
    api
      .getSpec(selectedTagId)
      .then(setReport)
      .catch((err) => setReportError(String(err)));
  }, [selectedTagId]);

  const filteredTags = useMemo(() => {
    if (!tags) return [];
    const q = filter.trim().toLowerCase();
    const list = q ? tags.filter((t) => t.id.toLowerCase().includes(q) || t.name.toLowerCase().includes(q)) : tags;
    return list.slice().sort((a, b) => a.id.localeCompare(b.id));
  }, [tags, filter]);

  const tagRules = report ? dedupeDecisions(report.entries.flatMap((e) => e.decisions?.filter((d) => d.target.type === 'tag') || [])) : [];

  return (
    <main class="spec-view">
      <h2>{strings.spec.heading}</h2>
      <p class="dim">{strings.spec.intro}</p>
      {error && <p class="error">{error}</p>}
      <input
        class="search-input spec-tag-filter"
        type="text"
        placeholder={strings.spec.searchPlaceholder}
        value={filter}
        onInput={(e) => setFilter((e.target as HTMLInputElement).value)}
      />
      <ul class="spec-tag-picker">
        {filteredTags.map((t) => (
          <li key={t.id}>
            <button
              type="button"
              class={'tag-node' + (t.id === selectedTagId ? ' selected' : '')}
              title={t.id}
              onClick={() => onSelectTag(t.id)}
            >
              {t.name || t.id}
            </button>
          </li>
        ))}
      </ul>

      {!selectedTagId && <p class="dim">{strings.spec.pickTag}</p>}
      {reportError && <p class="error">{reportError}</p>}

      {report && (
        <section class="spec-report">
          <header class="spec-report-header">
            <h3>{report.tag.name || report.tag.id}</h3>
            <p class="dim" title="内部 id（リンクのキー。参照時のみ使用）">
              {report.tag.id}
            </p>
            <Markdown text={report.tag.description} />
          </header>

          {tagRules.length > 0 && (
            <section class="spec-tag-rules">
              <h4>{strings.spec.tagRules}</h4>
              <DecisionList decisions={tagRules} />
            </section>
          )}

          {report.entries.length === 0 && <p class="dim">{strings.spec.noEntries}</p>}

          <ul class="spec-entry-list">
            {report.entries.map((e) => {
              const txRules = e.decisions?.filter((d) => d.target.type === 'transition') || [];
              return (
                <li key={e.transition.id} class="spec-entry">
                  <button
                    type="button"
                    class="spec-entry-open"
                    title={`${strings.spec.openInBrowse} (${e.transition.id})`}
                    onClick={() => onSelectTx(e.transition.id)}
                  >
                    {strings.spec.openInBrowse}
                  </button>
                  <TransitionFlow actionLabel={e.actionLabel} givenLabels={e.givenLabels} thenLabels={e.thenLabels} />
                  {e.transition.tests && e.transition.tests.length > 0 && (
                    <p class="dim spec-entry-tests">
                      {strings.spec.tests}: {e.transition.tests.join(', ')}
                    </p>
                  )}
                  <DecisionList decisions={txRules} />
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </main>
  );
}
