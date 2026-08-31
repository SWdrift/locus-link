export interface Scope {
  id: string
  kind: string
}

export interface ImportRef {
  alias: string
  scope_id: string
}

export interface RuntimeContext {
  current_entity?: string
  vantage: string
}

export interface LocusContext {
  active_scope: Scope
  imports: ImportRef[]
  bindings: Record<string, string>
  runtime: RuntimeContext
}

export async function getContext(): Promise<LocusContext> {
  const response = await fetch('/api/v0/context', {
    headers: { Accept: 'application/json' },
  })
  if (!response.ok) {
    throw new Error(await response.text())
  }
  return response.json() as Promise<LocusContext>
}
