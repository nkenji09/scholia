package render

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// mermaid (loaded lazily via `import('mermaid')` in web/src/components/
// Markdown.tsx, kept out of the eager bundle by web/vite.config.ts) is
// itself internally code-split by Rolldown into dozens of per-diagram-type
// chunk files (assets/flowDiagram-*.js, assets/sequenceDiagram-*.js, ...).
// ExportHTML's single index.html has no sibling files at all — see its own
// doc comment — so every one of those chunks needs to be pulled into that
// one file too, not just the entry chunk. jsImportRes finds every relative
// `./x.js` reference — static `from "..."`, static side-effect-only
// `import "..."` (no `from`, e.g. a chunk that only needs another chunk's
// module-init side effects), or dynamic `import("...")` — in a chunk's
// source; Vite's build output is flat (every chunk directly under
// dist/assets/, verified against the actual build), so a bare `./name.js`
// sibling reference is the only shape ever produced — no `../`, no
// subdirectories. Three variants per pattern because Go's RE2 engine has no
// backreferences to force open/close quotes to match within one regex.
var jsImportRes = []*regexp.Regexp{
	regexp.MustCompile(`from"(\./[^"]+\.js)"`),
	regexp.MustCompile(`from'(\./[^']+\.js)'`),
	regexp.MustCompile("from`(\\./[^`]+\\.js)`"),
	regexp.MustCompile(`import"(\./[^"]+\.js)"`),
	regexp.MustCompile(`import'(\./[^']+\.js)'`),
	regexp.MustCompile("import`(\\./[^`]+\\.js)`"),
	regexp.MustCompile(`import\(\s*"(\./[^"]+\.js)"\s*\)`),
	regexp.MustCompile(`import\(\s*'(\./[^']+\.js)'\s*\)`),
	regexp.MustCompile("import\\(\\s*`(\\./[^`]+\\.js)`\\s*\\)"),
}

func jsImportSpecifiers(src []byte) []string {
	seen := map[string]bool{}
	var out []string
	for _, re := range jsImportRes {
		for _, m := range re.FindAllSubmatch(src, -1) {
			spec := string(m[1])
			if !seen[spec] {
				seen[spec] = true
				out = append(out, spec)
			}
		}
	}
	return out
}

// collectChunkGraph walks every chunk transitively reachable from entrySrc
// (which is itself included, keyed by entryKey) and returns each one's raw
// source keyed by the literal specifier string used to reference it.
func collectChunkGraph(distFS fs.FS, assetsDir, entryKey string, entrySrc []byte) (map[string]string, error) {
	chunks := map[string]string{entryKey: string(entrySrc)}
	queue := jsImportSpecifiers(entrySrc)
	for len(queue) > 0 {
		spec := queue[0]
		queue = queue[1:]
		if _, ok := chunks[spec]; ok {
			continue
		}
		name := strings.TrimPrefix(spec, "./")
		src, err := fs.ReadFile(distFS, path.Join(assetsDir, name))
		if err != nil {
			return nil, fmt.Errorf("export: dist のチャンク %q を読み込めません: %w", spec, err)
		}
		chunks[spec] = string(src)
		queue = append(queue, jsImportSpecifiers(src)...)
	}
	return chunks, nil
}

// jsChunkResolverTemplate is the (fixed, hand-written — %s/%s are the only
// holes) runtime counterpart of collectChunkGraph: given every reachable
// chunk's raw source keyed by its own specifier string, it turns each into
// a Blob URL, rewriting that chunk's own static `from "./x.js"` /
// `import "./x.js"` references to the target's Blob URL (resolved first,
// recursively — a static specifier has to be a literal string) and its
// dynamic `import("./x.js")` calls to a runtime lookup through the same
// resolver instead (see the inline comment on why:
// the real chunk graph has cycles running through a dynamic edge). Then it
// imports the entry the same way. `URL.createObjectURL` and dynamic
// `import()` of a blob: URL both work with zero network access, so the
// whole graph runs entirely offline — this is what makes code-split output
// line up with ExportHTML's single-file constraint. Uses
// `String.fromCharCode(96)` instead of a literal backtick so this can stay
// one plain Go string; three quote variants because (like jsImportRes) a
// chunk's own `from`/`import(` specifiers may be quoted with any of `"`,
// `'`, or `` ` ``, and rewriting is a plain per-quote loop rather than a
// single backreferenced regex.
//
// The `vite:preloadError` listener is defense in depth: build.modulePreload
// = false (vite.config.ts) already empties out most chunks' own
// dependency-preload lists, but a couple of mermaid's diagram-type chunks
// still carry one (observed: a stray CSS preload) — that's a `<link>` Vite
// inserts as an optimization hint before running a dynamic import, not
// something the import itself depends on, so swallowing its failure here is
// safe and lets the actual import proceed regardless.
const jsChunkResolverTemplate = `window.addEventListener('vite:preloadError', function (e) { e.preventDefault(); });
(function () {
  var chunks = %s;
  var entryKey = %s;
  var blobUrls = {};
  var resolving = {};
  var quotes = ['"', "'", String.fromCharCode(96)];
  function resolve(spec) {
    if (Object.prototype.hasOwnProperty.call(blobUrls, spec)) return blobUrls[spec];
    if (!Object.prototype.hasOwnProperty.call(chunks, spec)) throw new Error('pmem export: missing inlined module ' + spec);
    if (resolving[spec]) throw new Error('pmem export: circular static import at ' + spec);
    resolving[spec] = true;
    var src = chunks[spec];
    for (var i = 0; i < quotes.length; i++) {
      var q = quotes[i];
      // Static "from" specifiers must be a literal string, so the target
      // has to be a real, already-created Blob URL by the time this chunk's
      // own Blob is created — resolve it (recursively) right now.
      var reFrom = new RegExp('from' + q + '((?:\\.\\.?/)[^' + q + ']+\\.js)' + q, 'g');
      src = src.replace(reFrom, function (m, dep) { return 'from' + q + resolve(dep) + q; });
      // Static side-effect-only imports ('import "./x.js"', no 'from') are
      // just as eager as a 'from' import — same reasoning, resolve now.
      var reBare = new RegExp('import' + q + '((?:\\.\\.?/)[^' + q + ']+\\.js)' + q, 'g');
      src = src.replace(reBare, function (m, dep) { return 'import' + q + resolve(dep) + q; });
      // Dynamic import(...) targets don't need to exist yet: rewritten to
      // call the resolver at the moment the import actually runs instead of
      // eagerly here. This matters because the real chunk graph has cycles
      // through this edge (e.g. entry --dynamic--> mermaid.core, but
      // mermaid.core and several of its own chunks --static--> entry, for
      // shared helper bindings) — eagerly resolving both directions would
      // deadlock. window.__pmemResolve is set once, below, before entryKey
      // is ever resolved, so it's always available by the time any blob's
      // code actually runs.
      var reImport = new RegExp('import\\(\\s*' + q + '((?:\\.\\.?/)[^' + q + ']+\\.js)' + q + '\\s*\\)', 'g');
      src = src.replace(reImport, function (m, dep) { return 'import(window.__pmemResolve(' + JSON.stringify(dep) + '))'; });
    }
    delete resolving[spec];
    var url = URL.createObjectURL(new Blob([src], { type: 'text/javascript' }));
    blobUrls[spec] = url;
    return url;
  }
  window.__pmemResolve = resolve;
  import(resolve(entryKey));
})();`

// bundleModuleGraph turns entrySrc (the module dist/index.html points at)
// plus every chunk it transitively imports into one dependency-free
// bootstrap <script> body (see jsChunkResolverTemplate for how).
func bundleModuleGraph(distFS fs.FS, assetsDir, entryKey string, entrySrc []byte) (string, error) {
	chunks, err := collectChunkGraph(distFS, assetsDir, entryKey, entrySrc)
	if err != nil {
		return "", err
	}
	chunksJSON, err := json.Marshal(chunks)
	if err != nil {
		return "", err
	}
	entryKeyJSON, err := json.Marshal(entryKey)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(jsChunkResolverTemplate, chunksJSON, entryKeyJSON), nil
}
