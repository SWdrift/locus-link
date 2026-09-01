import type {
  DependencySnapshot,
  DocumentContent,
  DocumentView,
  GraphView,
  LocusContext,
  LocusScopeEntry,
  ProbeResult,
  RefreshResult,
  ResolveResult,
  StatusView,
  ValidationResult,
} from './domain/locus'

async function requestJSON<T>(input: string, init?: RequestInit): Promise<T> {
  const response = await fetch(input, { headers: { Accept: 'application/json', ...init?.headers }, ...init })
  if (!response.ok) {
    const body = (await response.json().catch(() => ({ error: response.statusText }))) as { error?: string }
    throw new Error(body.error ?? response.statusText)
  }
  return response.json() as Promise<T>
}

function queryWithScope(scopeID?: string, values: Record<string, string> = {}) {
  const query = new URLSearchParams(values)
  if (scopeID) query.set('scope', scopeID)
  return query.toString()
}

export const getContext = (scopeID?: string) => requestJSON<LocusContext>(`/api/v0/context?${queryWithScope(scopeID)}`)
export const getGraph = (scopeID?: string) => requestJSON<GraphView>(`/api/v0/graph?${queryWithScope(scopeID)}`)
export const getValidation = (scopeID?: string) =>
  requestJSON<ValidationResult>(`/api/v0/validate?${queryWithScope(scopeID)}`)

export function getStatus(scopeID: string | undefined, from: string, vantage: string) {
  return requestJSON<StatusView>(`/api/v0/status?${queryWithScope(scopeID, { from, vantage })}`)
}

export const getKnowledge = (scopeID?: string) =>
  requestJSON<{ documents: DocumentView[] }>(`/api/v0/knowledge?${queryWithScope(scopeID)}`)
export const getDocument = (scopeID: string | undefined, id: string) =>
  requestJSON<DocumentContent>(`/api/v0/knowledge/${encodeURIComponent(id)}?${queryWithScope(scopeID)}`)

export function resolveRoute(
  scopeID: string | undefined,
  target: string,
  capability: string,
  from: string,
  vantage: string,
) {
  const query = queryWithScope(scopeID, { target, capability, from, vantage })
  return requestJSON<ResolveResult>(`/api/v0/resolve?${query}`)
}

export const probe = (scopeID: string | undefined, subject: string, from: string, vantage: string) =>
  requestJSON<ProbeResult>(`/api/v0/probes?${queryWithScope(scopeID)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ subject, from, vantage, timeout_ms: 10_000 }),
  })

export const getLocusScopes = () => requestJSON<{ scopes: LocusScopeEntry[] }>('/api/v0/locus/scopes')
export const getDependencies = (rootScopeID: string) =>
  requestJSON<DependencySnapshot>(`/api/v0/locus/dependencies?${new URLSearchParams({ root: rootScopeID })}`)

export const refreshDependencies = (
  scopeID: string,
  allowRegression = false,
  expectedCandidateDigest = '',
  aliasPath = '',
) =>
  requestJSON<RefreshResult>('/api/v0/locus/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      scope_id: scopeID,
      alias_path: aliasPath,
      allow_regression: allowRegression,
      expected_candidate_digest: expectedCandidateDigest,
    }),
  })
