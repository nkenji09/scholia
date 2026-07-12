import { useState } from 'preact/hooks';
import { useLookups } from '../../lookups';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import { api, ApiError } from '../../api';
import { Icon } from '../shared/Icon';
import { VocabPicker } from '../comments/VocabPicker';

interface Props {
  onClose: () => void;
  onCreated: (id: string) => void;
}

interface Draft {
  id: string;
  action: string;
  given: string[];
  then: string[];
  tags: string[];
}

const EMPTY: Draft = { id: '', action: '', given: [], then: [], tags: [] };

// NewTransitionForm — §8.8 P5・M-5「追加」の入口（§3 の 3種別表）: subject
// の仕様一覧に置く「＋ 新規 Transition を提案」導線。ProposalCard の語彙
// ピッカー UI と同じ構造ガード（vocab-id スロットのみ・自由記述不可）を、
// 「反映」ではなく「作成」1 回で POST する（body.id が未実在なら
// internal/viewer/transition_write.go 側が 201 作成として扱う）。
export function NewTransitionForm({ onClose, onCreated }: Props) {
  const t = useT();
  const { vocabById, tagById, vocabLabel, tagName, transitionById } = useLookups();
  const { refresh } = usePendingDiff();
  const [draft, setDraft] = useState<Draft>(EMPTY);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const id = draft.id.trim();
  // id が既存 transition と衝突すると POST は「編集」に倒れてしまう
  // （backend は存在有無だけで create/edit を分岐する）— この「新規作成」
  // フォームはそれを防ぐため事前に弾く。transitionById は app 起動時の
  // 1回読み込みキャッシュ（他タブでの同時作成までは検知しない）だが、
  // ローカル単一ユーザー向けツールとしては十分な防御線。
  const idCollides = id !== '' && transitionById.has(id);
  const idInvalid = id !== '' && (id === '.' || id === '..' || /[/\\]/.test(id));
  const canCreate = id !== '' && !idCollides && !idInvalid && draft.action !== '' && draft.then.length > 0 && !creating;

  const setAction = (vid: string) => setDraft((d) => ({ ...d, action: vid }));
  const addGiven = (vid: string) => setDraft((d) => (d.given.includes(vid) ? d : { ...d, given: [...d.given, vid] }));
  const removeGiven = (vid: string) => setDraft((d) => ({ ...d, given: d.given.filter((x) => x !== vid) }));
  const addThen = (vid: string) => setDraft((d) => (d.then.includes(vid) ? d : { ...d, then: [...d.then, vid] }));
  const removeThen = (vid: string) => setDraft((d) => ({ ...d, then: d.then.filter((x) => x !== vid) }));
  const moveThen = (index: number, dir: -1 | 1) =>
    setDraft((d) => {
      const j = index + dir;
      if (j < 0 || j >= d.then.length) return d;
      const then = [...d.then];
      [then[index], then[j]] = [then[j], then[index]];
      return { ...d, then };
    });
  const addTag = (tid: string) => setDraft((d) => (d.tags.includes(tid) ? d : { ...d, tags: [...d.tags, tid] }));
  const removeTag = (tid: string) => setDraft((d) => ({ ...d, tags: d.tags.filter((x) => x !== tid) }));

  const onCreate = async () => {
    if (!canCreate) return;
    setCreating(true);
    setError(null);
    try {
      const created = await api.createTransition({ id, action: draft.action, given: draft.given, then: draft.then, tags: draft.tags });
      refresh();
      onCreated(created.id);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setCreating(false);
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
    .filter((tg) => !draft.tags.includes(tg.id))
    .map((tg) => ({ id: tg.id, label: tg.name }))
    .sort((a, b) => a.label.localeCompare(b.label));

  return (
    <div class="card new-transition-form">
      <div class="proposal-card-head">
        <Icon name="plus" size={14} />
        <span class="proposal-card-title">{t.comments.newTransitionButton}</span>
      </div>

      <div class="proposal-row">
        <span class="proposal-row-key">{t.comments.newTransitionIdLabel}</span>
        <input
          class="new-transition-id-input"
          type="text"
          value={draft.id}
          placeholder={t.comments.newTransitionIdPlaceholder}
          onInput={(e) => setDraft((d) => ({ ...d, id: (e.target as HTMLInputElement).value }))}
        />
      </div>
      {idCollides && <p class="proposal-card-error">{t.comments.newTransitionIdDuplicate(id)}</p>}

      <div class="proposal-row">
        <span class="proposal-row-key">{t.flow.trigger}</span>
        <span class="proposal-row-atoms">
          {draft.action ? (
            <span class="proposal-atom proposal-atom-add">
              <span class="proposal-atom-label">{vocabLabel(draft.action)}</span>
            </span>
          ) : (
            <span class="dim">{t.comments.newTransitionActionUnset}</span>
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
          {draft.given.length === 0 && <span class="dim">{t.flow.noGiven}</span>}
          {draft.given.map((gid) => (
            <span key={gid} class="proposal-atom proposal-atom-add">
              <span class="proposal-atom-label">{vocabLabel(gid)}</span>
              <button type="button" class="proposal-atom-remove" title={t.comments.pickerRemoveTitle} onClick={() => removeGiven(gid)}>
                <Icon name="x" size={13} />
              </button>
            </span>
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
          {draft.then.map((tid, idx) => (
            <span key={tid} class="proposal-atom proposal-atom-add">
              <span class="proposal-atom-label">{vocabLabel(tid)}</span>
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
                disabled={idx >= draft.then.length - 1}
                onClick={() => moveThen(idx, 1)}
              >
                <Icon name="chevron-down" size={13} />
              </button>
              <button type="button" class="proposal-atom-remove" title={t.comments.pickerRemoveTitle} onClick={() => removeThen(tid)}>
                <Icon name="x" size={13} />
              </button>
            </span>
          ))}
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
          {draft.tags.map((tid) => (
            <span key={tid} class="proposal-atom proposal-atom-add">
              <span class="proposal-atom-label">{tagName(tid)}</span>
              <button type="button" class="proposal-atom-remove" title={t.comments.pickerRemoveTitle} onClick={() => removeTag(tid)}>
                <Icon name="x" size={13} />
              </button>
            </span>
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
        {error && <p class="proposal-card-error">{t.comments.newTransitionCreateError(error)}</p>}
        <button type="button" class="proposal-reflect-btn" disabled={!canCreate} onClick={onCreate}>
          <Icon name="save" size={13} /> {creating ? t.comments.newTransitionCreating : t.comments.newTransitionCreateButton}
        </button>
        <button type="button" class="comment-btn-secondary" onClick={onClose} disabled={creating}>
          {t.comments.newTransitionCancel}
        </button>
      </div>
    </div>
  );
}
