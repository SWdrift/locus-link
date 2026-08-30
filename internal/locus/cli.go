package locus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	coreGroup       = "core"
	inspectionGroup = "inspection"
	authoringGroup  = "authoring"
)

type CLI struct {
	stdout io.Writer
	stderr io.Writer
}

type options struct {
	Registry   string
	From       string
	Vantage    string
	Timeout    string
	Capability string
	ScopeKind  string
	ScopeID    string
}

type cliError struct {
	code int
	err  error
}

func (e cliError) Error() string { return e.err.Error() }
func (e cliError) Unwrap() error { return e.err }

func NewCLI(stdout, stderr io.Writer) *CLI { return &CLI{stdout: stdout, stderr: stderr} }

type commandState struct {
	result any
	json   bool
}

func (s *commandState) set(result any, err error) error {
	s.result = result
	if err == nil {
		return nil
	}
	var coded cliError
	if errors.As(err, &coded) {
		return err
	}
	return cliError{code: 1, err: err}
}

func (c *CLI) Run(args []string) int {
	state := &commandState{}
	root := c.rootCommand(state)
	root.SetArgs(args)
	err := root.Execute()

	if state.result != nil {
		encoder := json.NewEncoder(c.stdout)
		if !state.json {
			encoder.SetIndent("", "  ")
		}
		if encodeErr := encoder.Encode(state.result); encodeErr != nil {
			fmt.Fprintln(c.stderr, encodeErr)
			return 1
		}
	}
	if err == nil {
		return 0
	}
	fmt.Fprintln(c.stderr, err)
	var coded cliError
	if errors.As(err, &coded) {
		return coded.code
	}
	return 2
}

func (c *CLI) rootCommand(state *commandState) *cobra.Command {
	root := &cobra.Command{
		Use:           "locus",
		Short:         "Resolve explicit operational routes and inspect their evidence",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(c.stdout)
	root.SetErr(c.stderr)
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().BoolVar(&state.json, "json", false, "emit compact JSON")
	root.AddGroup(
		&cobra.Group{ID: coreGroup, Title: "Core workflow:"},
		&cobra.Group{ID: inspectionGroup, Title: "Inspection / diagnostics:"},
		&cobra.Group{ID: authoringGroup, Title: "Registry authoring / maintenance:"},
	)
	root.AddCommand(
		c.resolveCommand(state),
		c.probeCommand(state),
		c.contextCommand(state),
		c.showCommand(state),
		c.listCommand(state),
		c.statusCommand(state),
		c.initCommand(state),
		c.validateCommand(state),
	)
	return root
}

func (c *CLI) initCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:     "init --scope-kind <project|environment> --scope-id <id>",
		Short:   "Create a registry",
		GroupID: authoringGroup,
		Args:    exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			return state.set(c.init(opts))
		},
	}
	addRegistryFlag(command, &opts)
	command.Flags().StringVar(&opts.ScopeKind, "scope-kind", "", "scope kind: project or environment")
	command.Flags().StringVar(&opts.ScopeID, "scope-id", "", "canonical scope ID")
	return command
}

func (c *CLI) validateCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:     "validate",
		Short:   "Validate registry declarations",
		GroupID: authoringGroup,
		Args:    exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := c.validate(opts)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	return command
}

func (c *CLI) contextCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:     "context",
		Short:   "Show the current operational context and vantage",
		GroupID: inspectionGroup,
		Args:    exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := c.context(opts)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	addRuntimeFlags(command, &opts)
	return command
}

func (c *CLI) listCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:     "list [entity|link|route]",
		Short:   "List registry declarations",
		GroupID: inspectionGroup,
		Args:    maximumArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := c.list(opts, args)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	return command
}

func (c *CLI) showCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:     "show <ref-or-id>",
		Short:   "Resolve a reference and show its declaration",
		GroupID: inspectionGroup,
		Args:    exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := c.show(opts, args[0])
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	return command
}

func (c *CLI) resolveCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:     "resolve <target> --capability <name>",
		Short:   "Resolve an explicit route and attach current evidence",
		GroupID: coreGroup,
		Args:    exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := c.resolve(args[0], opts)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	addRuntimeFlags(command, &opts)
	command.Flags().StringVar(&opts.Capability, "capability", "", "required capability")
	return command
}

func (c *CLI) probeCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:     "probe <link-or-route-id>",
		Short:   "Safely measure links and write new observations",
		GroupID: coreGroup,
		Args:    exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := c.probe(context.Background(), args[0], opts)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	addRuntimeFlags(command, &opts)
	command.Flags().StringVar(&opts.Timeout, "timeout", "10s", "probe timeout")
	return command
}

func (c *CLI) statusCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:     "status [link-or-route-id]",
		Short:   "Inspect persisted link observations or derived route evidence",
		GroupID: inspectionGroup,
		Args:    maximumArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := c.status(args, opts)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	command.Flags().StringVar(&opts.Vantage, "vantage", "", "observation vantage")
	return command
}

func addRegistryFlag(command *cobra.Command, opts *options) {
	command.Flags().StringVar(&opts.Registry, "registry", "", "registry path override")
}

func addRuntimeFlags(command *cobra.Command, opts *options) {
	command.Flags().StringVar(&opts.From, "from", "", "current operational entity")
	command.Flags().StringVar(&opts.Vantage, "vantage", "", "network observation vantage")
}

func exactArgs(want int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if len(args) != want {
			return cliError{code: 2, err: fmt.Errorf("%s accepts %d argument(s), received %d", command.CommandPath(), want, len(args))}
		}
		return nil
	}
}

func maximumArgs(maximum int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if len(args) > maximum {
			return cliError{code: 2, err: fmt.Errorf("%s accepts at most %d argument(s), received %d", command.CommandPath(), maximum, len(args))}
		}
		return nil
	}
}

func (c *CLI) init(opts options) (any, error) {
	if opts.ScopeKind != "project" && opts.ScopeKind != "environment" {
		return nil, cliError{code: 2, err: errors.New("--scope-kind must be project or environment")}
	}
	if opts.ScopeID == "" {
		return nil, cliError{code: 2, err: errors.New("--scope-id is required")}
	}
	root := opts.Registry
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(cwd, ".locus", "registry")
	}
	if _, err := os.Stat(filepath.Join(root, "scope.yaml")); err == nil {
		return nil, cliError{code: 2, err: errors.New("scope.yaml already exists")}
	}
	for _, directory := range []string{"entities", "links", "routes", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			return nil, err
		}
	}
	manifest := Manifest{APIVersion: "locus/v0", Scope: Scope{ID: opts.ScopeID, Kind: opts.ScopeKind}}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(root, "scope.yaml"), data, 0o644); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return map[string]any{"registry": absolute, "scope": manifest.Scope}, nil
}

func (c *CLI) validate(opts options) (any, error) {
	registry, _, _, err := c.load(opts, false)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	return map[string]any{
		"valid":        true,
		"active_scope": registry.Manifest.Scope.ID,
		"entities":     len(registry.Entities),
		"links":        len(registry.Links),
		"routes":       len(registry.Routes),
	}, nil
}

func (c *CLI) context(opts options) (any, error) {
	registry, runtime, statePath, err := c.load(opts, true)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	imports := make([]map[string]string, 0, len(registry.Aliases))
	for alias, scope := range registry.Aliases {
		imports = append(imports, map[string]string{"alias": alias, "scope_id": scope})
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i]["alias"] < imports[j]["alias"] })
	return map[string]any{
		"active_scope":      registry.Manifest.Scope,
		"imports":           imports,
		"bindings":          registry.Bindings,
		"runtime":           runtime,
		"observation_store": statePath,
	}, nil
}

func (c *CLI) list(opts options, args []string) (any, error) {
	kind := ""
	if len(args) == 1 {
		kind = args[0]
	}
	if kind != "" && kind != "entity" && kind != "link" && kind != "route" {
		return nil, cliError{code: 2, err: fmt.Errorf("invalid kind %q", kind)}
	}
	registry, _, _, err := c.load(opts, false)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	return map[string]any{"objects": registry.ObjectIDs(kind)}, nil
}

func (c *CLI) show(opts options, inputRef string) (any, error) {
	registry, _, _, err := c.load(opts, false)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	if target, ok := registry.Bindings[inputRef]; ok {
		return map[string]any{
			"input_ref":        inputRef,
			"ref_type":         "binding",
			"canonical_target": target,
			"object":           registry.Entities[target],
		}, nil
	}
	id, kind, err := registry.ResolveAny(inputRef)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	result := map[string]any{"input_ref": inputRef, "ref_type": kind, "canonical_id": id}
	switch kind {
	case "entity":
		result["object"] = registry.Entities[id]
	case "link":
		result["object"] = registry.Links[id]
	case "route":
		result["object"] = registry.Routes[id]
	}
	return result, nil
}

func (c *CLI) resolve(target string, opts options) (any, error) {
	if opts.Capability == "" {
		return nil, cliError{code: 2, err: errors.New("--capability is required")}
	}
	registry, runtime, statePath, err := c.load(opts, true)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	store, err := OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	result, err := registry.Resolve(context.Background(), target, opts.Capability, runtime, NewProviders(), store)
	if err != nil {
		return nil, err
	}
	if result.Status != "resolved" {
		return result, cliError{code: 3, err: fmt.Errorf("resolve %s", result.Status)}
	}
	return result, nil
}

func (c *CLI) probe(parent context.Context, inputRef string, opts options) (any, error) {
	registry, runtime, statePath, err := c.load(opts, true)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	id, kind, err := registry.ResolveAny(inputRef)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	if kind != "link" && kind != "route" {
		return nil, cliError{code: 2, err: errors.New("probe accepts only Link or Route")}
	}
	timeout, err := time.ParseDuration(opts.Timeout)
	if err != nil || timeout <= 0 {
		return nil, cliError{code: 2, err: fmt.Errorf("invalid --timeout %q", opts.Timeout)}
	}
	store, err := OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	links := []string{id}
	if kind == "route" {
		links = make([]string, 0, len(registry.Routes[id].Steps))
		for _, step := range registry.Routes[id].Steps {
			links = append(links, step.Link)
		}
	}
	providers := NewProviders()
	observations := make([]Observation, 0, len(links))
	failed := false
	for _, linkID := range links {
		link := registry.Links[linkID]
		if link.From != runtime.CurrentEntity {
			return nil, cliError{code: 2, err: fmt.Errorf("link %s is not applicable from %s", linkID, runtime.CurrentEntity)}
		}
		provider, ok := providers.Get(link.Provider)
		if !ok {
			return nil, cliError{code: 2, err: fmt.Errorf("unsupported provider %s", link.Provider)}
		}
		observation, appendErr := store.Append(ctx, provider.Probe(ctx, link, runtime))
		if appendErr != nil {
			return nil, appendErr
		}
		observations = append(observations, observation)
		if observation.Status == "failure" {
			failed = true
			break
		}
	}
	status := "success"
	if failed {
		status = "failure"
	}
	result := map[string]any{
		"input_ref":    inputRef,
		"subject_type": kind,
		"subject_id":   id,
		"status":       status,
		"observations": observations,
	}
	if failed {
		return result, cliError{code: 4, err: errors.New("probe failed")}
	}
	return result, nil
}

func (c *CLI) status(args []string, opts options) (any, error) {
	registry, runtime, statePath, err := c.load(opts, false)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	store, err := OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx := context.Background()
	if len(args) == 1 {
		id, kind, resolveErr := registry.ResolveAny(args[0])
		if resolveErr != nil {
			return nil, cliError{code: 2, err: resolveErr}
		}
		switch kind {
		case "link":
			observation, latestErr := store.Latest(ctx, id, runtime.Vantage)
			return map[string]any{
				"link_id":  id,
				"vantage":  runtime.Vantage,
				"evidence": classifyLinkEvidence(id, observation),
			}, latestErr
		case "route":
			evidence, evidenceErr := registry.RouteEvidence(ctx, registry.Routes[id], runtime.Vantage, store)
			return map[string]any{"route_id": id, "vantage": runtime.Vantage, "evidence": evidence}, evidenceErr
		default:
			return nil, cliError{code: 2, err: errors.New("status accepts only Link or Route")}
		}
	}
	counts := map[string]int{"failure": 0, "stale": 0, "unknown": 0, "success": 0}
	for id := range registry.Links {
		observation, latestErr := store.Latest(ctx, id, runtime.Vantage)
		if latestErr != nil {
			return nil, latestErr
		}
		counts[classifyLinkEvidence(id, observation).Status]++
	}
	return map[string]any{"vantage": runtime.Vantage, "links": counts}, nil
}

func (c *CLI) load(opts options, runtimeRequired bool) (*Registry, RuntimeContext, string, error) {
	root := opts.Registry
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, RuntimeContext{}, "", err
		}
		root, err = DiscoverRegistry(cwd)
		if err != nil {
			return nil, RuntimeContext{}, "", err
		}
	}
	registry, err := LoadRegistry(root)
	if err != nil {
		return nil, RuntimeContext{}, "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, RuntimeContext{}, "", err
	}
	runtime := RuntimeContext{CWD: cwd, AvailableTools: NewProviders().Available(), Vantage: opts.Vantage}
	if runtime.Vantage == "" {
		host, hostErr := os.Hostname()
		if hostErr != nil {
			return nil, RuntimeContext{}, "", hostErr
		}
		runtime.Vantage = "host:" + host
	}
	if opts.From != "" {
		runtime.CurrentEntity, err = registry.ResolveEntity(opts.From)
		if err != nil {
			return nil, RuntimeContext{}, "", err
		}
	} else if runtimeRequired {
		return nil, RuntimeContext{}, "", errors.New("--from is required for this command")
	}
	statePath, err := DefaultStatePath()
	if err != nil {
		return nil, RuntimeContext{}, "", err
	}
	return registry, runtime, statePath, nil
}
