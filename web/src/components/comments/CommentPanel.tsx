import { useEffect, useState } from 'preact/hooks';
import { useComments, recordTypeMeta } from './useComments';
import type { CommentRecord, DisplayComment } from './useComments';
import { ProposalCard } from './ProposalCard';
import { RecordDiffCard } from './RecordDiffCard';
import { Markdown } from '../Markdown';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import type { Strings } from '../../i18n';
import { Icon } from '../shared/Icon';
import { useBodyScrollLock } from '../../scrollLock';
import { api, isStaticMode, ApiError } from '../../api';
import { Resizer } from '../layout/Resizer';
import { COMMENT_PANEL_WIDTH } from '../layout/resizableWidths';

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
    bindReviewDecision,
    removeReview,
    getDecision,
    cacheDecision,
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
  const { changedTransitionIds, changedVocabIds, changedTagIds, addedTransitionIds, addedVocabIds, addedTagIds, refresh: refreshPendingDiff } = usePendingDiff();

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

  // 採用/却下（change-cockpit-design-v3.md §8.5・P4／#35 tx.review.adopt/
  // -reject・tx.comment.adopt/-reject）: どのコメントを採用/却下中かの
  // ローカル UI 状態。why の下書きは POST 成功まで確定しない（P-1: 未コミッ
  // トの下書き合成）ので useComments 側の state ではなくここに置く。adopt
  // と reject は「decision 昇格＋昇格元コメント削除」という同じ束ね操作で、
  // decision の中身（採用の why／却下の理由）だけが違う — 1組の state で
  // どちらを処理中か（decidingKind）を持たせて分岐する。
  const [decidingId, setDecidingId] = useState<string | null>(null);
  const [decidingKind, setDecidingKind] = useState<'adopt' | 'reject' | null>(null);
  const [decideDraft, setDecideDraft] = useState('');
  const [deciding, setDeciding] = useState(false);
  const [decideError, setDecideError] = useState<string | null>(null);

  const startAdopt = (c: DisplayComment) => {
    setDecidingId(c.id);
    setDecidingKind('adopt');
    setDecideDraft(c.text);
    setDecideError(null);
  };
  const startReject = (c: DisplayComment) => {
    setDecidingId(c.id);
    setDecidingKind('reject');
    setDecideDraft(t.comments.rejectWhyDraft(c.text));
    setDecideError(null);
  };
  const cancelDecide = () => {
    setDecidingId(null);
    setDecidingKind(null);
    setDecideDraft('');
    setDecideError(null);
  };
  const confirmDecide = async (c: DisplayComment) => {
    const why = decideDraft.trim();
    if (!why) return;
    setDeciding(true);
    setDecideError(null);
    try {
      const decision = await api.postDecision({ on: `${c.recordType}:${c.recordId}`, why, commits: [] });
      // レビュー major fix-back（#27 P5b）: consider() の reload 専用
      // 再取得を待たず、POST 応答の Decision（編集後 why を含む）を即座に
      // キャッシュへ入れる — 「採用された why」表示が編集前の AI 原文/
      // 旧 text で固まらないようにする（transition/tag 共通）。
      cacheDecision(decision);
      // 昇格元コメント掃除（#35）: 削除は必ず昇格の後（then の順序）。
      if (c.source === 'ai') {
        // bindReviewDecision を先に呼んでおく — 直後の DELETE が失敗しても
        // decisionId は残るので、その review は「昇格済み」表示のまま留まり
        // 二重に decision を作られない（削除は再操作/`scholia review rm` 等で
        // 後追いできる）。
        bindReviewDecision(c.id, decision.id);
        await api.deleteReview(c.id);
        removeReview(c.id);
      } else {
        deleteComment(c.id);
      }
      setDecidingId(null);
      setDecidingKind(null);
      setDecideDraft('');
    } catch (e) {
      setDecideError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setDeciding(false);
    }
  };

  // Locks background scroll while the panel is open — unlike BrowseRail's
  // drawer, CommentPanel is a fixed slide-over at every viewport width (not
  // narrow-only), so the lock isn't gated on isNarrow (#20 drawerscroll fix).
  useBodyScrollLock(panelOpen);

  if (!panelOpen) return null;

  // #27 P2′-rework (change-cockpit-design-v3.md §8.2): a comment IS a
  // proposal when its record currently has a pending change — no separate
  // Proposal record, just this derived check plus an inline diff card.
  // Applies equally to AI comments (§8.4): an AI review on a changed record
  // is a proposal exactly like a human one, via the same derive.
  // §8.8 P5 vocab/tag: the same derive generalizes by recordType — only the
  // Set it consults changes (transition/vocab/tag each has its own pending-
  // diff Set from usePendingDiff()); 'page' comments never qualify (a page
  // isn't a .scholia record with a diff).
  // #32 A是正: 「pending change」は changed だけでなく added（main に無い
  // 新規レコード）も含む — ProposalCard/RecordDiffCard 側が added を
  // after-only で描画できるようになったのに合わせ、ここで added* Set も見る。
  const hasPendingChange = (recordType: DisplayComment['recordType'], recordId: string) => {
    if (recordType === 'transition') return changedTransitionIds.has(recordId) || addedTransitionIds.has(recordId);
    if (recordType === 'vocab') return changedVocabIds.has(recordId) || addedVocabIds.has(recordId);
    if (recordType === 'tag') return changedTagIds.has(recordId) || addedTagIds.has(recordId);
    return false;
  };
  const isProposalComment = (c: DisplayComment) => hasPendingChange(c.recordType, c.recordId);

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
        <Resizer config={COMMENT_PANEL_WIDTH} direction="panel" className="scholia-resizer--panel" />
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
              {composer.recordType === 'transition' && hasPendingChange('transition', composer.recordId) && <ProposalCard txId={composer.recordId} />}
              {composer.recordType === 'vocab' && hasPendingChange('vocab', composer.recordId) && (
                <RecordDiffCard recordType="vocab" recordId={composer.recordId} />
              )}
              {composer.recordType === 'tag' && hasPendingChange('tag', composer.recordId) && <RecordDiffCard recordType="tag" recordId={composer.recordId} />}
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
                      // AIDF minor-1: comments は合流後（人+AI）の配列 —
                      // 削除できるのは常にローカルコメントのみなので、配列
                      // 構築順 [...local,...ai] に暗黙依存せず source で
                      // 明示的に絞る。
                      const existing = comments.find((c) => c.source === 'local' && c.recordId === composer.recordId && c.anchor === composer.anchor);
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
            // §8.5/P4 の POST /api/decision は on=transition:<id>|tag:<id>
            // のみを受ける（internal/model.DecisionTarget・vocab は対象外
            // ＝backend は不可変更のスコープ）。vocab の提案は差分カード＋
            // バッジには乗るが、採用導線だけはここで外す（出しても 400 に
            // なるだけの死んだボタンにしない）。
            const isAdoptable = c.recordType === 'transition' || c.recordType === 'tag';
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
                {isProposal && c.recordType === 'transition' && <ProposalCard txId={c.recordId} />}
                {isProposal && (c.recordType === 'vocab' || c.recordType === 'tag') && (
                  <RecordDiffCard recordType={c.recordType} recordId={c.recordId} />
                )}
                <Markdown text={c.text} class="comment-item-text" />

                {adopted && (
                  <div class="comment-decision-card">
                    <div class="comment-decision-card-head">
                      <Icon name="gavel" size={13} />
                      <span class="comment-decision-card-title">{t.comments.adoptedWhyHeading}</span>
                    </div>
                    <Markdown text={decision?.why ?? c.text} class="comment-decision-why" />
                    <p class="comment-decision-note">{t.comments.adoptedNote}</p>
                  </div>
                )}

                {/* 採用/却下（§8.5・P4／#35）: 唯一の書込導線。static export
                    は書込不可なので Adopt/Reject 双方の導線ごと隠す
                    （handoff「静的モード gate...新設 Reject 導線にも継承」）。 */}
                {isProposal && isAdoptable && !isStaticMode && !adopted && decidingId !== c.id && (
                  <div class="comment-adopt-form-actions">
                    <button type="button" class="comment-adopt-btn" onClick={() => startAdopt(c)}>
                      <Icon name="gavel" size={13} /> {t.comments.adoptButton}
                    </button>
                    <button type="button" class="comment-reject-btn" onClick={() => startReject(c)}>
                      <Icon name="x" size={13} /> {t.comments.rejectButton}
                    </button>
                  </div>
                )}

                {isProposal && isAdoptable && decidingId === c.id && (
                  <div class={decidingKind === 'reject' ? 'comment-reject-form' : 'comment-adopt-form'}>
                    <div class="comment-adopt-form-label">
                      {decidingKind === 'reject' ? t.comments.rejectWhyLabel : t.comments.adoptWhyLabel}
                    </div>
                    <textarea
                      class="comment-adopt-form-input"
                      rows={3}
                      value={decideDraft}
                      disabled={deciding}
                      onInput={(e) => setDecideDraft((e.target as HTMLTextAreaElement).value)}
                    />
                    {decideError && <p class="comment-adopt-error">{decideError}</p>}
                    <div class="comment-adopt-form-actions">
                      <button
                        type="button"
                        class={decidingKind === 'reject' ? 'comment-btn-danger' : 'comment-btn-primary'}
                        onClick={() => confirmDecide(c)}
                        disabled={deciding || !decideDraft.trim()}
                      >
                        <Icon name="check" size={14} /> {decidingKind === 'reject' ? t.comments.rejectConfirm : t.comments.adoptConfirm}
                      </button>
                      <button type="button" class="comment-btn-secondary" onClick={cancelDecide} disabled={deciding}>
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
