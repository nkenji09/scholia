import type { ComponentChildren } from 'preact';
import { useEffect } from 'preact/hooks';
import { usePendingDiff } from '../../pendingDiff';
import { useLookups } from '../../lookups';
import { useT } from '../../i18n';
import { Icon } from '../shared/Icon';
import { Markdown } from '../Markdown';
import type { RecordType } from './useComments';

interface Props {
  recordType: Extract<RecordType, 'vocab' | 'tag'>;
  recordId: string;
}

type AtomKind = 'keep' | 'add' | 'del';

function Atom({ kind, children }: { kind: AtomKind; children: string }) {
  return (
    <span class={`proposal-atom proposal-atom-${kind}`}>
      <span class="proposal-atom-label">{children}</span>
    </span>
  );
}

// Set-diff for a row of chips (parentIds) — same before/after decoration
// logic as ProposalCard.tsx's setDiffIds, kept as its own small copy here
// since this card is read-only (no add/remove/move controls to attach to
// each atom, unlike ProposalCard's picker-driven rows).
function setDiffIds(before: string[], after: string[]): { id: string; kind: AtomKind }[] {
  const beforeSet = new Set(before);
  const afterSet = new Set(after);
  const kept = before.filter((id) => afterSet.has(id));
  const removed = before.filter((id) => !afterSet.has(id));
  const added = after.filter((id) => !beforeSet.has(id));
  return [
    ...kept.map((id) => ({ id, kind: 'keep' as AtomKind })),
    ...removed.map((id) => ({ id, kind: 'del' as AtomKind })),
    ...added.map((id) => ({ id, kind: 'add' as AtomKind })),
  ];
}

function ScalarRow({ label, before, after }: { label: string; before?: string; after?: string }) {
  if (!before && !after) return null;
  return (
    <div class="proposal-row">
      <span class="proposal-row-key">{label}</span>
      <span class="proposal-row-atoms">
        {before !== after ? (
          <>
            {before && <Atom kind="del">{before}</Atom>}
            {after && <Atom kind="add">{after}</Atom>}
          </>
        ) : (
          <Atom kind="keep">{before || ''}</Atom>
        )}
      </span>
    </div>
  );
}

function SetRow({ label, before, after, resolveLabel, emptyLabel }: { label: string; before: string[]; after: string[]; resolveLabel: (id: string) => string; emptyLabel: string }) {
  const atoms = setDiffIds(before, after);
  return (
    <div class="proposal-row">
      <span class="proposal-row-key">{label}</span>
      <span class="proposal-row-atoms">
        {atoms.length === 0 && <span class="dim">{emptyLabel}</span>}
        {atoms.map((a) => (
          <Atom key={a.id} kind={a.kind}>
            {resolveLabel(a.id)}
          </Atom>
        ))}
      </span>
    </div>
  );
}

function TextRow({ label, before, after }: { label: string; before?: string; after?: string }) {
  if (!before && !after) return null;
  const changed = before !== after;
  return (
    <div class="proposal-row proposal-row-text">
      <span class="proposal-row-key">{label}</span>
      <div class="proposal-text-diff">
        {changed && before && (
          <div class="proposal-text-before">
            <Markdown text={before} />
          </div>
        )}
        <div class={changed ? 'proposal-text-after' : undefined}>
          <Markdown text={changed ? after : before} />
        </div>
      </div>
    </div>
  );
}

// §8.8 P5 vocab/tag（change-cockpit-design-v3.md §8.8「vocab/tag も
// reconcile されバッジに乗る」）: ProposalCard.tsx はそのまま流用できない
// （語彙ピッカー手直しは Transition 専用の書込エンドポイント — POST
// /api/transition — 前提。vocab/tag に対応する書込エンドポイントは無い）
// ので、read-only の before/after 表示だけを持つ姉妹コンポーネントとして
// 新設する。`.proposal-card`/`.proposal-row`/`.proposal-atom` 系 CSS
// （P2/P3）はそのまま再利用。
export function RecordDiffCard({ recordType, recordId }: Props) {
  const t = useT();
  const { unavailable, getVocabChange, getTagChange, getAddedVocab, getAddedTag, refresh } = usePendingDiff();
  const { tagName } = useLookups();

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(refresh, [recordType, recordId]);

  if (unavailable === 'static') {
    return <p class="proposal-card-note dim">{t.api.unavailable(t.comments.proposalWhatLabel)}</p>;
  }
  if (unavailable === 'error') {
    return <p class="proposal-card-note dim">{t.comments.proposalUnavailableError}</p>;
  }

  const head = (isAdded: boolean, children: ComponentChildren) => (
    <div class={'proposal-card' + (isAdded ? ' proposal-card-added' : '')}>
      <div class="proposal-card-head">
        <Icon name="git-compare" size={14} />
        <span class="proposal-card-title">{t.comments.proposalHeading}</span>
        <span class={'proposal-card-badge' + (isAdded ? ' proposal-card-badge-added' : '')}>
          {isAdded ? t.comments.proposalAddedBadge : t.comments.proposalUncommitted}
        </span>
      </div>
      {children}
    </div>
  );

  // #32 A是正: main に無い新規レコード（before が存在しない）は
  // getVocabChange/getTagChange では拾えない — vocab.added[]/tags.added[]
  // 側から引く。ScalarRow/SetRow/TextRow は before が undefined でも
  // 「全項目 added」として描画できる（before 側の分岐が素通りするだけ）ので、
  // 変更後カードと同じ行コンポーネントをそのまま再利用できる。
  if (recordType === 'vocab') {
    const change = getVocabChange(recordId);
    const after = change?.after ?? getAddedVocab(recordId);
    if (!after) return null;
    const before = change?.before;
    return head(
      !change,
      <>
        <ScalarRow label={t.comments.recordDiffLabelField} before={before?.label} after={after.label} />
        <ScalarRow label={t.comments.recordDiffKindField} before={before?.kind} after={after.kind} />
        <ScalarRow label={t.vocab.owner} before={before?.owner} after={after.owner} />
        <TextRow label={t.comments.recordDiffDescriptionField} before={before?.description} after={after.description} />
      </>,
    );
  }

  const change = getTagChange(recordId);
  const after = change?.after ?? getAddedTag(recordId);
  if (!after) return null;
  const before = change?.before;
  return head(
    !change,
    <>
      <ScalarRow label={t.comments.recordDiffNameField} before={before?.name} after={after.name} />
      <ScalarRow label={t.comments.recordDiffKindField} before={before?.kind} after={after.kind} />
      <SetRow
        label={t.comments.recordDiffParentsField}
        before={before?.parentIds || []}
        after={after.parentIds || []}
        resolveLabel={tagName}
        emptyLabel={t.comments.recordDiffNoParents}
      />
    </>,
  );
}
