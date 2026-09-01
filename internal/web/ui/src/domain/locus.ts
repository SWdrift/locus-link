export type EvidenceStatus = 'success' | 'failure' | 'stale' | 'unknown'
export type Completeness = 'complete' | 'partial'
export type ScopeKind = 'user' | 'project' | 'remote' | 'root'
export type ScopeAvailability = 'available' | 'missing' | 'invalid' | 'identity_mismatch'

export interface Source {
  kind: string
  uri: string
  revision?: string
}

export interface BlockedImport {
  source_scope_id: string
  target_scope_id?: string
  alias_path: string[]
  source: Source
  reason: string
  cycle_scope_ids?: string[]
}

export interface ViewDiagnostics {
  completeness: Completeness
  blocked_imports: BlockedImport[]
}

export interface Scope {
  id: string
  content_digest: string
  source: Source
  resolved_revision?: string
  alias_paths: string[][]
}

export interface ImportRef {
  alias: string
  scope_id: string
}

export interface ImportEdge {
  source_scope_id: string
  target_scope_id: string
  alias: string
  alias_path: string[]
  source: Source
  content_digest?: string
}

export interface BindingView {
  canonical_id: string
  scope_id: string
  role: string
  target: string
}

export interface ProjectRegistration {
  scope_id: string
  registry_path: string
  registered_at: string
  available: boolean
}

export interface SourceCacheEntry {
  owner_scope_id: string
  import_alias: string
  configured_source_digest: string
  current_source_digest?: string
  configuration_changed: boolean
  expected_scope_id?: string
  actual_scope_id?: string
  active_content_digest?: string
  resolved_revision?: string
  object_path?: string
  etag?: string
  last_modified?: string
  last_refresh_status: string
  last_refresh_error?: string
  refreshed_at: string
}

export interface LocusScopeEntry {
  scope_id: string
  kind: 'user' | 'project'
  registry_path: string
  registered_at?: string
  availability: ScopeAvailability
  openable: boolean
  active: boolean
}

export interface DependencyNode {
  scope_id: string
  content_digest: string
  source: Source
  resolved_revision?: string
  alias_paths: string[][]
  root: boolean
  kind: ScopeKind
  openable: boolean
  availability: ScopeAvailability
}

export interface DependencyEdge {
  source_scope_id: string
  target_scope_id?: string
  alias: string
  alias_path: string[]
  source: Source
  content_digest?: string
  status: 'active' | 'blocked'
  reason?: string
  cycle_scope_ids?: string[]
}

export interface DependencySnapshot {
  root_scope_id: string
  root_digest: string
  snapshot_digest: string
  collected_at: string
  completeness: Completeness
  nodes: DependencyNode[]
  edges: DependencyEdge[]
  blocked_imports: BlockedImport[]
}

export interface DependencyNodeChange {
  scope_id: string
  change: 'added' | 'removed' | 'digest_changed' | 'availability_changed'
  before_digest?: string
  after_digest?: string
}

export interface DependencyEdgeChange {
  source_scope_id: string
  alias: string
  change: 'added' | 'removed' | 'target_changed' | 'source_changed' | 'status_changed'
  before?: DependencyEdge
  after?: DependencyEdge
}

export interface DependencyDiff {
  nodes: DependencyNodeChange[]
  edges: DependencyEdgeChange[]
  completeness_changed: boolean
  new_blocked_imports: BlockedImport[]
  resolved_blocked_imports: BlockedImport[]
  requires_confirmation: boolean
}

export interface RefreshResult {
  status: 'success' | 'partial' | 'failure' | 'confirmation_required'
  activated: Array<{ owner_scope_id: string; alias_path: string[]; scope_id: string; content_digest: string }>
  retained: Array<{ owner_scope_id: string; alias_path: string[]; scope_id: string; content_digest: string }>
  refresh_errors: Array<{ owner_scope_id: string; alias_path: string[]; reason: string }>
  completeness: Completeness
  blocked_imports: BlockedImport[]
  active_snapshot?: DependencySnapshot
  candidate_snapshot?: DependencySnapshot
  diff?: DependencyDiff
}

export interface RootContext {
  root_origin: string
  registry_path: string
  user_registry_path: string
  registered: boolean
  registration?: ProjectRegistration
  has_user_import: boolean
  source_cache?: SourceCacheEntry[]
}

export interface RuntimeContext {
  cwd: string
  current_entity?: string
  available_tools: string[]
  vantage: string
  mechanism_bindings_source?: string
}

export interface LocusContext extends ViewDiagnostics {
  active_scope: { id: string }
  root: RootContext
  imports: ImportRef[]
  import_edges: ImportEdge[]
  bindings: Record<string, string>
  binding_details: BindingView[]
  runtime: RuntimeContext
  observation_store: string
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

export interface GraphView extends ViewDiagnostics {
  scopes: Scope[]
  import_edges: ImportEdge[]
  bindings: BindingView[]
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
  provider: string
  observation?: Observation
}

export interface RouteEvidence {
  status: EvidenceStatus
  links: LinkEvidence[]
}

export interface StatusView extends ViewDiagnostics {
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
  canonical_id: string
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

export type ResolveStatus = 'resolved' | 'unresolved' | 'ambiguous' | 'incomplete'

export interface ResolveResult extends ViewDiagnostics {
  status: ResolveStatus
  input_target: string
  canonical_target?: string
  target_entity?: ResolvedEntity
  binding?: ResolvedBinding
  capability: string
  route?: ResolvedRoute
  candidates: ResolvedRoute[]
}

export interface ProbeResult extends ViewDiagnostics {
  input_ref: string
  subject_type: string
  subject_id: string
  status: EvidenceStatus
  observations: Observation[]
}

export interface ValidationResult extends ViewDiagnostics {
  valid: boolean
  active_scope: string
  entities: number
  links: number
  routes: number
}
