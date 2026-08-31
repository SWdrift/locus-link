package locus

import (
	"context"
	"fmt"
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
	Observation *Observation `json:"observation,omitempty"`
}

type ResolvedStep struct {
	LinkID     string       `json:"link_id"`
	Provider   string       `json:"provider"`
	NativeHint NativeHint   `json:"native_hint"`
	Evidence   LinkEvidence `json:"evidence"`
}

type ResolvedRoute struct {
	CanonicalID     string         `json:"canonical_id"`
	DerivedTarget   string         `json:"derived_target"`
	DerivedProvides []string       `json:"derived_provides"`
	EvidenceStatus  string         `json:"evidence_status"`
	Steps           []ResolvedStep `json:"steps"`
}

type ResolveResult struct {
	Status      string          `json:"status"`
	InputTarget string          `json:"input_target"`
	Target      string          `json:"canonical_target"`
	Capability  string          `json:"capability"`
	Route       *ResolvedRoute  `json:"route,omitempty"`
	Candidates  []ResolvedRoute `json:"candidates,omitempty"`
}

func (r *Registry) Resolve(ctx context.Context, targetRef, capability string, runtime RuntimeContext, providers *Providers, store *Store) (ResolveResult, error) {
	target, err := r.ResolveEntity(targetRef)
	if err != nil {
		return ResolveResult{}, err
	}
	result := ResolveResult{
		Status:      "unresolved",
		InputTarget: targetRef,
		Target:      target,
		Capability:  capability,
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

func (r *Registry) resolveRoute(ctx context.Context, route *Route, target, capability string, runtime RuntimeContext, providers *Providers, store *Store) (ResolvedRoute, bool, error) {
	if len(route.Steps) == 0 {
		return ResolvedRoute{}, false, nil
	}
	last := r.Links[route.Steps[len(route.Steps)-1].Link]
	if last.To != target {
		return ResolvedRoute{}, false, nil
	}
	available := map[string]bool{}
	result := ResolvedRoute{CanonicalID: route.CanonicalID, DerivedTarget: last.To}
	for _, step := range route.Steps {
		link := r.Links[step.Link]
		if link.From != runtime.CurrentEntity {
			return ResolvedRoute{}, false, nil
		}
		provider, ok := providers.Get(link.Provider)
		if !ok {
			return ResolvedRoute{}, false, fmt.Errorf("unsupported provider %s", link.Provider)
		}
		for _, issue := range provider.Validate(link) {
			return ResolvedRoute{}, false, fmt.Errorf("%s", issue)
		}
		hint, err := provider.Render(link, runtime)
		if err != nil {
			return ResolvedRoute{}, false, err
		}
		observation, err := store.Latest(ctx, link.CanonicalID, runtime.Vantage)
		if err != nil {
			return ResolvedRoute{}, false, err
		}
		evidence := ClassifyLinkEvidence(link.CanonicalID, observation)
		result.Steps = append(result.Steps, ResolvedStep{LinkID: link.CanonicalID, Provider: link.Provider, NativeHint: hint, Evidence: evidence})
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

func (r *Registry) RouteEvidence(ctx context.Context, route *Route, vantage string, store *Store) (RouteEvidence, error) {
	result := RouteEvidence{}
	var steps []ResolvedStep
	for _, step := range route.Steps {
		observation, err := store.Latest(ctx, step.Link, vantage)
		if err != nil {
			return result, err
		}
		evidence := ClassifyLinkEvidence(step.Link, observation)
		result.Links = append(result.Links, evidence)
		steps = append(steps, ResolvedStep{Evidence: evidence})
	}
	result.Status = aggregateEvidence(steps)
	return result, nil
}
