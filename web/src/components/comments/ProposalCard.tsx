import { useEffect } from 'preact/hooks';
import { usePendingDiff } from '../../pendingDiff';
import { useLookups } from '../../lookups';
import { useT } from '../../i18n';
import { Icon } from '../shared/Icon';

interface Props {
  txId: string;
}

type AtomKind = 'keep' | 'add' | 'del';

function Atom({ kind, children }: { kind: AtomKind; children: string }) {
  return <span class={`proposal-atom proposal-atom-${kind}`}>{children}</span>;
}

/** Set-diff for display only — before/after are already the full arrays the
    backend gave us (TransitionChange.before/after), so this doesn't derive
    any new domain fact, just orders them into keep/add/del for rendering.
    Given/Tags use the backend's own added/removed (authoritative, set
    comparison); Then has no such fields (it's an ordered list — see
    internal/diff/diff.go's comment on TransitionChange) so this falls back
    to the same treatment purely for the read-only diff card. */
function setDiffIds(before: string[], removed: string[] | undefined, added: string[] | undefined): { id: string; kind: AtomKind }[] {
  const removedSet = new Set(removed || []);
  const kept = before.filter((id) => !removedSet.has(id));
  return [
    ...kept.map((id) => ({ id, kind: 'keep' as AtomKind })),
    ...(removed || []).map((id) => ({ id, kind: 'del' as AtomKind })),
    ...(added || []).map((id) => ({ id, kind: 'add' as AtomKind })),
  ];
}

function thenDiff(before: string[], after: string[]): { id: string; kind: AtomKind }[] {
  const beforeSet = new Set(before);
  const afterSet = new Set(after);
  const removed = before.filter((id) => !afterSet.has(id));
  const added = after.filter((id) => !beforeSet.has(id));
  return setDiffIds(before, removed, added);
}

export function ProposalCard({ txId }: Props) {
  const t = useT();
  const { unavailable, getChange, refresh } = usePendingDiff();
  const { vocabLabel, tagName } = useLookups();

  // Refetch when the drawer's focus lands on a (possibly different)
  // transition, so a `.pmem/` edit made just before opening the drawer
  // shows up without a manual reload — no polling beyond this.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(refresh, [txId]);

  if (unavailable === 'static') {
    return <p class="proposal-card-note dim">{t.api.unavailable(t.comments.proposalWhatLabel)}</p>;
  }
  if (unavailable === 'error') {
    return <p class="proposal-card-note dim">{t.comments.proposalUnavailableError}</p>;
  }

  const change = getChange(txId);
  if (!change) return null;

  const givenAtoms = setDiffIds(change.before.given, change.givenRemoved, change.givenAdded);
  const thenAtoms = thenDiff(change.before.then, change.after.then);
  const tagsAtoms = setDiffIds(change.before.tags || [], change.tagsRemoved, change.tagsAdded);
  const thenSetUnchanged = !change.thenChanged && change.thenReordered;

  return (
    <div class="proposal-card">
      <div class="proposal-card-head">
        <Icon name="git-compare" size={14} />
        <span class="proposal-card-title">{t.comments.proposalHeading}</span>
        <span class="proposal-card-badge">{t.comments.proposalUncommitted}</span>
      </div>

      <div class="proposal-row">
        <span class="proposal-row-key">{t.flow.trigger}</span>
        <span class="proposal-row-atoms">
          {change.actionChanged ? (
            <>
              <Atom kind="del">{vocabLabel(change.before.action)}</Atom>
              <Atom kind="add">{vocabLabel(change.after.action)}</Atom>
            </>
          ) : (
            <Atom kind="keep">{vocabLabel(change.after.action)}</Atom>
          )}
        </span>
      </div>

      <div class="proposal-row">
        <span class="proposal-row-key">{t.flow.given}</span>
        <span class="proposal-row-atoms">
          {givenAtoms.length === 0 && <span class="dim">{t.flow.noGiven}</span>}
          {givenAtoms.map((a) => (
            <Atom key={a.id} kind={a.kind}>
              {vocabLabel(a.id)}
            </Atom>
          ))}
        </span>
      </div>

      <div class="proposal-row">
        <span class="proposal-row-key">{t.flow.result}</span>
        <span class="proposal-row-atoms">
          {thenAtoms.map((a) => (
            <Atom key={a.id} kind={a.kind}>
              {vocabLabel(a.id)}
            </Atom>
          ))}
          {thenSetUnchanged && <span class="proposal-reordered-note dim">{t.comments.proposalReordered}</span>}
        </span>
      </div>

      <div class="proposal-row">
        <span class="proposal-row-key">{t.browse.tagsHeading}</span>
        <span class="proposal-row-atoms">
          {tagsAtoms.map((a) => (
            <Atom key={a.id} kind={a.kind}>
              {tagName(a.id)}
            </Atom>
          ))}
        </span>
      </div>
    </div>
  );
}
