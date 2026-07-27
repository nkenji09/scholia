import { useT } from '../../i18n';
import type { Strings } from '../../i18n';
import { Icon } from '../shared/Icon';
import type { SupersedeLink, SupersedeTargetDetail } from '../../types';

// 採用が結線の検証で落ちたときの文言を、サーバの code から選ぶ。
//
// サーバ（internal/model.SupersedeError）の Error() は「どの id を直せばよいか」
// を含む CLI 向けの文言なので、そのまま出すとドロワーに生 ULID が出る
// （01KYCC2TF3NW3JRSSRK9ZHN078: viewer は生レコード id を表示しない）。code を
// 知らない失敗（閉路検査など・id を含まない）は message をそのまま出す。
export function supersedeErrorMessage(t: Strings, code: string | undefined, fallback: string): string {
  switch (code) {
    case 'supersedes-missing-target':
      return t.comments.supersedeErrorMissingTarget;
    case 'supersedes-invalid-mode':
      return t.comments.supersedeErrorInvalidMode;
    case 'supersedes-duplicate':
      return t.comments.supersedeErrorDuplicate;
    case 'supersedes-self-reference':
      return t.comments.supersedeErrorSelfReference;
    case 'supersedes-empty-id':
      return t.comments.supersedeErrorEmptyId;
    case 'supersedes-mode-rewrite':
      return t.comments.supersedeErrorModeRewrite;
    default:
      return fallback;
  }
}

// 結線の確認（adopt が現行性リンクまで束ねる要件・01KYHE08WNA8H1Q1DM2H45Y4TK）。
//
// Adopt を押すと decision 昇格と同時に supersedes[] が張られる。何を失効/改訂
// させるのかを押す前に見せ、人が昇格と結線をまとめて承認できるようにする
// （req.evaluate-change.adopt-dialogue の「重い項目は変更前/変更後を提示して
// から承認を得る」を、結線についても満たす面）。
//
// 生 ULID は表示しない（01KYCC2TF3NW3JRSSRK9ZHN078）——id は decision 詳細への
// deep-link の href としてだけ使い、読ませるのは対象レコードの名前・日付・
// why の1行要約。解決はサーバ（GET /api/reviews の supersedesDetail）が行う。
export function SupersedeConfirm({
  details,
  declared,
  priorDecisionCount,
}: {
  details?: SupersedeTargetDetail[];
  declared?: SupersedeLink[];
  priorDecisionCount?: number;
}) {
  const t = useT();

  // 宣言が無い場合: 対象レコードに既存の意思決定があるときだけ注意を出す
  // （純粋な新規要件の追加は正当なので、無条件に警告しない＝黙認もしないが
  // ブロックもしない）。
  if (!declared || declared.length === 0) {
    if (!priorDecisionCount) return null;
    return (
      <div class="comment-supersede comment-supersede-none">
        <Icon name="triangle-alert" size={13} />
        <span>{t.comments.supersedeNoneWithPrior(priorDecisionCount)}</span>
      </div>
    );
  }

  // supersedesDetail はサーバの derive。取れていない（静的モード等）ときは
  // 宣言そのものから mode だけでも見せる。
  const rows: SupersedeTargetDetail[] = details && details.length > 0 ? details : declared.map((l) => ({ id: l.id, mode: l.mode || 'amend' }));

  return (
    <div class="comment-supersede">
      <div class="comment-supersede-head">
        <Icon name="history" size={13} />
        <span class="comment-supersede-title">{t.comments.supersedeHeading}</span>
      </div>
      <ul class="comment-supersede-list">
        {rows.map((r) => (
          <li key={r.id} class={'comment-supersede-item' + (r.missing ? ' comment-supersede-item-missing' : '')}>
            <a class="comment-supersede-link" href={`#/decision/${encodeURIComponent(r.id)}`}>
              {r.targetName || r.targetId || t.comments.supersedeUnknownTarget}
            </a>
            <span class={'comment-supersede-mode comment-supersede-mode-' + r.mode}>{t.comments.supersedeModeLabel(r.mode)}</span>
            {r.at && <span class="comment-supersede-at dim">{r.at.slice(0, 10)}</span>}
            {r.missing ? (
              <span class="comment-supersede-missing">{t.comments.supersedeMissing}</span>
            ) : (
              r.whySummary && <span class="comment-supersede-why dim">{r.whySummary}</span>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
