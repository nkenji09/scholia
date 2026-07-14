import { useLookups } from '../../lookups';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import type { Decision, SpecReport, Tag, VocabEntry } from '../../types';
import { Markdown } from '../Markdown';
import { Chip, kindColor } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { useComments } from '../comments/useComments';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';
import { CollapsibleSection } from '../shared/CollapsibleSection';
import { HashLink } from '../shared/HashLink';
import { routeHash } from '../../router';

// VocabCard と同じ category→アイコン対応（きっかけ/前提/結果 = action/
// condition/effect の固定3軸）。関連語彙行（H3）で流用する。
const CATEGORY_ICON: Record<VocabEntry['category'], IconName> = {
  action: 'circle-play',
  condition: 'funnel',
  effect: 'arrow-right-to-line',
};

interface Props {
  tag: Tag;
  report: SpecReport | undefined;
  isGap: boolean | undefined; // undefined = this tag's kind isn't traceability-tracked
  parents: Tag[];
  children: Tag[];
  cardRef: (el: HTMLElement | null) => void;
  onFilterSelf: () => void;
  onSelectParent: (tagId: string) => void;
  onSelectChild: (tagId: string) => void;
  onSelectSpec: (txId: string) => void;
}

function dedupeDecisions(decisions: Decision[]): Decision[] {
  const seen = new Set<string>();
  const out: Decision[] = [];
  for (const d of decisions) {
    if (seen.has(d.id)) continue;
    seen.add(d.id);
    out.push(d);
  }
  return out;
}

export function TagCard({ tag, report, isGap, parents, children, cardRef, onFilterSelf, onSelectParent, onSelectChild, onSelectSpec }: Props) {
  const t = useT();
  const { tagKindLabel } = useLookups();
  const { changedTagIds } = usePendingDiff();
  const { openComposer, comments } = useComments();
  const entries = report?.entries || [];
  // H3: このタグを直接持つ語彙（Go 側 render.SpecReport.RelatedVocab・
  // VocabEntry.Tags の逆引き）。関連仕様の"上"に常時開きで出す。
  const relatedVocab = report?.relatedVocab || [];
  // このタグ自身に直接ぶら下がる decision（own-only・祖先/子孫の cross-cutting は
  // 出さない・req.comfortable-viewer.decision-display）。Go 側 render.SpecReport の
  // トップレベル tagDecisions を読む。従来は entries を flatMap して拾っていたが、
  // transition を持たないタグでは entries が空で decision が完全に消えていた
  // （tag-decision-visibility）。target.id 明示照合は「そのレコード自身の意思決定
  // だけ」を保証する保険として残す。
  const tagDecisions = dedupeDecisions((report?.tagDecisions || []).filter((d) => d.target.type === 'tag' && d.target.id === tag.id));
  // §8.8 P5 vocab/tag（generalized from SpecCard's hasUncommentedChange・
  // §8.3）: see VocabCard.tsx's identical comment.
  const hasUncommentedChange = changedTagIds.has(tag.id) && !comments.some((c) => c.recordType === 'tag' && c.recordId === tag.id);

  return (
    <article ref={cardRef} data-card-id={tag.id} class="card" title={tag.id}>
      {hasUncommentedChange && (
        <button
          type="button"
          class="spec-card-clean-flag"
          onClick={() =>
            openComposer({ recordType: 'tag', recordId: tag.id, recordTitle: tag.name || tag.id, anchor: 'card', anchorLabel: t.comments.cardAnchorLabel })
          }
        >
          <Icon name="git-compare" size={12} /> {t.comments.proposalCleanFlag}
        </button>
      )}
      <div class="tag-card-head">
        <div class="tag-card-badges">
          <Chip color={kindColor(tag.kind)} onClick={onFilterSelf}>
            {tag.kind ? tagKindLabel(tag.kind) : '?'}
          </Chip>
          {parents.length > 0 && (
            <span class="tag-card-parents dim">
              <Icon name="corner-down-right" size={13} />
              {parents.map((p) => (
                <HashLink key={p.id} href={routeHash({ view: 'spec', tagId: p.id })} class="tag-card-parent-link" onNavigate={() => onSelectParent(p.id)} title={t.browse.parentLinkTitle}>
                  {p.name || p.id}
                </HashLink>
              ))}
            </span>
          )}
          <span class="tag-card-spacer" />
          {isGap && (
            <span class="tag-card-gap-badge">
              <Icon name="triangle-alert" size={12} /> {t.browse.gapBadge}
            </span>
          )}
          {!isGap && entries.length > 0 && (
            <span class="tag-card-sat-badge">
              <Icon name="check" size={12} /> {t.browse.satBadge(entries.length)}
            </span>
          )}
          <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="card" anchorLabel={t.comments.cardAnchorLabel} />
        </div>
        <button type="button" class="tag-card-name" onClick={onFilterSelf} title={t.browse.clickToFilter}>
          {tag.name || tag.id}
          <Icon name="plus" size={13} class="filter-plus-icon" />
        </button>
      </div>

      {tag.description && (
        <div class="tag-card-body">
          <div class="card-section-heading-row">
            <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="body" anchorLabel={t.comments.descriptionAnchorLabel} />
          </div>
          <Markdown text={tag.description} />
        </div>
      )}

      {/* H3: 関連語彙（このタグを直接持つ vocab）。関連仕様の"上"・常時開き
          （ユーザー明示「開閉できなくて良い」＝CollapsibleSection ではなく素の
          card-section）。各行は category バッジ（きっかけ/前提/結果）＋kind
          （api/prop/user 等）＋ラベルで VocabCard の vocab 行を踏襲。 */}
      {relatedVocab.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="book-open" size={14} /> {t.browse.relatedVocab} <span class="card-section-count dim">({relatedVocab.length})</span>
            </span>
          </div>
          <div class="tag-card-spec-list">
            {relatedVocab.map((v) => (
              <div key={v.id} class="tag-card-vocab-row" title={v.id}>
                <Chip color={kindColor(v.category)}>
                  <Icon name={CATEGORY_ICON[v.category]} size={12} /> {t.vocab.categoryLabel(v.category)}
                </Chip>
                {v.kind && <span class="vocab-card-kind dim">{v.kind}</span>}
                <span class="tag-card-vocab-label">{v.label}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {entries.length > 0 && (
        <CollapsibleSection
          recordId={tag.id}
          section="specs"
          count={entries.length}
          icon="scroll-text"
          label={t.browse.satisfiedSpecs}
          extra={<CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="specs" anchorLabel={t.browse.satisfiedSpecs} />}
        >
          <div class="tag-card-spec-list">
            {entries.map((e) => (
              <HashLink key={e.transition.id} href={routeHash({ view: 'browse', txId: e.transition.id })} class="tag-card-spec-row" onNavigate={() => onSelectSpec(e.transition.id)} title={e.transition.id}>
                <span class="tag-card-spec-label">{e.actionLabel}</span>
              </HashLink>
            ))}
          </div>
        </CollapsibleSection>
      )}

      {tagDecisions.length > 0 && (
        <CollapsibleSection
          recordId={tag.id}
          section="decisions"
          count={tagDecisions.length}
          icon="gavel"
          label={t.browse.relatedDecisions}
          // decision はタグの核となる履歴なので件数しきい値で隠さず既定展開。
          // 特に「【不採用】」判断のように「一番残したい履歴」を折りたたまない
          // （tag-decision-visibility）。localStorage 済みのユーザー操作は従来通り最優先。
          defaultOpen={true}
          extra={
            <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="decisions" anchorLabel={t.browse.relatedDecisions} />
          }
        >
          {tagDecisions.map((d) => (
            <div key={d.id} class="tag-card-decision">
              <p>{d.why}</p>
              <span class="dim">
                {d.at.slice(0, 10)} {d.ref && `· ${d.ref}`}
              </span>
            </div>
          ))}
        </CollapsibleSection>
      )}

      {/* H2: 下位のタグを件数付きで開閉可能に（5件以上で既定折りたたみ＝
          CollapsibleSection の既定しきい値そのまま）。specs/decisions と同じ
          パターン。CommentButton は extra prop で維持。 */}
      {children.length > 0 && (
        <CollapsibleSection
          recordId={tag.id}
          section="children"
          count={children.length}
          icon="list-tree"
          label={t.browse.childTags}
          extra={<CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="children" anchorLabel={t.browse.childTags} />}
        >
          <div class="tag-card-children">
            {children.map((c) => (
              <HashLink key={c.id} href={routeHash({ view: 'spec', tagId: c.id })} class="tag-card-child-chip" onNavigate={() => onSelectChild(c.id)} title={t.browse.childLinkTitle}>
                <Icon name="corner-down-right" size={12} /> {c.name || c.id}
              </HashLink>
            ))}
          </div>
        </CollapsibleSection>
      )}
    </article>
  );
}
