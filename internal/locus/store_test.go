package locus

import (
	"context"
	"database/sql"
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

func TestStoreMigrationPreservesButInvalidatesLegacyObservations(t *testing.T) {
	ctx := context.Background()
	storePath := workspaceTestPath(t, "store-migration", "state.db")
	if err := os.RemoveAll(filepath.Dir(storePath)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", storePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE observations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subject TEXT NOT NULL,
		vantage TEXT NOT NULL,
		status TEXT NOT NULL,
		observed_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		provider TEXT NOT NULL,
		evidence TEXT NOT NULL,
		error TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO observations(subject,vantage,status,observed_at,expires_at,provider,evidence,error) VALUES(?,?,?,?,?,?,?,?)`,
		"project.test::link.legacy", "office-lan", "success", now, time.Time{}.Format(time.RFC3339Nano), "ssh", `{}`, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	legacy, err := store.Latest(ctx, "project.test::link.legacy", "office-lan")
	if err != nil || legacy == nil {
		t.Fatalf("legacy observation was not preserved: %#v, %v", legacy, err)
	}
	applicable, err := store.LatestApplicable(ctx, ObservationApplicability{
		Subject: "project.test::link.legacy", Vantage: "office-lan",
		DeclarationDigest: "sha256:new", SourceDigest: "sha256:new", BindingDigest: "sha256:new",
		ProbeKind: "tcp-connect", ProbeSemanticsVersion: "1", ContextFingerprint: "sha256:new",
	})
	if err != nil {
		t.Fatal(err)
	}
	if applicable != nil {
		t.Fatalf("legacy observation without provenance remained applicable: %#v", applicable)
	}
}
