import { useState } from 'preact/hooks';
import { useT } from '../../i18n';
import { useDrawer } from '../../drawer';
import { Chip } from '../shared/Chip';
import { Icon } from '../shared/Icon';

export interface KindOption {
  key: string;
  label: string;
  count: number;
}

export interface ConditionChip {
  label: string;
  color: string;
  onRemove: () => void;
}

export interface IndexItem {
  id: string;
  label: string;
  color: string;
  indent: number;
  isGap?: boolean;
  onClick: () => void;
  // 見出しの折りたたみ（依頼1）。統一ツリーの親ノードだけ hasChildren=true で
  // ▶/▼ トグルを出す。collapsed のとき子孫はこの indexItems 配列から除かれる
  // （呼び出し側が flatten で間引く）。leaf は spacer で桁を揃える。
  hasChildren?: boolean;
  collapsed?: boolean;
  onToggle?: () => void;
}

// A selectable tag/vocab candidate for the search box's combobox dropdown
// (2026-07-11 tweaks3 §3). Callers build this list from data they already
// have loaded (useLookups()/props already threaded into BrowseView/
// VocabView) — BrowseRail only does substring matching against `query` on
// whatever it's handed, no relationship lookups of its own.
export interface SuggestionItem {
  id: string;
  label: string;
  color: string;
  /** Short kind tag shown beside the label ("タグ"/"語彙") to disambiguate
      same-looking labels from different sources. */
  kindLabel: string;
  onSelect: () => void;
}

interface Props {
  query: string;
  onQueryChange: (q: string) => void;
  kindFacet: string;
  kindOptions: KindOption[];
  onKindFacetChange: (k: string) => void;
  conditions: ConditionChip[];
  onClearConditions: () => void;
  indexItems: IndexItem[];
  /** Tag/vocab candidates for the combobox dropdown. Omit (or pass []) on
      screens that don't offer suggestions — the free-text filter behavior
      is unaffected either way. */
  suggestions?: SuggestionItem[];
}

const MAX_SUGGESTIONS = 8;

// Off-canvas drawer on narrow viewports / sticky sidebar on wide ones
// (design reference lines ~178-179 backdrop, ~972-980 railStyle — see
// drawer.tsx for the shared isNarrow/drawerOpen state this reads). Actions
// that narrow/change what's being browsed (kind facet, combobox select,
// index-item scroll-to, filter chips) all close the drawer on select — that
// close() call lives with each action's own handler (BrowseView.tsx/
// VocabView.tsx's addFilter/addTagFilter, and the IndexItem.onClick they
// build), not here, so this component doesn't need to know which actions
// count as "a selection was made" versus rail-internal-only ones (kind
// facet and remove-filter don't close it, matching the design).
export function BrowseRail({
  query,
  onQueryChange,
  kindFacet,
  kindOptions,
  onKindFacetChange,
  conditions,
  onClearConditions,
  indexItems,
  suggestions = [],
}: Props) {
  const t = useT();
  const [focused, setFocused] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const { isNarrow, drawerOpen, closeDrawer } = useDrawer();

  const q = query.trim().toLowerCase();
  const matches = focused && q ? suggestions.filter((s) => (s.id + ' ' + s.label).toLowerCase().includes(q)).slice(0, MAX_SUGGESTIONS) : [];
  const open = matches.length > 0;
  const idx = open ? Math.max(0, Math.min(activeIndex, matches.length - 1)) : -1;

  const selectMatch = (m: SuggestionItem) => {
    m.onSelect();
    onQueryChange('');
    // Don't setFocused(false) here: the suggestion button's onMouseDown
    // preventDefault keeps DOM focus on the input, so `focused` should stay
    // true too — otherwise the next keystroke can't reopen the dropdown
    // (matches requires `focused && q`) until the input is blurred and
    // refocused. Clearing the query already closes the dropdown (q is
    // empty), so this is safe.
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (!open) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, matches.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      const m = matches[idx];
      if (m) {
        e.preventDefault();
        selectMatch(m);
      }
    } else if (e.key === 'Escape') {
      setFocused(false);
    }
  };

  return (
    <>
      {isNarrow && drawerOpen && <div class="browse-rail-backdrop" onClick={closeDrawer} />}
      <aside class={'browse-rail' + (isNarrow ? ' browse-rail-narrow' : '') + (isNarrow && drawerOpen ? ' browse-rail-open' : '')}>
        <div class="browse-rail-head">
          <Icon name="sliders-horizontal" size={14} class="dim" />
          <span class="browse-rail-label dim">{t.browse.railHeading}</span>
          {isNarrow && (
            <button type="button" class="browse-rail-close" aria-label={t.common.close} onClick={closeDrawer}>
              <Icon name="x" size={17} />
            </button>
          )}
        </div>

        <div class="browse-rail-search-wrap">
          <Icon name="search" size={15} class="browse-rail-search-icon dim" />
          <input
            class="browse-rail-search"
            type="text"
            role="combobox"
            aria-expanded={open}
            aria-controls="browse-rail-listbox"
            aria-autocomplete="list"
            autocomplete="off"
            placeholder={t.browse.searchPlaceholder}
            value={query}
            onInput={(e) => {
              onQueryChange((e.target as HTMLInputElement).value);
              setActiveIndex(0);
            }}
            onFocus={() => setFocused(true)}
            onBlur={() => setFocused(false)}
            onKeyDown={onKeyDown}
          />
          {open && (
            <ul id="browse-rail-listbox" role="listbox" class="browse-rail-suggestions">
              {matches.map((m, i) => (
                <li key={m.kindLabel + m.id} role="option" aria-selected={i === idx}>
                  <button
                    type="button"
                    class={'browse-rail-suggestion' + (i === idx ? ' active' : '')}
                    onMouseDown={(e) => e.preventDefault()}
                    onMouseEnter={() => setActiveIndex(i)}
                    onClick={() => selectMatch(m)}
                  >
                    <span class="browse-rail-suggestion-dot" style={{ background: m.color }} />
                    <span class="browse-rail-suggestion-label">{m.label}</span>
                    <span class="browse-rail-suggestion-kind dim">{m.kindLabel}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        {kindOptions.length > 0 && (
          <div class="browse-rail-section">
            <span class="browse-rail-label dim">{t.browse.kindHeading}</span>
            <div class="browse-rail-kinds">
              <button type="button" class={'browse-rail-kind' + (kindFacet === 'all' ? ' active' : '')} onClick={() => onKindFacetChange('all')}>
                <span>{t.browse.kindAll}</span>
                <span class="dim">{kindOptions.reduce((sum, k) => sum + k.count, 0)}</span>
              </button>
              {kindOptions.map((k) => (
                <button
                  key={k.key}
                  type="button"
                  class={'browse-rail-kind' + (kindFacet === k.key ? ' active' : '')}
                  onClick={() => onKindFacetChange(k.key)}
                >
                  <span>{k.label}</span>
                  <span class="dim">{k.count}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        {conditions.length > 0 && (
          <div class="browse-rail-conditions">
            <div class="browse-rail-conditions-head">
              <span class="browse-rail-label dim">
                <Icon name="filter" size={13} /> {t.browse.conditionsHeading} <span class="browse-rail-and">{t.browse.and}</span>
              </span>
              <button type="button" class="browse-rail-clear" onClick={onClearConditions}>
                {t.browse.clear}
              </button>
            </div>
            <div class="browse-rail-condition-chips">
              {conditions.map((c, i) => (
                <Chip key={i} color={c.color} onRemove={c.onRemove}>
                  {c.label}
                </Chip>
              ))}
            </div>
          </div>
        )}

        <div class="browse-rail-section browse-rail-index">
          <span class="browse-rail-label dim">
            <Icon name="list" size={13} /> {t.browse.indexHeading} <span class="browse-rail-index-count">{indexItems.length}</span>
          </span>
          <div class="browse-rail-index-list">
            {indexItems.map((item) => (
              <div key={item.id} class="browse-rail-index-row" style={{ paddingLeft: `${8 + item.indent * 14}px` }}>
                {item.hasChildren ? (
                  <button
                    type="button"
                    class="browse-rail-index-toggle"
                    aria-label={item.collapsed ? t.browse.indexExpand : t.browse.indexCollapse}
                    aria-expanded={!item.collapsed}
                    onClick={() => item.onToggle?.()}
                  >
                    <Icon name={item.collapsed ? 'chevron-right' : 'chevron-down'} size={13} />
                  </button>
                ) : (
                  <span class="browse-rail-index-toggle-spacer" />
                )}
                <button type="button" class="browse-rail-index-item" onClick={item.onClick}>
                  <span class="browse-rail-index-dot" style={{ background: item.color }} />
                  <span class="browse-rail-index-label">{item.label}</span>
                  {item.isGap && (
                    <span class="browse-rail-index-gap">
                      <Icon name="triangle-alert" size={12} />
                    </span>
                  )}
                </button>
              </div>
            ))}
            {indexItems.length === 0 && <span class="dim browse-rail-index-empty">{t.browse.indexEmpty}</span>}
          </div>
        </div>
      </aside>
    </>
  );
}
