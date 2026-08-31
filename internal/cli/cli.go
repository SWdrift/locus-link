package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"locus-link/internal/locus"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

const (
	coreGroup       = "core"
	inspectionGroup = "inspection"
	authoringGroup  = "authoring"
)

type CLI struct {
	stdout     io.Writer
	stderr     io.Writer
	extensions []*cobra.Command
}

type options struct {
	Registry          string
	From              string
	Vantage           string
	MechanismBindings string
	Timeout           string
	Capability        string
	ScopeID           string
	ImportUser        string
	Register          bool
}

type cliError struct {
	code int
	err  error
}

func (e cliError) Error() string { return e.err.Error() }
func (e cliError) Unwrap() error { return e.err }

func NewCLI(stdout, stderr io.Writer, extensions ...*cobra.Command) *CLI {
	return &CLI{stdout: stdout, stderr: stderr, extensions: extensions}
}

type commandState struct {
	result any
	json   bool
}

type declarationAssembly struct {
	registry  *locus.Registry
	root      locus.RootContext
	statePath string
}

type runtimeStateAssembly struct {
	declarationAssembly
	runtime locus.RuntimeContext
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
		Use: "locus", Short: "Resolve explicit operational routes and inspect their evidence",
		SilenceErrors: true, SilenceUsage: true,
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
		c.resolveCommand(state), c.probeCommand(state),
		c.contextCommand(state), c.graphCommand(state), c.showCommand(state), c.listCommand(state), c.statusCommand(state),
		c.initCommand(state), c.userCommand(state), c.projectCommand(state), c.refreshCommand(state), c.validateCommand(state),
	)
	root.AddCommand(c.extensions...)
	return root
}

func (c *CLI) initCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use: "init --scope-id <id>", Short: "Create a Scope registry", GroupID: authoringGroup, Args: exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := c.init(opts, false)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	command.Flags().StringVar(&opts.ScopeID, "scope-id", "", "canonical scope ID")
	command.Flags().StringVar(&opts.ImportUser, "import-user", "", "alias for an explicit user Registry import")
	command.Flags().BoolVar(&opts.Register, "register", false, "register the created project in the user Locus")
	return command
}

func (c *CLI) userCommand(state *commandState) *cobra.Command {
	command := &cobra.Command{Use: "user", Short: "Manage the user-level Locus", GroupID: authoringGroup}
	var opts options
	initCommand := &cobra.Command{
		Use: "init --scope-id <id>", Short: "Create the user Registry", Args: exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := c.init(opts, true)
			return state.set(result, err)
		},
	}
	initCommand.Flags().StringVar(&opts.ScopeID, "scope-id", "", "canonical scope ID")
	command.AddCommand(initCommand)
	return command
}

func (c *CLI) projectCommand(state *commandState) *cobra.Command {
	command := &cobra.Command{Use: "project", Short: "Manage project registrations", GroupID: authoringGroup}
	var registerOptions options
	register := &cobra.Command{
		Use: "register", Short: "Register a project Registry", Args: exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := c.projectRegister(registerOptions)
			return state.set(result, err)
		},
	}
	addRegistryFlag(register, &registerOptions)
	unregister := &cobra.Command{
		Use: "unregister <scope-id>", Short: "Remove a project registration", Args: exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := c.projectUnregister(args[0])
			return state.set(result, err)
		},
	}
	list := &cobra.Command{
		Use: "list", Short: "List project registrations", Args: exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := c.projectList()
			return state.set(result, err)
		},
	}
	command.AddCommand(register, unregister, list)
	return command
}

func (c *CLI) refreshCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use: "refresh [alias-path]", Short: "Refresh remote Scope imports", GroupID: authoringGroup, Args: maximumArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := c.refresh(opts, args)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	return command
}

func (c *CLI) validateCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use: "validate", Short: "Validate registry declarations", GroupID: authoringGroup, Args: exactArgs(0),
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
		Use: "context", Short: "Show the current operational context and vantage", GroupID: inspectionGroup, Args: exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error {
			result, err := c.context(opts)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	addRuntimeFlags(command, &opts)
	return command
}

func (c *CLI) graphCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use: "graph", Short: "Show the composed Scope graph", GroupID: inspectionGroup, Args: exactArgs(0),
		RunE: func(_ *cobra.Command, _ []string) error { result, err := c.graph(opts); return state.set(result, err) },
	}
	addRegistryFlag(command, &opts)
	return command
}

func (c *CLI) listCommand(state *commandState) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use: "list [binding|entity|link|route]", Short: "List registry declarations", GroupID: inspectionGroup, Args: maximumArgs(1),
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
		Use: "show <ref-or-id>", Short: "Resolve a reference and show its declaration", GroupID: inspectionGroup, Args: exactArgs(1),
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
		Use: "resolve <target> --capability <name>", Short: "Resolve an explicit route and attach current evidence", GroupID: coreGroup, Args: exactArgs(1),
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
		Use: "probe <link-or-route-id>", Short: "Safely measure links and write new observations", GroupID: coreGroup, Args: exactArgs(1),
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
		Use: "status [link-or-route-id]", Short: "Inspect persisted link observations or derived route evidence", GroupID: inspectionGroup, Args: maximumArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := c.status(args, opts)
			return state.set(result, err)
		},
	}
	addRegistryFlag(command, &opts)
	addRuntimeFlags(command, &opts)
	return command
}

func addRegistryFlag(command *cobra.Command, opts *options) {
	command.Flags().StringVar(&opts.Registry, "registry", "", "registry path override")
}

func addRuntimeFlags(command *cobra.Command, opts *options) {
	command.Flags().StringVar(&opts.From, "from", "", "current operational entity")
	command.Flags().StringVar(&opts.Vantage, "vantage", "", "network observation vantage")
	command.Flags().StringVar(&opts.MechanismBindings, "mechanism-bindings", "", "workstation-local mechanism bindings file")
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

func (c *CLI) init(opts options, user bool) (any, error) {
	if !locus.ValidIdentifier(opts.ScopeID) {
		return nil, cliError{code: 2, err: errors.New("--scope-id must be a valid canonical scope ID")}
	}
	layout, err := locus.LocusHomeLayout()
	if err != nil {
		return nil, err
	}
	root := opts.Registry
	if user {
		root = layout.Registry
	} else if root == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return nil, cwdErr
		}
		root = filepath.Join(cwd, ".locus", "registry")
	}
	manifest := locus.Manifest{APIVersion: locus.APIVersion, ScopeID: opts.ScopeID, Imports: map[string]locus.Import{}, Bindings: map[string]string{}}
	if opts.ImportUser != "" {
		if user {
			return nil, cliError{code: 2, err: errors.New("user init does not accept --import-user")}
		}
		if !locus.ValidIdentifier(opts.ImportUser) {
			return nil, cliError{code: 2, err: errors.New("--import-user must be a valid alias")}
		}
		userRegistry, loadErr := locus.LoadScopeRegistry(layout.Registry, false)
		if loadErr != nil {
			return nil, cliError{code: 2, err: fmt.Errorf("load user Registry: %w", loadErr)}
		}
		manifest.Imports[opts.ImportUser] = locus.Import{
			Alias: opts.ImportUser, ExpectedScopeID: userRegistry.Manifest.ScopeID,
			Source: locus.Source{Kind: "directory", URI: locus.LocusHomeRegistryURI},
		}
	}
	absolute, err := createRegistry(root, manifest)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	result := map[string]any{"registry": absolute, "scope_id": manifest.ScopeID}
	if opts.Register {
		if user {
			return nil, cliError{code: 2, err: errors.New("user init does not accept --register")}
		}
		store, statePath, openErr := openDefaultStore()
		if openErr != nil {
			return nil, openErr
		}
		defer store.Close()
		registration, registerErr := store.RegisterProject(context.Background(), absolute, layout.Root)
		if registerErr != nil {
			return nil, cliError{code: 2, err: registerErr}
		}
		result["registration"] = registration
		result["state_path"] = statePath
	}
	return result, nil
}

func createRegistry(root string, manifest locus.Manifest) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if entries, readErr := os.ReadDir(absolute); readErr == nil && len(entries) != 0 {
		return "", errors.New("registry directory already exists and is not empty")
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return "", readErr
	}
	for _, directory := range []string{"entities", "links", "routes", "docs"} {
		if err := os.MkdirAll(filepath.Join(absolute, directory), 0o755); err != nil {
			return "", err
		}
	}
	data, err := yaml.Dump(manifest, yaml.WithV3Defaults())
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(absolute, "scope.yaml"), data, 0o644); err != nil {
		return "", err
	}
	if _, err := locus.LoadScopeRegistry(absolute, false); err != nil {
		return "", err
	}
	return absolute, nil
}

func (c *CLI) projectRegister(opts options) (any, error) {
	root := opts.Registry
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root, err = locus.DiscoverRegistry(cwd)
		if err != nil {
			return nil, cliError{code: 2, err: err}
		}
	}
	layout, err := locus.LocusHomeLayout()
	if err != nil {
		return nil, err
	}
	store, statePath, err := openDefaultStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	registration, err := store.RegisterProject(context.Background(), root, layout.Root)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	return map[string]any{"registration": registration, "state_path": statePath}, nil
}

func (c *CLI) projectUnregister(scopeID string) (any, error) {
	store, statePath, err := openDefaultStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	removed, err := store.UnregisterProject(context.Background(), scopeID)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	return map[string]any{"scope_id": scopeID, "removed": removed, "state_path": statePath}, nil
}

func (c *CLI) projectList() (any, error) {
	store, statePath, err := openDefaultStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	projects, err := store.ListProjects(context.Background())
	if err != nil {
		return nil, err
	}
	return map[string]any{"projects": projects, "state_path": statePath}, nil
}

func (c *CLI) refresh(opts options, args []string) (any, error) {
	assembly, err := c.loadDeclarationState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	store, _, err := openDefaultStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	aliasPath := ""
	if len(args) == 1 {
		aliasPath = args[0]
	}
	layout, err := locus.LocusHomeLayout()
	if err != nil {
		return nil, err
	}
	result, err := locus.RefreshRegistry(context.Background(), assembly.registry.Root, aliasPath, locus.RefreshOptions{Home: layout.Root, Store: store})
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	if len(result.RefreshErrors) != 0 {
		return result, cliError{code: 5, err: errors.New("refresh completed with failures")}
	}
	return result, nil
}

func (c *CLI) validate(opts options) (any, error) {
	assembly, err := c.loadDeclarationState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	result := map[string]any{
		"valid":        assembly.registry.Completeness == locus.Complete,
		"active_scope": assembly.registry.RootScopeID,
		"entities":     len(assembly.registry.Entities), "links": len(assembly.registry.Links), "routes": len(assembly.registry.Routes),
	}
	attachViewDiagnostics(result, assembly.registry)
	if assembly.registry.Completeness == locus.Partial {
		return result, cliError{code: 2, err: errors.New("registry view is partial")}
	}
	return result, nil
}

func (c *CLI) context(opts options) (any, error) {
	assembly, err := c.assembleContextState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	result := map[string]any{
		"active_scope":      assembly.registry.RootScopeID,
		"root":              assembly.root,
		"imports":           assembly.registry.ImportEdges,
		"bindings":          assembly.registry.Bindings,
		"runtime":           assembly.runtime,
		"observation_store": assembly.statePath,
	}
	attachViewDiagnostics(result, assembly.registry)
	return result, nil
}

func (c *CLI) graph(opts options) (any, error) {
	assembly, err := c.loadDeclarationState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	return assembly.registry.Graph()
}

func (c *CLI) list(opts options, args []string) (any, error) {
	kind := ""
	if len(args) == 1 {
		kind = args[0]
	}
	if kind != "" && kind != "binding" && kind != "entity" && kind != "link" && kind != "route" {
		return nil, cliError{code: 2, err: fmt.Errorf("invalid kind %q", kind)}
	}
	assembly, err := c.loadDeclarationState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	result := map[string]any{"objects": assembly.registry.ObjectIDs(kind)}
	attachViewDiagnostics(result, assembly.registry)
	return result, nil
}

func (c *CLI) show(opts options, inputRef string) (any, error) {
	assembly, err := c.loadDeclarationState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	id, kind, err := assembly.registry.ResolveAny(inputRef)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	result := map[string]any{"input_ref": inputRef, "ref_type": kind, "canonical_id": id}
	switch kind {
	case "binding":
		binding := assembly.registry.Bindings[id]
		result["canonical_target"] = binding.Target
		result["object"] = binding
		result["target_object"] = assembly.registry.Entities[binding.Target]
	case "entity":
		result["object"] = assembly.registry.Entities[id]
	case "link":
		result["object"] = assembly.registry.Links[id]
	case "route":
		result["object"] = assembly.registry.Routes[id]
	}
	attachViewDiagnostics(result, assembly.registry)
	return result, nil
}

func (c *CLI) resolve(target string, opts options) (any, error) {
	if opts.Capability == "" {
		return nil, cliError{code: 2, err: errors.New("--capability is required")}
	}
	assembly, err := c.assembleRuntimeState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	store, err := locus.OpenStore(assembly.statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	result, err := assembly.registry.Resolve(context.Background(), target, opts.Capability, assembly.runtime, locus.NewProviders(), store)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	if result.Status != "resolved" {
		return result, cliError{code: 3, err: fmt.Errorf("resolve %s", result.Status)}
	}
	return result, nil
}

func (c *CLI) probe(parent context.Context, inputRef string, opts options) (any, error) {
	assembly, err := c.assembleRuntimeState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	timeout, err := time.ParseDuration(opts.Timeout)
	if err != nil || timeout <= 0 {
		return nil, cliError{code: 2, err: fmt.Errorf("invalid --timeout %q", opts.Timeout)}
	}
	store, err := locus.OpenStore(assembly.statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	result, err := assembly.registry.Probe(ctx, inputRef, assembly.runtime, locus.NewProviders(), store)
	if err != nil {
		var inputError locus.ProbeInputError
		if errors.As(err, &inputError) {
			return nil, cliError{code: 2, err: err}
		}
		return nil, err
	}
	if result.Status == "failure" {
		return result, cliError{code: 4, err: errors.New("probe failed")}
	}
	return result, nil
}

func (c *CLI) status(args []string, opts options) (any, error) {
	assembly, err := c.assembleRuntimeState(opts)
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	store, err := locus.OpenStore(assembly.statePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	ctx := context.Background()
	providers := locus.NewProviders()
	if len(args) == 0 {
		return assembly.registry.Status(ctx, assembly.runtime, providers, store)
	}
	id, kind, err := assembly.registry.ResolveAny(args[0])
	if err != nil {
		return nil, cliError{code: 2, err: err}
	}
	result := map[string]any{"current_entity": assembly.runtime.CurrentEntity, "vantage": assembly.runtime.Vantage}
	switch kind {
	case "link":
		evidence, evidenceErr := assembly.registry.LinkEvidence(ctx, id, assembly.runtime, providers, store)
		result["link_id"] = id
		result["evidence"] = evidence
		err = evidenceErr
	case "route":
		evidence, evidenceErr := assembly.registry.RouteEvidence(ctx, assembly.registry.Routes[id], assembly.runtime, providers, store)
		result["route_id"] = id
		result["evidence"] = evidence
		err = evidenceErr
	default:
		return nil, cliError{code: 2, err: errors.New("status accepts only Link or Route")}
	}
	attachViewDiagnostics(result, assembly.registry)
	return result, err
}

func (c *CLI) loadDeclarationState(opts options) (declarationAssembly, error) {
	store, statePath, err := openDefaultStore()
	if err != nil {
		return declarationAssembly{}, err
	}
	defer store.Close()
	registry, root, err := locus.LoadRegistryContext(opts.Registry, store)
	if err != nil {
		return declarationAssembly{}, err
	}
	return declarationAssembly{registry: registry, root: root, statePath: statePath}, nil
}

func (c *CLI) assembleContextState(opts options) (runtimeStateAssembly, error) {
	assembly, err := c.assembleRuntimeState(opts)
	if err != nil {
		return runtimeStateAssembly{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return runtimeStateAssembly{}, err
	}
	assembly.runtime.CWD = cwd
	assembly.runtime.AvailableTools = locus.NewProviders().Available()
	return assembly, nil
}

func (c *CLI) assembleRuntimeState(opts options) (runtimeStateAssembly, error) {
	declarations, err := c.loadDeclarationState(opts)
	if err != nil {
		return runtimeStateAssembly{}, err
	}
	runtime, err := locus.BuildRuntime(declarations.registry, locus.RuntimeInput{
		From: opts.From, Vantage: opts.Vantage, MechanismBindingsPath: opts.MechanismBindings,
	})
	if err != nil {
		return runtimeStateAssembly{}, err
	}
	return runtimeStateAssembly{declarationAssembly: declarations, runtime: runtime}, nil
}

func openDefaultStore() (*locus.Store, string, error) {
	statePath, err := locus.DefaultStatePath()
	if err != nil {
		return nil, "", err
	}
	store, err := locus.OpenStore(statePath)
	return store, statePath, err
}

func attachViewDiagnostics(result map[string]any, registry *locus.Registry) {
	result["completeness"] = registry.Completeness
	result["blocked_imports"] = registry.BlockedImports
}
