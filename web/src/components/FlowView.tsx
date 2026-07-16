import { useEffect, useRef, useState } from 'preact/hooks';
import { api } from '../api';
import { useT } from '../i18n';
import { useLookups } from '../lookups';
import { routeHash } from '../router';
import type { FlowOverlap, FlowReport } from '../types';

interface Props {
  actionId?: string;
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
// the SVG DOM and attach a real click handler to each mapped node (opening
// #/browse/tx/<id> in a new tab — see the click-wiring code below) before
// inserting it.
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
function buildDiagram(
  report: FlowReport,
  label: (id: string) => string,
  resultLabel: string,
  coOccurLabel: string,
): { def: string; txByToken: Map<string, string> } {
  const lines: string[] = ['flowchart TD'];
  const txByToken = new Map<string, string>();

  const undeclared = new Set(report.scope?.undeclaredGiven ?? []);
  const rowById = new Map((report.matrix.rows ?? []).map((r) => [r.transitionId, r]));

  const esc = (s: string) => s.replace(/"/g, "'");
  let counter = 0;
  const nextId = (prefix: string) => `${prefix}${counter++}`;

  // transitionId -> every occurrence's {known axis values so far, terminal
  // node token} — used to draw subset-shadow edges between compatible
  // occurrences (see below; occurrences terminate at different depths now
  // that a transition exits as soon as its own pinned axes are resolved,
  // so a plain leaf-key equality check no longer applies).
  const terminalsByTx = new Map<string, Array<{ known: Record<string, string>; token: string }>>();

  // Two partial axis-value assignments are compatible if every axis known
  // to BOTH agrees — axes known to only one are simply unconstrained by the
  // other and don't block a match. This is what lets a transition that
  // exited early (fewer known axes) still pair up with one that kept
  // branching deeper (more known axes) for subset-shadow/overlap matching.
  const compatible = (a: Record<string, string>, b: Record<string, string>) => {
    for (const k of Object.keys(a)) if (k in b && a[k] !== b[k]) return false;
    return true;
  };

  // A free/undeclared-given passthrough chain, then this transition's 結果
  // node, hung off `fromToken` (a junction reached once this transition's
  // own relevant axes — if any — are resolved). `known` is the partial
  // axis-value assignment established so far on this path; `overlapEntries`
  // is matched against it with `compatible` (partial match) rather than
  // requiring every axis, since a transition that exited early never has
  // values for axes it doesn't care about.
  function renderTail(fromToken: string, known: Record<string, string>, txId: string, overlapEntries: FlowOverlap[]) {
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

    const isOverlap = overlapEntries.some((o) => (o.transitions ?? []).includes(txId) && compatible(o.cell ?? {}, known));

    // 結果 collapses to ONE clickable node per transition occurrence — not
    // one per effect (user feedback: showing every effect's full sentence
    // made the diagram noisy; the full given/then text is still available
    // via `pmem flow`, so the diagram only needs a link, not the content).
    // Subset-shadow gets no node color of its own (user feedback: red read
    // as "this is broken", but two transitions firing together because one
    // given is a proven subset of the other isn't an error — it's simply
    // undefined *priority*) — only overlap (genuine cross-transition
    // ambiguity) still gets the amber highlight. The subset-shadow edge
    // itself carries a "同時に発生" label instead (see below).
    const then = row.then ?? [];
    const nodeLabel = then.length > 1 ? `${resultLabel}（${then.length}件）` : resultLabel;
    const rt = nextId('r');
    txByToken.set(rt, txId);
    lines.push(`  ${rt}["${esc(nodeLabel)}"]`);
    // Separate `class` statements, not a space/comma-joined list — mermaid's
    // flowchart `class` directive embeds a joined list as one literal SVG
    // class token rather than splitting it, so it would silently never
    // match any CSS selector.
    lines.push(`  class ${rt} effNode`);
    if (isOverlap) lines.push(`  class ${rt} overlapNode`);
    lines.push(`  ${cur} --> ${rt}`);

    if (!terminalsByTx.has(txId)) terminalsByTx.set(txId, []);
    terminalsByTx.get(txId)!.push({ known, token: rt });
  }

  const axes = report.axes ?? [];

  if (axes.length === 0) {
    // No declared axes: every given condition is "free" by definition (no
    // axis exists to structure it) — flat given→junction→then per
    // transition, same shape as the axis-tree's own no-axis leaf tail.
    const overlapEntries = report.overlaps ?? [];
    for (const row of report.matrix.rows ?? []) {
      const hub = nextId('h');
      lines.push(`  ${hub}((" "))`);
      lines.push(`  class ${hub} txHub`);
      renderTail(hub, {}, row.transitionId, overlapEntries);
    }
  } else {
    const overlapEntries = report.overlaps ?? [];
    // Mirrors internal/flow/analyze.go's axisSpan: pinned to whichever of
    // the axis's values this transition's given actually lists; if none,
    // "free" — compatible with every value of that axis.
    const axisSpan = (row: { given?: string[] }, axis: (typeof axes)[number]) => {
      const pinned = axis.values.filter((v) => (row.given ?? []).includes(v));
      return pinned.length > 0 ? new Set(pinned) : new Set(axis.values);
    };
    const isPinnedOnAxis = (row: { given?: string[] }, axis: (typeof axes)[number]) => (row.given ?? []).some((g) => axis.values.includes(g));

    // Walk axes only as far as each transition actually needs. At every
    // step, transitions that don't pin ANY remaining axis exit immediately
    // (their 結果 hangs directly off the current junction) instead of being
    // dragged through irrelevant further branches — user feedback: a
    // transition pinning only one axis (act.user.update's `cond.update-
    // check-flag`, axis.update.mode alone) was appearing repeated under
    // every unrelated install/platform/status combination it doesn't care
    // about. Only transitions still pinning something ahead cause a branch;
    // an axis value with zero such transitions isn't drawn (nothing new to
    // show there — anything that already applied was attached at the
    // junction above, before branching, and applies to every value alike).
    function walk(pendingTxIds: string[], axisIndex: number, parentToken: string | null, edgeLabel: string | null, known: Record<string, string>) {
      const resolvedNow: string[] = [];
      const stillPending: string[] = [];
      for (const txId of pendingTxIds) {
        const row = rowById.get(txId);
        if (!row) continue;
        const pinsFurther = axes.slice(axisIndex).some((a) => isPinnedOnAxis(row, a));
        (pinsFurther ? stillPending : resolvedNow).push(txId);
      }

      const hub = nextId('h');
      lines.push(`  ${hub}((" "))`);
      lines.push(`  class ${hub} txHub`);
      if (parentToken) lines.push(`  ${parentToken} -->|"${esc(edgeLabel ?? '')}"| ${hub}`);
      for (const txId of resolvedNow) renderTail(hub, known, txId, overlapEntries);
      if (stillPending.length === 0) return;

      // Next axis at least one still-pending transition actually pins —
      // skips axes nobody here cares about instead of branching on them for
      // nothing.
      let axisIdx = axisIndex;
      while (axisIdx < axes.length && !stillPending.some((id) => isPinnedOnAxis(rowById.get(id)!, axes[axisIdx]))) axisIdx++;
      if (axisIdx >= axes.length) return;

      const axis = axes[axisIdx];
      const decision = nextId('d');
      lines.push(`  ${decision}{"${esc(axis.name)}"}`);
      lines.push(`  class ${decision} axisNode`);
      lines.push(`  ${hub} --> ${decision}`);
      for (const value of axis.values) {
        const childTxIds = stillPending.filter((id) => axisSpan(rowById.get(id)!, axis).has(value));
        if (childTxIds.length === 0) continue;
        walk(childTxIds, axisIdx + 1, decision, label(value), { ...known, [axis.id]: value });
      }
    }
    walk(
      (report.matrix.rows ?? []).map((r) => r.transitionId),
      0,
      null,
      null,
      {},
    );
  }

  // subset-shadow: a one-directional dotted edge superset → subset (proven
  // multi-fire: any world satisfying superset's given also fires subset),
  // drawn between every pair of compatible occurrences (a subset transition
  // is looser and typically exits earlier/shallower than its superset — see
  // `compatible` above). Labeled "同時に発生" (not an error — both results
  // are legitimate, they just always happen together with no declared
  // priority between them — user feedback after the plain-red styling read
  // as "this is broken").
  for (const s of report.subsetShadows ?? []) {
    const supOccurrences = terminalsByTx.get(s.superset);
    const subOccurrences = terminalsByTx.get(s.subset);
    if (!supOccurrences || !subOccurrences) continue;
    for (const sup of supOccurrences) {
      for (const sub of subOccurrences) {
        if (compatible(sup.known, sub.known)) lines.push(`  ${sup.token} -.->|"${esc(coOccurLabel)}"| ${sub.token}`);
      }
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

export function FlowView({ actionId }: Props) {
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
  // `dragging` tracks whether the pointer has moved past DRAG_THRESHOLD
  // since pointerdown — NOT set on pointerdown itself. Calling
  // setPointerCapture unconditionally on every pointerdown (the original
  // implementation) redirects the eventual pointerup/click to the
  // *capturing* element (the viewport) rather than whatever node is under
  // the pointer, per the Pointer Events spec — so a plain click (press,
  // release, no movement) never reached a node's own click listener at all
  // (user feedback: "単純にクリックしても遷移しない"). Capture is now only
  // engaged once real dragging is confirmed, so a plain click's pointerup/
  // click naturally targets and fires on the actual node underneath.
  const dragState = useRef<{ startX: number; startY: number; panX: number; panY: number; dragging: boolean } | null>(null);
  // Set for one tick after a real drag ends, so the click a browser may
  // still synthesize on release doesn't get misread as a navigation click
  // on whatever node happens to be under the pointer at that moment.
  const suppressClickRef = useRef(false);
  const DRAG_THRESHOLD = 4;
  const ZOOM_MIN = 0.3;
  // ZOOM_MAX allows the fit-to-viewport zoom to go well past 100% for a
  // genuinely small diagram — see fitZoomRef above.
  const ZOOM_MAX = 6;
  const clampZoom = (z: number) => Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, z));
  const resetView = () => {
    setZoom(fitZoomRef.current);
    setPan({ x: 0, y: 0 });
  };
  // Cursor-centered zoom (ctrl/cmd+wheel), matching the reference pattern
  // the user pointed at: the content-space point under the cursor is
  // computed BEFORE the zoom, then pan is adjusted so that same point stays
  // under the cursor AFTER the zoom — a plain scale-in-place (what this used
  // to do) makes the diagram visibly drift away from the cursor as you
  // zoom. A plain wheel (no modifier) is left alone entirely — no
  // preventDefault, no zoom — so scrolling the page past the diagram works
  // normally instead of being captured by the viewport.
  const onWheelZoom = (e: WheelEvent) => {
    if (!e.ctrlKey && !e.metaKey) return;
    e.preventDefault();
    const viewport = viewportRef.current;
    // Gentle, magnitude-normalized step (user feedback: the previous flat
    // ±8%-per-event step felt too fast and overshot). A single mouse-wheel
    // notch (|deltaY| ≈ 100) still moves at most ~3%; smaller trackpad
    // deltas move proportionally less, instead of the same big jump either
    // device sends.
    const step = Math.min(Math.abs(e.deltaY), 100) / 100;
    const factor = 1 + (e.deltaY > 0 ? -1 : 1) * step * 0.03;
    const newZoom = clampZoom(zoom * factor);
    if (newZoom === zoom) return;
    if (viewport) {
      const rect = viewport.getBoundingClientRect();
      const cursorX = e.clientX - rect.left;
      const cursorY = e.clientY - rect.top;
      const contentX = (cursorX - pan.x) / zoom;
      const contentY = (cursorY - pan.y) / zoom;
      setPan({ x: cursorX - contentX * newZoom, y: cursorY - contentY * newZoom });
    }
    setZoom(newZoom);
  };
  const onPointerDownPan = (e: PointerEvent) => {
    dragState.current = { startX: e.clientX, startY: e.clientY, panX: pan.x, panY: pan.y, dragging: false };
  };
  const onPointerMovePan = (e: PointerEvent) => {
    const drag = dragState.current;
    if (!drag) return;
    const dx = e.clientX - drag.startX;
    const dy = e.clientY - drag.startY;
    if (!drag.dragging) {
      if (Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
      drag.dragging = true;
      // Only now claim the pointer — a plain click never reaches this line.
      (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    }
    setPan({ x: drag.panX + dx, y: drag.panY + dy });
  };
  const onPointerUpPan = () => {
    if (dragState.current?.dragging) suppressClickRef.current = true;
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
    const { def, txByToken } = buildDiagram(report, vocabLabel, t.flow.result, t.flow.coOccur);
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
            // Diagram result nodes open in a new tab (user feedback) — this
            // is the only navigable content on the page (the matrix
            // table/scope-disclosure text sections were removed), and
            // opening in a new tab keeps the diagram itself in place while
            // checking transitions one at a time.
            node.addEventListener('click', () => {
              // A click that's really the tail end of a pan drag (see
              // suppressClickRef above) must not open a tab — consume the
              // flag and bail instead of navigating.
              if (suppressClickRef.current) {
                suppressClickRef.current = false;
                return;
              }
              window.open(routeHash({ view: 'browse', txId: hit }), '_blank', 'noopener');
            });
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
    // vocabLabel itself is a new closure every render (not memoized), so it
    // can't be a dep directly without re-running on every render;
    // `lookupsReady` is the
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
  const empty = rows.length === 0;

  return (
    <main class="flow-view">
      <header class="flow-header">
        <h1>{t.flow.viewTitle(report.actionLabel || report.action)}</h1>
        <span class="dim flow-action-id">{report.action}</span>
      </header>

      {empty ? (
        <p class="dim flow-empty">{t.flow.emptyAction}</p>
      ) : (
        // Mermaid diagram (rendered by the effect above) is this view's only
        // content — the matrix/scope-disclosure text sections that used to
        // sit alongside it were removed (decision 01KXN6G0R4DSXEVV86K8W0CZYW
        // amending T-viewer-action-flow-render's then to mermaid-only): the
        // fine print went unread in the viewer, and the same
        // req.action-flow.scope-honesty text is still always available via
        // `pmem flow`/`pmem gaps` for AI/CLI use. Edges carry no repeated
        // per-edge text label (every subset-shadow edge means the same
        // thing) — the legend states the convention once instead. Pan/zoom:
        // ctrl/cmd+wheel to zoom (cursor-centered), drag to pan.
        <section class="flow-section flow-diagram-section">
          <div class="flow-diagram-toolbar">
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
      )}
    </main>
  );
}
