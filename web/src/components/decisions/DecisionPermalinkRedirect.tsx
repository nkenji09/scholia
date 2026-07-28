import { useEffect } from 'preact/hooks';
import { useT } from '../../i18n';
import { routeHash } from '../../router';
import { formatScopeTarget } from './decisionScope';

// 旧単票 `#/decision/<ulid>` の転送（01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// 単票は廃止したが、**共有済みの URL は生かす**。decision は append-only で
// 過去の記録が id で相手を指しており、他人に渡したリンクも残っている——黙って
// 殺さないための安い保険で、廃止が成立するための3条件のうちの1つ。
//
// 着く先は同じ中身である: `?on=decision:<ulid>` は「その1件に絞り込んだ一覧」で、
// 結果が1件なので開いた状態で着地する。
//
// `location.replace` を使うのは、履歴を1つ**積まない**ため。`location.hash = …`
// で転送すると旧 URL と新 URL の2エントリが並び、バックを押した利用者が旧 URL に
// 戻され、そこから即座に前へ送り返される（バックが効かなくなる）。
export function DecisionPermalinkRedirect({ decisionId }: { decisionId?: string }) {
  const t = useT();
  const target = decisionId
    ? routeHash({ view: 'decisions', decisionOn: formatScopeTarget({ type: 'decision', id: decisionId }) })
    : routeHash({ view: 'decisions' });

  useEffect(() => {
    // replace() は hashchange を発火するので、App の useHashRoute がそのまま
    // 新しいルートを拾う（自前で navigate を呼ばない＝経路を二重に持たない）。
    window.location.replace(target);
  }, [target]);

  return <main class="decisions-view dim">{t.decisions.loading}</main>;
}
