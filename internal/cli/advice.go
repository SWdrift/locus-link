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
			return []Diagnostic{{Code: "refresh.regression", Severity: "warning", Subject: value.CandidateSnapshot.SnapshotDigest}}, nil
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
	projection := withAdvice(result)
	payload, ok := projection.(map[string]any)
	if !ok {
		_, err := fmt.Fprintln(output, projection)
		return err
	}
	if status, ok := payload["status"].(string); ok {
		fmt.Fprintf(output, "Status: %s\n", status)
	}
	if target, ok := payload["canonical_target"].(string); ok {
		fmt.Fprintf(output, "Target: %s\n", target)
	}
	if route, ok := payload["route"].(map[string]any); ok {
		fmt.Fprintf(output, "Route: %v\nFrom: %v\nEvidence: %v\n", route["canonical_id"], route["from"], route["evidence_status"])
	}
	if diagnostics, ok := payload["diagnostics"].([]Diagnostic); ok && len(diagnostics) != 0 {
		fmt.Fprintf(output, "\nDiagnostic: %s\n", diagnostics[0].Code)
	}
	if actions, ok := payload["next_actions"].([]NextAction); ok && len(actions) != 0 {
		fmt.Fprintln(output, "\nNext:")
		for _, action := range actions {
			fmt.Fprintf(output, "  locus")
			for _, arg := range action.Args {
				fmt.Fprintf(output, " %s", arg)
			}
			fmt.Fprintln(output)
		}
		return nil
	}
	if _, shown := payload["status"]; shown {
		return nil
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, string(encoded))
	return err
}
