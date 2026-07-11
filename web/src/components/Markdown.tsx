import { renderMarkdown } from '../markdown';

interface Props {
  text?: string;
  class?: string;
}

// dangerouslySetInnerHTML is safe here specifically because renderMarkdown()
// only ever emits HTML it constructed itself around escaped text (see
// markdown.ts) — there is no path from source text to raw HTML output.
export function Markdown({ text, class: className }: Props) {
  if (!text || !text.trim()) return null;
  const html = renderMarkdown(text);
  if (!html) return null;
  return <div class={'markdown-body' + (className ? ` ${className}` : '')} dangerouslySetInnerHTML={{ __html: html }} />;
}
