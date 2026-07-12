import { useEffect, useState } from 'preact/hooks';
import { useComments, recordTypeMeta } from './useComments';
import type { CommentRecord, DisplayComment } from './useComments';
import { ProposalCard } from './ProposalCard';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import type { Strings } from '../../i18n';
import { Icon } from '../shared/Icon';
import { useBodyScrollLock } from '../../scrollLock';
import { api, isStaticMode, ApiError } from '../../api';

interface Props {
  onGoto: (c: CommentRecord) => void;
}

function formatTime(ms: number): string {
  const d = new Date(ms);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getMonth() + 1}/${d.getDate()} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

// Comment/reply submission is Cmd+Enter (Mac) / Ctrl+Enter (Windows/Linux)
// rather than plain Enter — a textarea needs Enter free for newlines, and
// unifying the reply input onto the same shortcut (it used to submit on
// plain Enter) makes the two composers behave the same way (2026-07-11
// 追加調整2件 §2). e.isComposing guards against an IME's Enter-to-confirm
// keystroke (e.g. finishing 変換) being misread as a submit.
function isSubmitKey(e: KeyboardEvent): boolean {
  return e.key === 'Enter' && (e.metaKey || e.ctrlKey) && !e.isComposing;
}

function submitHint(t: Strings): string {
  return typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform) ? t.comments.submitHintMac : t.comments.submitHintOther;
}

// Slide-over panel opened from the header's comment icon (or any per-section
// CommentButton): a composer for the currently-targeted section plus a
// summary list of every comment (each with its reply thread), each jumping
// back to its record via onGoto (App.tsx's existing openTagSpec/
// openTransition/openVocabEntry/setView routes).
export function CommentPanel({ onGoto }: Props) {
  const t = useT();
  const {
    comments,
    panelOpen,
    closePanel,
    composer,
    composerText,
    isEditingExisting,
    setComposerText,
    saveComposer,
    cancelComposer,
    deleteComment,
    editComment,
    replyDrafts,
    setReplyDraft,
    addReply,
    deleteReply,
    adoptLocalComment,
    adoptReview,
    getDecision,
    copyMsg,
    copyAll,
    tasks,
    activeTaskId,
    switchTask,
    creatingTask,
    taskDraftTitle,
    setTaskDraftTitle,
    startCreateTask,
    cancelCreateTask,
    saveNewTask,
  } = useComments();
  const { changedTransitionIds, refresh: refreshPendingDiff } = usePendingDiff();

  // 削除（提案）（change-cockpit-design-v3.md §8.8 P5・M-5「削除」・G-1′
  // 拡張）: composer が transition を指しているときだけ出るトグル。
  // DELETE は即座に作業ツリーから除去する（P3 の下書き→反映のような二段
  // 階ではない）ので、誤操作を防ぐため二段階確認にする（採用フローの
  // adoptingId パターンと同じ流儀）。
  const [deletingTx, setDeletingTx] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  // The confirm step is scoped to whichever record the composer currently
  // targets — switching composers (or closing it) must not leave a stale
  // "本当に削除しますか？" confirmation armed for the next record opened.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    setDeletingTx(false);
    setDeleteError(null);
  }, [composer?.recordId, composer?.anchor]);

  const confirmDeleteTransition = async (recordId: string) => {
    setDeleting(true);
    setDeleteError(null);
    try {
      await api.deleteTransition(recordId);
      refreshPendingDiff();
      setDeletingTx(false);
      cancelComposer();
    } catch (e) {
      setDeleteError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setDeleting(false);
    }
  };

  // 採用（change-cockpit-design-v3.md §8.5・P4）: どのコメントを採用中かの
  // ローカル UI 状態。why の下書きは POST 成功まで確定しない（P-1: 未コミッ
  // トの下書き合成）ので useComments 側の state ではなくここに置く。
  const [adoptingId, setAdoptingId] = useState<string | null>(null);
  const [adoptDraft, setAdoptDraft] = useState('');
  const [adopting, setAdopting] = useState(false);
  const [adoptError, setAdoptError] = useState<string | null>(null);

  const startAdopt = (c: DisplayComment) => {
    setAdoptingId(c.id);
    setAdoptDraft(c.text);
    setAdoptError(null);
  };
  const cancelAdopt = () => {
    setAdoptingId(null);
    setAdoptDraft('');
    setAdoptError(null);
  };
  const confirmAdopt = async (c: DisplayComment) => {
    const why = adoptDraft.trim();
    if (!why) return;
    setAdopting(true);
    setAdoptError(null);
    try {
      const decision = await api.postDecision({ on: `${c.recordType}:${c.recordId}`, why, commits: [] });
      if (c.source === 'ai') adoptReview(c.id, decision.id);
      else adoptLocalComment(c.id, decision.id, why);
      setAdoptingId(null);
      setAdoptDraft('');
    } catch (e) {
      setAdoptError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setAdopting(false);
    }
  };

  // Locks background scroll while the panel is open — unlike BrowseRail's
  // drawer, CommentPanel is a fixed slide-over at every viewport width (not
  // narrow-only), so the lock isn't gated on isNarrow (#20 drawerscroll fix).
  useBodyScrollLock(panelOpen);

  if (!panelOpen) return null;

  // #27 P2′-rework (change-cockpit-design-v3.md §8.2): a comment IS a
  // proposal when its record currently has a pending change — no separate
  // Proposal record, just this derived check plus an inline ProposalCard.
  // Applies equally to AI comments (§8.4): an AI review on a changed record
  // is a proposal exactly like a human one, via the same derive.
  const isProposalComment = (c: DisplayComment) => c.recordType === 'transition' && changedTransitionIds.has(c.recordId);

  // Proposal comments (change + comment) group to the front (§8.2/§8.8),
  // most-recently-updated first within each group (stable sort keeps the
  // relative order from the first sort pass).
  const sorted = comments
    .slice()
    .sort((a, b) => b.updatedAt - a.updatedAt)
    .sort((a, b) => (isProposalComment(b) ? 1 : 0) - (isProposalComment(a) ? 1 : 0));

  return (
    <>
      <div class="comment-backdrop" onClick={closePanel} />
      <aside class="comment-panel">
        <div class="comment-panel-head">
          <span class="comment-panel-title">
            <Icon name="message-filled" size={14} /> {t.comments.panelTitle} <span class="comment-panel-count">{comments.length}</span>
          </span>
          <span class="comment-panel-spacer" />
          {copyMsg && (
            <span class="comment-copy-msg">
              <Icon name="check" size={12} /> {t.comments.copied}
            </span>
          )}
          <button type="button" class="comment-copy-btn" title={t.comments.copyAllTitle} onClick={copyAll} disabled={comments.length === 0}>
            <Icon name="clipboard-copy" size={14} /> {t.comments.copyAll}
          </button>
          <button type="button" class="comment-close-btn" aria-label={t.common.close} onClick={closePanel}>
            <Icon name="x" size={17} />
          </button>
        </div>

        <div class="comment-task-bar">
          <label class="comment-task-label" for="comment-task-select">
            {t.comments.taskLabel}
          </label>
          <select
            id="comment-task-select"
            class="comment-task-select"
            value={activeTaskId}
            onChange={(e) => switchTask((e.target as HTMLSelectElement).value)}
          >
            {tasks.map((tk) => (
              <option key={tk.id} value={tk.id}>
                {tk.title}
              </option>
            ))}
          </select>
          {!creatingTask && (
            <button type="button" class="comment-task-new-btn" title={t.comments.taskNewTitle} onClick={startCreateTask}>
              <Icon name="plus" size={12} /> {t.comments.taskNew}
            </button>
          )}
        </div>
        {creatingTask && (
          <div class="comment-task-new-form">
            <input
              class="comment-task-new-input"
              placeholder={t.comments.taskNewPlaceholder}
              value={taskDraftTitle}
              autoFocus
              onInput={(e) => setTaskDraftTitle((e.target as HTMLInputElement).value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.isComposing) {
                  e.preventDefault();
                  saveNewTask();
                } else if (e.key === 'Escape') {
                  e.preventDefault();
                  cancelCreateTask();
                }
              }}
            />
            <button type="button" class="comment-btn-primary" onClick={saveNewTask} disabled={!taskDraftTitle.trim()}>
              <Icon name="check" size={13} /> {t.common.save}
            </button>
            <button type="button" class="comment-btn-secondary" onClick={cancelCreateTask}>
              {t.common.cancel}
            </button>
          </div>
        )}

        <div class="comment-panel-body">
          {composer && (
            <div class="comment-composer">
              <div class="comment-composer-target">
                <span class="comment-composer-type">{recordTypeMeta(t, composer.recordType).label}</span>
                <span class="comment-composer-title">{composer.recordTitle}</span>
                <span class="dim">›</span>
                <span class="dim">{composer.anchorLabel}</span>
              </div>
              {composer.recordType === 'transition' && changedTransitionIds.has(composer.recordId) && <ProposalCard txId={composer.recordId} />}
              {/* 削除（提案）（§8.8 P5・M-5「削除」・G-1′ 拡張）: どの
                  transition ドロワーからでも出せる（変更の有無を問わない）
                  — ProposalCard 上の「反映」と違い下書きを持たず、確定を
                  挟んで即座に作業ツリーから除去する。static export は
                  書込不可なので非表示。 */}
              {composer.recordType === 'transition' && !isStaticMode && !deletingTx && (
                <button type="button" class="delete-proposal-btn" onClick={() => setDeletingTx(true)}>
                  <Icon name="trash-2" size={13} /> {t.comments.deleteProposalButton}
                </button>
              )}
              {composer.recordType === 'transition' && !isStaticMode && deletingTx && (
                <div class="delete-proposal-confirm">
                  <p class="delete-proposal-confirm-label">{t.comments.deleteProposalConfirmLabel}</p>
                  {deleteError && <p class="proposal-card-error">{t.comments.deleteProposalError(deleteError)}</p>}
                  <div class="comment-adopt-form-actions">
                    <button
                      type="button"
                      class="comment-btn-danger"
                      disabled={deleting}
                      onClick={() => confirmDeleteTransition(composer.recordId)}
                    >
                      <Icon name="trash-2" size={13} /> {deleting ? t.comments.deleteProposalDeleting : t.comments.deleteProposalConfirmButton}
                    </button>
                    <button type="button" class="comment-btn-secondary" disabled={deleting} onClick={() => setDeletingTx(false)}>
                      {t.comments.deleteProposalCancel}
                    </button>
                  </div>
                </div>
              )}
              <textarea
                class="comment-composer-input"
                rows={3}
                placeholder={t.comments.composerPlaceholder}
                value={composerText}
                onInput={(e) => setComposerText((e.target as HTMLTextAreaElement).value)}
                onKeyDown={(e) => {
                  if (isSubmitKey(e)) {
                    e.preventDefault();
                    saveComposer();
                  }
                }}
              />
              <div class="comment-composer-actions">
                <button type="button" class="comment-btn-primary" onClick={saveComposer}>
                  <Icon name="check" size={14} /> {t.common.save}
                </button>
                <button type="button" class="comment-btn-secondary" onClick={cancelComposer}>
                  {t.common.cancel}
                </button>
                <span class="comment-kbd-hint dim">{submitHint(t)}</span>
                <span class="comment-panel-spacer" />
                {isEditingExisting && (
                  <button
                    type="button"
                    class="comment-btn-danger"
                    onClick={() => {
                      const existing = comments.find((c) => c.recordId === composer.recordId && c.anchor === composer.anchor);
                      if (existing) deleteComment(existing.id);
                      cancelComposer();
                    }}
                  >
                    <Icon name="trash-2" size={14} /> {t.common.delete}
                  </button>
                )}
              </div>
            </div>
          )}

          {comments.length === 0 && !composer && (
            <div class="comment-empty">
              {t.comments.emptyLine1}
              <br />
              {t.comments.emptyLine2Before} <Icon name="message-plus" size={13} /> {t.comments.emptyLine2After}
            </div>
          )}

          {sorted.map((c) => {
            const meta = recordTypeMeta(t, c.recordType);
            const isAi = c.source === 'ai';
            const isProposal = isProposalComment(c);
            const adopted = !!c.decisionId;
            const decision = c.decisionId ? getDecision(c.decisionId) : undefined;
            return (
              <div key={c.id} class={'comment-item' + (isAi ? ' comment-item-ai' : '')}>
                <div class="comment-item-head">
                  <span class="comment-item-type" style={{ color: meta.color }}>
                    <Icon name={meta.icon} size={12} /> {meta.label}
                  </span>
                  <span class="comment-item-title">{c.recordTitle}</span>
                  <span class="comment-item-location dim">
                    <Icon name="crosshair" size={10} /> {c.anchorLabel}
                  </span>
                  {/* AI配送（change-cockpit-design-v3.md §8.4）: AI コメントは
                      GET /api/reviews から合流した read-only 項目 — badge の
                      み表示し、下の編集/削除/返信 UI は出さない。 */}
                  {isAi && (
                    <span class="comment-ai-badge" title={t.comments.aiReadonlyNote}>
                      {t.comments.aiBadge}
                    </span>
                  )}
                  {adopted && (
                    <span class="comment-adopted-badge" title={t.comments.adoptedNote}>
                      <Icon name="check" size={11} /> {t.comments.adoptedBadge}
                    </span>
                  )}
                </div>
                {isProposal && <ProposalCard txId={c.recordId} />}
                <p class="comment-item-text">{c.text}</p>

                {adopted && (
                  <div class="comment-decision-card">
                    <div class="comment-decision-card-head">
                      <Icon name="gavel" size={13} />
                      <span class="comment-decision-card-title">{t.comments.adoptedWhyHeading}</span>
                    </div>
                    <p class="comment-decision-why">{decision?.why ?? c.text}</p>
                    <p class="comment-decision-note">{t.comments.adoptedNote}</p>
                  </div>
                )}

                {/* 採用（§8.5・P4）: 唯一の書込導線。static export は書込不可
                    なので導線ごと隠す（handoff「static では非活性/非表示」）。 */}
                {isProposal && !isStaticMode && !adopted && adoptingId !== c.id && (
                  <button type="button" class="comment-adopt-btn" onClick={() => startAdopt(c)}>
                    <Icon name="gavel" size={13} /> {t.comments.adoptButton}
                  </button>
                )}

                {isProposal && adoptingId === c.id && (
                  <div class="comment-adopt-form">
                    <div class="comment-adopt-form-label">{t.comments.adoptWhyLabel}</div>
                    <textarea
                      class="comment-adopt-form-input"
                      rows={3}
                      value={adoptDraft}
                      disabled={adopting}
                      onInput={(e) => setAdoptDraft((e.target as HTMLTextAreaElement).value)}
                    />
                    {adoptError && <p class="comment-adopt-error">{adoptError}</p>}
                    <div class="comment-adopt-form-actions">
                      <button
                        type="button"
                        class="comment-btn-primary"
                        onClick={() => confirmAdopt(c)}
                        disabled={adopting || !adoptDraft.trim()}
                      >
                        <Icon name="check" size={14} /> {t.comments.adoptConfirm}
                      </button>
                      <button type="button" class="comment-btn-secondary" onClick={cancelAdopt} disabled={adopting}>
                        {t.common.cancel}
                      </button>
                    </div>
                  </div>
                )}

                {!isAi && c.replies.length > 0 && (
                  <div class="comment-reply-list">
                    {c.replies.map((r) => (
                      <div key={r.id} class="comment-reply">
                        <Icon name="corner-down-right" size={12} class="dim comment-reply-icon" />
                        <div class="comment-reply-body">
                          <span class="comment-reply-text">{r.text}</span>
                          <span class="comment-reply-time dim">{formatTime(r.createdAt)}</span>
                        </div>
                        <button type="button" class="comment-reply-delete" aria-label={t.comments.replyDelete} onClick={() => deleteReply(c.id, r.id)}>
                          <Icon name="x" size={12} />
                        </button>
                      </div>
                    ))}
                  </div>
                )}

                {!isAi && (
                  <div class="comment-reply-composer">
                    <input
                      class="comment-reply-input"
                      placeholder={t.comments.replyPlaceholder}
                      title={submitHint(t)}
                      value={replyDrafts[c.id] || ''}
                      onInput={(e) => setReplyDraft(c.id, (e.target as HTMLInputElement).value)}
                      onKeyDown={(e) => {
                        if (isSubmitKey(e)) {
                          e.preventDefault();
                          addReply(c.id);
                        }
                      }}
                    />
                    <button type="button" class="comment-reply-add" onClick={() => addReply(c.id)}>
                      {t.comments.replyAdd}
                    </button>
                  </div>
                )}

                <div class="comment-item-actions">
                  <button type="button" class="comment-btn-chip" onClick={() => onGoto(c)}>
                    <Icon name="crosshair" size={13} /> {t.comments.gotoLocation}
                  </button>
                  {/* 採用済み（decisionId 付き）の人コメントは以後 read-only
                      （§8.5「採用後...本文 read-only」）— 編集/削除を隠す。 */}
                  {!isAi && !adopted && (
                    <button type="button" class="comment-btn-chip" onClick={() => editComment(c)}>
                      <Icon name="pencil" size={12} /> {t.common.edit}
                    </button>
                  )}
                  <span class="comment-panel-spacer" />
                  <span class="comment-item-time dim">{formatTime(c.updatedAt)}</span>
                  {!isAi && !adopted && (
                    <button type="button" class="comment-btn-icon-danger" aria-label={t.common.delete} onClick={() => deleteComment(c.id)}>
                      <Icon name="trash-2" size={13} />
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </aside>
    </>
  );
}
