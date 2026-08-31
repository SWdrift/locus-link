export type GraphSelection =
  | { kind: 'entity'; id: string }
  | { kind: 'link'; id: string }

export interface LayoutNodeData {
  label: string
  kind: string
  scope: string
  canonicalId: string
  documentationCount: number
}
