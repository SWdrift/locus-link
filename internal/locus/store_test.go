package locus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestObservationLifecyclePreservesEvidenceKind(t *testing.T) {
	const evidenceKind = "tcp-connect-and-ssh-config"
	base := newObservation(
		&Link{CanonicalID: "project.test::link.ssh"},
		RuntimeContext{Vantage: "office-lan"},
		"ssh",
		evidenceKind,
	)
	if base.Evidence["kind"] != evidenceKind {
		t.Fatalf("newObservation evidence kind = %#v, want %q", base.Evidence["kind"], evidenceKind)
	}

	cases := []struct {
		name   string
		err    error
		status string
	}{
		{name: "success", status: "success"},
		{name: "failure", err: errors.New("probe failed"), status: "failure"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			observation := finishObservation(base, test.err)
			if observation.Status != test.status {
				t.Fatalf("finishObservation status = %q, want %q", observation.Status, test.status)
			}
			if observation.Evidence["kind"] != evidenceKind {
				t.Fatalf("finishObservation evidence kind = %#v, want %q", observation.Evidence["kind"], evidenceKind)
			}
		})
	}
}

func TestStoreRoundTripsEvidenceKindAndZeroExpiry(t *testing.T) {
	ctx := context.Background()
	storePath := workspaceTestPath(t, "store-evidence", "state.db")
	if err := os.RemoveAll(filepath.Dir(storePath)); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	const evidenceKind = "salt-test-ping"
	original := Observation{
		Subject:    "project.test::link.salt",
		Vantage:    "office-lan",
		Status:     "success",
		ObservedAt: time.Now().UTC(),
		Provider:   "salt",
		Evidence:   map[string]any{"kind": evidenceKind},
	}
	if _, err := store.Append(ctx, original); err != nil {
		t.Fatal(err)
	}
	stored, err := store.Latest(ctx, original.Subject, original.Vantage)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil {
		t.Fatal("Latest returned no observation")
	}
	if !stored.ExpiresAt.IsZero() {
		t.Fatalf("Latest expiry = %s, want zero", stored.ExpiresAt)
	}
	if stored.Evidence["kind"] != evidenceKind {
		t.Fatalf("Latest evidence kind = %#v, want %q", stored.Evidence["kind"], evidenceKind)
	}
}
