package locus

import (
	"context"
	"sort"
	"time"
)

type RouteEvidence struct {
	Status string         `json:"status"`
	Links  []LinkEvidence `json:"links"`
}

type LinkEvidence struct {
	LinkID      string       `json:"link_id"`
	Status      string       `json:"status"`
	Provider    string       `json:"provider"`
	Observation *Observation `json:"observation,omitempty"`
}
type ResolvedBinding struct {
	CanonicalID string `json:"canonical_id"`
	Role        string `json:"role"`
	Target      string `json:"target"`
}

type ResolvedEntity struct {
	CanonicalID   string            `json:"canonical_id"`
	ScopeID       string            `json:"scope_id"`
	Kind          string            `json:"kind"`
	Name          string            `json:"name"`
	Labels        map[string]string `json:"labels,omitempty"`
	Documentation []Documentation   `json:"documentation,omitempty"`
}

type ResolvedStep struct {
	LinkID        string          `json:"link_id"`
	Provider      string          `json:"provider"`
	NativeHint    NativeHint      `json:"native_hint"`
	Documentation []Documentation `json:"documentation,omitempty"`
	Evidence      LinkEvidence    `json:"evidence"`
}

type ResolvedRoute struct {
	CanonicalID     string          `json:"canonical_id"`
	From            string          `json:"from"`
	DerivedTarget   string          `json:"derived_target"`
	DerivedProvides []string        `json:"derived_provides"`
	Documentation   []Documentation `json:"documentation,omitempty"`
	EvidenceStatus  string          `json:"evidence_status"`
	Steps           []ResolvedStep  `json:"steps"`
}

type ResolveExplanation struct {
	TargetResolution string   `json:"target_resolution"`
	CandidateRoutes  []string `json:"candidate_routes"`
	Origins          []string `json:"origins"`
	Completeness     string   `json:"completeness"`
}

type ResolveResult struct {
	Status         string              `json:"status"`
	InputTarget    string              `json:"input_target"`
	Target         string              `json:"canonical_target,omitempty"`
	TargetEntity   ResolvedEntity      `json:"target_entity,omitempty"`
	Binding        *ResolvedBinding    `json:"binding,omitempty"`
	Capability     string              `json:"capability"`
	Route          *ResolvedRoute      `json:"route,omitempty"`
	Candidates     []ResolvedRoute     `json:"candidates"`
	Completeness   Completeness        `json:"completeness"`
	BlockedImports []BlockedImport     `json:"blocked_imports"`
	Explanation    *ResolveExplanation `json:"explanation,omitempty"`
}

func (r *Registry) Resolve(ctx context.Context, targetRef, capability string, runtime RuntimeContext, providers *Providers, store *Store) (ResolveResult, error) {
	result := ResolveResult{
		Status: "unresolved", InputTarget: targetRef, Capability: capability, Completeness: r.Completeness,
		BlockedImports: append([]BlockedImport(nil), r.BlockedImports...), Candidates: []ResolvedRoute{},
	}
	target, err := r.ResolveEntity(targetRef)
	if err != nil {
		if r.Completeness == Partial {
			result.Status = "incomplete"
			return result, nil
		}
		return ResolveResult{}, err
	}
	targetEntity := r.Entities[target]
	result.Target = target
	result.TargetEntity = makeResolvedEntity(targetEntity)
	if bindingID, bindingErr := r.resolveRef(r.RootScopeID, targetRef, "binding"); bindingErr == nil {
		binding := r.Bindings[bindingID]
		result.Binding = &ResolvedBinding{CanonicalID: binding.CanonicalID, Role: binding.ID, Target: binding.Target}
	}
	for _, route := range r.Routes {
		candidate, applicable, resolveErr := r.resolveRoute(ctx, route, target, capability, runtime, providers, store)
		if resolveErr != nil {
			return ResolveResult{}, resolveErr
		}
		if applicable {
			result.Candidates = append(result.Candidates, candidate)
		}
	}
	sort.Slice(result.Candidates, func(i, j int) bool {
		return result.Candidates[i].CanonicalID < result.Candidates[j].CanonicalID
	})
	if r.Completeness == Partial {
		result.Status = "incomplete"
		return result, nil
	}
	switch len(result.Candidates) {
	case 0:
		return result, nil
	case 1:
		result.Status = "resolved"
		result.Route = &result.Candidates[0]
		result.Candidates = nil
	default:
		result.Status = "ambiguous"
	}
	return result, nil
}
func makeResolvedEntity(entity *Entity) ResolvedEntity {
	return ResolvedEntity{
		CanonicalID:   entity.CanonicalID,
		ScopeID:       entity.ScopeID,
		Kind:          entity.Kind,
		Name:          entity.Name,
		Labels:        entity.Labels,
		Documentation: append([]Documentation(nil), entity.Documentation...),
	}
}

func (r *Registry) resolveRoute(ctx context.Context, route *Route, target, capability string, runtime RuntimeContext, providers *Providers, store *Store) (ResolvedRoute, bool, error) {
	if len(route.Steps) == 0 {
		return ResolvedRoute{}, false, nil
	}
	last := r.Links[route.Steps[len(route.Steps)-1].Link]
	if last.To != target {
		return ResolvedRoute{}, false, nil
	}
	first := r.Links[route.Steps[0].Link]
	if runtime.CurrentEntity != "" && first.From != runtime.CurrentEntity {
		return ResolvedRoute{}, false, nil
	}
	effectiveRuntime := runtime
	effectiveRuntime.CurrentEntity = first.From
	available := map[string]bool{}
	result := ResolvedRoute{
		CanonicalID:   route.CanonicalID,
		From:          first.From,
		DerivedTarget: last.To,
		Documentation: append([]Documentation(nil), route.Documentation...),
	}
	for _, step := range route.Steps {
		prepared, err := r.prepareLink(step.Link, effectiveRuntime, providers)
		if err != nil {
			return ResolvedRoute{}, false, err
		}
		link, provider := prepared.link, prepared.provider
		hint, err := provider.Render(link, effectiveRuntime)
		if err != nil {
			return ResolvedRoute{}, false, err
		}
		observation, err := store.LatestApplicable(ctx, prepared.applicability)
		if err != nil {
			return ResolvedRoute{}, false, err
		}
		evidence := ClassifyLinkEvidence(link.CanonicalID, observation)
		evidence.Provider = link.Provider
		result.Steps = append(result.Steps, ResolvedStep{
			LinkID:        link.CanonicalID,
			Provider:      link.Provider,
			NativeHint:    hint,
			Documentation: append([]Documentation(nil), link.Documentation...),
			Evidence:      evidence,
		})
		for _, provided := range link.Provides {
			available[provided] = true
		}
	}
	if !available[capability] {
		return ResolvedRoute{}, false, nil
	}
	for provided := range available {
		result.DerivedProvides = append(result.DerivedProvides, provided)
	}
	sort.Strings(result.DerivedProvides)
	result.EvidenceStatus = aggregateEvidence(result.Steps)
	return result, true, nil
}

func ClassifyLinkEvidence(linkID string, observation *Observation) LinkEvidence {
	status := "unknown"
	if observation != nil {
		if !observation.ExpiresAt.IsZero() && observation.ExpiresAt.Before(time.Now()) {
			status = "stale"
		} else {
			status = observation.Status
		}
	}
	return LinkEvidence{LinkID: linkID, Status: status, Observation: observation}
}

func aggregateEvidence(steps []ResolvedStep) string {
	allSuccess, allObserved, hasStale := true, true, false
	for _, step := range steps {
		switch step.Evidence.Status {
		case "failure":
			return "failure"
		case "success":
		case "stale":
			allSuccess = false
			hasStale = true
		default:
			allSuccess = false
			allObserved = false
		}
	}
	if allSuccess {
		return "success"
	}
	if allObserved && hasStale {
		return "stale"
	}
	return "unknown"
}

func (r *Registry) LinkEvidence(ctx context.Context, linkID string, runtime RuntimeContext, providers *Providers, store *Store) (LinkEvidence, error) {
	if runtime.CurrentEntity == "" {
		runtime.CurrentEntity = r.Links[linkID].From
	}
	prepared, err := r.prepareLink(linkID, runtime, providers)
	if err != nil {
		return LinkEvidence{}, err
	}
	observation, err := store.LatestApplicable(ctx, prepared.applicability)
	if err != nil {
		return LinkEvidence{}, err
	}
	evidence := ClassifyLinkEvidence(linkID, observation)
	evidence.Provider = prepared.link.Provider
	return evidence, nil
}

func (r *Registry) RouteEvidence(ctx context.Context, route *Route, runtime RuntimeContext, providers *Providers, store *Store) (RouteEvidence, error) {
	if runtime.CurrentEntity == "" && len(route.Steps) != 0 {
		runtime.CurrentEntity = r.Links[route.Steps[0].Link].From
	}
	result := RouteEvidence{}
	var steps []ResolvedStep
	for _, step := range route.Steps {
		evidence, err := r.LinkEvidence(ctx, step.Link, runtime, providers, store)
		if err != nil {
			return result, err
		}
		result.Links = append(result.Links, evidence)
		steps = append(steps, ResolvedStep{Evidence: evidence})
	}
	result.Status = aggregateEvidence(steps)
	return result, nil
}
