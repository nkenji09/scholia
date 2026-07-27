import { useEffect, useRef, useState } from 'preact/hooks';
import { useT } from '../../i18n';
import { copyText } from '../../clipboard';
import { Icon } from '../shared/Icon';
import { rulesCommand } from './rulesCommand';
import type { RecordRef } from './rulesCommand';

// 「全体はどこで読めるか」の開示（01KYJV3FYMDFRWQ939NBV2BPAC 条項3）。
//
// 追補で確定したのは次の3つ:
//   1. 「この記録に効いている規則の全体を1つの並びで読む」受け皿は**現状 CLI だけ**。
//      viewer には無い（意思決定の一覧はタグの配下方向にしか絞れず、支配する
//      規則＝自身＋祖先とは向きが逆）。
//   2. カードが持つ一覧への入口は、その集合を指していない（RulesListLink）。
//   3. **カードは、規則の全体をどこで読めるかを利用者に開示する。**
//
// ここは 3。件数と継承元の開示（01KYHW4NBNVN9BFXYZMBX8MPF8 条項3＝InheritedRules）
// だけでは「全体を通しで読む」用途に答えていない——その差を黙って伏せない、が
// 01KXYED62CEKBY97D7X66BMC9A（省略はその旨を開示する）の要求。
//
// 見せ方の設計:
//
// ・**事実は畳まない。** 見出し行そのものが「viewer に面が無い」と述べる。畳んだ
//   中に事実を入れると、開かなかった利用者には省略が伝わらない＝開示にならない。
// ・**手段は畳む。** コマンドは必要になったときだけ要る。加えて id は name/label
//   ではないので（01KYCC2TF3NW3JRSSRK9ZHN078 は表示を name/label に限り、id は
//   deep-link の href としてのみ使うと定める）、既定の見え方に置かない。コマンドは
//   利用者が**実行するために要求したとき**にだけ出し、コピーもできるようにする——
//   ここで id を name に置き換えると動かないコマンドになり、開示として無意味。
// ・**継承0件のカードでも成立する。** この口は「継承した規則があるか」ではなく
//   「効いている規則が1件でもあるか」で出す（呼び出し側の InheritedRules 参照）。
//   実測で tag 21件が「継承0・own あり」で、そこにも全体を読む用途はある。
//   逆に効いている規則が1件も無い記録では読む全体が無いので出さない。
export function WholeRules({ record }: { record: RecordRef }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cmd = rulesCommand(record);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const copy = () => {
    copyText(cmd, () => {
      setCopied(true);
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 2000);
    });
  };

  return (
    <div class="whole-rules">
      <button type="button" class="whole-rules-head" aria-expanded={open} onClick={() => setOpen(!open)}>
        <Icon name="info" size={13} />
        <span>{t.browse.wholeRulesFact}</span>
        <Icon name={open ? 'chevron-down' : 'chevron-right'} size={13} />
      </button>
      {open && (
        <div class="whole-rules-body">
          <div class="whole-rules-note">{t.browse.wholeRulesHow}</div>
          <div class="whole-rules-cmd">
            <code>{cmd}</code>
            <button type="button" class="whole-rules-copy" onClick={copy}>
              <Icon name={copied ? 'check' : 'clipboard-copy'} size={12} />
              {copied ? t.browse.wholeRulesCopied : t.browse.wholeRulesCopy}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
