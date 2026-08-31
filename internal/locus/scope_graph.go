package locus

import (
	"fmt"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

type scopeGraph struct {
	graph      *simple.DirectedGraph
	nodes      map[string]graph.Node
	scopeIDs   map[int64]string
	nextNodeID int64
}

func newScopeGraph() *scopeGraph {
	return &scopeGraph{
		graph: simple.NewDirectedGraph(), nodes: map[string]graph.Node{}, scopeIDs: map[int64]string{},
	}
}

func (g *scopeGraph) addScope(scopeID string) graph.Node {
	if node := g.nodes[scopeID]; node != nil {
		return node
	}
	g.nextNodeID++
	node := simple.Node(g.nextNodeID)
	g.graph.AddNode(node)
	g.nodes[scopeID] = node
	g.scopeIDs[node.ID()] = scopeID
	return node
}

func (g *scopeGraph) addEdge(sourceScopeID, targetScopeID string) ([]string, bool) {
	source := g.addScope(sourceScopeID)
	target := g.addScope(targetScopeID)
	if source.ID() == target.ID() {
		return []string{sourceScopeID, targetScopeID}, false
	}
	cyclePath, _ := path.DijkstraFromTo(target, source, g.graph)
	if len(cyclePath) != 0 {
		cycle := make([]string, 1, len(cyclePath)+1)
		cycle[0] = sourceScopeID
		for _, node := range cyclePath {
			cycle = append(cycle, g.scopeIDs[node.ID()])
		}
		return cycle, false
	}
	g.graph.SetEdge(simple.Edge{F: source, T: target})
	return nil, true
}

func (g *scopeGraph) assertAcyclic() error {
	for _, component := range topo.TarjanSCC(g.graph) {
		if len(component) > 1 {
			return fmt.Errorf("active Scope graph contains SCC of %d nodes", len(component))
		}
		if len(component) == 1 && g.graph.HasEdgeFromTo(component[0].ID(), component[0].ID()) {
			return fmt.Errorf("active Scope graph contains self-loop for %s", g.scopeIDs[component[0].ID()])
		}
	}
	return nil
}
