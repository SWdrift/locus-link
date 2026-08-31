export type EvidenceStatus = 'success' | 'failure' | 'stale' | 'unknown'

export interface Scope {
  id: string
  kind: 'project' | 'environment'
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

export interface EntityView {
  canonical_id: string
  scope_id: string
  kind: string
  name: string
  labels?: Record<string, string>
  documentation_ids?: string[]
}

export interface LinkView {
  canonical_id: string
  scope_id: string
  from: string
  to: string
  provider: string
  requires?: string[]
  provides?: string[]
  documentation_ids?: string[]
}

export interface RouteView {
  canonical_id: string
  scope_id: string
  steps: string[]
  documentation_ids?: string[]
}

export interface GraphView {
  scopes: Scope[]
  bindings: Array<{ role: string; target: string }>
  entities: EntityView[]
  links: LinkView[]
  routes: RouteView[]
}

export interface Observation {
  subject: string
  vantage: string
  status: EvidenceStatus
  observed_at: string
  expires_at: string
  provider: string
  evidence?: Record<string, unknown>
  error?: string
}

export interface LinkEvidence {
  link_id: string
  status: EvidenceStatus
  observation?: Observation
}

export interface RouteEvidence {
  status: EvidenceStatus
  links: LinkEvidence[]
}

export interface StatusView {
  current_entity: string
  vantage: string
  summary: Record<EvidenceStatus, number>
  links: LinkEvidence[]
  routes: Array<{ route_id: string; evidence: RouteEvidence }>
}

export interface DocumentAssociation {
  object_id: string
  object_type: string
  ref: string
}

export interface DocumentView {
  id: string
  scope_id: string
  path: string
  title: string
  associations: DocumentAssociation[]
}

export interface DocumentContent extends DocumentView {
  format: 'markdown' | 'text'
  body: string
}

export interface DocumentationReference {
  ref: string
  title?: string
}

export interface NativeHint {
  executable: string
  args: string[]
  credential_refs?: string[]
}

export interface ResolvedBinding {
  role: string
  target: string
}

export interface ResolvedEntity {
  canonical_id: string
  scope_id: string
  kind: string
  name: string
  labels?: Record<string, string>
  documentation?: DocumentationReference[]
}

export interface ResolvedStep {
  link_id: string
  provider: string
  native_hint: NativeHint
  documentation?: DocumentationReference[]
  evidence: LinkEvidence
}

export interface ResolvedRoute {
  canonical_id: string
  derived_target: string
  derived_provides: string[]
  documentation?: DocumentationReference[]
  evidence_status: EvidenceStatus
  steps: ResolvedStep[]
}

export type ResolveStatus = 'resolved' | 'unresolved' | 'ambiguous'

export interface ResolveResult {
  status: ResolveStatus
  input_target: string
  canonical_target: string
  target_entity: ResolvedEntity
  binding?: ResolvedBinding
  capability: string
  route?: ResolvedRoute
  candidates?: ResolvedRoute[]
}

export interface ProbeResult {
  input_ref: string
  subject_type: string
  subject_id: string
  status: EvidenceStatus
  observations: Observation[]
}

export interface ValidationResult {
  valid: boolean
  active_scope: string
  entities: number
  links: number
  routes: number
}
