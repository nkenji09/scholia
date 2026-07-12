import type { ComponentChildren } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { usePendingDiff } from '../../pendingDiff';
import { useLookups } from '../../lookups';
import { useT } from '../../i18n';
import { Icon } from '../shared/Icon';
import { VocabPicker } from './VocabPicker';
import { api } from '../../api';
import type { Transition } from '../../types';

interface Props {
  txId: string;
}

type AtomKind = 'keep' | 'add' | 'del';

function Atom({ kind, actions, children }: { kind: AtomKind; actions?: ComponentChildren; children: string }) {
  return (
    <span class={`proposal-atom proposal-atom-${kind}`}>
      <span class="proposal-atom-label">{children}</span>
      {actions}
    </span>
  );
}

/** Set-diff for display — `before` is the pending-diff base (§2's `main`),
    `after` is whatever the caller wants compared against it: either the
    server's current working-tree value (no local edit in progress) or the
    live local draft (mid-edit, not yet written — §3's "クライアント下書き").
    Reusing the same function for both means the picker's live preview and
    the settled post-write state render identically once `after` converges. */
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

function sameSet(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const bs = new Set(b);
  return a.every((id) => bs.has(id));
}

function sameOrder(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((id, i) => id === b[i]);
}

function cloneTransition(t: Transition): Transition {
  return { id: t.id, action: t.action, given: [...t.given], then: [...t.then], tags: [...(t.tags || [])] };
}

export function ProposalCard({ txId }: Props) {
  const t = useT();
  const { unavailable, getChange, refresh } = usePendingDiff();
  const { vocabById, tagById, vocabLabel, tagName } = useLookups();

  // Refetch when the drawer's focus lands on a (possibly different)
  // transition, so a `.pmem/` edit made just before opening the drawer
  // shows up without a manual reload — no polling beyond this.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(refresh, [txId]);

  const change = getChange(txId);

  const [draft, setDraft] = useState<Transition | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // The draft (§3's "クライアント下書き") always starts from the current
  // working-tree state (`change.after`). Reset it whenever the focused
  // transition changes, or whenever `after` itself changes underneath us —
  // which happens right after a successful reflect, once refresh() above
  // re-fetches the diff and getChange(txId) starts returning the newly
  // written record. That reset is what makes "反映" converge: the draft
  // becomes indistinguishable from the just-saved after, so the diff
  // renders the same before/after picture it would from a page reload.
  const afterKey = change ? JSON.stringify(change.after) : '';
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    setDraft(change ? cloneTransition(change.after) : null);
    setError(null);
  }, [txId, afterKey]);

  if (unavailable === 'static') {
    return <p class="proposal-card-note dim">{t.api.unavailable(t.comments.proposalWhatLabel)}</p>;
  }
  if (unavailable === 'error') {
    return <p class="proposal-card-note dim">{t.comments.proposalUnavailableError}</p>;
  }
  if (!change || !draft) return null;

  const dirty =
    draft.action !== change.after.action ||
    !sameSet(draft.given, change.after.given) ||
    !sameOrder(draft.then, change.after.then) ||
    !sameSet(draft.tags || [], change.after.tags || []);

  const setAction = (id: string) => setDraft((d) => (d ? { ...d, action: id } : d));
  const addGiven = (id: string) => setDraft((d) => (d && !d.given.includes(id) ? { ...d, given: [...d.given, id] } : d));
  const removeGiven = (id: string) => setDraft((d) => (d ? { ...d, given: d.given.filter((x) => x !== id) } : d));
  const addThen = (id: string) => setDraft((d) => (d && !d.then.includes(id) ? { ...d, then: [...d.then, id] } : d));
  const removeThen = (id: string) => setDraft((d) => (d ? { ...d, then: d.then.filter((x) => x !== id) } : d));
  const moveThen = (index: number, dir: -1 | 1) =>
    setDraft((d) => {
      if (!d) return d;
      const j = index + dir;
      if (j < 0 || j >= d.then.length) return d;
      const then = [...d.then];
      [then[index], then[j]] = [then[j], then[index]];
      return { ...d, then };
    });
  const addTag = (id: string) => setDraft((d) => (d && !(d.tags || []).includes(id) ? { ...d, tags: [...(d.tags || []), id] } : d));
  const removeTag = (id: string) => setDraft((d) => (d ? { ...d, tags: (d.tags || []).filter((x) => x !== id) } : d));

  const onReflect = async () => {
    setSaving(true);
    setError(null);
    try {
      await api.putTransition({ id: draft.id, action: draft.action, given: draft.given, then: draft.then, tags: draft.tags || [] });
      refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  const actionOptions = [...vocabById.values()]
    .filter((v) => v.category === 'action' && v.id !== draft.action)
    .map((v) => ({ id: v.id, label: v.label }))
    .sort((a, b) => a.label.localeCompare(b.label));
  const givenOptions = [...vocabById.values()]
    .filter((v) => v.category === 'condition' && !draft.given.includes(v.id))
    .map((v) => ({ id: v.id, label: v.label }))
    .sort((a, b) => a.label.localeCompare(b.label));
  const thenOptions = [...vocabById.values()]
    .filter((v) => v.category === 'effect' && !draft.then.includes(v.id))
    .map((v) => ({ id: v.id, label: v.label }))
    .sort((a, b) => a.label.localeCompare(b.label));
  const tagOptions = [...tagById.values()]
    .filter((tg) => !(draft.tags || []).includes(tg.id))
    .map((tg) => ({ id: tg.id, label: tg.name }))
    .sort((a, b) => a.label.localeCompare(b.label));

  const givenAtoms = setDiffIds(change.before.given, draft.given);
  const thenAtoms = setDiffIds(change.before.then, draft.then);
  const tagsAtoms = setDiffIds(change.before.tags || [], draft.tags || []);

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
          {draft.action !== change.before.action ? (
            <>
              <Atom kind="del">{vocabLabel(change.before.action)}</Atom>
              <Atom kind="add">{vocabLabel(draft.action)}</Atom>
            </>
          ) : (
            <Atom kind="keep">{vocabLabel(draft.action)}</Atom>
          )}
          <VocabPicker
            options={actionOptions}
            onSelect={setAction}
            triggerLabel={t.comments.pickerAddButton}
            searchPlaceholder={t.comments.pickerSearchPlaceholder}
            emptyLabel={t.comments.pickerEmpty}
          />
        </span>
      </div>

      <div class="proposal-row">
        <span class="proposal-row-key">{t.flow.given}</span>
        <span class="proposal-row-atoms">
          {givenAtoms.length === 0 && <span class="dim">{t.flow.noGiven}</span>}
          {givenAtoms.map((a) => (
            <Atom
              key={a.id}
              kind={a.kind}
              actions={
                a.kind !== 'del' && (
                  <button type="button" class="proposal-atom-remove" title={t.comments.pickerRemoveTitle} onClick={() => removeGiven(a.id)}>
                    <Icon name="x" size={13} />
                  </button>
                )
              }
            >
              {vocabLabel(a.id)}
            </Atom>
          ))}
          <VocabPicker
            options={givenOptions}
            onSelect={addGiven}
            triggerLabel={t.comments.pickerAddButton}
            searchPlaceholder={t.comments.pickerSearchPlaceholder}
            emptyLabel={t.comments.pickerEmpty}
          />
        </span>
      </div>

      <div class="proposal-row">
        <span class="proposal-row-key">{t.flow.result}</span>
        <span class="proposal-row-atoms">
          {thenAtoms.map((a) => {
            const idx = draft.then.indexOf(a.id);
            return (
              <Atom
                key={a.id}
                kind={a.kind}
                actions={
                  a.kind !== 'del' && (
                    <>
                      <button
                        type="button"
                        class="proposal-atom-move"
                        title={t.comments.pickerMoveUpTitle}
                        disabled={idx <= 0}
                        onClick={() => moveThen(idx, -1)}
                      >
                        <Icon name="chevron-up" size={13} />
                      </button>
                      <button
                        type="button"
                        class="proposal-atom-move"
                        title={t.comments.pickerMoveDownTitle}
                        disabled={idx < 0 || idx >= draft.then.length - 1}
                        onClick={() => moveThen(idx, 1)}
                      >
                        <Icon name="chevron-down" size={13} />
                      </button>
                      <button type="button" class="proposal-atom-remove" title={t.comments.pickerRemoveTitle} onClick={() => removeThen(a.id)}>
                        <Icon name="x" size={13} />
                      </button>
                    </>
                  )
                }
              >
                {vocabLabel(a.id)}
              </Atom>
            );
          })}
          <VocabPicker
            options={thenOptions}
            onSelect={addThen}
            triggerLabel={t.comments.pickerAddButton}
            searchPlaceholder={t.comments.pickerSearchPlaceholder}
            emptyLabel={t.comments.pickerEmpty}
          />
        </span>
      </div>

      <div class="proposal-row">
        <span class="proposal-row-key">{t.browse.tagsHeading}</span>
        <span class="proposal-row-atoms">
          {tagsAtoms.map((a) => (
            <Atom
              key={a.id}
              kind={a.kind}
              actions={
                a.kind !== 'del' && (
                  <button type="button" class="proposal-atom-remove" title={t.comments.pickerRemoveTitle} onClick={() => removeTag(a.id)}>
                    <Icon name="x" size={13} />
                  </button>
                )
              }
            >
              {tagName(a.id)}
            </Atom>
          ))}
          <VocabPicker
            options={tagOptions}
            onSelect={addTag}
            triggerLabel={t.comments.pickerAddButton}
            searchPlaceholder={t.comments.pickerSearchPlaceholder}
            emptyLabel={t.comments.pickerEmpty}
          />
        </span>
      </div>

      <div class="proposal-card-actions">
        {error && <p class="proposal-card-error">{t.comments.reflectError(error)}</p>}
        <button type="button" class="proposal-reflect-btn" disabled={!dirty || saving} onClick={onReflect}>
          <Icon name="save" size={13} /> {saving ? t.comments.reflecting : t.comments.reflectButton}
        </button>
      </div>
    </div>
  );
}
