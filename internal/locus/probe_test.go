package locus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRouteProbeUsesFirstLinkForStartingContext(t *testing.T) {
	ctx := context.Background()
	storePath := workspaceTestPath(t, "probe-route-start", "state.db")
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
	first := resolverTestLink("project.test::link.first")
	first.To = "environment.test::bastion"
	first.Provides = []string{"tunnel"}
	second := resolverTestLink("project.test::link.second")
	second.From = first.To
	second.Requires = []string{"tunnel"}
	route := &Route{
		CanonicalID: "project.test::route.chain",
		Steps:       []RouteStep{{Link: first.CanonicalID}, {Link: second.CanonicalID}},
	}
	registry := &Registry{
		Entities: map[string]*Entity{},
		Links: map[string]*Link{
			first.CanonicalID:  first,
			second.CanonicalID: second,
		},
		Routes: map[string]*Route{route.CanonicalID: route},
	}

	result, err := registry.Probe(ctx, route.CanonicalID, RuntimeContext{
		CurrentEntity: first.From,
		Vantage:       "office-lan",
	}, providers, store)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "success" || len(result.Observations) != 2 || provider.probeCalls != 2 {
		t.Fatalf("explicit Route should probe all ordered Links from its first Link: %#v, calls=%d", result, provider.probeCalls)
	}
}
