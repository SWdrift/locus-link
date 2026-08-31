package locus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type ObservationApplicability struct {
	Subject               string `json:"subject"`
	Vantage               string `json:"vantage"`
	DeclarationDigest     string `json:"declaration_digest"`
	SourceDigest          string `json:"source_digest"`
	BindingDigest         string `json:"binding_digest"`
	ProbeKind             string `json:"probe_kind"`
	ProbeSemanticsVersion string `json:"probe_semantics_version"`
	ContextFingerprint    string `json:"context_fingerprint"`
}

type preparedLink struct {
	link          *Link
	provider      Provider
	applicability ObservationApplicability
}

func (r *Registry) prepareLink(linkID string, runtime RuntimeContext, providers *Providers) (preparedLink, error) {
	declared, ok := r.Links[linkID]
	if !ok {
		return preparedLink{}, fmt.Errorf("unknown Link %s", linkID)
	}
	provider, ok := providers.Get(declared.Provider)
	if !ok {
		return preparedLink{}, fmt.Errorf("unsupported provider %s", declared.Provider)
	}
	effective := runtime.effectiveLink(declared)
	if issues := provider.Validate(effective); len(issues) != 0 {
		return preparedLink{}, fmt.Errorf("%s", issues[0])
	}
	semantics := provider.ProbeSemantics()
	if semantics.Kind == "" || semantics.Version == "" {
		return preparedLink{}, fmt.Errorf("provider %s has incomplete Probe semantics", provider.Name())
	}
	declarationDigest, err := digestValue(declared)
	if err != nil {
		return preparedLink{}, err
	}
	sourceDigest := r.sourceDigest(linkID, declarationDigest)
	bindingDigest, err := digestValue(struct {
		Provider     string         `json:"provider"`
		Executable   string         `json:"executable"`
		ProviderData map[string]any `json:"provider_data"`
	}{
		Provider: provider.Name(), Executable: runtime.mechanismExecutable(linkID, provider.Executable()), ProviderData: effective.ProviderData,
	})
	if err != nil {
		return preparedLink{}, err
	}
	contextFingerprint, err := digestValue(struct{}{})
	if err != nil {
		return preparedLink{}, err
	}
	return preparedLink{
		link: effective, provider: provider,
		applicability: ObservationApplicability{
			Subject: linkID, Vantage: runtime.Vantage,
			DeclarationDigest: declarationDigest, SourceDigest: sourceDigest, BindingDigest: bindingDigest,
			ProbeKind: semantics.Kind, ProbeSemanticsVersion: semantics.Version, ContextFingerprint: contextFingerprint,
		},
	}, nil
}

func (r *Registry) sourceDigest(linkID, fallback string) string {
	if digest := r.sourceDigests[linkID]; digest != "" {
		return digest
	}
	return fallback
}

func digestValue(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("digest value: %w", err)
	}
	return digestBytes(data), nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func applyObservationApplicability(observation Observation, applicability ObservationApplicability) Observation {
	observation.Subject = applicability.Subject
	observation.Vantage = applicability.Vantage
	observation.DeclarationDigest = applicability.DeclarationDigest
	observation.SourceDigest = applicability.SourceDigest
	observation.BindingDigest = applicability.BindingDigest
	observation.ProbeKind = applicability.ProbeKind
	observation.ProbeSemanticsVersion = applicability.ProbeSemanticsVersion
	observation.ContextFingerprint = applicability.ContextFingerprint
	return observation
}
