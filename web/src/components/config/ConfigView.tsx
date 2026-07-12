import { useEffect, useRef, useState } from 'preact/hooks';
import { api, isStaticMode } from '../../api';
import { useT } from '../../i18n';
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
  productName: string;
  tagline: string;
  intro: string;
}

function toEditable(cfg: Config): EditableConfig {
  return {
    tagKinds: [...cfg.tagKinds],
    facetKinds: [...cfg.facetKinds],
    traceabilityKinds: [...cfg.traceabilityKinds],
    roots: [...cfg.roots],
    port: String(cfg.viewer.port),
    tagKindLabels: { ...(cfg.tagKindLabels || {}) },
    productName: cfg.display?.productName || '',
    tagline: cfg.display?.tagline || '',
    intro: cfg.display?.intro || '',
  };
}

function addUnique(arr: string[], value: string): string[] {
  return arr.includes(value) ? arr : [...arr, value];
}

export function ConfigView() {
  const t = useT();
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
  if (!remote || !draft) return <main class="config-view dim">{t.config.loading}</main>;

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
      setMessage({ kind: 'error', text: t.config.portInvalid(portStr || t.config.portEmptyWord) });
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
        display: { productName: draft.productName, tagline: draft.tagline, intro: draft.intro },
      })
      .then((cfg) => {
        setRemote(cfg);
        const ed = toEditable(cfg);
        baseline.current = JSON.stringify(ed);
        setDraft(ed);
        setMessage({ kind: 'ok', text: t.config.savedMessage });
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
          <h1>{t.config.heading}</h1>
          <p class="dim">
            {t.config.introBefore}
            <code>.pmem/config.json</code>
            {t.config.introAfter}
          </p>
        </div>
      </div>

      {editable ? (
        <div class="config-status-bar">
          <span class="config-status-ok">
            <Icon name="server" size={14} />
            {t.config.serverModeBefore}
            <code>config.json</code>
            {t.config.serverModeAfter}
          </span>
          {dirty && <span class="config-dirty-badge">{t.config.dirtyBadge}</span>}
          <span class="config-status-spacer" />
          {dirty && (
            <button type="button" class="config-btn-secondary" onClick={onReset}>
              {t.config.discard}
            </button>
          )}
          <button type="button" class="config-btn-primary" onClick={onSave}>
            <Icon name="save" size={14} />
            {t.common.save}
          </button>
        </div>
      ) : (
        <div class="config-readonly-banner">
          <div class="config-readonly-banner-head">
            <Icon name="eye" size={15} class="dim" />
            <span class="config-readonly-title">{t.config.readonlyTitle}</span>
          </div>
          <span class="dim">
            <code>pmem export --html</code>
            {t.config.readonlyBannerMid}
            <code>pmem view</code>
            {t.config.readonlyBannerSuffix}
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
          <span class="config-section-title">{t.config.sections.classification.title}</span>
          <span class="dim">{t.config.sections.classification.desc}</span>
        </div>
        <TokenSetField
          label={t.config.fields.tagKinds.label}
          mono="tagKinds"
          icon="tags"
          description={t.config.fields.tagKinds.description}
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
          label={t.config.fields.facetKinds.label}
          mono="facetKinds"
          icon="panel-left"
          description={t.config.fields.facetKinds.description}
          values={draft.facetKinds}
          editable={editable}
          subsetOf="tagKinds"
          isSubsetMember={(v) => tagKindSet.has(v)}
          onAdd={(v) => update({ facetKinds: addUnique(draft.facetKinds, v) })}
          onRemove={(v) => update({ facetKinds: draft.facetKinds.filter((x) => x !== v) })}
        />
        <TokenSetField
          label={t.config.fields.roots.label}
          mono="roots"
          icon="list-tree"
          description={t.config.fields.roots.description}
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
          <span class="config-section-title">{t.config.sections.traceability.title}</span>
          <span class="dim">{t.config.sections.traceability.desc}</span>
        </div>
        <TokenSetField
          label={t.config.fields.traceabilityKinds.label}
          mono="traceabilityKinds"
          icon="radar"
          description={t.config.fields.traceabilityKinds.description}
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
          <span class="config-section-title">{t.config.sections.viewer.title}</span>
          <span class="dim">{t.config.sections.viewer.desc}</span>
        </div>
        <div class="config-field">
          <div class="config-field-head">
            <span class="config-field-icon">
              <Icon name="plug" size={14} />
            </span>
            <span class="config-field-label">{t.config.fields.port.label}</span>
            <span class="config-field-mono">viewer.port</span>
          </div>
          <p class="config-field-desc dim">
            {t.config.fields.port.descriptionBefore}
            <code>pmem view</code>
            {t.config.fields.port.descriptionAfter}
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

      <section class="config-section">
        <div class="config-section-head">
          <span class="config-section-icon">
            <Icon name="pencil" size={16} />
          </span>
          <span class="config-section-title">{t.config.sections.display.title}</span>
          <span class="dim">{t.config.sections.display.desc}</span>
        </div>
        <div class="config-field">
          <div class="config-field-head">
            <span class="config-field-icon">
              <Icon name="box" size={14} />
            </span>
            <span class="config-field-label">{t.config.fields.productName.label}</span>
            <span class="config-field-mono">display.productName</span>
          </div>
          <p class="config-field-desc dim">{t.config.fields.productName.description}</p>
          {editable ? (
            <input
              class="config-port-input config-wide-input"
              value={draft.productName}
              placeholder="pmem"
              onInput={(e) => update({ productName: (e.target as HTMLInputElement).value })}
            />
          ) : (
            <span class="config-port-readonly">{draft.productName || 'pmem'}</span>
          )}
        </div>
        <div class="config-field">
          <div class="config-field-head">
            <span class="config-field-icon">
              <Icon name="scroll-text" size={14} />
            </span>
            <span class="config-field-label">{t.config.fields.tagline.label}</span>
            <span class="config-field-mono">display.tagline</span>
          </div>
          <p class="config-field-desc dim">{t.config.fields.tagline.description}</p>
          {editable ? (
            <input
              class="config-port-input config-wide-input"
              value={draft.tagline}
              placeholder={t.home.tagline}
              onInput={(e) => update({ tagline: (e.target as HTMLInputElement).value })}
            />
          ) : (
            <span class="config-port-readonly">{draft.tagline || t.home.tagline}</span>
          )}
        </div>
        <div class="config-field">
          <div class="config-field-head">
            <span class="config-field-icon">
              <Icon name="file-code-2" size={14} />
            </span>
            <span class="config-field-label">{t.config.fields.intro.label}</span>
            <span class="config-field-mono">display.intro</span>
          </div>
          <p class="config-field-desc dim">{t.config.fields.intro.description}</p>
          {editable ? (
            <textarea
              class="config-intro-textarea"
              value={draft.intro}
              rows={3}
              placeholder={t.home.intro}
              onInput={(e) => update({ intro: (e.target as HTMLTextAreaElement).value })}
            />
          ) : (
            <span class="config-port-readonly">{draft.intro}</span>
          )}
        </div>
      </section>

      <section class="config-section config-section-readonly">
        <div class="config-section-head">
          <span class="config-field-icon">
            <Icon name="lock" size={14} />
          </span>
          <span class="config-section-title">{t.config.sections.readonlyMeta.title}</span>
          <span class="config-readonly-tag">read-only</span>
        </div>
        <p class="dim config-readonly-desc">
          {t.config.sections.readonlyMeta.descBefore}
          <code>pmem config</code>
          {t.config.sections.readonlyMeta.descMid}
          <code>pmem kind</code>
          {t.config.sections.readonlyMeta.descAfter}
        </p>
        <div class="config-ro-row">
          <span class="config-ro-label">{t.config.schemaVersionLabel}</span>
          <span class="config-field-mono">pmemVersion</span>
          <span class="config-ro-value">{remote.pmemVersion}</span>
        </div>
        <div class="config-ro-vocab">
          <span class="config-ro-vocab-title">
            {t.config.vocabKindsHeading} <span class="dim">· idPrefix</span>
          </span>
          {(['condition', 'action', 'effect'] as const).map((cat) => (
            <div key={cat} class="config-ro-vocab-row">
              <span class="config-ro-vocab-cat">
                {cat} <span class="config-ro-vocab-prefix">{remote.idPrefix[cat] || '—'}</span>
              </span>
              <div class="config-field-chips">
                {remote.kinds[cat].length === 0 ? (
                  <span class="dim">{t.config.undefinedMarker}</span>
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
