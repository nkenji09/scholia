// クリップボードへ書く（コメントの一括コピーと、カードが出す CLI コマンドの共有）。
//
// navigator.clipboard は安全なコンテキストでしか生えず、生えていても権限で
// 拒否されうる。viewer は localhost で配るので live では使えるが、
// `export --html` した静的 HTML を file:// で開いた場合まで同じ保証は無い——
// そこを textarea + execCommand で拾う。この二段構えを面ごとに書き分けると、
// 片方だけが静的で黙って失敗する形になるので1箇所に置く。
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

/** text をクリップボードへ書き、完了（＝表示を「コピーしました」に切り替えて
    よい状態）で done を呼ぶ。失敗しても done は呼ぶ——best-effort。 */
export function copyText(text: string, done?: () => void) {
  const finish = () => done && done();
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(finish, () => {
      fallbackCopy(text);
      finish();
    });
  } else {
    fallbackCopy(text);
    finish();
  }
}
