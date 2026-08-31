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


export async function layoutGraph(value: GraphView): Promise<GraphLayout> {
  const entities = [...value.entities].sort((left, right) => left.canonical_id.localeCompare(right.canonical_id))
  const links = [...value.links].sort((left, right) => left.canonical_id.localeCompare(right.canonical_id))
  const fingerprint = JSON.stringify({ nodes: entities.map(item => item.canonical_id), edges: links.map(item => [item.canonical_id, item.from, item.to]) })
  const graph = await elk.layout({
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'RIGHT',
      'elk.layered.nodePlacement.strategy': 'BRANDES_KOEPF',
      'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
      'elk.layered.spacing.nodeNodeBetweenLayers': '88',
      'elk.spacing.nodeNode': '44',
      'elk.spacing.componentComponent': '72',
      'elk.separateConnectedComponents': 'true',
      'elk.padding': '[top=36,left=36,bottom=36,right=36]',
    },
    children: entities.map(entity => ({ id: entity.canonical_id, width: 196, height: 76 })),
    edges: links.map(link => ({ id: link.canonical_id, sources: [link.from], targets: [link.to] })),
  })
  const positions = new Map((graph.children ?? []).map(node => [node.id, node]))
  return {
    fingerprint,
    nodes: entities.map(entity => {
      const point = positions.get(entity.canonical_id)
      return {
        id: entity.canonical_id,
        position: { x: point?.x ?? 0, y: point?.y ?? 0 },
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
