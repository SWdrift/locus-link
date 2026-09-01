package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"locus-link/internal/locus"
)

type contextResponse struct {
	ActiveScope      scopeResponse         `json:"active_scope"`
	Root             locus.RootContext     `json:"root"`
	Imports          []importResponse      `json:"imports"`
	ImportEdges      []locus.ImportEdge    `json:"import_edges"`
	Bindings         map[string]string     `json:"bindings"`
	BindingDetails   []locus.BindingView   `json:"binding_details"`
	Runtime          locus.RuntimeContext  `json:"runtime"`
	ObservationStore string                `json:"observation_store"`
	Completeness     locus.Completeness    `json:"completeness"`
	BlockedImports   []locus.BlockedImport `json:"blocked_imports"`
}

type scopeResponse struct {
	ID string `json:"id"`
}

type importResponse struct {
	Alias   string `json:"alias"`
	ScopeID string `json:"scope_id"`
}

type validationResponse struct {
	Valid          bool                  `json:"valid"`
	ActiveScope    string                `json:"active_scope"`
	Entities       int                   `json:"entities"`
	Links          int                   `json:"links"`
	Routes         int                   `json:"routes"`
	Completeness   locus.Completeness    `json:"completeness"`
	BlockedImports []locus.BlockedImport `json:"blocked_imports"`
}

type probeRequest struct {
	Subject   string `json:"subject"`
	From      string `json:"from"`
	Vantage   string `json:"vantage"`
	TimeoutMS int    `json:"timeout_ms"`
}

type refreshRequest struct {
	ScopeID                 string `json:"scope_id"`
	AliasPath               string `json:"alias_path"`
	AllowRegression         bool   `json:"allow_regression"`
	ExpectedCandidateDigest string `json:"expected_candidate_digest"`
}

type apiHandler struct {
	registry              *locus.Registry
	providers             *locus.Providers
	statePath             string
	home                  string
	defaultFrom           string
	defaultVantage        string
	mechanismBindingsPath string
	context               contextResponse
}

func newHandler(config Config, uiFactory UIFactory) (http.Handler, error) {
	statePath, err := locus.DefaultStatePath()
	if err != nil {
		return nil, err
	}
	store, err := locus.OpenStore(statePath)
	if err != nil {
		return nil, err
	}
	registry, root, err := locus.LoadRegistryContext(config.Registry, store)
	closeErr := store.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	runtime := locus.RuntimeContext{MechanismBindingsSource: config.MechanismBindings}
	if config.From != "" {
		runtime, err = locus.BuildRuntime(registry, locus.RuntimeInput{
			From: config.From, Vantage: config.Vantage, MechanismBindingsPath: config.MechanismBindings,
		})
	} else {
		runtime.Vantage, err = locus.ObservationVantage(config.Vantage)
	}
	if err != nil {
		return nil, err
	}
	runtime.CWD, err = os.Getwd()
	if err != nil {
		return nil, err
	}
	providers := locus.NewProviders()
	runtime.AvailableTools = providers.Available()
	initialContext, err := buildContextResponse(registry, root, runtime, statePath)
	if err != nil {
		return nil, err
	}
	home, err := locus.DefaultHome()
	if err != nil {
		return nil, err
	}
	api := &apiHandler{
		registry: registry, providers: providers, statePath: statePath, home: home,
		defaultFrom: runtime.CurrentEntity, defaultVantage: runtime.Vantage, mechanismBindingsPath: config.MechanismBindings,
		context: initialContext,
	}
	var ui http.Handler = http.NotFoundHandler()
	if uiFactory != nil {
		ui, err = uiFactory()
		if err != nil {
			return nil, err
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/context", api.getContext)
	mux.HandleFunc("GET /api/v0/locus/scopes", api.getLocusScopes)
	mux.HandleFunc("GET /api/v0/locus/dependencies", api.getDependencies)
	mux.HandleFunc("POST /api/v0/locus/refresh", api.postRefresh)
	mux.HandleFunc("GET /api/v0/graph", api.getGraph)
	mux.HandleFunc("GET /api/v0/status", api.getStatus)
	mux.HandleFunc("GET /api/v0/knowledge", api.getKnowledge)
	mux.HandleFunc("GET /api/v0/knowledge/{id}", api.getDocument)
	mux.HandleFunc("GET /api/v0/validate", api.getValidation)
	mux.HandleFunc("GET /api/v0/resolve", api.getResolve)
	mux.HandleFunc("POST /api/v0/probes", api.postProbe)
	mux.HandleFunc("/api/", func(response http.ResponseWriter, _ *http.Request) {
		writeAPIError(response, http.StatusNotFound, errors.New("unknown API endpoint"))
	})
	mux.Handle("/", ui)
	return secureLocalHandler(mux), nil
}

func (a *apiHandler) getContext(response http.ResponseWriter, request *http.Request) {
	if request.URL.Query().Get("scope") == "" {
		writeJSON(response, http.StatusOK, a.context)
		return
	}
	registry, root, err := a.registryForRequest(request)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	runtime, err := a.defaultRuntime(registry)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	result, err := buildContextResponse(registry, root, runtime, a.statePath)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) getGraph(response http.ResponseWriter, request *http.Request) {
	registry, _, err := a.registryForRequest(request)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	result, err := registry.Graph()
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) getStatus(response http.ResponseWriter, request *http.Request) {
	registry, _, err := a.registryForRequest(request)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	runtime, err := a.runtime(registry, request.URL.Query().Get("from"), request.URL.Query().Get("vantage"))
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	store, err := locus.OpenStore(a.statePath)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	result, err := registry.Status(request.Context(), runtime, a.providers, store)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) getKnowledge(response http.ResponseWriter, request *http.Request) {
	registry, _, err := a.registryForRequest(request)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	result, err := registry.Documents()
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"documents": result})
}

func (a *apiHandler) getDocument(response http.ResponseWriter, request *http.Request) {
	registry, _, err := a.registryForRequest(request)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	result, err := registry.Document(request.PathValue("id"))
	if err != nil {
		writeAPIError(response, http.StatusNotFound, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) getValidation(response http.ResponseWriter, request *http.Request) {
	registry, _, err := a.registryForRequest(request)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	writeJSON(response, http.StatusOK, validationResponse{
		Valid: registry.Completeness == locus.Complete, ActiveScope: registry.RootScopeID,
		Entities: len(registry.Entities), Links: len(registry.Links), Routes: len(registry.Routes),
		Completeness:   registry.Completeness,
		BlockedImports: append([]locus.BlockedImport{}, registry.BlockedImports...),
	})
}

func (a *apiHandler) getResolve(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	target, capability := query.Get("target"), query.Get("capability")
	if target == "" || capability == "" {
		writeAPIError(response, http.StatusBadRequest, errors.New("target and capability are required"))
		return
	}
	registry, _, err := a.registryForRequest(request)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	runtime, err := a.runtime(registry, query.Get("from"), query.Get("vantage"))
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	store, err := locus.OpenStore(a.statePath)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	result, err := registry.Resolve(request.Context(), target, capability, runtime, a.providers, store)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) postProbe(response http.ResponseWriter, request *http.Request) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(response, http.StatusUnsupportedMediaType, errors.New("content type must be application/json"))
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 64<<10)
	var input probeRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(response, http.StatusBadRequest, errors.New("invalid probe request"))
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeAPIError(response, http.StatusBadRequest, errors.New("probe request must contain one JSON object"))
		return
	}
	if input.Subject == "" {
		writeAPIError(response, http.StatusBadRequest, errors.New("subject is required"))
		return
	}
	if input.TimeoutMS == 0 {
		input.TimeoutMS = 10_000
	}
	if input.TimeoutMS < 1 || input.TimeoutMS > 60_000 {
		writeAPIError(response, http.StatusBadRequest, errors.New("timeout_ms must be between 1 and 60000"))
		return
	}
	registry, _, err := a.registryForRequest(request)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	runtime, err := a.runtime(registry, input.From, input.Vantage)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	store, err := locus.OpenStore(a.statePath)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(request.Context(), time.Duration(input.TimeoutMS)*time.Millisecond)
	defer cancel()
	result, err := registry.Probe(ctx, input.Subject, runtime, a.providers, store)
	if err != nil {
		var inputError locus.ProbeInputError
		if errors.As(err, &inputError) {
			writeAPIError(response, http.StatusBadRequest, err)
		} else {
			writeAPIError(response, http.StatusInternalServerError, err)
		}
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) getLocusScopes(response http.ResponseWriter, request *http.Request) {
	store, err := locus.OpenStore(a.statePath)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	values, err := store.LocusCatalog(request.Context(), a.home, a.registry.RootScopeID)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"scopes": values})
}

func (a *apiHandler) getDependencies(response http.ResponseWriter, request *http.Request) {
	scopeID := firstNonEmpty(request.URL.Query().Get("root"), a.registry.RootScopeID)
	registry, _, err := a.registryForScope(request.Context(), scopeID)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	snapshot := locus.SnapshotDependency(registry)
	store, err := locus.OpenStore(a.statePath)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	catalog, err := store.LocusCatalog(request.Context(), a.home, a.registry.RootScopeID)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	entries := make(map[string]locus.LocusScopeEntry, len(catalog))
	for _, entry := range catalog {
		entries[entry.ScopeID] = entry
	}
	for index := range snapshot.Nodes {
		if entry, ok := entries[snapshot.Nodes[index].ScopeID]; ok {
			snapshot.Nodes[index].Kind = entry.Kind
			snapshot.Nodes[index].Openable = entry.Openable
			snapshot.Nodes[index].Availability = entry.Availability
		}
	}
	writeJSON(response, http.StatusOK, snapshot)
}

func (a *apiHandler) postRefresh(response http.ResponseWriter, request *http.Request) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(response, http.StatusUnsupportedMediaType, errors.New("content type must be application/json"))
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 64<<10)
	var input refreshRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(response, http.StatusBadRequest, errors.New("invalid refresh request"))
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeAPIError(response, http.StatusBadRequest, errors.New("refresh request must contain one JSON object"))
		return
	}
	scopeID := firstNonEmpty(input.ScopeID, a.registry.RootScopeID)
	_, root, err := a.registryForScope(request.Context(), scopeID)
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	store, err := locus.OpenStore(a.statePath)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	result, err := locus.RefreshRegistry(request.Context(), root.RegistryPath, input.AliasPath, locus.RefreshOptions{
		Home: a.home, Store: store, AllowRegression: input.AllowRegression,
		ExpectedCandidateDigest: input.ExpectedCandidateDigest,
	})
	if err != nil {
		writeAPIError(response, http.StatusBadRequest, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) registryForRequest(request *http.Request) (*locus.Registry, locus.RootContext, error) {
	scopeID := request.URL.Query().Get("scope")
	if scopeID == "" || scopeID == a.registry.RootScopeID {
		return a.registry, a.context.Root, nil
	}
	return a.registryForScope(request.Context(), scopeID)
}

func (a *apiHandler) registryForScope(ctx context.Context, scopeID string) (*locus.Registry, locus.RootContext, error) {
	if scopeID == a.registry.RootScopeID {
		return a.registry, a.context.Root, nil
	}
	store, err := locus.OpenStore(a.statePath)
	if err != nil {
		return nil, locus.RootContext{}, err
	}
	defer store.Close()
	root, err := store.OpenableScopePath(ctx, a.home, scopeID)
	if err != nil {
		return nil, locus.RootContext{}, err
	}
	return locus.LoadRegistryContext(root, store)
}

func (a *apiHandler) defaultRuntime(registry *locus.Registry) (locus.RuntimeContext, error) {
	return a.runtime(registry, "", a.defaultVantage)
}

func buildContextResponse(registry *locus.Registry, root locus.RootContext, runtime locus.RuntimeContext, statePath string) (contextResponse, error) {
	imports := make([]importResponse, 0, len(registry.Aliases))
	for alias, scopeID := range registry.Aliases {
		imports = append(imports, importResponse{Alias: alias, ScopeID: scopeID})
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Alias < imports[j].Alias })
	graph, err := registry.Graph()
	if err != nil {
		return contextResponse{}, err
	}
	bindings := map[string]string{}
	for _, binding := range registry.Bindings {
		if binding.ScopeID == registry.RootScopeID {
			bindings[binding.ID] = binding.Target
		}
	}
	return contextResponse{
		ActiveScope: scopeResponse{ID: registry.RootScopeID}, Root: root, Imports: imports,
		ImportEdges: append([]locus.ImportEdge{}, registry.ImportEdges...), Bindings: bindings,
		BindingDetails: graph.Bindings, Runtime: runtime, ObservationStore: statePath,
		Completeness: registry.Completeness, BlockedImports: append([]locus.BlockedImport{}, registry.BlockedImports...),
	}, nil
}

func (a *apiHandler) runtime(registry *locus.Registry, from, vantage string) (locus.RuntimeContext, error) {
	selectedFrom := from
	if selectedFrom == "" && registry.RootScopeID == a.registry.RootScopeID {
		selectedFrom = a.defaultFrom
	}
	if selectedFrom == "" {
		ids := make([]string, 0, len(registry.Entities))
		for id, entity := range registry.Entities {
			if entity.ScopeID == registry.RootScopeID {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		if len(ids) != 0 {
			selectedFrom = ids[0]
		}
	}
	if selectedFrom == "" {
		runtime := locus.RuntimeContext{Vantage: firstNonEmpty(vantage, a.defaultVantage), MechanismBindingsSource: a.mechanismBindingsPath}
		var err error
		runtime.CWD, err = os.Getwd()
		runtime.AvailableTools = a.providers.Available()
		return runtime, err
	}
	return locus.BuildRuntime(registry, locus.RuntimeInput{
		From: selectedFrom, Vantage: firstNonEmpty(vantage, a.defaultVantage),
		MechanismBindingsPath: a.mechanismBindingsPath,
	})
}

func secureLocalHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !localHost(request.Host) {
			http.Error(response, "invalid host", http.StatusForbidden)
			return
		}
		if site := request.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "same-site" && site != "none" {
			http.Error(response, "cross-site request rejected", http.StatusForbidden)
			return
		}
		response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(response, request)
	})
}

func localHost(value string) bool {
	host := value
	if parsed, _, err := net.SplitHostPort(value); err == nil {
		host = parsed
	} else if strings.HasPrefix(value, "[") {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeAPIError(response http.ResponseWriter, status int, err error) {
	writeJSON(response, status, map[string]string{"error": err.Error()})
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
