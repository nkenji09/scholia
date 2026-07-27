import type { Strings } from '../../i18n';

// 採用（Adopt）が失敗したときにドロワーへ出す文言を、サーバの code から選ぶ。
//
// サーバのエラー文字列は CLI 向け（どの id を直せばよいかを含む）なので、
// そのまま出すと生 ULID がドロワーに乗る——decision の ULID（supersedes 検証）
// も review の ULID（昇格元コメントの削除）も同じ。01KYCC2TF3NW3JRSSRK9ZHN078
// は「viewer はユーザーに生レコード id を表示しない。id は deep-link の href
// としてのみ用いる」と定めているので、読ませる文言はこちらが持つ。
//
// code を知らない失敗（閉路検査・body 不正など・そもそも id を含まない）は
// fallback（サーバの message）をそのまま出す。
export function adoptErrorMessage(t: Strings, code: string | undefined, fallback: string): string {
  switch (code) {
    // 現行性リンク（supersedes）の検証（POST /api/decision）
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
    // 昇格元コメントの掃除（DELETE /api/reviews/{id}）
    case 'review-not-found':
      return t.comments.reviewErrorNotFound;
    case 'review-invalid-id':
      return t.comments.reviewErrorInvalidId;
    default:
      return fallback;
  }
}
