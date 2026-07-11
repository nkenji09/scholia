import { useEffect, useRef, useState } from 'preact/hooks';
import { api, isStaticMode } from '../../api';
import type { Config } from '../../types';
import { TokenSetField } from './TokenSetField';
import { TagKindLabelsField } from './TagKindLabelsField';
import { Icon } from '../shared/Icon';

interface EditableConfig {
  tagKinds: string[];
  facetKinds: string[];
  traceabilityKinds: string[];
  roots: string[];
  port: string;
  tagKindLabels: Record<string, string>;
}

function toEditable(cfg: Config): EditableConfig {
  return {
    tagKinds: [...cfg.tagKinds],
    facetKinds: [...cfg.facetKinds],
    traceabilityKinds: [...cfg.traceabilityKinds],
    roots: [...cfg.roots],
    port: String(cfg.viewer.port),
    tagKindLabels: { ...(cfg.tagKindLabels || {}) },
  };
}

function addUnique(arr: string[], value: string): string[] {
  return arr.includes(value) ? arr : [...arr, value];
}

export function ConfigView() {
  const [remote, setRemote] = useState<Config | null>(null);
  const [draft, setDraft] = useState<EditableConfig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  const baseline = useRef<string | null>(null);

  useEffect(() => {
    api
      .getConfig()
      .then((cfg) => {
        setRemote(cfg);
        const ed = toEditable(cfg);
        baseline.current = JSON.stringify(ed);
        setDraft(ed);
      })
      .catch((err) => setError(String(err)));
  }, []);

  if (error) return <main class="config-view error">{error}</main>;
  if (!remote || !draft) return <main class="config-view dim">loading…</main>;

  const editable = !isStaticMode;
  const dirty = editable && baseline.current !== null && JSON.stringify(draft) !== baseline.current;
  const tagKindSet = new Set(draft.tagKinds);

  const update = (patch: Partial<EditableConfig>) => {
    setDraft((prev) => (prev ? { ...prev, ...patch } : prev));
    setMessage(null);
  };

  const onSave = () => {
    const portStr = draft.port.trim();
    if (!/^\d+$/.test(portStr) || Number(portStr) < 1 || Number(portStr) > 65535) {
      setMessage({ kind: 'error', text: `ポートは 1〜65535 の整数で入力してください（現在: ${portStr || '空'}）` });
      return;
    }
    api
      .putConfig({
        tagKinds: draft.tagKinds,
        facetKinds: draft.facetKinds,
        traceabilityKinds: draft.traceabilityKinds,
        roots: draft.roots,
        viewer: { port: Number(portStr) },
        tagKindLabels: draft.tagKindLabels,
      })
      .then((cfg) => {
        setRemote(cfg);
        const ed = toEditable(cfg);
        baseline.current = JSON.stringify(ed);
        setDraft(ed);
        setMessage({ kind: 'ok', text: '保存しました — .pmem/config.json に書き込みました' });
      })
      .catch((err) => setMessage({ kind: 'error', text: String(err) }));
  };

  const onReset = () => {
    if (!baseline.current) return;
    setDraft(JSON.parse(baseline.current));
    setMessage(null);
  };

  return (
    <main class="config-view">
      <div class="config-head">
        <div class="config-head-text">
          <h1>設定</h1>
          <p class="dim">
            プロジェクト設定 <code>.pmem/config.json</code>。語彙とタグの分類軸・派生の定義です。変更頻度は低いですが、lint・要件トレーサビリティ・facet
            ナビ全体に波及します。
          </p>
        </div>
      </div>

      {editable ? (
        <div class="config-status-bar">
          <span class="config-status-ok">
            <Icon name="server" size={14} />
            サーバモード — 変更は <code>config.json</code> に書き込まれます
          </span>
          {dirty && <span class="config-dirty-badge">未保存の変更</span>}
          <span class="config-status-spacer" />
          {dirty && (
            <button type="button" class="config-btn-secondary" onClick={onReset}>
              破棄
            </button>
          )}
          <button type="button" class="config-btn-primary" onClick={onSave}>
            <Icon name="save" size={14} />
            保存
          </button>
        </div>
      ) : (
        <div class="config-readonly-banner">
          <div class="config-readonly-banner-head">
            <Icon name="eye" size={15} class="dim" />
            <span class="config-readonly-title">閲覧専用（静的版）</span>
          </div>
          <span class="dim">
            <code>pmem export --html</code> で書き出した1ファイル版です。編集・保存するには <code>pmem view</code> でサーバを起動してください。
          </span>
        </div>
      )}

      {message && (
        <div class={message.kind === 'ok' ? 'config-message-ok' : 'config-message-error'}>
          <Icon name={message.kind === 'ok' ? 'check' : 'triangle-alert'} size={15} />
          {message.text}
        </div>
      )}

      <section class="config-section">
        <div class="config-section-head">
          <span class="config-section-icon">
            <Icon name="git-fork" size={16} />
          </span>
          <span class="config-section-title">分類軸</span>
          <span class="dim">タグをどう分類し、どの軸で見せるか</span>
        </div>
        <TokenSetField
          label="タグ種別"
          mono="tagKinds"
          icon="tags"
          description="タグに付けられる分類の種類。タグの「役割」を定義します。"
          values={draft.tagKinds}
          editable={editable}
          onAdd={(v) => update({ tagKinds: addUnique(draft.tagKinds, v) })}
          onRemove={(v) => update({ tagKinds: draft.tagKinds.filter((x) => x !== v) })}
        />
        <TagKindLabelsField
          tagKinds={draft.tagKinds}
          labels={draft.tagKindLabels}
          editable={editable}
          onChange={(kind, label) => update({ tagKindLabels: { ...draft.tagKindLabels, [kind]: label } })}
        />
        <TokenSetField
          label="facet 軸"
          mono="facetKinds"
          icon="panel-left"
          description="Browse 画面のサイドバー facet ナビに出す種類。通常 tagKinds の部分集合です。"
          values={draft.facetKinds}
          editable={editable}
          subsetOf="tagKinds"
          isSubsetMember={(v) => tagKindSet.has(v)}
          onAdd={(v) => update({ facetKinds: addUnique(draft.facetKinds, v) })}
          onRemove={(v) => update({ facetKinds: draft.facetKinds.filter((x) => x !== v) })}
        />
        <TokenSetField
          label="ルートタグ"
          mono="roots"
          icon="list-tree"
          description="タグ階層のルートに置くタグ。空でも構いません。"
          values={draft.roots}
          editable={editable}
          onAdd={(v) => update({ roots: addUnique(draft.roots, v) })}
          onRemove={(v) => update({ roots: draft.roots.filter((x) => x !== v) })}
        />
      </section>

      <section class="config-section">
        <div class="config-section-head">
          <span class="config-section-icon">
            <Icon name="radar" size={16} />
          </span>
          <span class="config-section-title">トレーサビリティ</span>
          <span class="dim">要件↔実装（仕様）の対応を追跡する対象</span>
        </div>
        <TokenSetField
          label="トレーサビリティ対象"
          mono="traceabilityKinds"
          icon="radar"
          description="要件トレーサビリティ（充足 gap 検出）の対象にする種類。通常 requirement のみ。"
          values={draft.traceabilityKinds}
          editable={editable}
          subsetOf="tagKinds"
          isSubsetMember={(v) => tagKindSet.has(v)}
          onAdd={(v) => update({ traceabilityKinds: addUnique(draft.traceabilityKinds, v) })}
          onRemove={(v) => update({ traceabilityKinds: draft.traceabilityKinds.filter((x) => x !== v) })}
        />
      </section>

      <section class="config-section">
        <div class="config-section-head">
          <span class="config-section-icon">
            <Icon name="monitor" size={16} />
          </span>
          <span class="config-section-title">ビューア</span>
          <span class="dim">ローカルサーバの設定</span>
        </div>
        <div class="config-field">
          <div class="config-field-head">
            <span class="config-field-icon">
              <Icon name="plug" size={14} />
            </span>
            <span class="config-field-label">待受ポート</span>
            <span class="config-field-mono">viewer.port</span>
          </div>
          <p class="config-field-desc dim">
            ローカルサーバ（<code>pmem view</code>）が待ち受けるポート。1〜65535 の整数。
          </p>
          {editable ? (
            <input
              class="config-port-input"
              value={draft.port}
              inputMode="numeric"
              onInput={(e) => update({ port: (e.target as HTMLInputElement).value })}
            />
          ) : (
            <span class="config-port-readonly">{draft.port}</span>
          )}
        </div>
      </section>

      <section class="config-section config-section-readonly">
        <div class="config-section-head">
          <span class="config-field-icon">
            <Icon name="lock" size={14} />
          </span>
          <span class="config-section-title">読み取り専用メタ</span>
          <span class="config-readonly-tag">read-only</span>
        </div>
        <p class="dim config-readonly-desc">
          語彙(vocab)の種別・接頭辞・スキーマ版。変更は CLI（<code>pmem config</code> / <code>pmem kind</code>）で行います。
        </p>
        <div class="config-ro-row">
          <span class="config-ro-label">スキーマ版</span>
          <span class="config-field-mono">pmemVersion</span>
          <span class="config-ro-value">{remote.pmemVersion}</span>
        </div>
        <div class="config-ro-vocab">
          <span class="config-ro-vocab-title">
            語彙の種別（category ごと） <span class="dim">· idPrefix</span>
          </span>
          {(['condition', 'action', 'effect'] as const).map((cat) => (
            <div key={cat} class="config-ro-vocab-row">
              <span class="config-ro-vocab-cat">
                {cat} <span class="config-ro-vocab-prefix">{remote.idPrefix[cat] || '—'}</span>
              </span>
              <div class="config-field-chips">
                {remote.kinds[cat].length === 0 ? (
                  <span class="dim">（未定義）</span>
                ) : (
                  remote.kinds[cat].map((v) => (
                    <span key={v} class="config-ro-chip">
                      {v}
                    </span>
                  ))
                )}
              </div>
            </div>
          ))}
        </div>
      </section>
    </main>
  );
}
