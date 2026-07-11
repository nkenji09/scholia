import { useComments, RECORD_TYPE_META } from './useComments';
import type { CommentRecord } from './useComments';
import { Icon } from '../shared/Icon';
import { useBodyScrollLock } from '../../scrollLock';

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

const SUBMIT_HINT = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘+Enter で投稿' : 'Ctrl+Enter で投稿';

// Slide-over panel opened from the header's comment icon (or any per-section
// CommentButton): a composer for the currently-targeted section plus a
// summary list of every comment (each with its reply thread), each jumping
// back to its record via onGoto (App.tsx's existing openTagSpec/
// openTransition/openVocabEntry/setView routes).
export function CommentPanel({ onGoto }: Props) {
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
    copyMsg,
    copyAll,
  } = useComments();

  // Locks background scroll while the panel is open — unlike BrowseRail's
  // drawer, CommentPanel is a fixed slide-over at every viewport width (not
  // narrow-only), so the lock isn't gated on isNarrow (#20 drawerscroll fix).
  useBodyScrollLock(panelOpen);

  if (!panelOpen) return null;

  const sorted = comments.slice().sort((a, b) => b.updatedAt - a.updatedAt);

  return (
    <>
      <div class="comment-backdrop" onClick={closePanel} />
      <aside class="comment-panel">
        <div class="comment-panel-head">
          <span class="comment-panel-title">
            <Icon name="message-filled" size={14} /> コメント <span class="comment-panel-count">{comments.length}</span>
          </span>
          <span class="comment-panel-spacer" />
          {copyMsg && (
            <span class="comment-copy-msg">
              <Icon name="check" size={12} /> コピーしました
            </span>
          )}
          <button type="button" class="comment-copy-btn" title="AI が修正するための情報をまとめてコピー" onClick={copyAll} disabled={comments.length === 0}>
            <Icon name="clipboard-copy" size={14} /> コピー
          </button>
          <button type="button" class="comment-close-btn" aria-label="閉じる" onClick={closePanel}>
            <Icon name="x" size={17} />
          </button>
        </div>

        <div class="comment-panel-body">
          {composer && (
            <div class="comment-composer">
              <div class="comment-composer-target">
                <span class="comment-composer-type">{RECORD_TYPE_META[composer.recordType].label}</span>
                <span class="comment-composer-title">{composer.recordTitle}</span>
                <span class="dim">›</span>
                <span class="dim">{composer.anchorLabel}</span>
              </div>
              <textarea
                class="comment-composer-input"
                rows={3}
                placeholder="コメントを入力…（このカードのこの箇所について）"
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
                  <Icon name="check" size={14} /> 保存
                </button>
                <button type="button" class="comment-btn-secondary" onClick={cancelComposer}>
                  キャンセル
                </button>
                <span class="comment-kbd-hint dim">{SUBMIT_HINT}</span>
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
                    <Icon name="trash-2" size={14} /> 削除
                  </button>
                )}
              </div>
            </div>
          )}

          {comments.length === 0 && !composer && (
            <div class="comment-empty">
              まだコメントはありません。
              <br />
              各カードの見出し横の <Icon name="message-plus" size={13} /> から追加できます。
            </div>
          )}

          {sorted.map((c) => {
            const meta = RECORD_TYPE_META[c.recordType];
            return (
              <div key={c.id} class="comment-item">
                <div class="comment-item-head">
                  <span class="comment-item-type" style={{ color: meta.color }}>
                    <Icon name={meta.icon} size={12} /> {meta.label}
                  </span>
                  <span class="comment-item-title">{c.recordTitle}</span>
                  <span class="comment-item-location dim">
                    <Icon name="crosshair" size={10} /> {c.anchorLabel}
                  </span>
                </div>
                <p class="comment-item-text">{c.text}</p>

                {c.replies.length > 0 && (
                  <div class="comment-reply-list">
                    {c.replies.map((r) => (
                      <div key={r.id} class="comment-reply">
                        <Icon name="corner-down-right" size={12} class="dim comment-reply-icon" />
                        <div class="comment-reply-body">
                          <span class="comment-reply-text">{r.text}</span>
                          <span class="comment-reply-time dim">{formatTime(r.createdAt)}</span>
                        </div>
                        <button type="button" class="comment-reply-delete" aria-label="返信を削除" onClick={() => deleteReply(c.id, r.id)}>
                          <Icon name="x" size={12} />
                        </button>
                      </div>
                    ))}
                  </div>
                )}

                <div class="comment-reply-composer">
                  <input
                    class="comment-reply-input"
                    placeholder="返信を追加…"
                    title={SUBMIT_HINT}
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
                    返信
                  </button>
                </div>

                <div class="comment-item-actions">
                  <button type="button" class="comment-btn-chip" onClick={() => onGoto(c)}>
                    <Icon name="crosshair" size={13} /> 位置へ移動
                  </button>
                  <button type="button" class="comment-btn-chip" onClick={() => editComment(c)}>
                    <Icon name="pencil" size={12} /> 編集
                  </button>
                  <span class="comment-panel-spacer" />
                  <span class="comment-item-time dim">{formatTime(c.updatedAt)}</span>
                  <button type="button" class="comment-btn-icon-danger" aria-label="削除" onClick={() => deleteComment(c.id)}>
                    <Icon name="trash-2" size={13} />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      </aside>
    </>
  );
}
