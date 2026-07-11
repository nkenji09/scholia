import { isStaticMode } from '../../api';
import { strings } from '../../strings';
import type { ViewName } from '../../router';
import { ACCENTS, useViewerSettings } from '../../settings';
import { SearchBox } from '../SearchBox';
import { Icon } from '../shared/Icon';

interface Props {
  view: ViewName;
  onSelectView: (v: ViewName) => void;
  onSelectTx: (id: string) => void;
}

const NAV: Array<[ViewName, string]> = [
  ['home', strings.nav.home],
  ['browse', 'Browse'],
  ['vocab', strings.nav.vocab],
  ['spec', strings.nav.spec],
  ['tags', strings.nav.tags],
  ['traceability', 'Traceability'],
  ['compare', 'Compare'],
  ['config', 'Config'],
];

export function Header({ view, onSelectView, onSelectTx }: Props) {
  const { settings, toggleTheme, setDensity, setAccent, incFont, decFont } = useViewerSettings();

  return (
    <header class="topbar">
      <h1>pmem view</h1>
      <SearchBox onSelectTx={onSelectTx} />
      <nav>
        {NAV.filter(([key]) => key !== 'compare' || !isStaticMode).map(([key, label]) => (
          <button key={key} type="button" class={view === key ? 'active' : ''} onClick={() => onSelectView(key)}>
            {label}
          </button>
        ))}
      </nav>
      <div class="header-switches">
        <div class="font-scale" role="group" aria-label="文字サイズ">
          <button type="button" aria-label={strings.header.fontDec} onClick={decFont}>
            <Icon name="minus" size={14} />
          </button>
          <span class="font-scale-pct">{Math.round(settings.fontScale * 100)}%</span>
          <button type="button" aria-label={strings.header.fontInc} onClick={incFont}>
            <Icon name="plus" size={14} />
          </button>
        </div>
        <label class="header-select">
          <span class="dim">{strings.header.accent}</span>
          <select value={settings.accent} onChange={(e) => setAccent((e.target as HTMLSelectElement).value as typeof settings.accent)}>
            {ACCENTS.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
        <label class="header-select">
          <span class="dim">密度</span>
          <select value={settings.density} onChange={(e) => setDensity((e.target as HTMLSelectElement).value as typeof settings.density)}>
            <option value="compact">{strings.header.density.compact}</option>
            <option value="normal">{strings.header.density.normal}</option>
            <option value="comfortable">{strings.header.density.comfortable}</option>
          </select>
        </label>
        <button type="button" class="theme-toggle" aria-label={strings.header.themeToggle} onClick={toggleTheme}>
          <Icon name={settings.theme === 'dark' ? 'moon' : 'sun'} size={16} />
        </button>
      </div>
    </header>
  );
}
