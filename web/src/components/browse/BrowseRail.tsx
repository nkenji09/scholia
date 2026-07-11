import { useState } from 'preact/hooks';
import { strings } from '../../strings';
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
  const [focused, setFocused] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);

  const q = query.trim().toLowerCase();
  const matches = focused && q ? suggestions.filter((s) => (s.id + ' ' + s.label).toLowerCase().includes(q)).slice(0, MAX_SUGGESTIONS) : [];
  const open = matches.length > 0;
  const idx = open ? Math.max(0, Math.min(activeIndex, matches.length - 1)) : -1;

  const selectMatch = (m: SuggestionItem) => {
    m.onSelect();
    onQueryChange('');
    setFocused(false);
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
    <aside class="browse-rail">
      <div class="browse-rail-head">
        <Icon name="sliders-horizontal" size={14} class="dim" />
        <span class="browse-rail-label dim">検索条件</span>
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
          placeholder={strings.browse.searchPlaceholder}
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
          <span class="browse-rail-label dim">種別</span>
          <div class="browse-rail-kinds">
            <button type="button" class={'browse-rail-kind' + (kindFacet === 'all' ? ' active' : '')} onClick={() => onKindFacetChange('all')}>
              <span>{strings.browse.kindAll}</span>
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
              <Icon name="filter" size={13} /> {strings.browse.conditionsHeading} <span class="browse-rail-and">{strings.browse.and}</span>
            </span>
            <button type="button" class="browse-rail-clear" onClick={onClearConditions}>
              {strings.browse.clear}
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
          <Icon name="list" size={13} /> {strings.browse.indexHeading} <span class="browse-rail-index-count">{indexItems.length}</span>
        </span>
        <div class="browse-rail-index-list">
          {indexItems.map((item) => (
            <button
              key={item.id}
              type="button"
              class="browse-rail-index-item"
              style={{ paddingLeft: `${8 + item.indent * 14}px` }}
              onClick={item.onClick}
            >
              <span class="browse-rail-index-dot" style={{ background: item.color }} />
              <span class="browse-rail-index-label">{item.label}</span>
              {item.isGap && (
                <span class="browse-rail-index-gap">
                  <Icon name="triangle-alert" size={12} />
                </span>
              )}
            </button>
          ))}
          {indexItems.length === 0 && <span class="dim browse-rail-index-empty">{strings.browse.indexEmpty}</span>}
        </div>
      </div>
    </aside>
  );
}
