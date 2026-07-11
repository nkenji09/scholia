// Minimal, dependency-free markdown → HTML renderer for project record
// descriptions (Tag.Description / VocabEntry.Description).
//
// Deliberately a small whitelist subset rather than full CommonMark: every
// HTML tag in the output is a literal string this module writes itself, and
// all source text is HTML-escaped *before* any inline formatting is layered
// on top of it. That ordering means there is no "parse markdown into a DOM
// tree, then sanitize it" step where a sanitizer bug could let raw HTML
// through — user-authored text can only ever become plain escaped text or
// one of the handful of tags this file constructs itself (b/i/code/a/h1-6/
// ul/ol/li/blockquote/pre/p). This ships as part of the Vite bundle (no
// runtime dependency), so it works identically in `pmem view` and in the
// self-contained `pmem export --html` output.

const ESCAPE_MAP: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
};

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ESCAPE_MAP[c]);
}

// Only these link schemes render as a clickable <a>; anything else (in
// particular `javascript:`/`data:`) falls back to plain escaped text.
const SAFE_URL = /^(https?:|mailto:)/i;

// Applied to already-escaped text: `**`/`` ` ``/`[`/`]` all survive
// escapeHtml() untouched (none of them are HTML-special), so this only ever
// wraps already-safe text in tags it writes itself.
function renderInline(escaped: string): string {
  let out = escaped;
  out = out.replace(/`([^`]+)`/g, (_m, code) => `<code>${code}</code>`);
  // Bold before italic: once `**x**` is consumed, any remaining single `*`
  // pairs are unambiguous italic markers (avoids needing lookbehind regex).
  out = out.replace(/\*\*([^*]+)\*\*/g, (_m, b) => `<strong>${b}</strong>`);
  out = out.replace(/\*([^*]+)\*/g, (_m, i) => `<em>${i}</em>`);
  out = out.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_m, text, url) => {
    if (!SAFE_URL.test(url)) return text;
    return `<a href="${url}" target="_blank" rel="noopener noreferrer">${text}</a>`;
  });
  return out;
}

interface Block {
  type: 'heading' | 'blockquote' | 'ul' | 'ol' | 'code' | 'p';
  level?: number;
  lines: string[];
}

const HEADING_RE = /^(#{1,6})\s+(.*)$/;
const QUOTE_RE = /^>\s?/;
const UL_RE = /^[-*]\s+/;
const OL_RE = /^\d+\.\s+/;

function parseBlocks(raw: string): Block[] {
  const lines = raw.replace(/\r\n?/g, '\n').split('\n');
  const blocks: Block[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.trim() === '') {
      i++;
      continue;
    }

    if (line.startsWith('```')) {
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !lines[i].startsWith('```')) {
        codeLines.push(lines[i]);
        i++;
      }
      i++; // skip closing fence (tolerate a missing one at EOF)
      blocks.push({ type: 'code', lines: codeLines });
      continue;
    }

    const heading = HEADING_RE.exec(line);
    if (heading) {
      blocks.push({ type: 'heading', level: heading[1].length, lines: [heading[2]] });
      i++;
      continue;
    }

    if (QUOTE_RE.test(line)) {
      const quoteLines: string[] = [];
      while (i < lines.length && QUOTE_RE.test(lines[i])) {
        quoteLines.push(lines[i].replace(QUOTE_RE, ''));
        i++;
      }
      blocks.push({ type: 'blockquote', lines: quoteLines });
      continue;
    }

    if (UL_RE.test(line)) {
      const itemLines: string[] = [];
      while (i < lines.length && UL_RE.test(lines[i])) {
        itemLines.push(lines[i].replace(UL_RE, ''));
        i++;
      }
      blocks.push({ type: 'ul', lines: itemLines });
      continue;
    }

    if (OL_RE.test(line)) {
      const itemLines: string[] = [];
      while (i < lines.length && OL_RE.test(lines[i])) {
        itemLines.push(lines[i].replace(OL_RE, ''));
        i++;
      }
      blocks.push({ type: 'ol', lines: itemLines });
      continue;
    }

    const paraLines: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== '' &&
      !lines[i].startsWith('```') &&
      !HEADING_RE.test(lines[i]) &&
      !QUOTE_RE.test(lines[i]) &&
      !UL_RE.test(lines[i]) &&
      !OL_RE.test(lines[i])
    ) {
      paraLines.push(lines[i]);
      i++;
    }
    blocks.push({ type: 'p', lines: paraLines });
  }

  return blocks;
}

export function renderMarkdown(source: string): string {
  if (!source || !source.trim()) return '';
  const blocks = parseBlocks(source);
  const html: string[] = [];

  for (const b of blocks) {
    switch (b.type) {
      case 'heading': {
        const tag = `h${Math.min(Math.max(b.level ?? 1, 1), 6)}`;
        html.push(`<${tag}>${renderInline(escapeHtml(b.lines[0]))}</${tag}>`);
        break;
      }
      case 'code':
        html.push(`<pre><code>${escapeHtml(b.lines.join('\n'))}</code></pre>`);
        break;
      case 'blockquote':
        html.push(`<blockquote><p>${b.lines.map((l) => renderInline(escapeHtml(l))).join('<br/>')}</p></blockquote>`);
        break;
      case 'ul':
        html.push(`<ul>${b.lines.map((l) => `<li>${renderInline(escapeHtml(l))}</li>`).join('')}</ul>`);
        break;
      case 'ol':
        html.push(`<ol>${b.lines.map((l) => `<li>${renderInline(escapeHtml(l))}</li>`).join('')}</ol>`);
        break;
      case 'p':
        html.push(`<p>${b.lines.map((l) => renderInline(escapeHtml(l))).join('<br/>')}</p>`);
        break;
    }
  }

  return html.join('\n');
}
