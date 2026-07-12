import { createContext } from 'preact';
import type { ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';
import { useT } from '../../i18n';
import type { Strings } from '../../i18n';
import type { IconName } from '../shared/Icon';

// Comments (#18) — volatile, per-browser annotations. Deliberately NOT part
// of the .pmem record model: everything here lives only in localStorage
// (never git, never the Go backend), exactly per
// claude-design-request-comments.md's "フロントエンド完結・localStorage の
// み" constraint — extending this file must never add a fetch/API call. This
// file is the one place that reads/writes that storage — components never
// touch localStorage directly.
//
// 2026-07-11 コメント拡張4件 (user-requested, beyond the original written
// requirement): reply threads (design's replies/replyDrafts model, restored
// after being out of scope in an earlier pass), a 'vocab' record type
// (VocabCard, not in the design mock but explicitly requested), and a
// 'page' record type for whole-page comments not tied to any one card
// (target = {recordType:'page', recordId:<view>, anchor:'page'} — reuses
// the existing recordId/anchor uniqueness key rather than adding new
// schema, since a page is just "the one thing this view's title stands
// for").

export type RecordType = 'tag' | 'transition' | 'vocab' | 'page';

const RECORD_TYPE_ICON: Record<RecordType, IconName> = {
  tag: 'tags',
  transition: 'scroll-text',
  vocab: 'book-open',
  page: 'layout-dashboard',
};

const RECORD_TYPE_COLOR: Record<RecordType, string> = {
  tag: 'var(--lm-primary-strong)',
  transition: 'var(--t-act)',
  vocab: 'var(--tag-teal)',
  page: 'var(--lm-text-dim)',
};

/** icon/color are language-independent constants; label comes from the active `t` (replaces the old static RECORD_TYPE_META). */
export function recordTypeMeta(t: Strings, type: RecordType): { label: string; icon: IconName; color: string } {
  return { label: t.comments.recordType[type], icon: RECORD_TYPE_ICON[type], color: RECORD_TYPE_COLOR[type] };
}

export interface CommentTarget {
  recordType: RecordType;
  recordId: string;
  recordTitle: string;
  anchor: string;
  anchorLabel: string;
}

export interface CommentReply {
  id: string;
  text: string;
  createdAt: number;
}

export interface CommentRecord extends CommentTarget {
  id: string;
  text: string;
  createdAt: number;
  updatedAt: number;
  replies: CommentReply[];
}

interface CommentsValue {
  comments: CommentRecord[];
  hasComment: (recordId: string, anchor: string) => boolean;
  panelOpen: boolean;
  openPanel: () => void;
  closePanel: () => void;
  composer: CommentTarget | null;
  composerText: string;
  isEditingExisting: boolean;
  openComposer: (target: CommentTarget) => void;
  editComment: (c: CommentRecord) => void;
  setComposerText: (text: string) => void;
  saveComposer: () => void;
  cancelComposer: () => void;
  deleteComment: (id: string) => void;
  replyDrafts: Record<string, string>;
  setReplyDraft: (commentId: string, text: string) => void;
  addReply: (commentId: string) => void;
  deleteReply: (commentId: string, replyId: string) => void;
  copyMsg: boolean;
  copyAll: () => void;
}

const STORAGE_KEY = 'pmem-comments-v1';
const CommentsContext = createContext<CommentsValue | null>(null);

function load(): CommentRecord[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const arr = JSON.parse(raw);
    if (!Array.isArray(arr)) return [];
    // Additive migration: comments saved before replies existed decode with
    // no `replies` key at all — normalize to [] so every consumer can just
    // read c.replies.length without an `|| []` at every call site.
    return arr.map((c) => ({ ...c, replies: Array.isArray(c.replies) ? c.replies : [] }));
  } catch {
    return [];
  }
}

function persist(arr: CommentRecord[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(arr));
  } catch {
    // Private-mode/quota — comments stay in-memory for this session only.
  }
}

function newId(prefix: string): string {
  return prefix + Math.random().toString(36).slice(2, 10) + Math.random().toString(36).slice(2, 6);
}

function buildCopyText(t: Strings, comments: CommentRecord[]): string {
  const lines = [t.comments.copyDocTitle, t.comments.copyIntro(comments.length), ''];
  comments.forEach((c, i) => {
    lines.push(t.comments.copyItemHeader(i + 1, recordTypeMeta(t, c.recordType).label, c.recordId, c.recordTitle));
    lines.push(t.comments.copyLocationLine(c.anchorLabel));
    lines.push(t.comments.copyCommentLine(c.text));
    if (c.replies.length > 0) {
      lines.push(t.comments.copyReplyHeading);
      c.replies.forEach((r) => lines.push(`     - ${r.text}`));
    }
    lines.push('');
  });
  return lines.join('\n');
}

function fallbackCopy(text: string) {
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.top = '-9999px';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
  } catch {
    // best-effort only
  }
}

export function CommentsProvider({ children }: { children: ComponentChildren }) {
  const t = useT();
  const [comments, setComments] = useState<CommentRecord[]>([]);
  const [panelOpen, setPanelOpen] = useState(false);
  const [composer, setComposer] = useState<CommentTarget | null>(null);
  const [composerText, setComposerTextState] = useState('');
  const [replyDrafts, setReplyDrafts] = useState<Record<string, string>>({});
  const [copyMsg, setCopyMsg] = useState(false);

  useEffect(() => {
    setComments(load());
  }, []);

  const hasComment = (recordId: string, anchor: string) => comments.some((c) => c.recordId === recordId && c.anchor === anchor);

  const openComposer = (target: CommentTarget) => {
    const existing = comments.find((c) => c.recordId === target.recordId && c.anchor === target.anchor);
    setComposer(target);
    setComposerTextState(existing?.text || '');
    setPanelOpen(true);
    setCopyMsg(false);
  };

  const editComment = (c: CommentRecord) => {
    setComposer({ recordType: c.recordType, recordId: c.recordId, recordTitle: c.recordTitle, anchor: c.anchor, anchorLabel: c.anchorLabel });
    setComposerTextState(c.text);
    setPanelOpen(true);
    setCopyMsg(false);
  };

  const cancelComposer = () => {
    setComposer(null);
    setComposerTextState('');
  };

  const saveComposer = () => {
    if (!composer) return;
    const text = composerText.trim();
    setComments((prev) => {
      const idx = prev.findIndex((c) => c.recordId === composer.recordId && c.anchor === composer.anchor);
      let next: CommentRecord[];
      if (!text) {
        next = idx >= 0 ? prev.filter((_, i) => i !== idx) : prev;
      } else if (idx >= 0) {
        next = prev.map((c, i) => (i === idx ? { ...c, text, updatedAt: Date.now() } : c));
      } else {
        next = [...prev, { ...composer, id: newId('c'), text, createdAt: Date.now(), updatedAt: Date.now(), replies: [] }];
      }
      persist(next);
      return next;
    });
    setComposer(null);
    setComposerTextState('');
  };

  const deleteComment = (id: string) => {
    setComments((prev) => {
      const next = prev.filter((c) => c.id !== id);
      persist(next);
      return next;
    });
    setReplyDrafts((prev) => {
      if (!(id in prev)) return prev;
      const next = { ...prev };
      delete next[id];
      return next;
    });
  };

  const setReplyDraft = (commentId: string, text: string) => setReplyDrafts((prev) => ({ ...prev, [commentId]: text }));

  const addReply = (commentId: string) => {
    const text = (replyDrafts[commentId] || '').trim();
    if (!text) return;
    setComments((prev) => {
      const next = prev.map((c) =>
        c.id === commentId ? { ...c, replies: [...c.replies, { id: newId('r'), text, createdAt: Date.now() }], updatedAt: Date.now() } : c,
      );
      persist(next);
      return next;
    });
    setReplyDrafts((prev) => {
      const next = { ...prev };
      delete next[commentId];
      return next;
    });
  };

  const deleteReply = (commentId: string, replyId: string) => {
    setComments((prev) => {
      const next = prev.map((c) => (c.id === commentId ? { ...c, replies: c.replies.filter((r) => r.id !== replyId) } : c));
      persist(next);
      return next;
    });
  };

  const copyAll = () => {
    if (comments.length === 0) return;
    const text = buildCopyText(t, comments);
    const done = () => {
      setCopyMsg(true);
      setTimeout(() => setCopyMsg(false), 2000);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, () => {
        fallbackCopy(text);
        done();
      });
    } else {
      fallbackCopy(text);
      done();
    }
  };

  const value: CommentsValue = {
    comments,
    hasComment,
    panelOpen,
    openPanel: () => {
      setPanelOpen(true);
      setComposer(null);
      setCopyMsg(false);
    },
    closePanel: () => {
      setPanelOpen(false);
      setComposer(null);
      setComposerTextState('');
    },
    composer,
    composerText,
    isEditingExisting: !!composer && comments.some((c) => c.recordId === composer.recordId && c.anchor === composer.anchor),
    openComposer,
    editComment,
    setComposerText: setComposerTextState,
    saveComposer,
    cancelComposer,
    deleteComment,
    replyDrafts,
    setReplyDraft,
    addReply,
    deleteReply,
    copyMsg,
    copyAll,
  };

  return <CommentsContext.Provider value={value}>{children}</CommentsContext.Provider>;
}

export function useComments(): CommentsValue {
  const ctx = useContext(CommentsContext);
  if (!ctx) throw new Error('useComments() must be called within a CommentsProvider');
  return ctx;
}
