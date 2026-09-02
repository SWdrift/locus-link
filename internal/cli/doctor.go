package cli

import (
	"errors"
	"fmt"
	"locus-link/internal/locus"
	"locus-link/internal/migration"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorResult struct {
	Checks      []doctorCheck `json:"checks"`
	Diagnostics []Diagnostic  `json:"diagnostics,omitempty"`
	NextActions []NextAction  `json:"next_actions,omitempty"`
}

func (c *CLI) doctorCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use: "doctor", Short: "Diagnose local Locus configuration without side effects", GroupID: inspectionGroup, Args: exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := c.doctor(opts)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	command.Flags().StringVar(&opts.MechanismBindings, "mechanism-bindings", "", "workstation-local mechanism bindings file")
	return command
}

func (c *CLI) doctor(opts options) (any, error) {
	root, rootErr := doctorRegistryPath(opts.Registry)
	if rootErr != nil {
		result := doctorResult{
			Checks:      []doctorCheck{{Name: "Registry", Status: "ERROR", Message: rootErr.Error()}},
			Diagnostics: []Diagnostic{{Code: "root.not_found", Severity: "error", Message: rootErr.Error()}},
			NextActions: []NextAction{{ID: "init-registry", Label: "Create a Registry", Reason: "root_not_found", Args: []string{"init", "--scope-id", "<id>"}, Effect: "write_registry", Confirmation: "required"}},
		}
		return result, cliError{code: 2, err: rootErr}
	}

	statePath, stateErr := locus.DefaultStatePath()
	var store *locus.Store
	stateCheck := doctorCheck{Name: "State", Status: "WARN", Message: "state database does not exist"}
	var diagnostics []Diagnostic
	var actions []NextAction
	if stateErr != nil {
		stateCheck = doctorCheck{Name: "State", Status: "ERROR", Message: stateErr.Error()}
	} else if _, err := os.Stat(statePath); err == nil {
		store, stateErr = locus.OpenStoreReadOnly(statePath)
		if stateErr == nil {
			defer store.Close()
			version, versionErr := store.SchemaVersion()
			switch {
			case versionErr != nil:
				stateCheck = doctorCheck{Name: "State", Status: "ERROR", Message: versionErr.Error()}
			case version != migration.CurrentStateSchemaVersion:
				stateCheck = doctorCheck{Name: "State", Status: "WARN", Message: fmt.Sprintf("%s (schema %d; expected %d)", statePath, version, migration.CurrentStateSchemaVersion)}
				diagnostics = append(diagnostics, Diagnostic{Code: "state.migration_required", Severity: "warning", Subject: statePath})
				actions = append(actions, NextAction{ID: "migrate-state", Label: "Migrate local state", Reason: "state_schema_outdated", Args: []string{"migrate", "--state", statePath}, Effect: "migrate_state", Confirmation: "required"})
			default:
				stateCheck = doctorCheck{Name: "State", Status: "OK", Message: statePath}
			}
		} else {
			stateCheck = doctorCheck{Name: "State", Status: "ERROR", Message: stateErr.Error()}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		stateCheck = doctorCheck{Name: "State", Status: "ERROR", Message: err.Error()}
	}

	registry, err := locus.CollectRegistry(root, locus.CollectorOptions{Store: store})
	if err != nil {
		result := doctorResult{Checks: []doctorCheck{{Name: "Registry", Status: "ERROR", Message: err.Error()}, stateCheck}}
		return result, cliError{code: 2, err: err}
	}
	checks := []doctorCheck{{Name: "Registry", Status: "OK", Message: registry.RootScopeID}}
	importStatus := "OK"
	if registry.Completeness == locus.Partial {
		importStatus = "WARN"
		diagnostics = append(diagnostics, Diagnostic{Code: "view.partial", Severity: "warning", Subject: registry.RootScopeID})
		if len(registry.BlockedImports) != 0 {
			alias := joinAliasPath(registry.BlockedImports[0].AliasPath)
			actions = append(actions, NextAction{ID: "refresh-import", Label: "Fetch blocked import", Reason: registry.BlockedImports[0].Reason, Args: []string{"refresh", alias}, Effect: "activate_cache", Confirmation: "none"})
		}
	}
	checks = append(checks, doctorCheck{Name: "Imports", Status: importStatus, Message: fmt.Sprintf("%s (%d blocked)", registry.Completeness, len(registry.BlockedImports))})
	checks = append(checks, stateCheck)

	bindingStatus, bindingMessage := "OK", "not configured"
	if opts.MechanismBindings != "" {
		if _, err := locus.BuildRuntime(registry, locus.RuntimeInput{MechanismBindingsPath: opts.MechanismBindings}); err != nil {
			bindingStatus, bindingMessage = "ERROR", err.Error()
			diagnostics = append(diagnostics, Diagnostic{Code: "bindings.invalid", Severity: "error", Subject: opts.MechanismBindings})
		} else {
			bindingMessage, _ = filepath.Abs(opts.MechanismBindings)
		}
	}
	checks = append(checks, doctorCheck{Name: "Bindings", Status: bindingStatus, Message: bindingMessage})

	available := locus.NewProviders().Available()
	missing := missingTools(available, []string{"frpc", "salt", "ssh"})
	providerStatus, providerMessage := "OK", strings.Join(available, ", ")
	if len(missing) != 0 {
		providerStatus, providerMessage = "WARN", strings.Join(missing, ", ")+" not found"
		diagnostics = append(diagnostics, Diagnostic{Code: "provider.executable_missing", Severity: "warning", Subject: strings.Join(missing, ",")})
	}
	checks = append(checks, doctorCheck{Name: "Providers", Status: providerStatus, Message: providerMessage})
	return doctorResult{Checks: checks, Diagnostics: diagnostics, NextActions: actions}, nil
}

func doctorRegistryPath(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if root, discoverErr := locus.DiscoverRegistry(cwd); discoverErr == nil {
		return root, nil
	}
	layout, err := locus.LocusHomeLayout()
	if err != nil {
		return "", err
	}
	if info, statErr := os.Stat(filepath.Join(layout.Registry, "scope.yaml")); statErr == nil && !info.IsDir() {
		return layout.Registry, nil
	}
	return "", fmt.Errorf("no Registry found from %s or %s", cwd, layout.Registry)
}

func missingTools(available, expected []string) []string {
	found := make(map[string]bool, len(available))
	for _, tool := range available {
		found[tool] = true
	}
	var missing []string
	for _, tool := range expected {
		if !found[tool] {
			missing = append(missing, tool)
		}
	}
	sort.Strings(missing)
	return missing
}
