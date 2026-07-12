import { useEffect, useRef, useState } from 'preact/hooks';
import { api, ApiError, isStaticMode } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { Icon } from '../shared/Icon';
import { Chip, kindColor } from '../shared/Chip';
import type { DecisionDiff, DiffResult, Tag, TagDiff, Transition, TransitionChange, TransitionDiff, VocabDiff } from '../../types';

interface Props {
  onGoToSpec: (txId: string) => void;
  onGoToTagSpec: (tagId: string) => void;
}

// The comparison itself is entirely server-side (GET /api/diff, §2 R-2's
// DiffRefs) — this component only ever picks base/head and renders the
// returned diff.Result. It never re-derives added/removed/changed itself
// (§9 single source of truth).
export function CompareView({ onGoToSpec, onGoToTagSpec }: Props) {
  const t = useT();
  const { tagById, vocabLabel } = useLookups();
  const [base, setBase] = useState('main');
  const [head, setHead] = useState('');
  const [result, setResult] = useState<DiffResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  // The mount effect's initial fetch and a user-triggered 比較 click can both
  // be in flight at once; without a sequence guard whichever *resolves*
  // last wins the state update even if it was fired first, so a fast reply
  // to a stale request can clobber a fresher one already on screen.
  const requestSeq = useRef(0);

  const fetchDiff = (b: string, h: string) => {
    const seq = ++requestSeq.current;
    setError(null);
    setLoading(true);
    api
      .getDiff({ ref: b.trim() || undefined, head: h.trim() || undefined })
      .then((res) => {
        if (seq !== requestSeq.current) return;
        setResult(res);
      })
      .catch((err) => {
        if (seq !== requestSeq.current) return;
        setResult(null);
        setError(err instanceof ApiError ? err.message : String(err));
      })
      .finally(() => {
        if (seq !== requestSeq.current) return;
        setLoading(false);
      });
  };

  // Load once on mount with the defaults (pending: 作業ツリー vs main) — a
  // useful first view without requiring the user to press 比較 first.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (isStaticMode) return;
    fetchDiff(base, head);
  }, []);

  if (isStaticMode) {
    return (
      <main class="compare-view">
        <div class="compare-static-banner">
          <Icon name="info" size={18} />
          <div>
            <strong>{t.compare.staticUnavailableTitle}</strong>
            <p>{t.compare.staticUnavailableBody}</p>
          </div>
        </div>
      </main>
    );
  }

  const onSubmit = (e: Event) => {
    e.preventDefault();
    fetchDiff(base, head);
  };

  return (
    <main class="compare-view">
      <div class="compare-head">
        <h1>
          <Icon name="git-compare" size={20} /> {t.compare.heading}
        </h1>
        <p class="dim">{t.compare.intro}</p>
      </div>

      <form class="compare-form" onSubmit={onSubmit}>
        <label>
          {t.compare.baseLabel}
          <input value={base} onInput={(e) => setBase((e.target as HTMLInputElement).value)} placeholder="main" />
        </label>
        <label>
          {t.compare.headLabel}
          <input value={head} onInput={(e) => setHead((e.target as HTMLInputElement).value)} placeholder={t.compare.headPlaceholder} />
        </label>
        <button type="submit" disabled={loading}>
          {loading ? t.compare.running : t.compare.run}
        </button>
      </form>

      {error && (
        <p class="compare-error">
          <Icon name="triangle-alert" size={14} /> {error}
        </p>
      )}

      {result && (
        <div class="compare-report">
          <p class="dim compare-report-note">
            {result.afterRef ? t.compare.landedNote(result.ref, result.afterRef) : t.compare.pendingNote(result.ref)}
          </p>
          {result.baselineMissing && (
            <p class="compare-note">
              <Icon name="info" size={14} /> {t.compare.baselineMissingNote}
            </p>
          )}
          {isDiffResultEmpty(result) ? (
            <p class="card-empty">{t.compare.empty}</p>
          ) : (
            <>
              <DecisionSection diff={result.decisions} />
              <TransitionSection diff={result.transitions} tagById={tagById} vocabLabel={vocabLabel} onGoToSpec={onGoToSpec} onGoToTagSpec={onGoToTagSpec} />
              <VocabSection diff={result.vocab} />
              <TagSection diff={result.tags} onGoToTagSpec={onGoToTagSpec} />
            </>
          )}
        </div>
      )}
    </main>
  );
}

function isDiffResultEmpty(r: DiffResult): boolean {
  return isFlatEmpty(r.vocab) && isFlatEmpty(r.tags) && isFlatEmpty(r.transitions) && isFlatEmpty(r.decisions);
}

function isFlatEmpty(d: { added?: unknown[]; removed?: unknown[]; changed?: unknown[] }): boolean {
  return !d.added?.length && !d.removed?.length && !d.changed?.length;
}

function VocabSection({ diff }: { diff: VocabDiff }) {
  const t = useT();
  if (isFlatEmpty(diff)) return null;
  return (
    <section class="card compare-section">
      <span class="card-section-heading">{t.compare.vocabHeading}</span>
      <ul class="compare-flat-list">
        {diff.added?.map((v) => (
          <li key={'a' + v.id} class="compare-added">
            + {v.id}（{v.label}）
          </li>
        ))}
        {diff.removed?.map((v) => (
          <li key={'r' + v.id} class="compare-removed">
            − {v.id}（{v.label}）
          </li>
        ))}
        {diff.changed?.map((c) => (
          <li key={'c' + c.id} class="compare-changed">
            〜 {c.id}
          </li>
        ))}
      </ul>
    </section>
  );
}

function TagSection({ diff, onGoToTagSpec }: { diff: TagDiff; onGoToTagSpec: (tagId: string) => void }) {
  const t = useT();
  if (isFlatEmpty(diff)) return null;
  return (
    <section class="card compare-section">
      <span class="card-section-heading">{t.compare.tagsHeading}</span>
      <ul class="compare-flat-list">
        {diff.added?.map((tag) => (
          <li key={'a' + tag.id} class="compare-added">
            <button type="button" class="compare-link" onClick={() => onGoToTagSpec(tag.id)}>
              + {tag.id}（{tag.name}）
            </button>
          </li>
        ))}
        {diff.removed?.map((tag) => (
          <li key={'r' + tag.id} class="compare-removed">
            − {tag.id}（{tag.name}）
          </li>
        ))}
        {diff.changed?.map((c) => (
          <li key={'c' + c.id} class="compare-changed">
            <button type="button" class="compare-link" onClick={() => onGoToTagSpec(c.id)}>
              〜 {c.id}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

function DecisionSection({ diff }: { diff: DecisionDiff }) {
  const t = useT();
  if (isFlatEmpty(diff)) return null;
  const violation = !!diff.removed?.length || !!diff.changed?.length;
  return (
    <section class={'card compare-section' + (violation ? ' compare-violation-section' : '')}>
      <span class="card-section-heading">{t.compare.decisionsHeading}</span>
      {violation && (
        <div class="compare-violation-banner">
          <Icon name="triangle-alert" size={16} />
          <div>
            <strong>{t.compare.decisionViolationHeading}</strong>
            <p>{t.compare.decisionViolationBody}</p>
          </div>
        </div>
      )}
      <ul class="compare-flat-list">
        {diff.added?.map((d) => (
          <li key={'a' + d.id} class="compare-added">
            + {d.id}（{d.target.type}: {d.target.id}）— {d.why}
          </li>
        ))}
        {diff.removed?.map((d) => (
          <li key={'r' + d.id} class="compare-removed compare-violation">
            ! {t.compare.decisionRemovedLabel}: {d.id}（{d.target.type}: {d.target.id}）
          </li>
        ))}
        {diff.changed?.map((c) => (
          <li key={'c' + c.id} class="compare-changed compare-violation">
            ! {t.compare.decisionChangedLabel}: {c.id}
          </li>
        ))}
      </ul>
    </section>
  );
}

interface TransitionGroup {
  tagId: string;
  added: Transition[];
  removed: Transition[];
  changed: TransitionChange[];
}

// 主題（タグ）でグルーピングする（§2「何を・どう見せるか」）。diff の
// Transition は直接付与された tags のみを持つ（effectiveTags は /api/diff
// に無い＝§9 に沿い再導出しない）ので、そのままグループ鍵に使う。1つの
// transition が複数タグを持てば複数グループに現れる（多重所属）。
function groupTransitionsByTag(diff: TransitionDiff): TransitionGroup[] {
  const groups = new Map<string, TransitionGroup>();
  const group = (tagId: string) => {
    let g = groups.get(tagId);
    if (!g) {
      g = { tagId, added: [], removed: [], changed: [] };
      groups.set(tagId, g);
    }
    return g;
  };
  const tagsOf = (tags: string[] | undefined): string[] => (tags && tags.length > 0 ? tags : ['']);

  for (const tx of diff.added || []) for (const tagId of tagsOf(tx.tags)) group(tagId).added.push(tx);
  for (const tx of diff.removed || []) for (const tagId of tagsOf(tx.tags)) group(tagId).removed.push(tx);
  for (const c of diff.changed || []) {
    const tagIds = Array.from(new Set([...(c.before.tags || []), ...(c.after.tags || [])]));
    for (const tagId of tagsOf(tagIds)) group(tagId).changed.push(c);
  }

  return Array.from(groups.values()).sort((a, b) => {
    if (a.tagId === '') return 1;
    if (b.tagId === '') return -1;
    return a.tagId.localeCompare(b.tagId);
  });
}

function TransitionSection({
  diff,
  tagById,
  vocabLabel,
  onGoToSpec,
  onGoToTagSpec,
}: {
  diff: TransitionDiff;
  tagById: Map<string, Tag>;
  vocabLabel: (id: string) => string;
  onGoToSpec: (txId: string) => void;
  onGoToTagSpec: (tagId: string) => void;
}) {
  const t = useT();
  if (isFlatEmpty(diff)) return null;
  const groups = groupTransitionsByTag(diff);

  return (
    <section class="card compare-section">
      <span class="card-section-heading">{t.compare.transitionsHeading}</span>
      <div class="compare-tx-groups">
        {groups.map((g) => (
          <div key={g.tagId || '(untagged)'} class="compare-tx-group">
            <div class="compare-tx-group-heading">
              {g.tagId ? (
                <Chip color={kindColor(tagById.get(g.tagId)?.kind)} onClick={() => onGoToTagSpec(g.tagId)}>
                  {tagById.get(g.tagId)?.name || g.tagId}
                </Chip>
              ) : (
                <span class="dim">{t.compare.untaggedGroup}</span>
              )}
            </div>
            <ul class="compare-tx-list">
              {g.added.map((tx) => (
                <li key={'a' + tx.id} class="compare-tx-row compare-added">
                  <button type="button" class="compare-link" onClick={() => onGoToSpec(tx.id)}>
                    + {vocabLabel(tx.action)}
                  </button>
                  <span class="dim compare-tx-then">
                    {tx.then.length > 0 ? `→ ${tx.then.map(vocabLabel).join('、')}` : t.flow.noResult}
                  </span>
                </li>
              ))}
              {g.removed.map((tx) => (
                <li key={'r' + tx.id} class="compare-tx-row compare-removed">
                  <span>− {vocabLabel(tx.action)}</span>
                  <span class="dim compare-tx-then">
                    {tx.then.length > 0 ? `→ ${tx.then.map(vocabLabel).join('、')}` : t.flow.noResult}
                  </span>
                </li>
              ))}
              {g.changed.map((c) => (
                <li key={'c' + c.id} class="compare-tx-row compare-tx-row-changed">
                  <div class="compare-tx-changed-head">
                    <button type="button" class="compare-link compare-changed" onClick={() => onGoToSpec(c.id)}>
                      〜 {vocabLabel(c.after.action)}
                    </button>
                    {c.thenReordered && <span class="compare-badge compare-badge-reorder">{t.compare.thenReordered}</span>}
                  </div>
                  <ul class="compare-tx-changed-detail dim">
                    {c.actionChanged && <li>{t.compare.actionChangedLabel(vocabLabel(c.before.action), vocabLabel(c.after.action))}</li>}
                    {!!(c.givenAdded?.length || c.givenRemoved?.length) && (
                      <li>
                        {t.compare.givenChangedLabel}: {c.givenAdded?.map((g) => `+${vocabLabel(g)}`).join(' ')} {c.givenRemoved?.map((g) => `-${vocabLabel(g)}`).join(' ')}
                      </li>
                    )}
                    {c.thenReordered ? (
                      <li>
                        {t.compare.thenChangedLabel}（{t.compare.thenReordered}）: [{c.before.then.map(vocabLabel).join('、')}] → [{c.after.then.map(vocabLabel).join('、')}]
                      </li>
                    ) : (
                      c.thenChanged && (
                        <li>
                          {t.compare.thenChangedLabel}: [{c.before.then.map(vocabLabel).join('、')}] → [{c.after.then.map(vocabLabel).join('、')}]
                        </li>
                      )
                    )}
                    {!!(c.tagsAdded?.length || c.tagsRemoved?.length) && (
                      <li>
                        {t.compare.tagsChangedLabel}: {c.tagsAdded?.map((g) => `+${tagById.get(g)?.name || g}`).join(' ')} {c.tagsRemoved?.map((g) => `-${tagById.get(g)?.name || g}`).join(' ')}
                      </li>
                    )}
                    {!!(c.testsAdded?.length || c.testsRemoved?.length) && (
                      <li>
                        {t.compare.testsChangedLabel}: {c.testsAdded?.map((g) => `+${g}`).join(' ')} {c.testsRemoved?.map((g) => `-${g}`).join(' ')}
                      </li>
                    )}
                  </ul>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </section>
  );
}
