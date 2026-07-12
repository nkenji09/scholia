import { useState } from 'preact/hooks';
import { useLookups } from '../../lookups';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import { api, ApiError } from '../../api';
import type { Transition } from '../../types';
import { Icon } from '../shared/Icon';

interface Props {
  transition: Transition;
  cardRef: (el: HTMLElement | null) => void;
}

// TombstoneCard — §3/§8.8「3種別の表し方」の削除表現（M-5）: a transition
// present at base but removed from the working tree. There's no
// TransitionDetail for it (GET /api/transitions/{id} 404s once the file is
// gone), so this renders straight from the diff's Before snapshot rather
// than reusing SpecCard, which expects backend-derived provenance (labels,
// effective tags) that no longer exists for a deleted record. Stays in the
// list "採用まで残す" (kept until adopted) per §3's table — nothing here
// ever removes it from view; only a git commit (human) or explicit
// re-creation (button below) changes what /api/diff reports next load.
export function TombstoneCard({ transition, cardRef }: Props) {
  const t = useT();
  const { vocabLabel, tagName } = useLookups();
  const { refresh } = usePendingDiff();
  const [restoring, setRestoring] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onRestore = async () => {
    setRestoring(true);
    setError(null);
    try {
      await api.createTransition({
        id: transition.id,
        action: transition.action,
        given: transition.given,
        then: transition.then,
        tags: transition.tags || [],
      });
      refresh();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setRestoring(false);
    }
  };

  return (
    <article ref={cardRef} data-card-id={transition.id} class="card card-removed-tombstone">
      <span class="tombstone-badge">
        <Icon name="trash-2" size={12} /> {t.comments.tombstoneBadge}
      </span>
      <div class="tombstone-fields tombstone-strike">
        <span>
          {t.flow.trigger}: {vocabLabel(transition.action)}
        </span>
        {transition.given.length > 0 && (
          <span>
            {t.flow.given}: {transition.given.map(vocabLabel).join('、')}
          </span>
        )}
        <span>
          {t.flow.result}: {transition.then.map(vocabLabel).join('、')}
        </span>
        {(transition.tags || []).length > 0 && (
          <span>
            {t.browse.tagsHeading}: {(transition.tags || []).map(tagName).join('、')}
          </span>
        )}
      </div>
      <div class="tombstone-actions">
        {error && <p class="proposal-card-error">{t.comments.tombstoneRestoreError(error)}</p>}
        <button type="button" class="proposal-reflect-btn" disabled={restoring} onClick={onRestore}>
          <Icon name="git-compare" size={13} /> {restoring ? t.comments.tombstoneRestoring : t.comments.tombstoneRestoreButton}
        </button>
      </div>
    </article>
  );
}
