import type { JSX } from 'preact';

// Inline SVG icon registry (.concierge/decision.md §D): the design
// references iconify's `lucide:*` set via a CDN script, which the
// self-contained viewer (file:// export) cannot load. Each entry below is
// copied from lucide-static's source (MIT, 24x24, stroke-based) as plain
// path/shape data — no npm dependency, no network fetch at runtime. Grown
// incrementally: only the icons a given phase's components actually render
// are added here, not all ~57 the full design uses.
type Shape = { tag: 'path'; d: string } | { tag: 'circle'; cx: number; cy: number; r: number };

const ICONS = {
  sun: [
    { tag: 'circle', cx: 12, cy: 12, r: 4 },
    { tag: 'path', d: 'M12 2v2' },
    { tag: 'path', d: 'M12 20v2' },
    { tag: 'path', d: 'm4.93 4.93 1.41 1.41' },
    { tag: 'path', d: 'm17.66 17.66 1.41 1.41' },
    { tag: 'path', d: 'M2 12h2' },
    { tag: 'path', d: 'M20 12h2' },
    { tag: 'path', d: 'm6.34 17.66-1.41 1.41' },
    { tag: 'path', d: 'm19.07 4.93-1.41 1.41' },
  ],
  moon: [
    {
      tag: 'path',
      d: 'M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401',
    },
  ],
  plus: [
    { tag: 'path', d: 'M5 12h14' },
    { tag: 'path', d: 'M12 5v14' },
  ],
  minus: [{ tag: 'path', d: 'M5 12h14' }],
} satisfies Record<string, Shape[]>;

export type IconName = keyof typeof ICONS;

interface Props {
  name: IconName;
  size?: number;
  class?: string;
}

export function Icon({ name, size = 16, class: className }: Props) {
  const shapes = ICONS[name];
  return (
    <svg
      class={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      {shapes.map((s, i): JSX.Element => (s.tag === 'circle' ? <circle key={i} cx={s.cx} cy={s.cy} r={s.r} /> : <path key={i} d={s.d} />))}
    </svg>
  );
}
