package cli

import (
	"fmt"
	"locus-link/internal/locus"
	"os"

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
	root := opts.Registry
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root, err = locus.DiscoverRegistry(cwd)
		if err != nil {
			return doctorResult{Checks: []doctorCheck{{Name: "Registry", Status: "ERROR", Message: err.Error()}}, Diagnostics: []Diagnostic{{Code: "root.not_found", Severity: "error"}}}, cliError{code: 2, err: err}
		}
	}
	registry, err := locus.CollectRegistry(root, locus.CollectorOptions{})
	if err != nil {
		return doctorResult{Checks: []doctorCheck{{Name: "Registry", Status: "ERROR", Message: err.Error()}}}, cliError{code: 2, err: err}
	}
	checks := []doctorCheck{{Name: "Registry", Status: "OK", Message: registry.RootScopeID}}
	importStatus := "OK"
	if registry.Completeness == locus.Partial {
		importStatus = "WARN"
	}
	checks = append(checks, doctorCheck{Name: "Imports", Status: importStatus, Message: fmt.Sprintf("%s (%d blocked)", registry.Completeness, len(registry.BlockedImports))})
	statePath, stateErr := locus.DefaultStatePath()
	stateStatus, stateMessage := "OK", statePath
	if stateErr != nil {
		stateStatus, stateMessage = "ERROR", stateErr.Error()
	} else if _, statErr := os.Stat(statePath); statErr != nil {
		stateStatus, stateMessage = "WARN", "state database does not exist"
	}
	checks = append(checks, doctorCheck{Name: "State", Status: stateStatus, Message: stateMessage})
	available := locus.NewProviders().Available()
	providerStatus := "OK"
	if len(available) < 3 {
		providerStatus = "WARN"
	}
	checks = append(checks, doctorCheck{Name: "Providers", Status: providerStatus, Message: fmt.Sprintf("available: %v", available)})
	return doctorResult{Checks: checks}, nil
}
