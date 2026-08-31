package locus

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Completeness string

const (
	Complete Completeness = "complete"
	Partial  Completeness = "partial"
)

type BlockedImport struct {
	SourceScopeID string   `json:"source_scope_id"`
	AliasPath     []string `json:"alias_path"`
	Source        Source   `json:"source"`
	Reason        string   `json:"reason"`
	CycleScopeIDs []string `json:"cycle_scope_ids,omitempty"`
}

type ScopeProvenance struct {
	ScopeID          string     `json:"scope_id"`
	ContentDigest    string     `json:"content_digest"`
	Source           Source     `json:"source"`
	ResolvedRevision string     `json:"resolved_revision,omitempty"`
	ObjectPath       string     `json:"object_path,omitempty"`
	AliasPaths       [][]string `json:"alias_paths"`
}

type ImportEdge struct {
	SourceScopeID string   `json:"source_scope_id"`
	TargetScopeID string   `json:"target_scope_id"`
	Alias         string   `json:"alias"`
	AliasPath     []string `json:"alias_path"`
	Source        Source   `json:"source"`
	ContentDigest string   `json:"content_digest,omitempty"`
}

type Registry struct {
	Root            string                     `json:"-"`
	RootScopeID     string                     `json:"root_scope_id"`
	Manifest        Manifest                   `json:"manifest"`
	Scopes          map[string]Manifest        `json:"scopes"`
	Aliases         map[string]string          `json:"aliases"`
	AliasPaths      map[string][][]string      `json:"alias_paths"`
	Bindings        map[string]*Binding        `json:"bindings"`
	Entities        map[string]*Entity         `json:"entities"`
	Links           map[string]*Link           `json:"links"`
	Routes          map[string]*Route          `json:"routes"`
	Provenance      map[string]ScopeProvenance `json:"scope_provenance"`
	ImportEdges     []ImportEdge               `json:"import_edges"`
	Completeness    Completeness               `json:"completeness"`
	BlockedImports  []BlockedImport            `json:"blocked_imports"`
	local           map[string]map[string]string
	scopeAliases    map[string]map[string]string
	sourceDigests   map[string]string
	sources         map[string]string
	scopeRegistries map[string]*ScopeRegistry
}

func newRegistry(root *ScopeRegistry) *Registry {
	return &Registry{
		Root: root.Root, RootScopeID: root.Manifest.ScopeID, Manifest: root.Manifest,
		Scopes: map[string]Manifest{}, Aliases: map[string]string{}, AliasPaths: map[string][][]string{},
		Bindings: map[string]*Binding{}, Entities: map[string]*Entity{}, Links: map[string]*Link{}, Routes: map[string]*Route{},
		Provenance: map[string]ScopeProvenance{}, ImportEdges: []ImportEdge{}, BlockedImports: []BlockedImport{}, Completeness: Complete,
		local: map[string]map[string]string{}, scopeAliases: map[string]map[string]string{},
		sourceDigests: map[string]string{}, sources: map[string]string{}, scopeRegistries: map[string]*ScopeRegistry{},
	}
}

func (r *Registry) addScope(scope *ScopeRegistry, provenance ScopeProvenance) {
	scopeID := scope.Manifest.ScopeID
	r.Scopes[scopeID] = scope.Manifest
	r.Provenance[scopeID] = provenance
	r.scopeRegistries[scopeID] = scope
	r.local[scopeID] = make(map[string]string, len(scope.local))
	for localID, canonicalID := range scope.local {
		r.local[scopeID][localID] = canonicalID
	}
	for id, value := range scope.Bindings {
		copy := *value
		r.Bindings[id] = &copy
		r.sourceDigests[id] = scope.Digest
		r.sources[id] = scope.sources[id]
	}
	for id, value := range scope.Entities {
		copy := *value
		r.Entities[id] = &copy
		r.sourceDigests[id] = scope.Digest
		r.sources[id] = scope.sources[id]
	}
	for id, value := range scope.Links {
		copy := *value
		r.Links[id] = &copy
		r.sourceDigests[id] = scope.Digest
		r.sources[id] = scope.sources[id]
	}
	for id, value := range scope.Routes {
		copy := *value
		copy.Steps = append([]RouteStep(nil), value.Steps...)
		r.Routes[id] = &copy
		r.sourceDigests[id] = scope.Digest
		r.sources[id] = scope.sources[id]
	}
}

func (r *Registry) rootBindings() map[string]string {
	values := map[string]string{}
	for _, binding := range r.Bindings {
		if binding.ScopeID == r.RootScopeID {
			values[binding.ID] = binding.Target
		}
	}
	return values
}

func (r *Registry) resolveRef(scopeID, ref, expected string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("unknown %s reference %q", expected, ref)
	}
	parts := strings.Split(ref, "::")
	candidate := ""
	switch {
	case len(parts) == 1:
		candidate = canonical(scopeID, parts[0])
	case len(parts) == 2 && r.Scopes[parts[0]].ScopeID == parts[0]:
		candidate = canonical(parts[0], parts[1])
	default:
		current := scopeID
		for _, alias := range parts[:len(parts)-1] {
			next := r.scopeAliases[current][alias]
			if next == "" {
				return "", fmt.Errorf("unknown %s reference %q", expected, ref)
			}
			current = next
		}
		candidate = canonical(current, parts[len(parts)-1])
	}
	if !r.exists(candidate, expected) {
		return "", fmt.Errorf("unknown %s reference %q", expected, ref)
	}
	return candidate, nil
}

func (r *Registry) exists(id, kind string) bool {
	switch kind {
	case "binding":
		_, ok := r.Bindings[id]
		return ok
	case "entity":
		_, ok := r.Entities[id]
		return ok
	case "link":
		_, ok := r.Links[id]
		return ok
	case "route":
		_, ok := r.Routes[id]
		return ok
	}
	return false
}

func (r *Registry) Validate() []string {
	var issues []string
	partial := r.Completeness == Partial
	for id, binding := range r.Bindings {
		target, err := r.resolveRef(binding.ScopeID, binding.Target, "entity")
		if err != nil {
			if partial {
				delete(r.Bindings, id)
				continue
			}
			issues = append(issues, id+": "+err.Error())
			continue
		}
		binding.Target = target
	}
	for id, link := range r.Links {
		from, fromErr := r.resolveRef(link.ScopeID, link.From, "entity")
		to, toErr := r.resolveRef(link.ScopeID, link.To, "entity")
		if fromErr != nil || toErr != nil {
			if partial {
				delete(r.Links, id)
				continue
			}
			if fromErr != nil {
				issues = append(issues, id+": "+fromErr.Error())
			}
			if toErr != nil {
				issues = append(issues, id+": "+toErr.Error())
			}
			continue
		}
		link.From, link.To = from, to
	}
	for id, route := range r.Routes {
		available := map[string]bool{}
		valid := true
		for index := range route.Steps {
			linkID, err := r.resolveRef(route.ScopeID, route.Steps[index].Link, "link")
			if err != nil {
				if partial {
					valid = false
					break
				}
				issues = append(issues, id+": "+err.Error())
				valid = false
				continue
			}
			route.Steps[index].Link = linkID
			link := r.Links[linkID]
			for _, required := range link.Requires {
				if !available[required] {
					issues = append(issues, fmt.Sprintf("%s: step %s requires unavailable capability %s", id, linkID, required))
				}
			}
			for _, provided := range link.Provides {
				available[provided] = true
			}
		}
		if !valid {
			delete(r.Routes, id)
		}
	}
	sort.Strings(issues)
	return issues
}

func (r *Registry) ResolveEntity(ref string) (string, error) {
	if bindingID, err := r.resolveRef(r.RootScopeID, ref, "binding"); err == nil {
		return r.Bindings[bindingID].Target, nil
	}
	return r.resolveRef(r.RootScopeID, ref, "entity")
}

func (r *Registry) ResolveAny(ref string) (string, string, error) {
	for _, kind := range []string{"binding", "entity", "link", "route"} {
		if value, err := r.resolveRef(r.RootScopeID, ref, kind); err == nil {
			return value, kind, nil
		}
	}
	return "", "", fmt.Errorf("unknown object reference %q", ref)
}

func (r *Registry) ObjectIDs(kind string) []string {
	ids := []string{}
	if kind == "" || kind == "binding" {
		for id := range r.Bindings {
			ids = append(ids, id)
		}
	}
	if kind == "" || kind == "entity" {
		for id := range r.Entities {
			ids = append(ids, id)
		}
	}
	if kind == "" || kind == "link" {
		for id := range r.Links {
			ids = append(ids, id)
		}
	}
	if kind == "" || kind == "route" {
		for id := range r.Routes {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func LoadRegistry(root string) (*Registry, error) {
	return CollectRegistry(root, CollectorOptions{})
}

func (r *Registry) validateComplete() error {
	if issues := r.Validate(); len(issues) != 0 {
		return errors.New(strings.Join(issues, "; "))
	}
	return nil
}
