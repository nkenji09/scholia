import { useComments } from './useComments';
import type { RecordType } from './useComments';
import { useT } from '../../i18n';
import { Icon } from '../shared/Icon';

interface Props {
  recordType: RecordType;
  recordId: string;
  recordTitle: string;
  anchor: string;
  anchorLabel: string;
}

// Small affordance next to a card section heading — the design's "付与済
// み箇所の視覚マーカー": filled icon once a comment exists there, plain
// outline otherwise. Always present regardless of comment count (unlike
// the header icon, which only shows once count > 0).
export function CommentButton({ recordType, recordId, recordTitle, anchor, anchorLabel }: Props) {
  const t = useT();
  const { hasComment, openComposer } = useComments();
  const has = hasComment(recordId, anchor);
  return (
    <button
      type="button"
      class={'comment-button' + (has ? ' has-comment' : '')}
      title={t.comments.addHere}
      onClick={() => openComposer({ recordType, recordId, recordTitle, anchor, anchorLabel })}
    >
      <Icon name={has ? 'message-filled' : 'message-plus'} size={13} />
    </button>
  );
}
