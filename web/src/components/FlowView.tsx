import { useEffect, useRef, useState } from 'preact/hooks';
import { api } from '../api';
import { useT } from '../i18n';
import { useLookups } from '../lookups';
import type { FlowReport } from '../types';

interface Props {
  actionId?: string;
  onGoToTransition: (txId: string) => void;
}

// ── mermaid click-navigation approach: (a) post-process the rendered SVG ──
// We call mermaid.render() with securityLevel:'strict', identical to the
// shared Markdown.tsx. Strict is the ONLY value either caller ever passes, so
// even if FlowView and Markdown are mounted in the same document (a user
// edits the hash to #/flow/... in an already-open app tab) the global
// mermaid singleton config can never be left in a conflicting state — there
// is nothing to race. mermaid.render(id, def) in v11 takes no per-call config
// override (its signature is (id, text, container?)), so approach (b)'s
// native `click ... href` directive would require a global initialize() with
// a looser securityLevel — which is exactly the cross-mount race we avoid.
// Instead we encode each transition's id into a sanitized node token, render
// to an SVG string, then walk the SVG DOM and attach a real click handler
// (calling onGoToTransition) to each transition node before inserting it.
// Diagram source is built from .pmem-derived ids only (never free text), so
// no untrusted HTML reaches mermaid regardless.

// mermaid node ids must be simple identifiers; map an arbitrary id to a safe
// token and keep the reverse mapping so SVG post-processing can recover the
// real transition/gap id.
function sanitize(id: string): string {
  return id.replace(/[^a-zA-Z0-9]/g, '_');
}

// Build a flowchart definition plus a token→transitionId map for the clickable
// transition nodes. Gap nodes are non-clickable (there is no transition to
// link to — a total-gap is *missing* coverage) so they carry no map entry.
function buildDiagram(report: FlowReport, label: (id: string) => string): { def: string; txByToken: Map<string, string> } {
  const lines: string[] = ['flowchart TD'];
  const txByToken = new Map<string, string>();

  const subsetSubset = new Set(report.subsetShadows?.map((s) => s.subset) ?? []);
  const subsetSuperset = new Set(report.subsetShadows?.map((s) => s.superset) ?? []);
  const overlapTx = new Set<string>();
  for (const o of report.overlaps ?? []) for (const tx of o.transitions) overlapTx.add(tx);

  const esc = (s: string) => s.replace(/"/g, "'");

  // One node per transition of the action. Node label = short given→then
  // summary (human labels), never claiming coverage.
  for (const row of report.matrix.rows) {
    const token = 'tx_' + sanitize(row.transitionId);
    txByToken.set(token, row.transitionId);
    const given = row.given.length ? row.given.map(label).join(' ∧ ') : '∅';
    const then = row.then.length ? row.then.map(label).join(', ') : '—';
    lines.push(`  ${token}["${esc(`${given}\n→ ${then}`)}"]`);

    const classes: string[] = ['txNode'];
    if (overlapTx.has(row.transitionId)) classes.push('overlapNode');
    if (subsetSubset.has(row.transitionId)) classes.push('subsetNode');
    if (subsetSuperset.has(row.transitionId)) classes.push('supersetNode');
    lines.push(`  class ${token} ${classes.join(',')}`);
  }

  // subset-shadow: draw an edge superset → subset (proven multi-fire: any
  // world satisfying superset's given also fires subset).
  for (const s of report.subsetShadows ?? []) {
    const subset = 'tx_' + sanitize(s.subset);
    const superset = 'tx_' + sanitize(s.superset);
    lines.push(`  ${superset} -. "⊊ shadow" .-> ${subset}`);
  }

  // overlap: link the contending transitions of each cell with a dashed edge.
  for (const o of report.overlaps ?? []) {
    const txs = o.transitions;
    for (let i = 0; i + 1 < txs.length; i++) {
      const a = 'tx_' + sanitize(txs[i]);
      const b = 'tx_' + sanitize(txs[i + 1]);
      lines.push(`  ${a} <-. "overlap" .-> ${b}`);
    }
  }

  // total-gaps: a distinct, non-clickable "missing coverage" marker node per
  // gap (there is no transition to click through to).
  let gi = 0;
  for (const g of report.totalGaps ?? []) {
    const token = `gap_${gi++}`;
    lines.push(`  ${token}["⚠ ${esc(`${label(g.value)}\n(${g.axisId})`)}"]`);
    lines.push(`  class ${token} gapNode`);
  }

  // classDef styling — visually alarming (amber/red) for the honesty signals,
  // neutral for plain transition nodes.
  lines.push('  classDef txNode fill:#f2f2f5,stroke:#cbcbd4,color:#1a1a20;');
  lines.push('  classDef overlapNode fill:#fef3c7,stroke:#d97706,color:#7c2d12,stroke-width:2px;');
  lines.push('  classDef subsetNode fill:#fde8e8,stroke:#dc2626,color:#7f1d1d,stroke-dasharray:4 2;');
  lines.push('  classDef supersetNode fill:#f2f2f5,stroke:#dc2626,color:#1a1a20;');
  lines.push('  classDef gapNode fill:#fee2e2,stroke:#dc2626,color:#7f1d1d,stroke-width:2px;');

  return { def: lines.join('\n'), txByToken };
}

export function FlowView({ actionId, onGoToTransition }: Props) {
  const t = useT();
  const { vocabLabel } = useLookups();
  const [report, setReport] = useState<FlowReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const diagramRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!actionId) {
      setReport(null);
      setError(null);
      return;
    }
    let cancelled = false;
    setReport(null);
    setError(null);
    api
      .getFlow(actionId)
      .then((r) => {
        if (!cancelled) setReport(r);
      })
      .catch((err) => {
        if (!cancelled) setError(String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [actionId]);

  // Render the mermaid diagram (approach (a) — see the header comment): render
  // to an SVG string with securityLevel:'strict', inject it, then attach click
  // handlers to transition nodes via the token→txId map.
  useEffect(() => {
    const host = diagramRef.current;
    if (!host || !report || report.matrix.rows.length === 0) return;
    let cancelled = false;
    const { def, txByToken } = buildDiagram(report, vocabLabel);
    import('mermaid')
      .then(({ default: mermaid }) => {
        if (cancelled) return;
        mermaid.initialize({ startOnLoad: false, theme: 'neutral', securityLevel: 'strict' });
        // Unique id per render so mermaid's internal element ids don't collide
        // across re-renders in the same document.
        const renderId = 'flowview-' + Math.random().toString(36).slice(2);
        return mermaid.render(renderId, def).then(({ svg }) => {
          if (cancelled || !diagramRef.current) return;
          diagramRef.current.innerHTML = svg;
          // mermaid v11 gives each node <g> a domId of the form
          // "<diagramId>-flowchart-<nodeToken>-<n>" (the "<diagramId>-" prefix
          // comes from FlowDB.lookUpDomId when render() is passed an id, which
          // we do). Rather than depend on the exact affix scheme, match each
          // of our known transition tokens as a delimited substring of the
          // node id — robust to the diagramId prefix and any trailing index.
          const nodes = diagramRef.current.querySelectorAll<SVGGElement>('g.node');
          nodes.forEach((node) => {
            const raw = node.id || '';
            let hit: string | undefined;
            for (const [token, txId] of txByToken) {
              if (raw.includes('flowchart-' + token + '-') || raw.endsWith('flowchart-' + token)) {
                hit = txId;
                break;
              }
            }
            if (!hit) return;
            node.style.cursor = 'pointer';
            node.classList.add('flow-node-clickable');
            node.addEventListener('click', () => onGoToTransition(hit));
          });
        });
      })
      .catch(() => {
        if (!cancelled && diagramRef.current) {
          diagramRef.current.textContent = t.flow.diagramError;
        }
      });
    return () => {
      cancelled = true;
    };
    // vocabLabel/onGoToTransition are stable enough for this derived render;
    // report is the meaningful dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [report]);

  if (!actionId) return <main class="flow-view dim">{t.flow.emptyAction}</main>;
  if (error) return <main class="flow-view error">{error}</main>;
  if (!report) return <main class="flow-view dim">{t.flow.loading}</main>;

  const empty = report.matrix.rows.length === 0;
  const scope = report.scope;
  const subsetShadows = report.subsetShadows ?? [];
  const totalGaps = report.totalGaps ?? [];
  const overlaps = report.overlaps ?? [];
  const axes = report.axes ?? [];
  const remainder = report.remainder ?? [];

  const cellLabel = (cell: Record<string, string>) =>
    Object.keys(cell)
      .sort()
      .map((k) => `${k}=${vocabLabel(cell[k])}`)
      .join(', ');

  return (
    <main class="flow-view">
      <header class="flow-header">
        <h1>{t.flow.viewTitle(report.actionLabel || report.action)}</h1>
        <span class="dim flow-action-id">{report.action}</span>
      </header>

      {empty ? (
        <p class="dim flow-empty">{t.flow.emptyAction}</p>
      ) : (
        <>
          {/* 1. Matrix — every transition, every distinct given condition. */}
          <section class="flow-section">
            <h2 class="flow-section-heading">{t.flow.matrixHeading}</h2>
            <p class="flow-conditions">
              <span class="dim">{t.flow.matrixConditionsLabel}: </span>
              {report.matrix.conditions.length === 0
                ? t.flow.matrixNoConditions
                : report.matrix.conditions.map((c) => vocabLabel(c)).join('、')}
            </p>
            <div class="flow-matrix-scroll">
              <table class="flow-matrix">
                <thead>
                  <tr>
                    <th>#</th>
                    <th style={{ color: 'var(--t-giv)' }}>{t.flow.given}</th>
                    <th style={{ color: 'var(--t-then)' }}>{t.flow.result}</th>
                  </tr>
                </thead>
                <tbody>
                  {report.matrix.rows.map((row) => (
                    <tr key={row.transitionId}>
                      <td>
                        <button type="button" class="flow-tx-link" onClick={() => onGoToTransition(row.transitionId)}>
                          {row.transitionId}
                        </button>
                      </td>
                      <td>{row.given.length ? row.given.map((g) => vocabLabel(g)).join(' ∧ ') : <span class="dim">{t.flow.noGiven}</span>}</td>
                      <td>{row.then.length ? row.then.map((th) => vocabLabel(th)).join('、') : <span class="dim">{t.flow.noResult}</span>}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>

          {/* 3. Mermaid diagram (rendered by the effect above). */}
          <section class="flow-section">
            <h2 class="flow-section-heading">{t.flow.diagramHeading}</h2>
            <div ref={diagramRef} class="flow-diagram" />
          </section>
        </>
      )}

      {/* 2. Scope disclosure — ALWAYS rendered, even when every list is empty
          (honesty-first: never a bare "no gaps"). Signal counts appear next to
          what was actually checked. */}
      <section class="flow-section flow-scope">
        <h2 class="flow-section-heading">{t.flow.scopeHeading}</h2>

        <div class="flow-scope-row">
          <span class="dim">{t.flow.scopeDeclaredAxes}: </span>
          {scope.declaredAxes.length ? scope.declaredAxes.join('、') : t.flow.scopeNone}
        </div>
        <div class="flow-scope-row">
          <span class="dim">{t.flow.scopeUndeclaredGiven}: </span>
          {scope.undeclaredGiven.length ? scope.undeclaredGiven.map((c) => vocabLabel(c)).join('、') : t.flow.scopeNone}
        </div>
        {scope.hasRemainder && <div class="flow-scope-row">{t.flow.scopeHasRemainder}</div>}

        {/* subset-shadow / gaps / overlaps — captioned with their counts even
            when zero, so emptiness is stated, never silently omitted. */}
        <div class="flow-signal">
          <span class="flow-signal-heading">{t.flow.subsetShadowHeading(subsetShadows.length)}</span>
          {subsetShadows.length > 0 && (
            <ul>
              {subsetShadows.map((s) => (
                <li key={`${s.subset}<${s.superset}`} class="flow-signal-subset">
                  {t.flow.subsetShadowRow(s.subset, s.superset)}
                </li>
              ))}
            </ul>
          )}
        </div>

        <div class="flow-signal">
          <span class="flow-signal-heading">{t.flow.totalGapsHeading(totalGaps.length)}</span>
          {totalGaps.length > 0 && (
            <ul>
              {totalGaps.map((g) => (
                <li key={`${g.axisId}=${g.value}`} class="flow-signal-gap">
                  {t.flow.totalGapRow(g.axisId, vocabLabel(g.value))}
                </li>
              ))}
            </ul>
          )}
        </div>

        <div class="flow-signal">
          <span class="flow-signal-heading">{t.flow.overlapsHeading(overlaps.length)}</span>
          {overlaps.length > 0 && (
            <ul>
              {overlaps.map((o, i) => (
                <li key={i} class="flow-signal-overlap">
                  {t.flow.overlapRow(cellLabel(o.cell), o.transitions.join('、'))}
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Declared axes / cell count — listed here (not crammed into the
            diagram) so larger cell products stay readable. */}
        <div class="flow-signal">
          <span class="flow-signal-heading">{t.flow.axesHeading(axes.length)}</span>
          {axes.length === 0 ? (
            <p class="dim flow-axes-empty">{t.flow.axesEmpty}</p>
          ) : (
            <>
              <ul>
                {axes.map((a) => (
                  <li key={a.id}>{t.flow.axisRow(a.id, a.name, a.total, a.values.map((v) => vocabLabel(v)).join('、'))}</li>
                ))}
              </ul>
              <p class="dim">{t.flow.cellCountLabel(report.cells?.length ?? 0)}</p>
            </>
          )}
        </div>

        {remainder.length > 0 && (
          <div class="flow-signal">
            <span class="flow-signal-heading">{t.flow.remainderHeading(remainder.length)}</span>
            <ul>
              {remainder.map((r) => (
                <li key={r.transitionId}>
                  <button type="button" class="flow-tx-link" onClick={() => onGoToTransition(r.transitionId)}>
                    {r.transitionId}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}

        <ul class="flow-out-of-guarantee">
          {scope.outOfGuarantee.map((line, i) => (
            <li key={i}>{line}</li>
          ))}
        </ul>
      </section>
    </main>
  );
}
