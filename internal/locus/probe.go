package locus

import (
	"context"
	"errors"
	"fmt"
)

type ProbeResult struct {
	InputRef     string        `json:"input_ref"`
	SubjectType  string        `json:"subject_type"`
	SubjectID    string        `json:"subject_id"`
	Status       string        `json:"status"`
	Observations []Observation `json:"observations"`
}

type ProbeInputError struct {
	Err error
}

func (e ProbeInputError) Error() string { return e.Err.Error() }
func (e ProbeInputError) Unwrap() error { return e.Err }

func (r *Registry) Probe(ctx context.Context, inputRef string, runtime RuntimeContext, providers *Providers, store *Store) (ProbeResult, error) {
	id, kind, err := r.ResolveAny(inputRef)
	if err != nil {
		return ProbeResult{}, ProbeInputError{Err: err}
	}
	if kind != "link" && kind != "route" {
		return ProbeResult{}, ProbeInputError{Err: errors.New("probe accepts only Link or Route")}
	}
	links := []string{id}
	if kind == "route" {
		links = make([]string, 0, len(r.Routes[id].Steps))
		for _, step := range r.Routes[id].Steps {
			links = append(links, step.Link)
		}
	}
	result := ProbeResult{InputRef: inputRef, SubjectType: kind, SubjectID: id, Status: "success", Observations: make([]Observation, 0, len(links))}
	for _, linkID := range links {
		link := r.Links[linkID]
		if link.From != runtime.CurrentEntity {
			return ProbeResult{}, ProbeInputError{Err: fmt.Errorf("link %s is not applicable from %s", linkID, runtime.CurrentEntity)}
		}
		provider, ok := providers.Get(link.Provider)
		if !ok {
			return ProbeResult{}, ProbeInputError{Err: fmt.Errorf("unsupported provider %s", link.Provider)}
		}
		observation, err := store.Append(ctx, provider.Probe(ctx, link, runtime))
		if err != nil {
			return ProbeResult{}, err
		}
		result.Observations = append(result.Observations, observation)
		if observation.Status == "failure" {
			result.Status = "failure"
			break
		}
	}
	return result, nil
}
