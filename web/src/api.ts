import type {
  Config,
  ConfigPatch,
  DiffResult,
  FacetsResponse,
  LintResult,
  SpecReport,
  Tag,
  TransitionDetail,
  TransitionsResponse,
  VocabEntry,
  Decision,
} from './types';

class ApiError extends Error {}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body && typeof body.error === 'string') message = body.error;
    } catch {
      // response body wasn't JSON; fall back to statusText
    }
    throw new ApiError(message);
  }
  return res.json() as Promise<T>;
}

function query(params: Record<string, string | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v) q.set(k, v);
  }
  const qs = q.toString();
  return qs ? `?${qs}` : '';
}

export const api = {
  getConfig: () => request<Config>('/api/config'),
  putConfig: (patch: ConfigPatch) =>
    request<Config>('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch),
    }),
  getFacets: () => request<FacetsResponse>('/api/facets'),
  getTags: (kind?: string) => request<Tag[]>('/api/tags' + query({ kind })),
  getVocab: (category?: string) => request<VocabEntry[]>('/api/vocab' + query({ category })),
  getTransitions: (params: { facet?: string; tag?: string; kind?: string }) =>
    request<TransitionsResponse>('/api/transitions' + query(params)),
  getTransition: (id: string) => request<TransitionDetail>(`/api/transitions/${encodeURIComponent(id)}`),
  getSpec: (tagId: string) => request<SpecReport>(`/api/spec/${encodeURIComponent(tagId)}`),
  getRules: (params: { tx?: string; tag?: string; facet?: string }) =>
    request<{ decisions: Decision[] }>('/api/rules' + query(params)),
  getLint: () => request<LintResult>('/api/lint'),
  getDiff: (ref?: string) => request<DiffResult>('/api/diff' + query({ ref })),
};

export { ApiError };
