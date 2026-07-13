// Markdown → HTML renderer for project record descriptions
// (Tag.Description / VocabEntry.Description), built on markdown-it.
//
// Safety: markdown-it runs with `html: false`, so raw HTML in source text is
// never passed through — it's escaped like any other text token. The only
// tags this module's output can ever contain are the ones markdown-it (or
// the `highlight`/`link_open` overrides below) construct themselves around
// escaped/highlighted text. `validateLink` narrows link destinations to the
// same http(s)/mailto allowlist the previous hand-rolled renderer used
// (markdown-it's own default already blocks `javascript:`/`vbscript:`, but
// this keeps the allowlist explicit rather than relying on a blocklist).
// highlight.js's `highlight()` similarly only ever wraps its own escaped
// copy of the input in `<span class="hljs-*">` — it does not interpret the
// input as HTML. So: user-authored text can only ever become plain escaped
// text, a highlight.js span around escaped text, or one of the literal tags
// this module writes. This ships as part of the Vite bundle (no CDN
// dependency), so it renders identically in `pmem view` and in the
// self-contained `pmem export --html` output.
//
// Fenced ```mermaid blocks are rendered as `<pre class="mermaid">` holding
// the escaped-but-unrendered diagram source; `Markdown.tsx` turns those into
// diagrams after mount via a lazily-loaded mermaid (see its comment for why
// that split exists).

import MarkdownIt from 'markdown-it';
import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import css from 'highlight.js/lib/languages/css';
import diff from 'highlight.js/lib/languages/diff';
import dockerfile from 'highlight.js/lib/languages/dockerfile';
import go from 'highlight.js/lib/languages/go';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import markdownLang from 'highlight.js/lib/languages/markdown';
import plaintext from 'highlight.js/lib/languages/plaintext';
import python from 'highlight.js/lib/languages/python';
import sql from 'highlight.js/lib/languages/sql';
import typescript from 'highlight.js/lib/languages/typescript';
import xml from 'highlight.js/lib/languages/xml';
import yaml from 'highlight.js/lib/languages/yaml';

hljs.registerLanguage('bash', bash);
hljs.registerLanguage('sh', bash);
hljs.registerLanguage('shell', bash);
hljs.registerLanguage('css', css);
hljs.registerLanguage('diff', diff);
hljs.registerLanguage('dockerfile', dockerfile);
hljs.registerLanguage('go', go);
hljs.registerLanguage('golang', go);
hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('js', javascript);
hljs.registerLanguage('json', json);
hljs.registerLanguage('markdown', markdownLang);
hljs.registerLanguage('md', markdownLang);
hljs.registerLanguage('plaintext', plaintext);
hljs.registerLanguage('text', plaintext);
hljs.registerLanguage('python', python);
hljs.registerLanguage('py', python);
hljs.registerLanguage('sql', sql);
hljs.registerLanguage('typescript', typescript);
hljs.registerLanguage('ts', typescript);
hljs.registerLanguage('tsx', typescript);
hljs.registerLanguage('html', xml);
hljs.registerLanguage('xml', xml);
hljs.registerLanguage('yaml', yaml);
hljs.registerLanguage('yml', yaml);

// Only these link schemes render as a clickable <a>; anything else (in
// particular `javascript:`/`data:`) falls back to markdown-it's default
// escaped-text handling for an invalid link.
const SAFE_URL = /^(https?:|mailto:)/i;

const md: MarkdownIt = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: false,
  breaks: false,
  highlight(code, lang) {
    if (lang === 'mermaid') {
      return `<pre class="mermaid">${md.utils.escapeHtml(code)}</pre>`;
    }
    if (lang && hljs.getLanguage(lang)) {
      try {
        const highlighted = hljs.highlight(code, { language: lang, ignoreIllegals: true }).value;
        return `<pre class="hljs"><code>${highlighted}</code></pre>`;
      } catch {
        // fall through to plain escaped output below
      }
    }
    return `<pre><code>${md.utils.escapeHtml(code)}</code></pre>`;
  },
});

md.validateLink = (url) => SAFE_URL.test(url);

const defaultLinkOpen =
  md.renderer.rules.link_open ??
  ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));
md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  tokens[idx].attrSet('target', '_blank');
  tokens[idx].attrSet('rel', 'noopener noreferrer');
  return defaultLinkOpen(tokens, idx, options, env, self);
};

export function renderMarkdown(source: string): string {
  if (!source || !source.trim()) return '';
  return md.render(source);
}
