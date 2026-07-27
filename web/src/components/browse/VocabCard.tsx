import { useLookups } from '../../lookups';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import type { Decision, Transition, VocabEntry } from '../../types';
import { Markdown } from '../Markdown';
import { Chip, kindColor, OWNER_COLOR } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { useComments } from '../comments/useComments';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';
import { CollapsibleSection } from '../shared/CollapsibleSection';
import { HashLink } from '../shared/HashLink';
import { KebabMenu } from '../shared/KebabMenu';
import type { KebabMenuItem } from '../shared/KebabMenu';
import { routeHash } from '../../router';
import { DecisionList } from '../decisions/DecisionList';
import { InheritedRules } from './InheritedRules';

interface Props {
  entry: VocabEntry;
  uses: Transition[];
  /** vocab-target decision（#45 D5）— この語彙自身を target とする意思決定。 */
  decisions: Decision[];
  /** establishes 逆引き（#45 D5）— この語彙（condition）を成立させる effect の id 群。 */
  establishedBy: string[];
  cardRef: (el: HTMLElement | null) => void;
  onFilterTag: (id: string) => void;
  onFilterOwner: (owner: string) => void;
  onSelectTx: (txId: string) => void;
}

const CATEGORY_ICON: Record<VocabEntry['category'], IconName> = {
  action: 'circle-play',
  condition: 'funnel',
  effect: 'arrow-right-to-line',
};

// Mirrors TagCard's layout (kind badge → name/id → description →
// card-sections) so Vocab reads as the same design language as タグ/仕様
// rather than its own bespoke list — see .concierge/decision.md's tweaks2
// handoff §4. Several classnames (tag-card-head/-badges/-name/-id,
// tag-card-spec-list/-row/-label/-id) are reused as-is from TagCard's CSS;
// they're generic "card head" / "row of records" patterns, not actually
// tag-specific.
export function VocabCard({ entry, uses, decisions, establishedBy, cardRef, onFilterTag, onFilterOwner, onSelectTx }: Props) {
  const t = useT();
  const { tagById, transitionLabel, vocabLabel, ownerKind } = useLookups();
  const { changedVocabIds } = usePendingDiff();
  const { openComposer, comments } = useComments();
  const tags = entry.tags || [];
  const altLabels = entry.altLabels || [];
  const establishes = entry.establishes || [];
  // §8.8 P5 vocab/tag（generalized from SpecCard's hasUncommentedChange・
  // §8.3）: a pending change with no comment yet is a quiet pending-change
  // flag, not a "proposal" — it steps aside once someone comments on this
  // entry (the comment itself then carries the diff card and badge count).
  const hasUncommentedChange = changedVocabIds.has(entry.id) && !comments.some((c) => c.recordType === 'vocab' && c.recordId === entry.id);

  // #45 D9 amend（01KXY6Q49Z…）: owner の⋮メニュー。ownerKind 宣言下では owner が
  // subject タグ id 参照＝正準ルート（タグ詳細）を持つので「リンク先を開く」を足す。
  // ownerKind 未宣言、または owner が実在タグに解決しないプロジェクトでは従来どおり
  // 「検索条件に追加」のみ（オプトイン原則・根拠の変化のみを反映）。
  const ownerMenuItems = (owner: string): KebabMenuItem[] => {
    const items: KebabMenuItem[] = [
      { key: 'filter', label: t.browse.menuAddFilter, icon: 'plus', onSelect: () => onFilterOwner(owner) },
    ];
    if (ownerKind && tagById.has(owner)) {
      items.push({ key: 'open', label: t.browse.menuOpenLink, icon: 'external-link', href: routeHash({ view: 'spec', tagId: owner }) });
    }
    return items;
  };

  return (
    <article ref={cardRef} data-card-id={entry.id} class="card" title={entry.id}>
      {hasUncommentedChange && (
        <button
          type="button"
          class="spec-card-clean-flag"
          onClick={() => openComposer({ recordType: 'vocab', recordId: entry.id, recordTitle: entry.label, anchor: 'card', anchorLabel: t.comments.cardAnchorLabel })}
        >
          <Icon name="git-compare" size={12} /> {t.comments.proposalCleanFlag}
        </button>
      )}
      <div class="tag-card-head">
        <div class="tag-card-badges">
          <Chip color={kindColor(entry.category)}>
            <Icon name={CATEGORY_ICON[entry.category]} size={12} /> {t.vocab.categoryLabel(entry.category)}
          </Chip>
          {entry.kind && <span class="vocab-card-kind dim">{entry.kind}</span>}
          <span class="tag-card-spacer" />
          <CommentButton recordType="vocab" recordId={entry.id} recordTitle={entry.label} anchor="card" anchorLabel={t.comments.cardAnchorLabel} />
        </div>
        {/* action 語彙のカード頭ケバブ（#45 D10b-5・tx.viewer.vocab-flow-link）:
            TagCard のカード頭ケバブと同型のカード頭 ⋮ を新設し「フローを表示」を
            置く。VocabCard にカード頭 ⋮ は現存しなかった（既存 ⋮ は owner/参照
            タグの per-chip のみ）。ラベルへの ⋮ を見送った既決（01KXM6H6XT66…）の
            理由は「自分1件へ絞る no-op な AND 条件」で、flow への遷移は no-op で
            ないため非矛盾。action 以外のカテゴリ（condition/effect）はフロー図を
            持たないので ⋮ を出さず従来どおり plain heading のまま。 */}
        <div class="tag-card-name-row">
          <span class="tag-card-name vocab-card-name">{entry.label}</span>
          {entry.category === 'action' && (
            <KebabMenu
              triggerLabel={t.browse.menuTrigger}
              items={[{ key: 'flow', label: t.flow.menuShowFlow, icon: 'git-fork', href: routeHash({ view: 'flow', actionId: entry.id }) }]}
            />
          )}
        </div>
      </div>

      {entry.description && (
        <div class="tag-card-body">
          <Markdown text={entry.description} />
        </div>
      )}

      {entry.ref && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="external-link" size={14} /> {t.vocab.refHeading}
            </span>
          </div>
          {/^https?:\/\//.test(entry.ref) ? (
            <a class="tag-card-ref" href={entry.ref} target="_blank" rel="noreferrer noopener">
              {entry.ref}
            </a>
          ) : (
            <span class="tag-card-ref dim">{entry.ref}</span>
          )}
        </div>
      )}

      {altLabels.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="book-open" size={14} /> {t.vocab.altLabelsHeading}
            </span>
          </div>
          <div class="spec-card-chip-row">
            {altLabels.map((al) => (
              <Chip key={al} color={kindColor(entry.category)}>
                {al}
              </Chip>
            ))}
          </div>
        </div>
      )}

      {entry.owner && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="user" size={14} /> {t.vocab.owner}{' '}
              <span class="spec-card-hint dim">
                <Icon name="plus" size={11} class="filter-plus-icon" /> {t.browse.clickToFilter}
              </span>
            </span>
          </div>
          <div class="spec-card-chip-row">
            <Chip
              color={OWNER_COLOR}
              trailing={
                <KebabMenu
                  triggerLabel={t.browse.menuTrigger}
                  items={ownerMenuItems(entry.owner!)}
                />
              }
            >
              {entry.owner}
            </Chip>
          </div>
        </div>
      )}

      {tags.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="tags" size={14} /> {t.browse.tagsHeading}{' '}
              <span class="spec-card-hint dim">
                <Icon name="plus" size={11} class="filter-plus-icon" /> {t.browse.clickToFilter}
              </span>
            </span>
          </div>
          <div class="spec-card-chip-row">
            {tags.map((id) => (
              <Chip
                key={id}
                color={kindColor(tagById.get(id)?.kind)}
                trailing={
                  <KebabMenu
                    triggerLabel={t.browse.menuTrigger}
                    items={[
                      { key: 'filter', label: t.browse.menuAddFilter, icon: 'plus', onSelect: () => onFilterTag(id) },
                      { key: 'open', label: t.browse.menuOpenLink, icon: 'external-link', href: routeHash({ view: 'spec', tagId: id }) },
                    ]}
                  />
                }
              >
                {tagById.get(id)?.name || id}
              </Chip>
            ))}
          </div>
        </div>
      )}

      {uses.length === 0 ? (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="scroll-text" size={14} /> {t.vocab.usageCount(uses.length)}
            </span>
          </div>
          <span class="vocab-card-no-usage dim">{t.vocab.noUsage}</span>
        </div>
      ) : (
        <CollapsibleSection recordId={entry.id} section="usage" count={uses.length} icon="scroll-text" label={t.vocab.usageHeading}>
          <div class="tag-card-spec-list">
            {uses.map((tx) => {
              const label = transitionLabel(tx.id);
              // 同一 action 複数遷移が同文に潰れる縮退の解消（D10a item 7）: 同じ
              // action・同じ then を持つ複数遷移は primary/secondary だけでは同文に
              // 見えるので、区別している given のラベルを dim 接尾で添える（uses は
              // Transition[] なので given を vocabLabel で解決。TagCard の
              // satisfied-specs 行と同じ流儀。無条件遷移は given が無く接尾も出ない）。
              const given = (tx.given || []).map(vocabLabel).filter((g) => !!g);
              return (
                <HashLink key={tx.id} href={routeHash({ view: 'browse', txId: tx.id })} class="tag-card-spec-row" onNavigate={() => onSelectTx(tx.id)} title={tx.id}>
                  <span class="tag-card-spec-label">
                    {label.primary}
                    {label.secondary && <span class="dim"> {label.secondary}</span>}
                    {given.length > 0 && <span class="dim tag-card-spec-given"> · {given.join('、')}</span>}
                  </span>
                </HashLink>
              );
            })}
          </div>
        </CollapsibleSection>
      )}

      {/* establishes 双方向逆引き（#45 D5）: effect 側は「この効果が成立させる
          条件」、condition 側は「この条件を成立させる効果」。図の導出本体は後
          フェーズだが、辺は双方向にたどれる。 */}
      {establishes.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="git-fork" size={14} /> {t.vocab.establishesHeading} <span class="card-section-count dim">({establishes.length})</span>
            </span>
          </div>
          <div class="tag-card-spec-list">
            {establishes.map((id) => (
              <HashLink
                key={id}
                href={routeHash({ view: 'vocab', vocabId: id })}
                class="tag-card-vocab-row"
                onNavigate={() => {
                  window.location.hash = routeHash({ view: 'vocab', vocabId: id });
                }}
                title={id}
              >
                <span class="tag-card-vocab-label">{vocabLabel(id)}</span>
              </HashLink>
            ))}
          </div>
        </div>
      )}

      {establishedBy.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="arrow-right-to-line" size={14} /> {t.vocab.establishedByHeading} <span class="card-section-count dim">({establishedBy.length})</span>
            </span>
          </div>
          <div class="tag-card-spec-list">
            {establishedBy.map((id) => (
              <HashLink
                key={id}
                href={routeHash({ view: 'vocab', vocabId: id })}
                class="tag-card-vocab-row"
                onNavigate={() => {
                  window.location.hash = routeHash({ view: 'vocab', vocabId: id });
                }}
                title={id}
              >
                <span class="tag-card-vocab-label">{vocabLabel(id)}</span>
              </HashLink>
            ))}
          </div>
        </div>
      )}

      <DecisionList recordId={entry.id} decisions={decisions} label={t.vocab.decisionsHeading} />

      {/* governs 並置（#45 D10b-1）: own の vocab-target decision（上）は不変で、
          own＋vocab.tags/祖先経由の decision を出自バッジ付きで並置する既定折りたたみ欄。 */}
      <InheritedRules record={{ kind: 'vocab', id: entry.id }} />
    </article>
  );
}
