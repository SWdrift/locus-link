import type {
  DocumentContent,
  DocumentView,
  GraphView,
  LocusContext,
  ProbeResult,
  ResolveResult,
  StatusView,
  ValidationResult,
} from './domain/locus'

async function requestJSON<T>(input: string, init?: RequestInit): Promise<T> {
  const response = await fetch(input, { headers: { Accept: 'application/json', ...init?.headers }, ...init })
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: response.statusText })) as { error?: string }
    throw new Error(body.error ?? response.statusText)
  }
  return response.json() as Promise<T>
}

export const getContext = () => requestJSON<LocusContext>('/api/v0/context')
export const getGraph = () => requestJSON<GraphView>('/api/v0/graph')
export const getValidation = () => requestJSON<ValidationResult>('/api/v0/validate')

export function getStatus(from: string, vantage: string) {
  const query = new URLSearchParams({ from, vantage })
  return requestJSON<StatusView>(`/api/v0/status?${query}`)
}

export const getKnowledge = () => requestJSON<{ documents: DocumentView[] }>('/api/v0/knowledge')
export const getDocument = (id: string) => requestJSON<DocumentContent>(`/api/v0/knowledge/${encodeURIComponent(id)}`)

export function resolveRoute(target: string, capability: string, from: string, vantage: string) {
  const query = new URLSearchParams({ target, capability, from, vantage })
  return requestJSON<ResolveResult>(`/api/v0/resolve?${query}`)
}

export const probe = (subject: string, from: string, vantage: string) => requestJSON<ProbeResult>('/api/v0/probes', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ subject, from, vantage, timeout_ms: 10_000 }),
})
