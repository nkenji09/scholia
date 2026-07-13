import type { JSX } from 'preact';

// Inline SVG icon registry (.concierge/decision.md §D): the design
// references iconify's `lucide:*` set via a CDN script, which the
// self-contained viewer (file:// export) cannot load. Each entry below is
// copied from lucide-static's source (MIT, 24x24, stroke-based) as plain
// path/shape data — no npm dependency, no network fetch at runtime. Grown
// incrementally: only the icons a given phase's components actually render
// are added here, not all ~57 the full design uses.
type Shape =
  | { tag: 'path'; d: string }
  | { tag: 'circle'; cx: number; cy: number; r: number; fill?: 'currentColor' }
  | { tag: 'line'; x1: number; y1: number; x2: number; y2: number }
  | { tag: 'rect'; x: number; y: number; width: number; height: number; rx?: number };

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
  'message-plus': [
    { tag: 'path', d: 'M22 17a2 2 0 0 1-2 2H6.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 2 21.286V5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2z' },
    { tag: 'path', d: 'M12 8v6' },
    { tag: 'path', d: 'M9 11h6' },
  ],
  'message-filled': [
    { tag: 'path', d: 'M22 17a2 2 0 0 1-2 2H6.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 2 21.286V5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2z' },
    { tag: 'path', d: 'M7 11h10' },
    { tag: 'path', d: 'M7 15h6' },
    { tag: 'path', d: 'M7 7h8' },
  ],
  x: [
    { tag: 'path', d: 'M18 6 6 18' },
    { tag: 'path', d: 'm6 6 12 12' },
  ],
  'trash-2': [
    { tag: 'path', d: 'M10 11v6' },
    { tag: 'path', d: 'M14 11v6' },
    { tag: 'path', d: 'M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6' },
    { tag: 'path', d: 'M3 6h18' },
    { tag: 'path', d: 'M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2' },
  ],
  crosshair: [
    { tag: 'circle', cx: 12, cy: 12, r: 10 },
    { tag: 'line', x1: 22, y1: 12, x2: 18, y2: 12 },
    { tag: 'line', x1: 6, y1: 12, x2: 2, y2: 12 },
    { tag: 'line', x1: 12, y1: 6, x2: 12, y2: 2 },
    { tag: 'line', x1: 12, y1: 22, x2: 12, y2: 18 },
  ],
  pencil: [
    {
      tag: 'path',
      d: 'M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z',
    },
    { tag: 'path', d: 'm15 5 4 4' },
  ],
  'clipboard-copy': [
    { tag: 'rect', x: 8, y: 2, width: 8, height: 4, rx: 1 },
    { tag: 'path', d: 'M8 4H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2' },
    { tag: 'path', d: 'M16 4h2a2 2 0 0 1 2 2v4' },
    { tag: 'path', d: 'M21 14H11' },
    { tag: 'path', d: 'm15 10-4 4 4 4' },
  ],
  check: [{ tag: 'path', d: 'M20 6 9 17l-5-5' }],

  // Header rebuild (design fidelity pass): logo + nav + icon buttons.
  box: [
    { tag: 'path', d: 'M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z' },
    { tag: 'path', d: 'm3.3 7 8.7 5 8.7-5' },
    { tag: 'path', d: 'M12 22V12' },
  ],
  'layout-dashboard': [
    { tag: 'rect', x: 3, y: 3, width: 7, height: 9, rx: 1 },
    { tag: 'rect', x: 14, y: 3, width: 7, height: 5, rx: 1 },
    { tag: 'rect', x: 14, y: 12, width: 7, height: 9, rx: 1 },
    { tag: 'rect', x: 3, y: 16, width: 7, height: 5, rx: 1 },
  ],
  tags: [
    { tag: 'path', d: 'M13.172 2a2 2 0 0 1 1.414.586l6.71 6.71a2.4 2.4 0 0 1 0 3.408l-4.592 4.592a2.4 2.4 0 0 1-3.408 0l-6.71-6.71A2 2 0 0 1 6 9.172V3a1 1 0 0 1 1-1z' },
    { tag: 'path', d: 'M2 7v6.172a2 2 0 0 0 .586 1.414l6.71 6.71a2.4 2.4 0 0 0 3.191.193' },
    { tag: 'circle', cx: 10.5, cy: 6.5, r: 0.5, fill: 'currentColor' },
  ],
  'scroll-text': [
    { tag: 'path', d: 'M15 12h-5' },
    { tag: 'path', d: 'M15 8h-5' },
    { tag: 'path', d: 'M19 17V5a2 2 0 0 0-2-2H4' },
    { tag: 'path', d: 'M8 21h12a2 2 0 0 0 2-2v-1a1 1 0 0 0-1-1H11a1 1 0 0 0-1 1v1a2 2 0 1 1-4 0V5a2 2 0 1 0-4 0v2a1 1 0 0 0 1 1h3' },
  ],
  'book-open': [
    { tag: 'path', d: 'M12 7v14' },
    {
      tag: 'path',
      d: 'M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z',
    },
  ],
  radar: [
    { tag: 'path', d: 'M19.07 4.93A10 10 0 0 0 6.99 3.34' },
    { tag: 'path', d: 'M4 6h.01' },
    { tag: 'path', d: 'M2.29 9.62A10 10 0 1 0 21.31 8.35' },
    { tag: 'path', d: 'M16.24 7.76A6 6 0 1 0 8.23 16.67' },
    { tag: 'path', d: 'M12 18h.01' },
    { tag: 'path', d: 'M17.99 11.66A6 6 0 0 1 15.77 16.67' },
    { tag: 'circle', cx: 12, cy: 12, r: 2 },
    { tag: 'path', d: 'm13.41 10.59 5.66-5.66' },
  ],
  'git-compare': [
    { tag: 'circle', cx: 18, cy: 18, r: 3 },
    { tag: 'circle', cx: 6, cy: 6, r: 3 },
    { tag: 'path', d: 'M13 6h3a2 2 0 0 1 2 2v7' },
    { tag: 'path', d: 'M11 18H8a2 2 0 0 1-2-2V9' },
  ],
  settings: [
    {
      tag: 'path',
      d: 'M9.671 4.136a2.34 2.34 0 0 1 4.659 0 2.34 2.34 0 0 0 3.319 1.915 2.34 2.34 0 0 1 2.33 4.033 2.34 2.34 0 0 0 0 3.831 2.34 2.34 0 0 1-2.33 4.033 2.34 2.34 0 0 0-3.319 1.915 2.34 2.34 0 0 1-4.659 0 2.34 2.34 0 0 0-3.32-1.915 2.34 2.34 0 0 1-2.33-4.033 2.34 2.34 0 0 0 0-3.831A2.34 2.34 0 0 1 6.35 6.051a2.34 2.34 0 0 0 3.319-1.915',
    },
    { tag: 'circle', cx: 12, cy: 12, r: 3 },
  ],
  search: [
    { tag: 'path', d: 'm21 21-4.34-4.34' },
    { tag: 'circle', cx: 11, cy: 11, r: 8 },
  ],
  'sliders-horizontal': [
    { tag: 'path', d: 'M10 5H3' },
    { tag: 'path', d: 'M12 19H3' },
    { tag: 'path', d: 'M14 3v4' },
    { tag: 'path', d: 'M16 17v4' },
    { tag: 'path', d: 'M21 12h-9' },
    { tag: 'path', d: 'M21 19h-5' },
    { tag: 'path', d: 'M21 5h-7' },
    { tag: 'path', d: 'M8 10v4' },
    { tag: 'path', d: 'M8 12H3' },
  ],

  // Card/rail icon sweep (replacing plain unicode symbols).
  'triangle-alert': [
    { tag: 'path', d: 'm21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3' },
    { tag: 'path', d: 'M12 9v4' },
    { tag: 'path', d: 'M12 17h.01' },
  ],
  'corner-down-right': [
    { tag: 'path', d: 'm15 10 5 5-5 5' },
    { tag: 'path', d: 'M4 4v7a4 4 0 0 0 4 4h12' },
  ],
  'chevron-down': [{ tag: 'path', d: 'm6 9 6 6 6-6' }],
  'chevron-up': [{ tag: 'path', d: 'm18 15-6-6-6 6' }],
  'chevron-right': [{ tag: 'path', d: 'm9 18 6-6-6-6' }],
  'circle-play': [
    { tag: 'path', d: 'M9 9.003a1 1 0 0 1 1.517-.859l4.997 2.997a1 1 0 0 1 0 1.718l-4.997 2.997A1 1 0 0 1 9 14.996z' },
    { tag: 'circle', cx: 12, cy: 12, r: 10 },
  ],
  funnel: [
    {
      tag: 'path',
      d: 'M10 20a1 1 0 0 0 .553.895l2 1A1 1 0 0 0 14 21v-7a2 2 0 0 1 .517-1.341L21.74 4.67A1 1 0 0 0 21 3H3a1 1 0 0 0-.742 1.67l7.225 7.989A2 2 0 0 1 10 14z',
    },
  ],
  'arrow-right-to-line': [
    { tag: 'path', d: 'M17 12H3' },
    { tag: 'path', d: 'm11 18 6-6-6-6' },
    { tag: 'path', d: 'M21 5v14' },
  ],
  'flask-conical': [
    { tag: 'path', d: 'M14 2v6a2 2 0 0 0 .245.96l5.51 10.08A2 2 0 0 1 18 22H6a2 2 0 0 1-1.755-2.96l5.51-10.08A2 2 0 0 0 10 8V2' },
    { tag: 'path', d: 'M6.453 15h11.094' },
    { tag: 'path', d: 'M8.5 2h7' },
  ],
  gavel: [
    { tag: 'path', d: 'm14 13-8.381 8.38a1 1 0 0 1-3.001-3l8.384-8.381' },
    { tag: 'path', d: 'm16 16 6-6' },
    { tag: 'path', d: 'm21.5 10.5-8-8' },
    { tag: 'path', d: 'm8 8 6-6' },
    { tag: 'path', d: 'm8.5 7.5 8 8' },
  ],
  'list-tree': [
    { tag: 'path', d: 'M8 5h13' },
    { tag: 'path', d: 'M13 12h8' },
    { tag: 'path', d: 'M13 19h8' },
    { tag: 'path', d: 'M3 10a2 2 0 0 0 2 2h3' },
    { tag: 'path', d: 'M3 5v12a2 2 0 0 0 2 2h3' },
  ],
  'external-link': [
    { tag: 'path', d: 'M15 3h6v6' },
    { tag: 'path', d: 'M10 14 21 3' },
    { tag: 'path', d: 'M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6' },
  ],
  filter: [
    {
      tag: 'path',
      d: 'M10 20a1 1 0 0 0 .553.895l2 1A1 1 0 0 0 14 21v-7a2 2 0 0 1 .517-1.341L21.74 4.67A1 1 0 0 0 21 3H3a1 1 0 0 0-.742 1.67l7.225 7.989A2 2 0 0 1 10 14z',
    },
  ],
  list: [
    { tag: 'path', d: 'M3 5h.01' },
    { tag: 'path', d: 'M3 12h.01' },
    { tag: 'path', d: 'M3 19h.01' },
    { tag: 'path', d: 'M8 5h13' },
    { tag: 'path', d: 'M8 12h13' },
    { tag: 'path', d: 'M8 19h13' },
  ],

  // Config icons.
  'git-fork': [
    { tag: 'circle', cx: 12, cy: 18, r: 3 },
    { tag: 'circle', cx: 6, cy: 6, r: 3 },
    { tag: 'circle', cx: 18, cy: 6, r: 3 },
    { tag: 'path', d: 'M18 9v2c0 .6-.4 1-1 1H7c-.6 0-1-.4-1-1V9' },
    { tag: 'path', d: 'M12 12v3' },
  ],
  'panel-left': [
    { tag: 'rect', x: 3, y: 3, width: 18, height: 18, rx: 2 },
    { tag: 'path', d: 'M9 3v18' },
  ],
  monitor: [
    { tag: 'rect', x: 2, y: 3, width: 20, height: 14, rx: 2 },
    { tag: 'line', x1: 8, y1: 21, x2: 16, y2: 21 },
    { tag: 'line', x1: 12, y1: 17, x2: 12, y2: 21 },
  ],
  plug: [
    { tag: 'path', d: 'M12 22v-5' },
    { tag: 'path', d: 'M15 8V2' },
    { tag: 'path', d: 'M17 8a1 1 0 0 1 1 1v4a4 4 0 0 1-4 4h-4a4 4 0 0 1-4-4V9a1 1 0 0 1 1-1z' },
    { tag: 'path', d: 'M9 8V2' },
  ],
  lock: [
    { tag: 'rect', x: 3, y: 11, width: 18, height: 11, rx: 2 },
    { tag: 'path', d: 'M7 11V7a5 5 0 0 1 10 0v4' },
  ],
  eye: [
    {
      tag: 'path',
      d: 'M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0',
    },
    { tag: 'circle', cx: 12, cy: 12, r: 3 },
  ],
  server: [
    { tag: 'rect', x: 2, y: 2, width: 20, height: 8, rx: 2 },
    { tag: 'rect', x: 2, y: 14, width: 20, height: 8, rx: 2 },
    { tag: 'line', x1: 6, y1: 6, x2: 6.01, y2: 6 },
    { tag: 'line', x1: 6, y1: 18, x2: 6.01, y2: 18 },
  ],
  'file-code-2': [
    { tag: 'path', d: 'M4 12.15V4a2 2 0 0 1 2-2h8a2.4 2.4 0 0 1 1.706.706l3.588 3.588A2.4 2.4 0 0 1 20 8v12a2 2 0 0 1-2 2h-3.35' },
    { tag: 'path', d: 'M14 2v5a1 1 0 0 0 1 1h5' },
    { tag: 'path', d: 'm5 16-3 3 3 3' },
    { tag: 'path', d: 'm9 22 3-3-3-3' },
  ],
  info: [
    { tag: 'circle', cx: 12, cy: 12, r: 10 },
    { tag: 'path', d: 'M12 16v-4' },
    { tag: 'path', d: 'M12 8h.01' },
  ],
  'arrow-right': [
    { tag: 'path', d: 'M5 12h14' },
    { tag: 'path', d: 'm12 5 7 7-7 7' },
  ],
  save: [
    { tag: 'path', d: 'M15.2 3a2 2 0 0 1 1.4.6l3.8 3.8a2 2 0 0 1 .6 1.4V19a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z' },
    { tag: 'path', d: 'M17 21v-7a1 1 0 0 0-1-1H8a1 1 0 0 0-1 1v7' },
    { tag: 'path', d: 'M7 3v4a1 1 0 0 0 1 1h7' },
  ],

  // Vocab owner pill (vocab-owner-tag).
  user: [
    { tag: 'path', d: 'M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2' },
    { tag: 'circle', cx: 12, cy: 7, r: 4 },
  ],

  // Header language toggle (i18n, #16).
  languages: [
    { tag: 'path', d: 'm5 8 6 6' },
    { tag: 'path', d: 'm4 14 6-6 2-3' },
    { tag: 'path', d: 'M2 5h12' },
    { tag: 'path', d: 'M7 2h1' },
    { tag: 'path', d: 'm22 22-5-10-5 10' },
    { tag: 'path', d: 'M14 18h6' },
  ],
} satisfies Record<string, Shape[]>;

export type IconName = keyof typeof ICONS;

interface Props {
  name: IconName;
  size?: number;
  class?: string;
}

export function Icon({ name, size = 16, class: className }: Props) {
  const shapes = ICONS[name] as Shape[];
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
      {shapes.map((s, i): JSX.Element => {
        if (s.tag === 'circle') return <circle key={i} cx={s.cx} cy={s.cy} r={s.r} fill={s.fill} />;
        if (s.tag === 'line') return <line key={i} x1={s.x1} y1={s.y1} x2={s.x2} y2={s.y2} />;
        if (s.tag === 'rect') return <rect key={i} x={s.x} y={s.y} width={s.width} height={s.height} rx={s.rx} />;
        return <path key={i} d={s.d} />;
      })}
    </svg>
  );
}
