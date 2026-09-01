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

type apiHandler struct {
	registry              *locus.Registry
	providers             *locus.Providers
	statePath             string
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
	imports := make([]importResponse, 0, len(registry.Aliases))
	for alias, scopeID := range registry.Aliases {
		imports = append(imports, importResponse{Alias: alias, ScopeID: scopeID})
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Alias < imports[j].Alias })
	graph, err := registry.Graph()
	if err != nil {
		return nil, err
	}
	bindings := map[string]string{}
	for _, binding := range registry.Bindings {
		if binding.ScopeID == registry.RootScopeID {
			bindings[binding.ID] = binding.Target
		}
	}
	api := &apiHandler{
		registry: registry, providers: providers, statePath: statePath,
		defaultFrom: runtime.CurrentEntity, defaultVantage: runtime.Vantage, mechanismBindingsPath: config.MechanismBindings,
		context: contextResponse{
			ActiveScope: scopeResponse{ID: registry.RootScopeID}, Root: root, Imports: imports,
			ImportEdges: append([]locus.ImportEdge{}, registry.ImportEdges...), Bindings: bindings,
			BindingDetails: graph.Bindings, Runtime: runtime, ObservationStore: statePath,
			Completeness: registry.Completeness, BlockedImports: append([]locus.BlockedImport{}, registry.BlockedImports...),
		},
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

func (a *apiHandler) getContext(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, a.context)
}

func (a *apiHandler) getGraph(response http.ResponseWriter, _ *http.Request) {
	result, err := a.registry.Graph()
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) getStatus(response http.ResponseWriter, request *http.Request) {
	runtime, err := a.runtime(request.URL.Query().Get("from"), request.URL.Query().Get("vantage"))
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
	result, err := a.registry.Status(request.Context(), runtime, a.providers, store)
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) getKnowledge(response http.ResponseWriter, _ *http.Request) {
	result, err := a.registry.Documents()
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"documents": result})
}

func (a *apiHandler) getDocument(response http.ResponseWriter, request *http.Request) {
	result, err := a.registry.Document(request.PathValue("id"))
	if err != nil {
		writeAPIError(response, http.StatusNotFound, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (a *apiHandler) getValidation(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, validationResponse{
		Valid: a.registry.Completeness == locus.Complete, ActiveScope: a.registry.RootScopeID,
		Entities: len(a.registry.Entities), Links: len(a.registry.Links), Routes: len(a.registry.Routes),
		Completeness:   a.registry.Completeness,
		BlockedImports: append([]locus.BlockedImport{}, a.registry.BlockedImports...),
	})
}

func (a *apiHandler) getResolve(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	target, capability := query.Get("target"), query.Get("capability")
	if target == "" || capability == "" {
		writeAPIError(response, http.StatusBadRequest, errors.New("target and capability are required"))
		return
	}
	runtime, err := a.runtime(query.Get("from"), query.Get("vantage"))
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
	result, err := a.registry.Resolve(request.Context(), target, capability, runtime, a.providers, store)
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
	runtime, err := a.runtime(input.From, input.Vantage)
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
	result, err := a.registry.Probe(ctx, input.Subject, runtime, a.providers, store)
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

func (a *apiHandler) runtime(from, vantage string) (locus.RuntimeContext, error) {
	return locus.BuildRuntime(a.registry, locus.RuntimeInput{
		From:                  firstNonEmpty(from, a.defaultFrom),
		Vantage:               firstNonEmpty(vantage, a.defaultVantage),
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
