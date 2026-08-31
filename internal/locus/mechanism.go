package locus

import (
	"errors"
	"fmt"
	"path/filepath"
)

type RuntimeInput struct {
	From                  string
	Vantage               string
	MechanismBindingsPath string
}

type MechanismBinding struct {
	Executable   string         `yaml:"executable,omitempty" json:"executable,omitempty"`
	ProviderData map[string]any `yaml:"provider_data,omitempty" json:"-"`
}

type mechanismBindingsFile struct {
	APIVersion string                      `yaml:"api_version"`
	Bindings   map[string]MechanismBinding `yaml:"bindings"`
}

func BuildRuntime(registry *Registry, input RuntimeInput) (RuntimeContext, error) {
	resolvedVantage, err := ObservationVantage(input.Vantage)
	if err != nil {
		return RuntimeContext{}, err
	}
	if input.From == "" {
		return RuntimeContext{}, errors.New("--from is required for this command")
	}
	currentEntity, err := registry.ResolveEntity(input.From)
	if err != nil {
		return RuntimeContext{}, err
	}
	bindings, source, err := loadMechanismBindings(registry, input.MechanismBindingsPath)
	if err != nil {
		return RuntimeContext{}, err
	}
	return RuntimeContext{
		CurrentEntity:           currentEntity,
		Vantage:                 resolvedVantage,
		MechanismBindings:       bindings,
		MechanismBindingsSource: source,
	}, nil
}

func loadMechanismBindings(registry *Registry, path string) (map[string]MechanismBinding, string, error) {
	if path == "" {
		return nil, "", nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	var file mechanismBindingsFile
	if err := decodeYAML(absolute, &file); err != nil {
		return nil, "", err
	}
	if file.APIVersion != "locus/v0" {
		return nil, "", fmt.Errorf("%s: api_version must be locus/v0", absolute)
	}
	resolved := make(map[string]MechanismBinding, len(file.Bindings))
	for ref, binding := range file.Bindings {
		id, kind, err := registry.ResolveAny(ref)
		if err != nil {
			return nil, "", fmt.Errorf("%s: binding %s: %w", absolute, ref, err)
		}
		if kind != "link" {
			return nil, "", fmt.Errorf("%s: binding %s must reference a Link", absolute, ref)
		}
		if binding.Executable == "" && len(binding.ProviderData) == 0 {
			return nil, "", fmt.Errorf("%s: binding %s is empty", absolute, ref)
		}
		resolved[id] = binding
	}
	return resolved, absolute, nil
}

func (runtime RuntimeContext) effectiveLink(declared *Link) *Link {
	binding, ok := runtime.MechanismBindings[declared.CanonicalID]
	if !ok || len(binding.ProviderData) == 0 {
		return declared
	}
	effective := *declared
	effective.ProviderData = make(map[string]any, len(declared.ProviderData)+len(binding.ProviderData))
	for key, value := range declared.ProviderData {
		effective.ProviderData[key] = value
	}
	for key, value := range binding.ProviderData {
		effective.ProviderData[key] = value
	}
	return &effective
}

func (runtime RuntimeContext) mechanismExecutable(linkID, fallback string) string {
	if binding, ok := runtime.MechanismBindings[linkID]; ok && binding.Executable != "" {
		return binding.Executable
	}
	return fallback
}
