package locus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type GraphView struct {
	Scopes   []Scope       `json:"scopes"`
	Bindings []BindingView `json:"bindings"`
	Entities []EntityView  `json:"entities"`
	Links    []GraphLink   `json:"links"`
	Routes   []GraphRoute  `json:"routes"`
}

type BindingView struct {
	Role   string `json:"role"`
	Target string `json:"target"`
}

type EntityView struct {
	CanonicalID      string            `json:"canonical_id"`
	ScopeID          string            `json:"scope_id"`
	Kind             string            `json:"kind"`
	Name             string            `json:"name"`
	Labels           map[string]string `json:"labels,omitempty"`
	DocumentationIDs []string          `json:"documentation_ids,omitempty"`
}

type GraphLink struct {
	CanonicalID      string   `json:"canonical_id"`
	ScopeID          string   `json:"scope_id"`
	From             string   `json:"from"`
	To               string   `json:"to"`
	Provider         string   `json:"provider"`
	Requires         []string `json:"requires,omitempty"`
	Provides         []string `json:"provides,omitempty"`
	DocumentationIDs []string `json:"documentation_ids,omitempty"`
}

type GraphRoute struct {
	CanonicalID      string   `json:"canonical_id"`
	ScopeID          string   `json:"scope_id"`
	Steps            []string `json:"steps"`
	DocumentationIDs []string `json:"documentation_ids,omitempty"`
}

type StatusView struct {
	Vantage string            `json:"vantage"`
	Summary map[string]int    `json:"summary"`
	Links   []LinkEvidence    `json:"links"`
	Routes  []RouteStatusView `json:"routes"`
}

type RouteStatusView struct {
	RouteID  string        `json:"route_id"`
	Evidence RouteEvidence `json:"evidence"`
}

type DocumentView struct {
	ID           string                `json:"id"`
	ScopeID      string                `json:"scope_id"`
	Path         string                `json:"path"`
	Title        string                `json:"title"`
	Associations []DocumentAssociation `json:"associations"`
}

type DocumentAssociation struct {
	ObjectID   string `json:"object_id"`
	ObjectType string `json:"object_type"`
	Ref        string `json:"ref"`
}

type DocumentContent struct {
	DocumentView
	Format string `json:"format"`
	Body   string `json:"body"`
}

type documentRecord struct {
	view DocumentView
	path string
}

func (r *Registry) Graph() (GraphView, error) {
	documents, err := r.documentationRecords()
	if err != nil {
		return GraphView{}, err
	}
	documentIDs := map[string][]string{}
	for _, record := range documents {
		for _, association := range record.view.Associations {
			documentIDs[association.ObjectID] = append(documentIDs[association.ObjectID], record.view.ID)
		}
	}
	result := GraphView{}
	for _, manifest := range r.Scopes {
		result.Scopes = append(result.Scopes, manifest.Scope)
	}
	sort.Slice(result.Scopes, func(i, j int) bool { return result.Scopes[i].ID < result.Scopes[j].ID })
	for role, target := range r.Bindings {
		result.Bindings = append(result.Bindings, BindingView{Role: role, Target: target})
	}
	sort.Slice(result.Bindings, func(i, j int) bool { return result.Bindings[i].Role < result.Bindings[j].Role })
	for _, entity := range r.Entities {
		result.Entities = append(result.Entities, EntityView{
			CanonicalID: entity.CanonicalID, ScopeID: entity.ScopeID, Kind: entity.Kind, Name: entity.Name,
			Labels: entity.Labels, DocumentationIDs: sortedUnique(documentIDs[entity.CanonicalID]),
		})
	}
	sort.Slice(result.Entities, func(i, j int) bool { return result.Entities[i].CanonicalID < result.Entities[j].CanonicalID })
	for _, link := range r.Links {
		result.Links = append(result.Links, GraphLink{
			CanonicalID: link.CanonicalID, ScopeID: link.ScopeID, From: link.From, To: link.To, Provider: link.Provider,
			Requires: append([]string(nil), link.Requires...), Provides: append([]string(nil), link.Provides...),
			DocumentationIDs: sortedUnique(documentIDs[link.CanonicalID]),
		})
	}
	sort.Slice(result.Links, func(i, j int) bool { return result.Links[i].CanonicalID < result.Links[j].CanonicalID })
	for _, route := range r.Routes {
		view := GraphRoute{CanonicalID: route.CanonicalID, ScopeID: route.ScopeID, DocumentationIDs: sortedUnique(documentIDs[route.CanonicalID])}
		for _, step := range route.Steps {
			view.Steps = append(view.Steps, step.Link)
		}
		result.Routes = append(result.Routes, view)
	}
	sort.Slice(result.Routes, func(i, j int) bool { return result.Routes[i].CanonicalID < result.Routes[j].CanonicalID })
	return result, nil
}

func (r *Registry) Status(ctx context.Context, vantage string, store *Store) (StatusView, error) {
	result := StatusView{Vantage: vantage, Summary: map[string]int{"failure": 0, "stale": 0, "unknown": 0, "success": 0}}
	for _, id := range sortedMapKeys(r.Links) {
		observation, err := store.Latest(ctx, id, vantage)
		if err != nil {
			return StatusView{}, err
		}
		evidence := ClassifyLinkEvidence(id, observation)
		result.Links = append(result.Links, evidence)
		result.Summary[evidence.Status]++
	}
	for _, id := range sortedMapKeys(r.Routes) {
		evidence, err := r.RouteEvidence(ctx, r.Routes[id], vantage, store)
		if err != nil {
			return StatusView{}, err
		}
		result.Routes = append(result.Routes, RouteStatusView{RouteID: id, Evidence: evidence})
	}
	return result, nil
}

func (r *Registry) Documents() ([]DocumentView, error) {
	records, err := r.documentationRecords()
	if err != nil {
		return nil, err
	}
	result := make([]DocumentView, 0, len(records))
	for _, record := range records {
		result = append(result, record.view)
	}
	return result, nil
}

func (r *Registry) Document(id string) (DocumentContent, error) {
	records, err := r.documentationRecords()
	if err != nil {
		return DocumentContent{}, err
	}
	for _, record := range records {
		if record.view.ID != id {
			continue
		}
		body, err := os.ReadFile(record.path)
		if err != nil {
			return DocumentContent{}, err
		}
		format := "text"
		if strings.EqualFold(filepath.Ext(record.path), ".md") || strings.EqualFold(filepath.Ext(record.path), ".markdown") {
			format = "markdown"
		}
		return DocumentContent{DocumentView: record.view, Format: format, Body: string(body)}, nil
	}
	return DocumentContent{}, fmt.Errorf("unknown document %q", id)
}

func (r *Registry) documentationRecords() ([]documentRecord, error) {
	byID := map[string]*documentRecord{}
	appendDocuments := func(objectID, objectType string, documents []Documentation) error {
		for _, document := range documents {
			record, err := r.resolveDocument(objectID, document)
			if err != nil {
				return err
			}
			existing := byID[record.view.ID]
			if existing == nil {
				byID[record.view.ID] = &record
				existing = &record
			}
			if existing.view.Title == "" && document.Title != "" {
				existing.view.Title = document.Title
			}
			existing.view.Associations = append(existing.view.Associations, DocumentAssociation{ObjectID: objectID, ObjectType: objectType, Ref: document.Ref})
		}
		return nil
	}
	for _, id := range sortedMapKeys(r.Entities) {
		if err := appendDocuments(id, "entity", r.Entities[id].Documentation); err != nil {
			return nil, err
		}
	}
	for _, id := range sortedMapKeys(r.Links) {
		if err := appendDocuments(id, "link", r.Links[id].Documentation); err != nil {
			return nil, err
		}
	}
	for _, id := range sortedMapKeys(r.Routes) {
		if err := appendDocuments(id, "route", r.Routes[id].Documentation); err != nil {
			return nil, err
		}
	}
	result := make([]documentRecord, 0, len(byID))
	for _, record := range byID {
		if record.view.Title == "" {
			record.view.Title = filepath.Base(record.view.Path)
		}
		sort.Slice(record.view.Associations, func(i, j int) bool {
			left, right := record.view.Associations[i], record.view.Associations[j]
			return left.ObjectID+left.Ref < right.ObjectID+right.Ref
		})
		result = append(result, *record)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].view.ScopeID+result[i].view.Path < result[j].view.ScopeID+result[j].view.Path
	})
	return result, nil
}

func (r *Registry) resolveDocument(objectID string, document Documentation) (documentRecord, error) {
	source := r.sources[objectID]
	if source == "" {
		return documentRecord{}, fmt.Errorf("missing source for %s", objectID)
	}
	refPath, _, _ := strings.Cut(document.Ref, "#")
	scopeRoot := filepath.Dir(filepath.Dir(source))
	docsRoot := filepath.Join(scopeRoot, "docs")
	candidate := filepath.Clean(filepath.Join(filepath.Dir(source), filepath.FromSlash(refPath)))
	resolvedDocs, err := filepath.EvalSymlinks(docsRoot)
	if err != nil {
		return documentRecord{}, err
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return documentRecord{}, err
	}
	if !pathContainedBy(resolvedDocs, resolvedCandidate) {
		return documentRecord{}, fmt.Errorf("documentation %q resolves outside scope docs", document.Ref)
	}
	relative, err := filepath.Rel(resolvedDocs, resolvedCandidate)
	if err != nil {
		return documentRecord{}, err
	}
	relative = filepath.ToSlash(relative)
	scopeID := strings.SplitN(objectID, "::", 2)[0]
	digest := sha256.Sum256([]byte(scopeID + "\x00" + relative))
	id := hex.EncodeToString(digest[:12])
	return documentRecord{view: DocumentView{ID: id, ScopeID: scopeID, Path: relative, Title: document.Title}, path: resolvedCandidate}, nil
}

func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func sortedMapKeys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
