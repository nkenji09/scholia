import { useEffect, useRef } from 'preact/hooks';
import { renderMarkdown } from '../markdown';

interface Props {
  text?: string;
  class?: string;
}

// dangerouslySetInnerHTML is safe here specifically because renderMarkdown()
// only ever emits HTML it constructed itself around escaped/highlighted text
// (see markdown.ts) — there is no path from source text to raw HTML output.
//
// renderMarkdown() stays synchronous (```mermaid fences become an inert
// `<pre class="mermaid">` holding escaped diagram source — see markdown.ts).
// mermaid itself is heavy, so it's not a static dependency of the base
// bundle: this effect dynamically imports it, and only when the rendered
// HTML actually contains a `.mermaid` block, then turns those placeholders
// into diagrams in place. That keeps `<Markdown>`'s public API
// ({text, class}) and call sites untouched.
export function Markdown({ text, class: className }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const html = text && text.trim() ? renderMarkdown(text) : '';

  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    const nodes = root.querySelectorAll<HTMLElement>('pre.mermaid');
    if (nodes.length === 0) return;
    let cancelled = false;
    import('mermaid').then(({ default: mermaid }) => {
      if (cancelled) return;
      mermaid.initialize({ startOnLoad: false, theme: 'neutral', securityLevel: 'strict' });
      void mermaid.run({ nodes, suppressErrors: true });
    });
    return () => {
      cancelled = true;
    };
  }, [html]);

  if (!html) return null;
  return (
    <div
      ref={ref}
      class={'markdown-body' + (className ? ` ${className}` : '')}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
