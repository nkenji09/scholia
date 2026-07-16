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
// Instead we generate a plain sequential node token per node and keep a
// token→transitionId map (txByToken), render to an SVG string, then walk
// the SVG DOM and attach a real click handler (calling onGoToTransition) to
// each mapped node before inserting it.
// Diagram source is built from .pmem-derived ids only (never free text), so
// no untrusted HTML reaches mermaid regardless.

// Build a flowchart definition plus a token→transitionId map for the clickable
// 結果 nodes. Gap nodes are non-clickable (there is no transition to link to
// — a total-gap is *missing* coverage) so they carry no map entry.
//
// Layout: a DECISION TREE over the declared axes (user design, #39 dogfood
// round 3) — one diamond per axis, branching Yes/No-style on that axis's
// value; transitions sharing the same axis values share the same path.
// report.cells already IS this tree's leaf set (the bounded product of
// declared axes computed server-side) — building the tree is just grouping
// `cells` by axis value one axis at a time (walk()), never re-deriving the
// analysis itself. At each leaf, every covering transition (cell.
// transitions) gets its own tail: any given condition NOT covered by a
// declared axis ("free"/don't-care, report.scope.undeclaredGiven) becomes a
// non-branching parallelogram passthrough step — still a real, visible
// precondition, just not part of the Yes/No decision structure — followed
// by that transition's 結果(then) node(s), the diagram's only clickable
// link → #/browse/tx/<id> (results, not the transition itself, are the
// link: a transition has no human label of its own, only its bare `T-xxx`
// id, and showing that as if it were a meaningful step read as noise — user
// feedback: "Transition のステップは表示の必要がない"). Each occurrence of an
// effect is its own node scoped to the transition that produced it (not
// deduped across transitions, unlike conditions), because a single click
// target must resolve to exactly one transition.
//
// overlap is matched per-leaf (cell) against report.overlaps' own per-cell
// transitions list, not just "this transition overlaps somewhere" — a
// transition free on some axis can appear at several leaves and only
// genuinely contends at some of them. subset-shadow is matched per shared
// leaf too: a proper-subset transition is looser on some axis than its
// superset (so it's "free" and appears at every leaf the superset does),
// so an edge is drawn at every leaf where both occur.
//
// When the action has no declared axes at all, there is no tree to build —
// falls back to the flat given→junction→then shape (every given condition
// is then implicitly "free" since no axis exists to structure it).
function buildDiagram(report: FlowReport, label: (id: string) => string): { def: string; txByToken: Map<string, string> } {
  const lines: string[] = ['flowchart TD'];
  const txByToken = new Map<string, string>();

  const subsetSubset = new Set(report.subsetShadows?.map((s) => s.subset) ?? []);
  const subsetSuperset = new Set(report.subsetShadows?.map((s) => s.superset) ?? []);
  const undeclared = new Set(report.scope?.undeclaredGiven ?? []);
  const rowById = new Map((report.matrix.rows ?? []).map((r) => [r.transitionId, r]));

  const esc = (s: string) => s.replace(/"/g, "'");
  let counter = 0;
  const nextId = (prefix: string) => `${prefix}${counter++}`;

  // transitionId -> (leafKey -> representative terminal node token), for
  // drawing subset-shadow edges between occurrences that share a leaf.
  const terminalsByTx = new Map<string, Map<string, string>>();

  // A free/undeclared-given passthrough chain, then this transition's 結果
  // node(s), hung off `fromToken` (a leaf junction or, in the no-axes
  // fallback, the transition's own hub). `overlapTxAtLeaf` scopes the amber
  // overlap highlight to just this occurrence.
  function renderTail(fromToken: string, leafKey: string, txId: string, overlapTxAtLeaf: Set<string>) {
    const row = rowById.get(txId);
    if (!row) return;
    let cur = fromToken;
    for (const g of row.given ?? []) {
      if (!undeclared.has(g)) continue;
      const ct = nextId('f');
      // Parallelogram (mermaid's `[/"text"/]` shape) — same shape as a
      // declared-axis precondition, but a plain single-in/single-out
      // passthrough (no branching): it's a real precondition, just outside
      // the Yes/No decision structure since it has no declared axis.
      lines.push(`  ${ct}[/"${esc(label(g))}"/]`);
      lines.push(`  class ${ct} condNode`);
      lines.push(`  ${cur} --> ${ct}`);
      cur = ct;
    }

    const isOverlap = overlapTxAtLeaf.has(txId);
    const isSubset = subsetSubset.has(txId);
    const isSuperset = subsetSuperset.has(txId);
    let terminal = cur;
    let ei = 0;
    for (const e of row.then ?? []) {
      const et = nextId('r');
      txByToken.set(et, txId);
      lines.push(`  ${et}["${esc(label(e))}"]`);
      // One `class` statement per class name — mermaid's flowchart `class`
      // directive embeds a comma-joined list as a single literal SVG class
      // token rather than splitting it into separate space-separated
      // classes, so a comma-joined list silently never matches any CSS
      // selector. Separate statements avoid that trap.
      const classes: string[] = ['effNode'];
      if (isOverlap) classes.push('overlapNode');
      if (isSubset) classes.push('subsetNode');
      if (isSuperset) classes.push('supersetNode');
      for (const c of classes) lines.push(`  class ${et} ${c}`);
      lines.push(`  ${cur} --> ${et}`);
      if (ei === 0) terminal = et;
      ei++;
    }

    if (!terminalsByTx.has(txId)) terminalsByTx.set(txId, new Map());
    terminalsByTx.get(txId)!.set(leafKey, terminal);
  }

  const axes = report.axes ?? [];
  const cells = report.cells ?? [];

  if (axes.length === 0 || cells.length === 0) {
    // No declared axes: every given condition is "free" by definition (no
    // axis exists to structure it) — flat given→junction→then per
    // transition, same shape as the axis-tree's own no-axis leaf tail.
    // Overlap has no per-cell scoping to consult here (there are no axes/
    // cells at all), so every transition that overlaps anywhere is flagged.
    const overlapAnywhere = new Set<string>();
    for (const o of report.overlaps ?? []) for (const t of o.transitions ?? []) overlapAnywhere.add(t);
    // Constant leafKey ('') for every transition here — there is no
    // cell/leaf concept without declared axes, so subset-shadow's
    // leaf-matching below (see the axis-tree branch) must treat every
    // transition as occurring at the same single "leaf" for pairs to match
    // and draw an edge at all (each transition's own id would never equal
    // another's, so a per-transition key would silently draw zero edges).
    for (const row of report.matrix.rows ?? []) {
      const hub = nextId('h');
      lines.push(`  ${hub}((" "))`);
      lines.push(`  class ${hub} txHub`);
      renderTail(hub, '', row.transitionId, overlapAnywhere);
    }
  } else {
    const overlapEntries = report.overlaps ?? [];
    const cellMatches = (a: Record<string, string>, b: Record<string, string> | undefined) => {
      if (!b) return false;
      for (const k of Object.keys(a)) if (a[k] !== b[k]) return false;
      return true;
    };
    const leafKeyOf = (values: Record<string, string>) =>
      Object.keys(values)
        .sort()
        .map((k) => `${k}=${values[k]}`)
        .join(';');

    // Group `cells` by axis value one axis at a time — report.cells is
    // already the full bounded product computed server-side (productCells),
    // so grouping it recreates the decision tree exactly; no re-derivation.
    function walk(subset: typeof cells, axisIndex: number, parentToken: string | null, edgeLabel: string | null) {
      if (axisIndex >= axes.length) {
        const cell = subset[0];
        if (!cell) return;
        const leafKey = leafKeyOf(cell.values ?? {});
        const leaf = nextId('h');
        lines.push(`  ${leaf}((" "))`);
        lines.push(`  class ${leaf} txHub`);
        if (parentToken) lines.push(`  ${parentToken} -->|"${esc(edgeLabel ?? '')}"| ${leaf}`);
        const overlapHere = new Set<string>();
        for (const o of overlapEntries) if (cellMatches(o.cell ?? {}, cell.values)) for (const t of o.transitions ?? []) overlapHere.add(t);
        for (const txId of cell.transitions ?? []) renderTail(leaf, leafKey, txId, overlapHere);
        return;
      }
      const axis = axes[axisIndex];
      const decision = nextId('d');
      lines.push(`  ${decision}{"${esc(axis.name)}"}`);
      lines.push(`  class ${decision} axisNode`);
      if (parentToken) lines.push(`  ${parentToken} -->|"${esc(edgeLabel ?? '')}"| ${decision}`);
      const byValue = new Map<string, typeof cells>();
      for (const cell of subset) {
        const v = cell.values?.[axis.id];
        if (v === undefined) continue;
        if (!byValue.has(v)) byValue.set(v, []);
        byValue.get(v)!.push(cell);
      }
      for (const [v, group] of byValue) walk(group, axisIndex + 1, decision, label(v));
    }
    walk(cells, 0, null, null);
  }

  // subset-shadow: a one-directional dotted edge superset → subset (proven
  // multi-fire: any world satisfying superset's given also fires subset),
  // drawn at every leaf where both occur (a subset transition is looser on
  // some axis, so it's "free" there and appears at every leaf its superset
  // does — see header comment). No per-edge text label — every
  // subset-shadow edge means the same thing, so a repeated label is just
  // noise; the single-arrowhead direction plus the legend carry the meaning
  // instead.
  for (const s of report.subsetShadows ?? []) {
    const supTerminals = terminalsByTx.get(s.superset);
    const subTerminals = terminalsByTx.get(s.subset);
    if (!supTerminals || !subTerminals) continue;
    for (const [leafKey, supToken] of supTerminals) {
      const subToken = subTerminals.get(leafKey);
      if (subToken) lines.push(`  ${supToken} -.-> ${subToken}`);
    }
  }

  // total-gaps: a distinct, non-clickable "missing coverage" marker node per
  // gap (there is no transition to click through to). Left as standalone
  // markers, not integrated into the tree — a gap is about one axis VALUE
  // never being specifically pinned by any transition, which doesn't map to
  // one specific tree path (other transitions can still cover that value's
  // cells by being "free" on that axis).
  let gi = 0;
  for (const g of report.totalGaps ?? []) {
    const token = `gap_${gi++}`;
    lines.push(`  ${token}["⚠ ${esc(`${label(g.value)}\n(${g.axisId})`)}"]`);
    lines.push(`  class ${token} gapNode`);
  }

  // No classDef color declarations here — mermaid's classDef style-value
  // grammar rejects CSS functions like var()/color-mix() (only plain
  // literals: hex colors, NUM/UNIT, etc.), so it can't express the app's
  // themed custom properties. `class <node> <name>` above still attaches
  // each name as a plain SVG class with no built-in style; the actual
  // colors (reusing the app's 前提/結果 grammar, light/dark aware) live in
  // flow.css targeting those class names directly — real CSS supports
  // var() natively, mermaid's DSL doesn't.

  return { def: lines.join('\n'), txByToken };
}

export function FlowView({ actionId, onGoToTransition }: Props) {
  const t = useT();
  // `ready` (not `vocabLabel` itself, which is a fresh closure every render)
  // is in the diagram effect's deps below: vocabLabel falls back to the raw
  // id until LookupsProvider's fetch resolves, so without this the diagram
  // — built once per `report` change — could permanently bake in ids instead
  // of labels if it renders before lookups finish loading.
  const { vocabLabel, ready: lookupsReady } = useLookups();
  const [report, setReport] = useState<FlowReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const diagramRef = useRef<HTMLDivElement>(null);
  const viewportRef = useRef<HTMLDivElement>(null);

  // Pan/zoom for the diagram (user feedback: mermaid's default layout can
  // outgrow the viewport for actions with many transitions). No new
  // dependency — a CSS transform on the canvas div plus wheel/drag handlers
  // is enough; mermaid's own SVG stays untouched, only its container moves.
  const [zoom, setZoom] = useState(1);
  // The fit-to-viewport zoom computed once per diagram render (see the
  // render effect below) — "reset" restores to this, not a hardcoded 1
  // (100%), since 100% can be an illegibly tiny fraction of the viewport
  // for a small diagram (mermaid sizes its <svg> tightly to content).
  const fitZoomRef = useRef(1);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const dragState = useRef<{ startX: number; startY: number; panX: number; panY: number } | null>(null);
  const ZOOM_MIN = 0.3;
  // ZOOM_MAX allows the fit-to-viewport zoom to go well past 100% for a
  // genuinely small diagram — see fitZoomRef above.
  const ZOOM_MAX = 6;
  const clampZoom = (z: number) => Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, z));
  const resetView = () => {
    setZoom(fitZoomRef.current);
    setPan({ x: 0, y: 0 });
  };
  const onWheelZoom = (e: WheelEvent) => {
    e.preventDefault();
    setZoom((z) => clampZoom(z - e.deltaY * 0.001));
  };
  const onPointerDownPan = (e: PointerEvent) => {
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    dragState.current = { startX: e.clientX, startY: e.clientY, panX: pan.x, panY: pan.y };
  };
  const onPointerMovePan = (e: PointerEvent) => {
    const drag = dragState.current;
    if (!drag) return;
    setPan({ x: drag.panX + (e.clientX - drag.startX), y: drag.panY + (e.clientY - drag.startY) });
  };
  const onPointerUpPan = () => {
    dragState.current = null;
  };

  // Reset pan/zoom whenever a new action's diagram loads — a stale
  // offset/scale from a previous action would otherwise carry over and the
  // new diagram could render off-screen or too small/large. This is a plain
  // 1/{0,0} clear, not resetView()/fitZoomRef — the new action's fit-zoom
  // isn't known yet (its diagram hasn't rendered), so there is nothing
  // meaningful to "reset" to until the render effect below computes it.
  useEffect(() => {
    setZoom(1);
    setPan({ x: 0, y: 0 });
  }, [actionId]);

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
    if (!host || !report || (report.matrix.rows ?? []).length === 0) return;
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

          // mermaid emits width="100%" (responsive) on the root <svg>, sized
          // against whatever containing block resolves to. `.flow-diagram-
          // canvas` is display:inline-block (shrink-to-fit, no explicit
          // width) so the SVG's percentage width has no definite containing
          // block to resolve against — the browser falls back to the CSS
          // spec's default replaced-element intrinsic size (300px), NOT the
          // diagram's actual content size. That silently produced a tiny
          // diagram regardless of zoom (user feedback: "why is 100% so
          // small"). Pinning the SVG to its own viewBox's pixel dimensions
          // gives it an unambiguous natural size, matching what the viewBox
          // numbers actually mean, so `scale(zoom)` below scales from the
          // diagram's true rendered size instead of that fallback default.
          const svgEl = diagramRef.current.querySelector('svg');
          const viewBox = svgEl?.getAttribute('viewBox');
          const viewportWidth = viewportRef.current?.clientWidth;
          if (svgEl && viewBox) {
            const parts = viewBox.split(/\s+/).map(Number);
            const naturalWidth = parts[2];
            const naturalHeight = parts[3];
            if (naturalWidth > 0 && naturalHeight > 0) {
              svgEl.style.width = `${naturalWidth}px`;
              svgEl.style.height = `${naturalHeight}px`;
              svgEl.removeAttribute('width');
              // Fit-to-width initial zoom, now measured against the real
              // natural size rather than the 300px fallback.
              if (viewportWidth) {
                const fit = clampZoom((viewportWidth - 32) / naturalWidth);
                fitZoomRef.current = fit;
                setZoom(fit);
              }
            }
          }

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
      .catch((err) => {
        console.error('FlowView: mermaid render failed', err);
        if (!cancelled && diagramRef.current) {
          diagramRef.current.textContent = t.flow.diagramError;
        }
      });
    return () => {
      cancelled = true;
    };
    // onGoToTransition is stable enough for this derived render. vocabLabel
    // itself is a new closure every render (not memoized), so it can't be a
    // dep directly without re-running on every render; `lookupsReady` is the
    // stable proxy for "vocabLabel now resolves real labels, not ids" (see
    // the useLookups() destructure comment above) — re-running once when it
    // flips true re-renders the diagram with resolved labels even if it
    // first rendered before lookups finished loading.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [report, lookupsReady]);

  if (!actionId) return <main class="flow-view dim">{t.flow.emptyAction}</main>;
  if (error) return <main class="flow-view error">{error}</main>;
  if (!report) return <main class="flow-view dim">{t.flow.loading}</main>;

  // Go's encoding/json marshals a nil (never-appended-to) slice field as JSON
  // `null`, not `[]` — internal/flow/analyze.go declares several slice
  // fields without `omitempty` (e.g. ScopeDisclosure.UndeclaredGiven,
  // MatrixRow.Given/Then) whose zero value is exactly that nil case (e.g. an
  // action with no undeclared given, or a transition with an empty given).
  // `?? []` at every read site defends against that without touching
  // analyze.go's analysis logic.
  const rows = report.matrix.rows ?? [];
  const conditions = report.matrix.conditions ?? [];
  const empty = rows.length === 0;
  const scope = report.scope;
  const declaredAxes = scope.declaredAxes ?? [];
  const undeclaredGiven = scope.undeclaredGiven ?? [];
  const outOfGuarantee = scope.outOfGuarantee ?? [];
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
              {conditions.length === 0 ? t.flow.matrixNoConditions : conditions.map((c) => vocabLabel(c)).join('、')}
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
                  {rows.map((row) => {
                    const given = row.given ?? [];
                    const then = row.then ?? [];
                    return (
                      <tr key={row.transitionId}>
                        <td>
                          <button type="button" class="flow-tx-link" onClick={() => onGoToTransition(row.transitionId)}>
                            {row.transitionId}
                          </button>
                        </td>
                        <td>{given.length ? given.map((g) => vocabLabel(g)).join(' ∧ ') : <span class="dim">{t.flow.noGiven}</span>}</td>
                        <td>{then.length ? then.map((th) => vocabLabel(th)).join('、') : <span class="dim">{t.flow.noResult}</span>}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </section>

          {/* 3. Mermaid diagram (rendered by the effect above). Edges carry no
              repeated per-edge text label (every subset-shadow edge means the
              same thing) — this legend states the convention once instead.
              Pan/zoom: wheel to zoom, drag to pan (user feedback — mermaid's
              own layout can outgrow the viewport for busy actions). */}
          <section class="flow-section">
            <div class="flow-diagram-toolbar">
              <h2 class="flow-section-heading">{t.flow.diagramHeading(report.actionLabel || report.action)}</h2>
              <div class="flow-zoom-controls">
                <button type="button" onClick={() => setZoom((z) => clampZoom(z - 0.2))} aria-label={t.flow.zoomOut}>
                  −
                </button>
                <span class="flow-zoom-level">{Math.round(zoom * 100)}%</span>
                <button type="button" onClick={() => setZoom((z) => clampZoom(z + 0.2))} aria-label={t.flow.zoomIn}>
                  ＋
                </button>
                <button type="button" onClick={resetView}>
                  {t.flow.zoomReset}
                </button>
              </div>
            </div>
            <div
              ref={viewportRef}
              class="flow-diagram-viewport"
              onWheel={onWheelZoom}
              onPointerDown={onPointerDownPan}
              onPointerMove={onPointerMovePan}
              onPointerUp={onPointerUpPan}
              onPointerLeave={onPointerUpPan}
            >
              <div class="flow-diagram-canvas" style={{ transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})` }}>
                <div ref={diagramRef} class="flow-diagram" />
              </div>
            </div>
            <div class="flow-diagram-legend">
              <span>
                <span class="flow-legend-swatch" style={{ color: 'var(--t-giv)' }} /> {t.flow.given}
              </span>
              <span>
                <span class="flow-legend-swatch" style={{ color: 'var(--t-then)' }} /> {t.flow.legendClickable}
              </span>
              <span>
                <span class="flow-legend-swatch" style={{ color: 'var(--lm-warning)' }} /> {t.flow.legendOverlap}
              </span>
              <span>{t.flow.legendSubsetShadow}</span>
            </div>
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
          {declaredAxes.length ? declaredAxes.join('、') : t.flow.scopeNone}
        </div>
        <div class="flow-scope-row">
          <span class="dim">{t.flow.scopeUndeclaredGiven}: </span>
          {undeclaredGiven.length ? undeclaredGiven.map((c) => vocabLabel(c)).join('、') : t.flow.scopeNone}
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
                  {t.flow.overlapRow(cellLabel(o.cell ?? {}), (o.transitions ?? []).join('、'))}
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
                  <li key={a.id}>{t.flow.axisRow(a.id, a.name, a.total, (a.values ?? []).map((v) => vocabLabel(v)).join('、'))}</li>
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
          {outOfGuarantee.map((line, i) => (
            <li key={i}>{line}</li>
          ))}
        </ul>
      </section>
    </main>
  );
}
