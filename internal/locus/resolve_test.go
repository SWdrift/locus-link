package locus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type recordingProvider struct {
	probeCalls int
}

func (p *recordingProvider) Name() string       { return "recording" }
func (p *recordingProvider) Executable() string { return "recording" }
func (p *recordingProvider) Validate(*Link) []string {
	return nil
}
func (p *recordingProvider) Render(link *Link, _ RuntimeContext) (NativeHint, error) {
	return NativeHint{Executable: p.Executable(), Args: []string{link.CanonicalID}}, nil
}
func (p *recordingProvider) Probe(context.Context, *Link, RuntimeContext) Observation {
	p.probeCalls++
	return Observation{}
}

func TestResolveRouteCardinalityAndNoRanking(t *testing.T) {
	ctx := context.Background()
	storePath := workspaceTestPath(t, "resolve-cardinality", "state.db")
	if err := os.RemoveAll(filepath.Dir(storePath)); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	provider := &recordingProvider{}
	providers := &Providers{values: map[string]Provider{provider.Name(): provider}}
	runtime := RuntimeContext{CurrentEntity: "project.test::workstation", Vantage: "office-lan"}
	registry := resolverTestRegistry()

	unresolved, err := registry.Resolve(ctx, "target", "shell", runtime, providers, store)
	if err != nil {
		t.Fatal(err)
	}
	if unresolved.Status != "unresolved" || unresolved.Route != nil || len(unresolved.Candidates) != 0 {
		t.Fatalf("unexpected unresolved result: %#v", unresolved)
	}

	failureLink := resolverTestLink("project.test::link.failure")
	registry.Links[failureLink.CanonicalID] = failureLink
	registry.Routes["project.test::route.only"] = resolverTestRoute("project.test::route.only", failureLink.CanonicalID)
	resolved, err := registry.Resolve(ctx, "target", "shell", runtime, providers, store)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != "resolved" || resolved.Route == nil || resolved.Route.CanonicalID != "project.test::route.only" || len(resolved.Candidates) != 0 {
		t.Fatalf("unexpected resolved result: %#v", resolved)
	}

	delete(registry.Routes, "project.test::route.only")
	successLink := resolverTestLink("project.test::link.success")
	registry.Links[successLink.CanonicalID] = successLink
	registry.Routes["project.test::route.a"] = resolverTestRoute("project.test::route.a", failureLink.CanonicalID)
	registry.Routes["project.test::route.b"] = resolverTestRoute("project.test::route.b", successLink.CanonicalID)
	now := time.Now()
	for _, observation := range []Observation{
		{Subject: failureLink.CanonicalID, Vantage: runtime.Vantage, Status: "failure", ObservedAt: now, ExpiresAt: now.Add(time.Hour), Provider: provider.Name()},
		{Subject: successLink.CanonicalID, Vantage: runtime.Vantage, Status: "success", ObservedAt: now, ExpiresAt: now.Add(time.Hour), Provider: provider.Name()},
	} {
		if _, err := store.Append(ctx, observation); err != nil {
			t.Fatal(err)
		}
	}
	ambiguous, err := registry.Resolve(ctx, "target", "shell", runtime, providers, store)
	if err != nil {
		t.Fatal(err)
	}
	if ambiguous.Status != "ambiguous" || ambiguous.Route != nil || len(ambiguous.Candidates) != 2 {
		t.Fatalf("unexpected ambiguous result: %#v", ambiguous)
	}
	if ambiguous.Candidates[0].CanonicalID != "project.test::route.a" || ambiguous.Candidates[0].EvidenceStatus != "failure" {
		t.Fatalf("candidate presentation must not rank by evidence: %#v", ambiguous.Candidates)
	}
	if ambiguous.Candidates[1].CanonicalID != "project.test::route.b" || ambiguous.Candidates[1].EvidenceStatus != "success" {
		t.Fatalf("unexpected second candidate: %#v", ambiguous.Candidates)
	}
	if provider.probeCalls != 0 {
		t.Fatalf("resolve invoked Provider.Probe %d time(s)", provider.probeCalls)
	}
}

func resolverTestRegistry() *Registry {
	return &Registry{
		Manifest: Manifest{Scope: Scope{ID: "project.test", Kind: "project"}},
		Bindings: map[string]string{"target": "environment.test::host"},
		Entities: map[string]*Entity{
			"environment.test::host": {CanonicalID: "environment.test::host"},
		},
		Links:  map[string]*Link{},
		Routes: map[string]*Route{},
	}
}

func resolverTestLink(id string) *Link {
	return &Link{
		CanonicalID: id,
		From:        "project.test::workstation",
		To:          "environment.test::host",
		Provider:    "recording",
		Provides:    []string{"shell"},
	}
}

func resolverTestRoute(id, linkID string) *Route {
	return &Route{CanonicalID: id, Steps: []RouteStep{{Link: linkID}}}
}

func workspaceTestPath(t *testing.T, elements ...string) string {
	t.Helper()
	parts := append([]string{"..", "..", "temp", "unit-tests"}, elements...)
	path, err := filepath.Abs(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return path
}
