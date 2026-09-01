package locus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type DependencyNode struct {
	ScopeID          string     `json:"scope_id"`
	ContentDigest    string     `json:"content_digest"`
	Source           Source     `json:"source"`
	ResolvedRevision string     `json:"resolved_revision,omitempty"`
	AliasPaths       [][]string `json:"alias_paths"`
	Root             bool       `json:"root"`
	Kind             string     `json:"kind"`
	Openable         bool       `json:"openable"`
	Availability     string     `json:"availability"`
}

type DependencyEdge struct {
	SourceScopeID string   `json:"source_scope_id"`
	TargetScopeID string   `json:"target_scope_id,omitempty"`
	Alias         string   `json:"alias"`
	AliasPath     []string `json:"alias_path"`
	Source        Source   `json:"source"`
	ContentDigest string   `json:"content_digest,omitempty"`
	Status        string   `json:"status"`
	Reason        string   `json:"reason,omitempty"`
	CycleScopeIDs []string `json:"cycle_scope_ids,omitempty"`
}

type DependencySnapshot struct {
	RootScopeID    string           `json:"root_scope_id"`
	RootDigest     string           `json:"root_digest"`
	SnapshotDigest string           `json:"snapshot_digest"`
	CollectedAt    time.Time        `json:"collected_at"`
	Completeness   Completeness     `json:"completeness"`
	Nodes          []DependencyNode `json:"nodes"`
	Edges          []DependencyEdge `json:"edges"`
	BlockedImports []BlockedImport  `json:"blocked_imports"`
}

type DependencyNodeChange struct {
	ScopeID      string `json:"scope_id"`
	Change       string `json:"change"`
	BeforeDigest string `json:"before_digest,omitempty"`
	AfterDigest  string `json:"after_digest,omitempty"`
}

type DependencyEdgeChange struct {
	SourceScopeID string          `json:"source_scope_id"`
	Alias         string          `json:"alias"`
	Change        string          `json:"change"`
	Before        *DependencyEdge `json:"before,omitempty"`
	After         *DependencyEdge `json:"after,omitempty"`
}

type DependencyDiff struct {
	Nodes                []DependencyNodeChange `json:"nodes"`
	Edges                []DependencyEdgeChange `json:"edges"`
	CompletenessChanged  bool                   `json:"completeness_changed"`
	NewBlockedImports    []BlockedImport        `json:"new_blocked_imports"`
	ResolvedBlocked      []BlockedImport        `json:"resolved_blocked_imports"`
	RequiresConfirmation bool                   `json:"requires_confirmation"`
}

func SnapshotDependency(registry *Registry) DependencySnapshot {
	snapshot := DependencySnapshot{
		RootScopeID: registry.RootScopeID, CollectedAt: time.Now().UTC(), Completeness: registry.Completeness,
		Nodes: []DependencyNode{}, Edges: []DependencyEdge{}, BlockedImports: append([]BlockedImport{}, registry.BlockedImports...),
	}
	for scopeID, provenance := range registry.Provenance {
		node := DependencyNode{
			ScopeID: scopeID, ContentDigest: provenance.ContentDigest, Source: provenance.Source,
			ResolvedRevision: provenance.ResolvedRevision, AliasPaths: clonePaths(provenance.AliasPaths),
			Root: scopeID == registry.RootScopeID, Kind: "remote", Availability: "available",
		}
		if node.Root {
			node.Kind = "root"
			snapshot.RootDigest = node.ContentDigest
		}
		snapshot.Nodes = append(snapshot.Nodes, node)
	}
	for _, edge := range registry.ImportEdges {
		snapshot.Edges = append(snapshot.Edges, DependencyEdge{
			SourceScopeID: edge.SourceScopeID, TargetScopeID: edge.TargetScopeID, Alias: edge.Alias,
			AliasPath: append([]string{}, edge.AliasPath...), Source: edge.Source,
			ContentDigest: edge.ContentDigest, Status: "active",
		})
	}
	for _, blocked := range registry.BlockedImports {
		alias := ""
		if len(blocked.AliasPath) != 0 {
			alias = blocked.AliasPath[len(blocked.AliasPath)-1]
		}
		snapshot.Edges = append(snapshot.Edges, DependencyEdge{
			SourceScopeID: blocked.SourceScopeID, TargetScopeID: blocked.TargetScopeID, Alias: alias,
			AliasPath: append([]string{}, blocked.AliasPath...), Source: blocked.Source,
			Status: "blocked", Reason: blocked.Reason, CycleScopeIDs: append([]string{}, blocked.CycleScopeIDs...),
		})
	}
	sort.Slice(snapshot.Nodes, func(i, j int) bool { return snapshot.Nodes[i].ScopeID < snapshot.Nodes[j].ScopeID })
	sort.Slice(snapshot.Edges, func(i, j int) bool {
		return dependencyEdgeKey(snapshot.Edges[i]) < dependencyEdgeKey(snapshot.Edges[j])
	})
	snapshot.SnapshotDigest = dependencySnapshotDigest(snapshot)
	return snapshot
}

func dependencySnapshotDigest(snapshot DependencySnapshot) string {
	snapshot.SnapshotDigest = ""
	snapshot.CollectedAt = time.Time{}
	payload, _ := json.Marshal(snapshot)
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func DiffDependencies(active, candidate DependencySnapshot) DependencyDiff {
	result := DependencyDiff{Nodes: []DependencyNodeChange{}, Edges: []DependencyEdgeChange{}, NewBlockedImports: []BlockedImport{}, ResolvedBlocked: []BlockedImport{}}
	activeNodes, candidateNodes := dependencyNodesByScope(active.Nodes), dependencyNodesByScope(candidate.Nodes)
	for _, scopeID := range unionKeys(activeNodes, candidateNodes) {
		before, beforeOK := activeNodes[scopeID]
		after, afterOK := candidateNodes[scopeID]
		change := "unchanged"
		switch {
		case !beforeOK:
			change = "added"
		case !afterOK:
			change = "removed"
		case before.ContentDigest != after.ContentDigest:
			change = "digest_changed"
		case before.Availability != after.Availability:
			change = "availability_changed"
		}
		if change != "unchanged" {
			result.Nodes = append(result.Nodes, DependencyNodeChange{ScopeID: scopeID, Change: change, BeforeDigest: before.ContentDigest, AfterDigest: after.ContentDigest})
		}
	}
	activeEdges, candidateEdges := dependencyEdgesByIdentity(active.Edges), dependencyEdgesByIdentity(candidate.Edges)
	for _, key := range unionKeys(activeEdges, candidateEdges) {
		before, beforeOK := activeEdges[key]
		after, afterOK := candidateEdges[key]
		change := "unchanged"
		switch {
		case !beforeOK:
			change = "added"
		case !afterOK:
			change = "removed"
		case before.TargetScopeID != after.TargetScopeID:
			change = "target_changed"
		case sourceDigest(before.Source) != sourceDigest(after.Source):
			change = "source_changed"
		case before.Status != after.Status || before.Reason != after.Reason:
			change = "status_changed"
		}
		if change != "unchanged" {
			item := DependencyEdgeChange{SourceScopeID: after.SourceScopeID, Alias: after.Alias, Change: change}
			if !afterOK {
				item.SourceScopeID, item.Alias = before.SourceScopeID, before.Alias
			}
			if beforeOK {
				copy := before
				item.Before = &copy
			}
			if afterOK {
				copy := after
				item.After = &copy
			}
			result.Edges = append(result.Edges, item)
		}
	}
	activeBlocked, candidateBlocked := blockedByIdentity(active.BlockedImports), blockedByIdentity(candidate.BlockedImports)
	for _, key := range unionKeys(activeBlocked, candidateBlocked) {
		before, beforeOK := activeBlocked[key]
		after, afterOK := candidateBlocked[key]
		if !beforeOK {
			result.NewBlockedImports = append(result.NewBlockedImports, after)
		}
		if !afterOK {
			result.ResolvedBlocked = append(result.ResolvedBlocked, before)
		}
	}
	result.CompletenessChanged = active.Completeness != candidate.Completeness
	result.RequiresConfirmation = len(result.NewBlockedImports) != 0 || active.Completeness == Complete && candidate.Completeness == Partial
	return result
}

func clonePaths(paths [][]string) [][]string {
	result := make([][]string, len(paths))
	for index := range paths {
		result[index] = append([]string{}, paths[index]...)
	}
	return result
}

func dependencyEdgeKey(edge DependencyEdge) string {
	return edge.SourceScopeID + "\x00" + edge.Alias + "\x00" + strings.Join(edge.AliasPath, "\x00")
}

func dependencyEdgeIdentity(edge DependencyEdge) string {
	return edge.SourceScopeID + "\x00" + edge.Alias
}

func dependencyNodesByScope(values []DependencyNode) map[string]DependencyNode {
	result := make(map[string]DependencyNode, len(values))
	for _, value := range values {
		result[value.ScopeID] = value
	}
	return result
}

func dependencyEdgesByIdentity(values []DependencyEdge) map[string]DependencyEdge {
	result := make(map[string]DependencyEdge, len(values))
	for _, value := range values {
		result[dependencyEdgeIdentity(value)] = value
	}
	return result
}

func blockedByIdentity(values []BlockedImport) map[string]BlockedImport {
	result := make(map[string]BlockedImport, len(values))
	for _, value := range values {
		key := value.SourceScopeID + "\x00" + strings.Join(value.AliasPath, "\x00") + "\x00" + value.Reason
		result[key] = value
	}
	return result
}

func unionKeys[T any](left, right map[string]T) []string {
	seen := make(map[string]bool, len(left)+len(right))
	for key := range left {
		seen[key] = true
	}
	for key := range right {
		seen[key] = true
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
