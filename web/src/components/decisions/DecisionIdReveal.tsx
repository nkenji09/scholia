import { useEffect, useRef, useState } from 'preact/hooks';
import { useT } from '../../i18n';
import { copyText } from '../../clipboard';
import { Icon } from '../shared/Icon';

// 意思決定の生成 id を開示する（01KYK4YNCYGZHHXB4H90Q996T2 条項3〜5）。
//
// id そのものは読む理由が無く触っても何も起きない不透明な生成 id なので、
// 既定の見え方には置かない（条項3）。ただし単票にはこれ以外に id へ到達する
// 経路が無いので、単に消すと `scholia decision show <id>` や `--supersedes` に
// 渡す id を得る手段が黙って失われる（条項5）。そこで、利用者が実行・貼り付ける
// ための文字列として、求めたときにだけ実行できる形＋コピーで出す（条項4）。
//
// WholeRules（browse/WholeRules.tsx・追補 01KYJV3FYMDFRWQ939NBV2BPAC 条項3）と
// 同じ開示の型（見出しは畳まず、手段だけを畳んでコピーできるようにする）を踏襲する。
export function DecisionIdReveal({ id }: { id: string }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cmd = `scholia decision show ${id}`;

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
        <span>{t.decisions.idRevealHead}</span>
        <Icon name={open ? 'chevron-down' : 'chevron-right'} size={13} />
      </button>
      {open && (
        <div class="whole-rules-body">
          <div class="whole-rules-note">{t.decisions.idRevealNote}</div>
          <div class="whole-rules-cmd">
            <code>{cmd}</code>
            <button type="button" class="whole-rules-copy" onClick={copy}>
              <Icon name={copied ? 'check' : 'clipboard-copy'} size={12} />
              {copied ? t.decisions.idRevealCopied : t.decisions.idRevealCopy}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
