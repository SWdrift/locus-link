import type { Edge, Node } from '@vue-flow/core'
import ELK from 'elkjs/lib/elk-api'
import type { ElkExtendedEdge } from 'elkjs/lib/elk-api'
import ELKWorker from 'elkjs/lib/elk-worker.min.js?worker'
import type { GraphView } from '../../api'

export interface LayoutEdgeData {
  label: string
  path: string
  labelX: number
  labelY: number
  linkId: string
}

export interface GraphLayout {
  fingerprint: string
  nodes: Node[]
  edges: Edge<LayoutEdgeData>[]
}

const elk = new ELK({ workerFactory: () => new ELKWorker() })


function edgePath(edge?: ElkExtendedEdge): { path: string; x: number; y: number } {
  const sections = edge?.sections ?? []
  const path = sections.map(section => {
    const points = [section.startPoint, ...(section.bendPoints ?? []), section.endPoint]
    return points.map((point, index) => `${index ? 'L' : 'M'} ${point.x} ${point.y}`).join(' ')
  }).join(' ')
  const points = sections.flatMap(section => [section.startPoint, ...(section.bendPoints ?? []), section.endPoint])
  const x = points.length ? (Math.min(...points.map(point => point.x)) + Math.max(...points.map(point => point.x))) / 2 : 0
  const y = points.length ? (Math.min(...points.map(point => point.y)) + Math.max(...points.map(point => point.y))) / 2 : 0
  return { path, x, y }
}

export async function layoutGraph(value: GraphView): Promise<GraphLayout> {
  const entities = [...value.entities].sort((left, right) => left.canonical_id.localeCompare(right.canonical_id))
  const links = [...value.links].sort((left, right) => left.canonical_id.localeCompare(right.canonical_id))
  const fingerprint = JSON.stringify({ nodes: entities.map(item => item.canonical_id), edges: links.map(item => [item.canonical_id, item.from, item.to]) })
  const graph = await elk.layout({
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'RIGHT',
      'elk.edgeRouting': 'ORTHOGONAL',
      'elk.layered.nodePlacement.strategy': 'BRANDES_KOEPF',
      'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
      'elk.layered.spacing.nodeNodeBetweenLayers': '112',
      'elk.spacing.nodeNode': '64',
      'elk.spacing.componentComponent': '96',
      'elk.separateConnectedComponents': 'true',
      'elk.padding': '[top=48,left=48,bottom=48,right=48]',
    },
    children: entities.map(entity => ({ id: entity.canonical_id, width: 200, height: 84 })),
    edges: links.map(link => ({ id: link.canonical_id, sources: [link.from], targets: [link.to] })),
  })
  const positions = new Map((graph.children ?? []).map(node => [node.id, node]))
  const laidOutEdges = new Map((graph.edges ?? []).map(edge => [edge.id, edge]))
  return {
    fingerprint,
    nodes: entities.map(entity => {
      const point = positions.get(entity.canonical_id)
      return {
        id: entity.canonical_id,
        position: { x: point?.x ?? 0, y: point?.y ?? 0 },
        data: { label: entity.name, kind: entity.kind, scope: entity.scope_id, docs: entity.documentation_ids?.length ?? 0 },
        type: 'default',
      }
    }),
    edges: links.map(link => {
      const route = edgePath(laidOutEdges.get(link.canonical_id) as ElkExtendedEdge | undefined)
      return {
        id: link.canonical_id,
        source: link.from,
        target: link.to,
        type: 'elk',
        data: { label: link.provider, path: route.path, labelX: route.x, labelY: route.y, linkId: link.canonical_id },
      }
    }),
  }
}
