package locus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestObservationApplicabilityInvalidatesOnlyRelevantChanges(t *testing.T) {
	ctx := context.Background()
	storePath := workspaceTestPath(t, "observation-applicability", "state.db")
	if err := os.RemoveAll(filepath.Dir(storePath)); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	registry := resolverTestRegistry()
	link := resolverTestLink("project.test::link.observed")
	registry.Links[link.CanonicalID] = link
	provider := &recordingProvider{}
	providers := &Providers{values: map[string]Provider{provider.Name(): provider}}
	baseline := RuntimeContext{CurrentEntity: link.From, Vantage: "office-lan", CWD: "workstation-a"}
	appendApplicableSuccess(t, ctx, registry, link.CanonicalID, baseline, providers, store)

	irrelevantContext := baseline
	irrelevantContext.CWD = "workstation-b"
	assertApplicableObservation(t, ctx, registry, link.CanonicalID, irrelevantContext, providers, store, true)

	originalProvides := append([]string(nil), link.Provides...)
	link.Provides = append(link.Provides, "changed-capability")
	assertApplicableObservation(t, ctx, registry, link.CanonicalID, baseline, providers, store, false)
	link.Provides = originalProvides

	changedSemantics := &recordingProvider{version: "2"}
	changedProviders := &Providers{values: map[string]Provider{changedSemantics.Name(): changedSemantics}}
	assertApplicableObservation(t, ctx, registry, link.CanonicalID, baseline, changedProviders, store, false)

	changedVantage := baseline
	changedVantage.Vantage = "home-lan"
	assertApplicableObservation(t, ctx, registry, link.CanonicalID, changedVantage, providers, store, false)

	bindingA := baseline
	bindingA.MechanismBindings = map[string]MechanismBinding{
		link.CanonicalID: {Executable: "recording-a", ProviderData: map[string]any{"endpoint": "a"}},
	}
	appendApplicableSuccess(t, ctx, registry, link.CanonicalID, bindingA, providers, store)
	bindingB := baseline
	bindingB.MechanismBindings = map[string]MechanismBinding{
		link.CanonicalID: {Executable: "recording-b", ProviderData: map[string]any{"endpoint": "b"}},
	}
	assertApplicableObservation(t, ctx, registry, link.CanonicalID, bindingB, providers, store, false)
}

func appendApplicableSuccess(t *testing.T, ctx context.Context, registry *Registry, linkID string, runtime RuntimeContext, providers *Providers, store *Store) {
	t.Helper()
	prepared, err := registry.prepareLink(linkID, runtime, providers)
	if err != nil {
		t.Fatal(err)
	}
	observation := applyObservationApplicability(Observation{
		Status: "success", ObservedAt: time.Now().UTC(), Provider: prepared.provider.Name(),
	}, prepared.applicability)
	if _, err := store.Append(ctx, observation); err != nil {
		t.Fatal(err)
	}
}

func assertApplicableObservation(t *testing.T, ctx context.Context, registry *Registry, linkID string, runtime RuntimeContext, providers *Providers, store *Store, want bool) {
	t.Helper()
	prepared, err := registry.prepareLink(linkID, runtime, providers)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := store.LatestApplicable(ctx, prepared.applicability)
	if err != nil {
		t.Fatal(err)
	}
	if (observation != nil) != want {
		t.Fatalf("applicable observation present = %t, want %t: %#v", observation != nil, want, prepared.applicability)
	}
}
