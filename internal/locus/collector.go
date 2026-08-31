package locus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gonum.org/v1/gonum/graph/path"
)

type CollectorOptions struct {
	Home  string
	Store *Store
}

type collectedCandidate struct {
	registry   *ScopeRegistry
	provenance ScopeProvenance
}

type collectedEdge struct {
	sourceScopeID string
	targetScopeID string
	targetDigest  string
	alias         string
	aliasPath     []string
	source        Source
	blocked       *BlockedImport
}

type collector struct {
	options    CollectorOptions
	graph      *scopeGraph
	candidates map[string]map[string]*collectedCandidate
	edges      []*collectedEdge
	expanded   map[string]bool
}

func CollectRegistry(root string, options CollectorOptions) (*Registry, error) {
	rootRegistry, err := LoadScopeRegistry(root, false)
	if err != nil {
		return nil, err
	}
	if options.Home == "" {
		options.Home, err = DefaultHome()
		if err != nil {
			return nil, err
		}
	}
	value := &collector{
		options: options, graph: newScopeGraph(), candidates: map[string]map[string]*collectedCandidate{}, expanded: map[string]bool{},
	}
	rootSource := Source{Kind: "directory", URI: rootRegistry.Root}
	value.addCandidate(rootRegistry, ScopeProvenance{
		ScopeID: rootRegistry.Manifest.ScopeID, ContentDigest: rootRegistry.Digest, Source: rootSource, AliasPaths: [][]string{{}},
	})
	value.graph.addScope(rootRegistry.Manifest.ScopeID)
	value.collectImports(rootRegistry, nil)
	registry, err := value.compose(rootRegistry)
	if err != nil {
		return nil, err
	}
	return registry, nil
}

func (c *collector) addCandidate(registry *ScopeRegistry, provenance ScopeProvenance) {
	scopeID := registry.Manifest.ScopeID
	byDigest := c.candidates[scopeID]
	if byDigest == nil {
		byDigest = map[string]*collectedCandidate{}
		c.candidates[scopeID] = byDigest
	}
	if current := byDigest[registry.Digest]; current != nil {
		current.provenance.AliasPaths = appendUniquePath(current.provenance.AliasPaths, provenance.AliasPaths...)
		return
	}
	byDigest[registry.Digest] = &collectedCandidate{registry: registry, provenance: provenance}
}

func (c *collector) collectImports(scope *ScopeRegistry, parentPath []string) {
	key := scope.Manifest.ScopeID + "@" + scope.Digest
	if c.expanded[key] {
		return
	}
	c.expanded[key] = true
	for _, alias := range sortedMapKeys(scope.Manifest.Imports) {
		imported := scope.Manifest.Imports[alias]
		aliasPath := append(append([]string(nil), parentPath...), alias)
		edge := &collectedEdge{
			sourceScopeID: scope.Manifest.ScopeID, alias: alias, aliasPath: aliasPath, source: sanitizeSource(imported.Source),
		}
		candidate, provenance, reason, err := c.loadImport(scope, imported, aliasPath)
		if err != nil {
			edge.blocked = &BlockedImport{SourceScopeID: scope.Manifest.ScopeID, AliasPath: aliasPath, Source: edge.source, Reason: reason}
			c.edges = append(c.edges, edge)
			continue
		}
		edge.targetScopeID = candidate.Manifest.ScopeID
		edge.targetDigest = candidate.Digest
		if imported.ExpectedScopeID != "" && imported.ExpectedScopeID != candidate.Manifest.ScopeID {
			edge.blocked = &BlockedImport{SourceScopeID: scope.Manifest.ScopeID, AliasPath: aliasPath, Source: edge.source, Reason: "scope_id_mismatch"}
			c.edges = append(c.edges, edge)
			continue
		}
		if cycle, ok := c.graph.addEdge(scope.Manifest.ScopeID, candidate.Manifest.ScopeID); !ok {
			edge.blocked = &BlockedImport{
				SourceScopeID: scope.Manifest.ScopeID, AliasPath: aliasPath, Source: edge.source, Reason: "cycle", CycleScopeIDs: cycle,
			}
			c.addCandidate(candidate, provenance)
			c.edges = append(c.edges, edge)
			continue
		}
		c.addCandidate(candidate, provenance)
		c.edges = append(c.edges, edge)
		c.collectImports(candidate, aliasPath)
	}
}

func (c *collector) loadImport(owner *ScopeRegistry, imported Import, aliasPath []string) (*ScopeRegistry, ScopeProvenance, string, error) {
	source := imported.Source
	switch source.Kind {
	case "directory":
		path, err := resolveDirectorySource(source, owner.Root, c.options.Home)
		if err != nil {
			return nil, ScopeProvenance{}, "source_unavailable", err
		}
		registry, err := LoadScopeRegistry(path, false)
		if err != nil {
			reason := "invalid_registry"
			if _, statErr := os.Stat(filepath.Join(path, "scope.yaml")); statErr != nil {
				reason = "source_unavailable"
			}
			return nil, ScopeProvenance{}, reason, err
		}
		return registry, ScopeProvenance{
			ScopeID: registry.Manifest.ScopeID, ContentDigest: registry.Digest, Source: sanitizeSource(source), AliasPaths: [][]string{aliasPath}, ObjectPath: registry.Root,
		}, "", nil
	case "git", "url":
		if c.options.Store == nil {
			return nil, ScopeProvenance{}, "missing_active_cache", errors.New("remote source has no active cache store")
		}
		entry, err := c.options.Store.SourceCacheEntry(owner.Manifest.ScopeID, imported.Alias)
		if err != nil {
			return nil, ScopeProvenance{}, "source_unavailable", err
		}
		if entry == nil || entry.ActiveContentDigest == "" || entry.ObjectPath == "" {
			return nil, ScopeProvenance{}, "missing_active_cache", errors.New("remote source has no active cache")
		}
		registryRoot := entry.ObjectPath
		registry, err := LoadScopeRegistry(registryRoot, true)
		if err != nil {
			return nil, ScopeProvenance{}, "invalid_registry", err
		}
		if registry.Digest != entry.ActiveContentDigest || entry.ActualScopeID != "" && registry.Manifest.ScopeID != entry.ActualScopeID {
			return nil, ScopeProvenance{}, "invalid_registry", errors.New("active cache metadata does not match immutable object")
		}
		if imported.ExpectedScopeID != "" && registry.Manifest.ScopeID != imported.ExpectedScopeID {
			return nil, ScopeProvenance{}, "scope_id_mismatch", errors.New("active cache scope_id does not match import")
		}
		return registry, ScopeProvenance{
			ScopeID: registry.Manifest.ScopeID, ContentDigest: registry.Digest, Source: sanitizeSource(source),
			ResolvedRevision: entry.ResolvedRevision, ObjectPath: entry.ObjectPath, AliasPaths: [][]string{aliasPath},
		}, "", nil
	default:
		return nil, ScopeProvenance{}, "source_unavailable", fmt.Errorf("unsupported source kind %q", source.Kind)
	}
}

func (c *collector) compose(root *ScopeRegistry) (*Registry, error) {
	chosen := map[string]*collectedCandidate{}
	conflicted := map[string]bool{}
	authorityOnly := map[string]bool{}
	for scopeID, byDigest := range c.candidates {
		if scopeID == root.Manifest.ScopeID {
			chosen[scopeID] = byDigest[root.Digest]
			conflicted[scopeID] = len(byDigest) > 1
			continue
		}
		if c.options.Store != nil {
			authority, err := c.options.Store.ScopeAuthority(scopeID)
			if err != nil {
				return nil, err
			}
			if authority != nil {
				conflicted[scopeID] = len(byDigest) > 1
				if candidate := byDigest[authority.ActiveContentDigest]; candidate != nil {
					chosen[scopeID] = candidate
					continue
				}
				authorityRegistry, loadErr := LoadScopeRegistry(authority.ObjectPath, true)
				if loadErr == nil && authorityRegistry.Manifest.ScopeID == scopeID && authorityRegistry.Digest == authority.ActiveContentDigest {
					chosen[scopeID] = &collectedCandidate{
						registry: authorityRegistry,
						provenance: ScopeProvenance{
							ScopeID: scopeID, ContentDigest: authorityRegistry.Digest, Source: authority.Provenance,
							ObjectPath: authority.ObjectPath,
						},
					}
					authorityOnly[scopeID] = true
					conflicted[scopeID] = true
				}
				continue
			}
		}
		if len(byDigest) == 1 {
			for _, candidate := range byDigest {
				chosen[scopeID] = candidate
			}
			continue
		}
		conflicted[scopeID] = true
	}
	if chosen[root.Manifest.ScopeID] == nil {
		chosen[root.Manifest.ScopeID] = c.candidates[root.Manifest.ScopeID][root.Digest]
	}
	finalGraph := newScopeGraph()
	for scopeID := range chosen {
		if chosen[scopeID] != nil {
			finalGraph.addScope(scopeID)
		}
	}
	for _, edge := range c.edges {
		if edge.blocked != nil || chosen[edge.sourceScopeID] == nil || chosen[edge.targetScopeID] == nil {
			continue
		}
		if conflicted[edge.targetScopeID] && chosen[edge.targetScopeID].registry.Digest != edge.targetDigest && !authorityOnly[edge.targetScopeID] {
			continue
		}
		finalGraph.addEdge(edge.sourceScopeID, edge.targetScopeID)
	}
	if err := finalGraph.assertAcyclic(); err != nil {
		return nil, err
	}
	reachable := map[string]bool{root.Manifest.ScopeID: true}
	rootNode := finalGraph.nodes[root.Manifest.ScopeID]
	for scopeID, node := range finalGraph.nodes {
		if scopeID == root.Manifest.ScopeID {
			continue
		}
		graphPath, _ := path.DijkstraFromTo(rootNode, node, finalGraph.graph)
		reachable[scopeID] = len(graphPath) != 0
	}
	registry := newRegistry(root)
	for scopeID, candidate := range chosen {
		if candidate == nil || !reachable[scopeID] {
			continue
		}
		provenance := candidate.provenance
		provenance.AliasPaths = nil
		if scopeID == root.Manifest.ScopeID {
			provenance.AliasPaths = [][]string{{}}
		}
		registry.addScope(candidate.registry, provenance)
		registry.scopeAliases[scopeID] = map[string]string{}
	}
	for _, edge := range c.edges {
		if !reachable[edge.sourceScopeID] {
			continue
		}
		if edge.blocked != nil {
			registry.BlockedImports = append(registry.BlockedImports, *edge.blocked)
			continue
		}
		target := chosen[edge.targetScopeID]
		if target == nil || !reachable[edge.targetScopeID] || (conflicted[edge.targetScopeID] && target.registry.Digest != edge.targetDigest) {
			registry.BlockedImports = append(registry.BlockedImports, BlockedImport{
				SourceScopeID: edge.sourceScopeID, AliasPath: edge.aliasPath, Source: edge.source, Reason: "authority_conflict",
			})
			continue
		}
		registry.scopeAliases[edge.sourceScopeID][edge.alias] = edge.targetScopeID
		aliasKey := strings.Join(edge.aliasPath, "::")
		if previous := registry.Aliases[aliasKey]; previous != "" && previous != edge.targetScopeID {
			delete(registry.Aliases, aliasKey)
			registry.BlockedImports = append(registry.BlockedImports, BlockedImport{
				SourceScopeID: edge.sourceScopeID, AliasPath: edge.aliasPath, Source: edge.source, Reason: "alias_conflict",
			})
			continue
		}
		registry.Aliases[aliasKey] = edge.targetScopeID
		registry.AliasPaths[edge.targetScopeID] = appendUniquePath(registry.AliasPaths[edge.targetScopeID], edge.aliasPath)
		provenance := registry.Provenance[edge.targetScopeID]
		provenance.AliasPaths = appendUniquePath(provenance.AliasPaths, edge.aliasPath)
		registry.Provenance[edge.targetScopeID] = provenance
		registry.ImportEdges = append(registry.ImportEdges, ImportEdge{
			SourceScopeID: edge.sourceScopeID, TargetScopeID: edge.targetScopeID, Alias: edge.alias,
			AliasPath: append([]string(nil), edge.aliasPath...), Source: edge.source, ContentDigest: edge.targetDigest,
		})
	}
	registry.AliasPaths[root.Manifest.ScopeID] = [][]string{{}}
	sort.Slice(registry.ImportEdges, func(i, j int) bool {
		return strings.Join(registry.ImportEdges[i].AliasPath, "::") < strings.Join(registry.ImportEdges[j].AliasPath, "::")
	})
	sort.Slice(registry.BlockedImports, func(i, j int) bool {
		left := registry.BlockedImports[i].SourceScopeID + "\x00" + strings.Join(registry.BlockedImports[i].AliasPath, "::") + "\x00" + registry.BlockedImports[i].Reason
		right := registry.BlockedImports[j].SourceScopeID + "\x00" + strings.Join(registry.BlockedImports[j].AliasPath, "::") + "\x00" + registry.BlockedImports[j].Reason
		return left < right
	})
	if len(registry.BlockedImports) != 0 {
		registry.Completeness = Partial
	}
	if err := registry.validateComplete(); err != nil {
		return nil, err
	}
	return registry, nil
}

func appendUniquePath(paths [][]string, additions ...[]string) [][]string {
	for _, addition := range additions {
		key := strings.Join(addition, "\x00")
		found := false
		for _, current := range paths {
			if strings.Join(current, "\x00") == key {
				found = true
				break
			}
		}
		if !found {
			paths = append(paths, append([]string(nil), addition...))
		}
	}
	sort.Slice(paths, func(i, j int) bool { return strings.Join(paths[i], "\x00") < strings.Join(paths[j], "\x00") })
	return paths
}
