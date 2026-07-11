package render

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nkenji09/product-memory/internal/diff"
	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/lint"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
	webdist "github.com/nkenji09/product-memory/web"
)

// staticData is baked into the exported page as `window.__PMEM_STATIC__`
// (§7 pmem export --html). Every field is produced by calling the exact
// same internal/index, internal/render, internal/lint functions the live
// HTTP API calls (single source of truth, §9) — export never reimplements
// derived-view logic, only decides which inputs (which tag ids, which
// transition ids) to precompute for, since a static page has no server to
// answer arbitrary queries against.
type staticData struct {
	Config           model.Config                      `json:"config"`
	Facets           facetsPayload                     `json:"facets"`
	Traceability     traceabilityPayload               `json:"traceability"`
	TransitionsByTag map[string]transitionsPayload     `json:"transitionsByTag"`
	TransitionDetail map[string]index.TransitionDetail `json:"transitionDetail"`
	SearchCorpus     []index.TransitionSearchDoc       `json:"searchCorpus"`
	Lint             lintPayload                       `json:"lint"`
	Spec             map[string]SpecReport             `json:"spec"`
	// Tags / Vocab mirror GET /api/tags (no kind filter) and GET /api/vocab
	// (no category filter) — the viewer views 語彙(vocab)/タグ階層 need the
	// full unfiltered lists to filter/group client-side, the same way the
	// live handlers already do (internal/viewer/facets.go), sorted by id for
	// stable output.
	Tags  []model.Tag        `json:"tags"`
	Vocab []model.VocabEntry `json:"vocab"`
	// Decisions mirrors GET /api/rules with no tag/tx/facet selector (§F of
	// .concierge/decision.md): every decision in the project, chronological
	// (index.SortedRulesFor's "no selector" case) — HOME's recent-decisions
	// widget needs this in the static export too, not just `pmem view`.
	Decisions []model.Decision `json:"decisions"`
}

// facetsPayload / transitionsPayload / traceabilityPayload / lintPayload
// mirror the JSON shape of internal/viewer's GET /api/facets, GET
// /api/transitions, GET /api/traceability, GET /api/lint responses exactly
// (same field names) so the frontend's existing TypeScript types decode
// static and live data identically — only the envelope struct is
// necessarily redeclared here (internal/render cannot import
// internal/viewer: viewer already imports render for GET /api/spec, and
// Go forbids import cycles); every field's *value* still comes from the
// shared internal/index / internal/lint functions, not from re-derived
// logic.
type facetsPayload struct {
	FacetKinds []string                         `json:"facetKinds"`
	Trees      map[string][]index.FacetTreeNode `json:"trees"`
}

type transitionsPayload struct {
	Transitions []model.Transition `json:"transitions,omitempty"`
}

type traceabilityPayload struct {
	Kinds   []string                  `json:"kinds"`
	Entries []index.TraceabilityEntry `json:"entries"`
}

type lintPayload struct {
	Findings   []lint.Finding `json:"findings"`
	ErrorCount int            `json:"errorCount"`
	WarnCount  int            `json:"warnCount"`
	InfoCount  int            `json:"infoCount"`
}

// collectStaticData bakes every resource the SPA's read-only views (Browse /
// Traceability / search) need. Transitions are precomputed per tag id
// reachable from a facet tree (plus "" for the unfiltered list) because
// that's the only shape the UI ever requests (Sidebar → TransitionList); an
// arbitrary facet/kind combination is not part of the static contract since
// nothing in the SPA calls it (§7 static mode is read-only, not a full
// offline query engine).
func collectStaticData(s *store.Store) (staticData, error) {
	snap, err := s.LoadAll()
	if err != nil {
		return staticData{}, err
	}
	// Branch is live/derived (model.Config's doc comment), not part of
	// config.json — LoadAll() won't have set it, so it's baked in here from
	// whatever branch is checked out at export time (2026-07-11 tweaks5 §2).
	snap.Config.Branch = diff.CurrentBranch(filepath.Dir(s.Dir))
	ix := index.Build(&snap)

	facets := facetsPayload{FacetKinds: snap.Config.FacetKinds, Trees: map[string][]index.FacetTreeNode{}}
	for _, kind := range snap.Config.FacetKinds {
		facets.Trees[kind] = index.BuildFacetTreeNodes(ix.FacetTree(kind))
	}

	tagIDs := map[string]bool{"": true}
	var collectTagIDs func(nodes []index.FacetTreeNode)
	collectTagIDs = func(nodes []index.FacetTreeNode) {
		for _, n := range nodes {
			tagIDs[n.Tag.ID] = true
			collectTagIDs(n.Children)
		}
	}
	for _, tree := range facets.Trees {
		collectTagIDs(tree)
	}

	transitionsByTag := make(map[string]transitionsPayload, len(tagIDs))
	for id := range tagIDs {
		transitionsByTag[id] = transitionsPayload{Transitions: index.FilterTransitions(ix, ix.AllTransitions(), id, "")}
	}

	txDetail := make(map[string]index.TransitionDetail, len(ix.TransitionByID))
	for _, t := range ix.AllTransitions() {
		detail, ok, err := index.BuildTransitionDetail(&snap, ix, t.ID)
		if err != nil {
			return staticData{}, err
		}
		if ok {
			txDetail[t.ID] = detail
		}
	}

	traceEntries := index.Traceability(ix, snap.Config.TraceabilityKinds)
	if traceEntries == nil {
		traceEntries = []index.TraceabilityEntry{}
	}
	trace := traceabilityPayload{Kinds: snap.Config.TraceabilityKinds, Entries: traceEntries}

	findings := lint.Run(snap)
	if findings == nil {
		findings = []lint.Finding{}
	}
	var errorCount, warnCount, infoCount int
	for _, f := range findings {
		switch f.Severity {
		case lint.SeverityError:
			errorCount++
		case lint.SeverityWarn:
			warnCount++
		case lint.SeverityInfo:
			infoCount++
		}
	}

	spec := make(map[string]SpecReport, len(snap.Tags))
	for _, t := range snap.Tags {
		report, err := Spec(&snap, ix, t.ID)
		if err != nil {
			return staticData{}, err
		}
		spec[t.ID] = report
	}

	tags := append([]model.Tag{}, snap.Tags...)
	sort.Slice(tags, func(i, j int) bool { return tags[i].ID < tags[j].ID })
	vocab := append([]model.VocabEntry{}, snap.Vocab...)
	sort.Slice(vocab, func(i, j int) bool { return vocab[i].ID < vocab[j].ID })

	decisions, err := index.SortedRulesFor(&snap, "", "", "")
	if err != nil {
		return staticData{}, err
	}
	if decisions == nil {
		decisions = []model.Decision{}
	}

	return staticData{
		Config:           snap.Config,
		Facets:           facets,
		Traceability:     trace,
		TransitionsByTag: transitionsByTag,
		TransitionDetail: txDetail,
		SearchCorpus:     index.SearchCorpus(ix),
		Lint:             lintPayload{Findings: findings, ErrorCount: errorCount, WarnCount: warnCount, InfoCount: infoCount},
		Spec:             spec,
		Tags:             tags,
		Vocab:            vocab,
		Decisions:        decisions,
	}, nil
}

var (
	moduleScriptRe   = regexp.MustCompile(`<script[^>]*type="module"[^>]*src="([^"]+)"[^>]*></script>`)
	stylesheetLinkRe = regexp.MustCompile(`<link[^>]*rel="stylesheet"[^>]*href="([^"]+)"[^>]*/?>`)
	scriptCloseRe    = regexp.MustCompile(`(?i)</script`)
	styleCloseRe     = regexp.MustCompile(`(?i)</style`)
)

// ExportHTML writes a self-contained static export of the viewer to dir: a
// single index.html with the SPA's JS/CSS and the derived data above all
// inlined (§7 "自己完結の静的HTML・サーバ不要").
//
// The dist/*.html produced by `npm run build` references its JS/CSS as
// separate files via absolute paths (`<script type="module" src="/assets/
// ...">`, `<link ... href="/assets/...">`) for the HTTP-served case
// (pmem view). Opening that file directly via file:// fails in Chrome:
// verified empirically — an absolute-path fetch resolves against the
// filesystem root, not the HTML file's directory, and a `type="module"`
// script (even once the path resolves) is blocked by Chrome's CORS policy
// for the file: scheme ("Cross origin requests are only supported for
// protocol schemes: ... http, https ..."). Inlining the JS as a same-document
// `<script type="module">` (no src) and the CSS as `<style>` sidesteps both:
// neither triggers a network fetch, so file: CORS never applies. This is why
// export produces one inlined index.html rather than dist's assets +
// index.html + a separate data.js, even though multiple files would also
// have worked when served over HTTP/GitHub Pages.
func ExportHTML(s *store.Store, dir string) error {
	data, err := collectStaticData(s)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// A tag/vocab/decision label containing the literal text "</script>"
	// would otherwise prematurely close the data <script> tag it's embedded
	// in; `<\/script` is valid JS (an ordinary string) and terminates
	// identically to `</script` once parsed, so this is behavior-preserving.
	payload = scriptCloseRe.ReplaceAll(payload, []byte(`<\/script`))

	distFS, err := fs.Sub(webdist.FS, "dist")
	if err != nil {
		return err
	}
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return err
	}

	html := string(indexHTML)

	if loc := stylesheetLinkRe.FindStringSubmatchIndex(html); loc != nil {
		cssPath := strings.TrimPrefix(html[loc[2]:loc[3]], "/")
		css, err := fs.ReadFile(distFS, cssPath)
		if err != nil {
			return fmt.Errorf("export: dist の CSS %q を読み込めません: %w", cssPath, err)
		}
		css = styleCloseRe.ReplaceAll(css, []byte(`<\/style`))
		html = html[:loc[0]] + "<style>\n" + string(css) + "\n</style>" + html[loc[1]:]
	}

	jsLoc := moduleScriptRe.FindStringSubmatchIndex(html)
	if jsLoc == nil {
		return fmt.Errorf("export: dist/index.html に <script type=%q src=...> が見つかりません", "module")
	}
	jsPath := strings.TrimPrefix(html[jsLoc[2]:jsLoc[3]], "/")
	js, err := fs.ReadFile(distFS, jsPath)
	if err != nil {
		return fmt.Errorf("export: dist の JS %q を読み込めません: %w", jsPath, err)
	}
	js = scriptCloseRe.ReplaceAll(js, []byte(`<\/script`))

	dataScript := "<script>\nwindow.__PMEM_STATIC__ = " + string(payload) + ";\n</script>\n"
	moduleScript := "<script type=\"module\">\n" + string(js) + "\n</script>"
	html = html[:jsLoc[0]] + dataScript + moduleScript + html[jsLoc[1]:]

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644)
}
