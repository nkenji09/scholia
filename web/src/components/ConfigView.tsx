import { useEffect, useState } from 'preact/hooks';
import { api } from '../api';
import type { Config } from '../types';

function toCsv(arr: string[]): string {
  return arr.join(', ');
}

function fromCsv(s: string): string[] {
  return s
    .split(',')
    .map((v) => v.trim())
    .filter((v) => v.length > 0);
}

export function ConfigView() {
  const [config, setConfig] = useState<Config | null>(null);
  const [tagKinds, setTagKinds] = useState('');
  const [facetKinds, setFacetKinds] = useState('');
  const [traceabilityKinds, setTraceabilityKinds] = useState('');
  const [roots, setRoots] = useState('');
  const [port, setPort] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    api
      .getConfig()
      .then((cfg) => {
        setConfig(cfg);
        setTagKinds(toCsv(cfg.tagKinds));
        setFacetKinds(toCsv(cfg.facetKinds));
        setTraceabilityKinds(toCsv(cfg.traceabilityKinds));
        setRoots(toCsv(cfg.roots));
        setPort(String(cfg.viewer.port));
      })
      .catch((err) => setError(String(err)));
  }, []);

  const onSubmit = (e: Event) => {
    e.preventDefault();
    setError(null);
    setMessage(null);
    const portNum = Number(port);
    if (!Number.isInteger(portNum)) {
      setError('viewer.port は数値である必要があります');
      return;
    }
    api
      .putConfig({
        tagKinds: fromCsv(tagKinds),
        facetKinds: fromCsv(facetKinds),
        traceabilityKinds: fromCsv(traceabilityKinds),
        roots: fromCsv(roots),
        viewer: { port: portNum },
      })
      .then((cfg) => {
        setConfig(cfg);
        setMessage('config を更新しました');
      })
      .catch((err) => setError(String(err)));
  };

  if (!config) return <main class="config-view dim">loading…</main>;

  return (
    <main class="config-view">
      <h2>config</h2>
      <form onSubmit={onSubmit}>
        <label>
          tagKinds (comma-separated)
          <input value={tagKinds} onInput={(e) => setTagKinds((e.target as HTMLInputElement).value)} />
        </label>
        <label>
          facetKinds (comma-separated)
          <input value={facetKinds} onInput={(e) => setFacetKinds((e.target as HTMLInputElement).value)} />
        </label>
        <label>
          traceabilityKinds (comma-separated)
          <input
            value={traceabilityKinds}
            onInput={(e) => setTraceabilityKinds((e.target as HTMLInputElement).value)}
          />
        </label>
        <label>
          roots (comma-separated)
          <input value={roots} onInput={(e) => setRoots((e.target as HTMLInputElement).value)} />
        </label>
        <label>
          viewer.port
          <input value={port} onInput={(e) => setPort((e.target as HTMLInputElement).value)} />
        </label>
        <button type="submit">保存</button>
      </form>
      {error && <p class="error">{error}</p>}
      {message && <p>{message}</p>}
      <details>
        <summary>読み取り専用（config set 非対応）</summary>
        <p>pmemVersion: {config.pmemVersion}</p>
        <p>kinds.condition: {config.kinds.condition.join(', ')}</p>
        <p>kinds.action: {config.kinds.action.join(', ')}</p>
        <p>kinds.effect: {config.kinds.effect.join(', ')}</p>
      </details>
    </main>
  );
}
