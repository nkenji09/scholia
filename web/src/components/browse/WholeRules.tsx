import { useEffect, useRef, useState } from 'preact/hooks';
import { useT } from '../../i18n';
import { copyText } from '../../clipboard';
import { routeHash } from '../../router';
import { HashLink } from '../shared/HashLink';
import { Icon } from '../shared/Icon';
import { rulesCommand } from './rulesCommand';
import type { RecordRef } from './rulesCommand';

// 「この記録を支配する規則の全体」への入口（01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// ここは長らく**お詫び**だった。追補 01KYJV3FYMDFRWQ939NBV2BPAC 条項1 が
// 「その用途の受け皿は現状 CLI だけである。viewer には無い」と確定し、同 条項3 が
// 「カードはその事実と、いま使える手段を利用者が知れる形にする」と定めたので、
// 見出し行そのものが「viewer にありません」と述べていた——**面が無いことを黙って
// 伏せない**ための開示である（01KXYED62CEKBY97D7X66BMC9A）。
//
// その面ができた。同 追補 条項4 は「支配方向で絞れる面を viewer に足すかどうかは
// 本 decision では決めない（見直しの入口）」と明示的に保留しており、
// 01KYKS4Y56FAHRVCWKMQJK4RT6 がその入口を通った。よって開示は
// **実際に踏めるリンク**に置き換わる。
//
// 判定は viewer 側で作り直していない。リンクが渡すのは「対象＝この記録」「向き＝
// 自身＋祖先」という絞り込み条件だけで、その集合を決めるのは GET /api/governs
// （CLI `scholia rules` と同じ Go コア）である——**同じ選択規則を2箇所に書かない**
// （01KXYED61J6QBEX75H2XHVHW7Y の診断・追補の「採らなかった選択肢」の警告）。
//
// 端末で読む手段は**残す**。リンクができたからといって消すと、全文を1つの並びで
// 一気に読む／貼り付ける経路が失われる（01KYK4YNCYGZHHXB4H90Q996T2 条項5・
// 01KXYED62CEKBY97D7X66BMC9A の「省略はその旨を開示する」）。形は従来どおり
// ——手段は畳み、コピーできるようにする。
//
// 件数は**効いている規則の数**（01KYHW54B8ZXH0NEPH2J7N1X39 条項5 と同じ数え方）。
// 呼び出し側（InheritedRules）が governs から数えて渡す。開示した件数とリンク先で
// 読める件数が食い違わないように、数え方を2箇所に持たない。

/** 記録の種別 → 絞り込み条件の対象の種別。`transition` は綴りまで CLI の
    `--on transition:<id>` に揃えてある（同じ語彙を2通りに綴らない）。 */
function scopeRef(record: RecordRef): string {
  return `${record.kind}:${record.id}`;
}

export function WholeRules({ record, inForceCount }: { record: RecordRef; inForceCount: number }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cmd = rulesCommand(record);
  const href = routeHash({ view: 'decisions', decisionOn: scopeRef(record), decisionScope: 'governing' });

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
      {/* 入口そのものは畳まない。畳んだ内側に入れると、開かなかった利用者には
          到達手段が伝わらない——お詫びだった時代と同じ理由がそのまま効く。 */}
      <HashLink
        href={href}
        onNavigate={() => {
          // 共有部品なので親のコールバックに頼らない（修飾クリックは HashLink が
          // 別タブに回すので、リンクとしての性質は保たれる）。
          window.location.hash = href;
        }}
        class="whole-rules-link"
        title={t.browse.wholeRulesLinkTitle}
      >
        <Icon name="gavel" size={13} />
        <span>{t.browse.wholeRulesLink(inForceCount)}</span>
        <Icon name="arrow-up-right" size={12} />
      </HashLink>
      <button type="button" class="whole-rules-head" aria-expanded={open} onClick={() => setOpen(!open)}>
        <Icon name="info" size={13} />
        <span>{t.browse.wholeRulesCliHead}</span>
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
