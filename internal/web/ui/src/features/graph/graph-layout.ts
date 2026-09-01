import type { Edge, Node } from '@vue-flow/core'
import ELK from 'elkjs/lib/elk-api'
import ELKWorker from 'elkjs/lib/elk-worker.min.js?worker'
import type { GraphView } from '../../domain/locus'
import type { LayoutNodeData } from './graph-types'

export interface LayoutEdgeData {
  label: string
}

export interface GraphLayout {
  fingerprint: string
  nodes: Node<LayoutNodeData>[]
  edges: Edge<LayoutEdgeData>[]
}

const elk = new ELK({ workerFactory: () => new ELKWorker() })
export const GRAPH_NODE_WIDTH = 196
export const GRAPH_NODE_HEIGHT = 76

interface LayeredEdge {
  id: string
  source: string
  target: string
}

const layeredLayoutOptions = {
  'elk.algorithm': 'layered',
  'elk.direction': 'RIGHT',
  'elk.edgeRouting': 'ORTHOGONAL',
  'elk.layered.nodePlacement.strategy': 'BRANDES_KOEPF',
  'elk.layered.nodePlacement.bk.fixedAlignment': 'BALANCED',
  'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
  'elk.layered.crossingMinimization.greedySwitch.type': 'TWO_SIDED',
  'elk.layered.spacing.nodeNodeBetweenLayers': '88',
  'elk.spacing.nodeNode': '44',
  'elk.spacing.componentComponent': '72',
  'elk.separateConnectedComponents': 'true',
  'elk.padding': '[top=36,left=36,bottom=36,right=36]',
}

export async function layoutLayeredGraph(nodeIDs: string[], edges: LayeredEdge[]) {
  const graph = await elk.layout({
    id: 'root',
    layoutOptions: layeredLayoutOptions,
    children: nodeIDs.map(id => ({ id, width: GRAPH_NODE_WIDTH, height: GRAPH_NODE_HEIGHT })),
    edges: edges.map(edge => ({ id: edge.id, sources: [edge.source], targets: [edge.target] })),
  })
  return new Map((graph.children ?? []).map(node => [node.id, { x: node.x ?? 0, y: node.y ?? 0 }]))
}
export async function layoutGraph(value: GraphView): Promise<GraphLayout> {
  const entities = [...value.entities].sort((left, right) => left.canonical_id.localeCompare(right.canonical_id))
  const links = [...value.links].sort((left, right) => left.canonical_id.localeCompare(right.canonical_id))
  const fingerprint = JSON.stringify({
    nodes: entities.map(item => item.canonical_id),
    edges: links.map(item => [item.canonical_id, item.from, item.to]),
  })
  const positions = await layoutLayeredGraph(
    entities.map(entity => entity.canonical_id),
    links.map(link => ({ id: link.canonical_id, source: link.from, target: link.to })),
  )
  return {
    fingerprint,
    nodes: entities.map(entity => {
      const point = positions.get(entity.canonical_id)
      return {
        id: entity.canonical_id,
        position: { x: point?.x ?? 0, y: point?.y ?? 0 },
        style: { width: `${GRAPH_NODE_WIDTH}px`, height: `${GRAPH_NODE_HEIGHT}px` },
        data: {
          label: entity.name,
          kind: entity.kind,
          scope: entity.scope_id,
          canonicalId: entity.canonical_id,
          documentationCount: entity.documentation_ids?.length ?? 0,
        },
        type: 'default',
      }
    }),
    edges: links.map(link => ({
      id: link.canonical_id,
      source: link.from,
      target: link.to,
      type: 'elk',
      data: { label: link.provider },
    })),
  }
}
