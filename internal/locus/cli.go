package locus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type CLI struct{ stdout, stderr *os.File }

type options struct {
	JSON, Registry, From, Vantage, Timeout, Capability, ScopeKind, ScopeID string
}

type cliError struct {
	code int
	err  error
}

func (e cliError) Error() string { return e.err.Error() }

func NewCLI(stdout, stderr *os.File) *CLI { return &CLI{stdout: stdout, stderr: stderr} }

func (c *CLI) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.stderr, "usage: locus <init|validate|context|list|show|resolve|check|status>")
		return 2
	}
	command := args[0]
	positional, opts, err := parseOptions(args[1:])
	if err != nil {
		fmt.Fprintln(c.stderr, err)
		return 2
	}
	ctx := context.Background()
	var result any
	switch command {
	case "init":
		result, err = c.init(opts)
	case "validate":
		result, err = c.validate(ctx, opts)
	case "context":
		result, err = c.context(ctx, opts)
	case "list":
		result, err = c.list(ctx, opts, positional)
	case "show":
		result, err = c.show(ctx, opts, positional)
	case "resolve":
		result, err = c.resolve(ctx, opts, positional)
	case "check":
		result, err = c.check(ctx, opts, positional)
	case "status":
		result, err = c.status(ctx, opts, positional)
	default:
		err = cliError{2, fmt.Errorf("unknown command %q", command)}
	}
	if err != nil {
		fmt.Fprintln(c.stderr, err)
		var coded cliError
		if errors.As(err, &coded) {
			return coded.code
		}
		return 1
	}
	encoder := json.NewEncoder(c.stdout)
	if opts.JSON == "" {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(c.stderr, err)
		return 1
	}
	return 0
}

func parseOptions(args []string) ([]string, options, error) {
	var opts options
	var positional []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--json" {
			opts.JSON = "true"
			continue
		}
		if !strings.HasPrefix(argument, "--") {
			positional = append(positional, argument)
			continue
		}
		if index+1 >= len(args) {
			return nil, opts, fmt.Errorf("%s requires a value", argument)
		}
		index++
		value := args[index]
		switch argument {
		case "--registry":
			opts.Registry = value
		case "--from":
			opts.From = value
		case "--vantage":
			opts.Vantage = value
		case "--timeout":
			opts.Timeout = value
		case "--capability":
			opts.Capability = value
		case "--scope-kind":
			opts.ScopeKind = value
		case "--scope-id":
			opts.ScopeID = value
		default:
			return nil, opts, fmt.Errorf("unknown option %s", argument)
		}
	}
	return positional, opts, nil
}

func (c *CLI) init(opts options) (any, error) {
	if opts.ScopeKind != "project" && opts.ScopeKind != "environment" {
		return nil, cliError{2, errors.New("--scope-kind must be project or environment")}
	}
	if opts.ScopeID == "" {
		return nil, cliError{2, errors.New("--scope-id is required")}
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
		return nil, cliError{2, errors.New("scope.yaml already exists")}
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
	absolute, _ := filepath.Abs(root)
	return map[string]any{"registry": absolute, "scope": manifest.Scope}, nil
}

func (c *CLI) validate(_ context.Context, opts options) (any, error) {
	registry, _, _, err := c.load(opts, false)
	if err != nil {
		return nil, cliError{2, err}
	}
	return map[string]any{"valid": true, "active_scope": registry.Manifest.Scope.ID, "entities": len(registry.Entities), "links": len(registry.Links), "routes": len(registry.Routes)}, nil
}

func (c *CLI) context(_ context.Context, opts options) (any, error) {
	registry, runtime, statePath, err := c.load(opts, true)
	if err != nil {
		return nil, cliError{2, err}
	}
	imports := make([]map[string]string, 0, len(registry.Aliases))
	for alias, scope := range registry.Aliases {
		imports = append(imports, map[string]string{"alias": alias, "scope_id": scope})
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i]["alias"] < imports[j]["alias"] })
	return map[string]any{"active_scope": registry.Manifest.Scope, "imports": imports, "bindings": registry.Bindings, "runtime": runtime, "observation_store": statePath}, nil
}

func (c *CLI) list(_ context.Context, opts options, positional []string) (any, error) {
	if len(positional) > 1 {
		return nil, cliError{2, errors.New("list accepts at most one kind")}
	}
	kind := ""
	if len(positional) == 1 {
		kind = positional[0]
	}
	if kind != "" && kind != "entity" && kind != "link" && kind != "route" {
		return nil, cliError{2, fmt.Errorf("invalid kind %q", kind)}
	}
	registry, _, _, err := c.load(opts, false)
	if err != nil {
		return nil, cliError{2, err}
	}
	return map[string]any{"objects": registry.ObjectIDs(kind)}, nil
}

func (c *CLI) show(ctx context.Context, opts options, positional []string) (any, error) {
	if len(positional) != 1 {
		return nil, cliError{2, errors.New("show requires one id")}
	}
	registry, runtime, statePath, err := c.load(opts, true)
	if err != nil {
		return nil, cliError{2, err}
	}
	store, err := OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	id, kind, err := registry.ResolveAny(positional[0])
	if err != nil {
		return nil, cliError{2, err}
	}
	result := map[string]any{"kind": kind}
	switch kind {
	case "entity":
		result["object"] = registry.Entities[id]
	case "link":
		result["object"] = registry.Links[id]
		result["observations"], err = store.AllLatest(ctx, id)
	case "route":
		result["object"] = registry.Routes[id]
		result["evidence"], err = registry.RouteEvidence(ctx, registry.Routes[id], runtime.Vantage, store)
	}
	return result, err
}

func (c *CLI) resolve(ctx context.Context, opts options, positional []string) (any, error) {
	if len(positional) != 1 || opts.Capability == "" {
		return nil, cliError{2, errors.New("resolve requires target and --capability")}
	}
	registry, runtime, statePath, err := c.load(opts, true)
	if err != nil {
		return nil, cliError{2, err}
	}
	store, err := OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	result, err := registry.Resolve(ctx, positional[0], opts.Capability, runtime, NewProviders(), store)
	if err != nil {
		if strings.HasPrefix(err.Error(), "no route") {
			return nil, cliError{3, err}
		}
		return nil, err
	}
	return result, nil
}

func (c *CLI) check(parent context.Context, opts options, positional []string) (any, error) {
	if len(positional) != 1 {
		return nil, cliError{2, errors.New("check requires one Link or Route id")}
	}
	registry, runtime, statePath, err := c.load(opts, true)
	if err != nil {
		return nil, cliError{2, err}
	}
	id, kind, err := registry.ResolveAny(positional[0])
	if err != nil {
		return nil, cliError{2, err}
	}
	if kind != "link" && kind != "route" {
		return nil, cliError{2, errors.New("check accepts only Link or Route")}
	}
	store, err := OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	timeout := 10 * time.Second
	if opts.Timeout != "" {
		timeout, err = time.ParseDuration(opts.Timeout)
		if err != nil {
			return nil, cliError{2, err}
		}
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	providers := NewProviders()
	links := []string{id}
	if kind == "route" {
		links = nil
		for _, step := range registry.Routes[id].Steps {
			links = append(links, step.Link)
		}
	}
	var observations []Observation
	failed := false
	for _, linkID := range links {
		if failed {
			break
		}
		link := registry.Links[linkID]
		if link.From != runtime.CurrentEntity {
			return nil, cliError{2, fmt.Errorf("link %s is not applicable from %s", linkID, runtime.CurrentEntity)}
		}
		provider, ok := providers.Get(link.Provider)
		if !ok {
			return nil, cliError{2, fmt.Errorf("unsupported provider %s", link.Provider)}
		}
		observation := provider.Probe(ctx, link, runtime)
		observation, err = store.Append(ctx, observation)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
		failed = observation.Status == "failure"
	}
	result := map[string]any{"observations": observations}
	if failed {
		return result, cliError{4, errors.New("check failed")}
	}
	return result, nil
}

func (c *CLI) status(ctx context.Context, opts options, positional []string) (any, error) {
	if len(positional) > 1 {
		return nil, cliError{2, errors.New("status accepts at most one id")}
	}
	registry, runtime, statePath, err := c.load(opts, true)
	if err != nil {
		return nil, cliError{2, err}
	}
	store, err := OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	if len(positional) == 1 {
		id, kind, err := registry.ResolveAny(positional[0])
		if err != nil {
			return nil, cliError{2, err}
		}
		switch kind {
		case "link":
			values, err := store.AllLatest(ctx, id)
			return map[string]any{"link_id": id, "observations": values}, err
		case "route":
			evidence, err := registry.RouteEvidence(ctx, registry.Routes[id], runtime.Vantage, store)
			return map[string]any{"route_id": id, "evidence": evidence}, err
		default:
			return nil, cliError{2, errors.New("status accepts only Link or Route")}
		}
	}
	counts := map[string]int{"failure": 0, "stale": 0, "unknown": 0, "success": 0}
	for id := range registry.Links {
		observation, err := store.Latest(ctx, id, runtime.Vantage)
		if err != nil {
			return nil, err
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
		host, _ := os.Hostname()
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
