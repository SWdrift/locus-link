package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"locus-link/internal/locus"
	"sort"
)

type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Subject  string `json:"subject,omitempty"`
	Message  string `json:"message,omitempty"`
}

type NextAction struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Reason       string   `json:"reason"`
	Args         []string `json:"args"`
	Effect       string   `json:"effect"`
	Confirmation string   `json:"confirmation"`
}

func withAdvice(result any) any {
	if result == nil {
		return nil
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return result
	}
	var projection map[string]any
	if json.Unmarshal(payload, &projection) != nil {
		return result
	}
	diagnostics, actions := buildAdvice(result)
	if len(diagnostics) != 0 {
		projection["diagnostics"] = diagnostics
	}
	if len(actions) != 0 {
		projection["next_actions"] = actions
	}
	return projection
}

func buildAdvice(result any) ([]Diagnostic, []NextAction) {
	switch value := result.(type) {
	case locus.ResolveResult:
		return resolveAdvice(value)
	case locus.RefreshResult:
		if value.Status == "confirmation_required" && value.CandidateSnapshot != nil {
			digest := value.CandidateSnapshot.SnapshotDigest
			return []Diagnostic{{Code: "refresh.regression", Severity: "warning", Subject: digest, Message: "candidate would regress the active dependency view"}}, []NextAction{{
				ID: "confirm-refresh", Label: "Activate the reviewed candidate", Reason: "dependency_regression",
				Args:   []string{"refresh", "--allow-regression", "--expected-candidate-digest", digest},
				Effect: "activate_cache", Confirmation: "required",
			}}
		}
	}
	return nil, nil
}

func resolveAdvice(result locus.ResolveResult) ([]Diagnostic, []NextAction) {
	if result.Status == "incomplete" && len(result.BlockedImports) != 0 {
		blocked := result.BlockedImports[0]
		args := []string{"refresh"}
		if len(blocked.AliasPath) != 0 {
			args = append(args, joinAliasPath(blocked.AliasPath))
		}
		return []Diagnostic{{Code: "view.partial", Severity: "warning", Subject: joinAliasPath(blocked.AliasPath)}}, []NextAction{{ID: "refresh-import", Label: "Fetch blocked import", Reason: "missing_active_cache", Args: args, Effect: "activate_cache", Confirmation: "none"}}
	}
	candidates := result.Candidates
	if result.Route != nil {
		candidates = []locus.ResolvedRoute{*result.Route}
	}
	origins := map[string]bool{}
	for _, route := range candidates {
		origins[route.From] = true
	}
	if len(origins) > 1 {
		keys := make([]string, 0, len(origins))
		for origin := range origins {
			keys = append(keys, origin)
		}
		sort.Strings(keys)
		actions := make([]NextAction, 0, len(keys))
		for _, origin := range keys {
			actions = append(actions, NextAction{ID: "resolve-from", Label: "Resolve from " + origin, Reason: "origin_ambiguous", Args: []string{"resolve", result.InputTarget, result.Capability, "--from", origin}, Effect: "none", Confirmation: "none"})
		}
		return []Diagnostic{{Code: "resolve.origin_ambiguous", Severity: "warning", Subject: result.Target}}, actions
	}
	if result.Route != nil && (result.Route.EvidenceStatus == "unknown" || result.Route.EvidenceStatus == "stale") {
		return []Diagnostic{{Code: "evidence." + result.Route.EvidenceStatus, Severity: "info", Subject: result.Route.CanonicalID}}, []NextAction{{ID: "probe-route", Label: "Measure this route", Reason: "evidence_" + result.Route.EvidenceStatus, Args: []string{"probe", result.Route.CanonicalID}, Effect: "append_observation", Confirmation: "none"}}
	}
	return nil, nil
}

func joinAliasPath(path []string) string {
	result := ""
	for index, part := range path {
		if index != 0 {
			result += "::"
		}
		result += part
	}
	return result
}

func writeHuman(output io.Writer, result any) error {
	switch value := result.(type) {
	case locus.ResolveResult:
		return writeHumanResolve(output, value)
	case locus.RefreshResult:
		fmt.Fprintf(output, "Refresh: %s\nDeclaration view: %s\n", value.Status, value.Completeness)
		_, actions := buildAdvice(value)
		return writeHumanActions(output, actions, "Next")
	case doctorResult:
		for _, check := range value.Checks {
			fmt.Fprintf(output, "%-14s %-7s %s\n", check.Name, check.Status, check.Message)
		}
		return writeHumanActions(output, value.NextActions, "Suggested")
	}
	payload, err := json.MarshalIndent(withAdvice(result), "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, string(payload))
	return err
}

func writeHumanResolve(output io.Writer, result locus.ResolveResult) error {
	if result.Status == "ambiguous" {
		fmt.Fprintf(output, "Cannot infer an origin for %s / %s.\n\nOrigins:\n", result.InputTarget, result.Capability)
		for _, route := range result.Candidates {
			fmt.Fprintf(output, "  %-32s %s\n", route.From, route.CanonicalID)
		}
		_, actions := buildAdvice(result)
		return writeHumanActions(output, actions, "Choose one")
	}
	if result.Status == "incomplete" {
		fmt.Fprintln(output, "Declaration view: partial")
		if len(result.BlockedImports) != 0 {
			blocked := result.BlockedImports[0]
			fmt.Fprintf(output, "\nBlocked import:\n  %s\n  Owner: %s\n  Expected scope: %s\n  Reason: %s\n",
				joinAliasPath(blocked.AliasPath), blocked.SourceScopeID, blocked.TargetScopeID, blocked.Reason)
		}
		_, actions := buildAdvice(result)
		return writeHumanActions(output, actions, "Fetch explicitly")
	}
	if result.Status != "resolved" || result.Route == nil {
		fmt.Fprintf(output, "No declared route matches %s / %s.\n", result.InputTarget, result.Capability)
		return nil
	}
	route := result.Route
	fmt.Fprintf(output, "%s → %s\nRoute: %s\nFrom: %s\nEvidence: %s\n\n",
		result.InputTarget, result.Target, route.CanonicalID, route.From, route.EvidenceStatus)
	for index, step := range route.Steps {
		fmt.Fprintf(output, "  %d. %-40s %s\n", index+1, step.LinkID, step.Provider)
	}
	_, actions := buildAdvice(result)
	return writeHumanActions(output, actions, "Next")
}

func writeHumanActions(output io.Writer, actions []NextAction, heading string) error {
	if len(actions) == 0 {
		return nil
	}
	fmt.Fprintf(output, "\n%s:\n", heading)
	for _, action := range actions {
		fmt.Fprint(output, "  locus")
		for _, arg := range action.Args {
			fmt.Fprintf(output, " %s", arg)
		}
		fmt.Fprintln(output)
	}
	return nil
}
