package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"locus-link/internal/locus"
)

type contextResponse struct {
	ActiveScope locus.Scope       `json:"active_scope"`
	Imports     []importResponse  `json:"imports"`
	Bindings    map[string]string `json:"bindings"`
	Runtime     runtimeResponse   `json:"runtime"`
}

type importResponse struct {
	Alias   string `json:"alias"`
	ScopeID string `json:"scope_id"`
}

type runtimeResponse struct {
	CurrentEntity string `json:"current_entity,omitempty"`
	Vantage       string `json:"vantage"`
}

type probeRequest struct {
	Subject   string `json:"subject"`
	From      string `json:"from"`
	Vantage   string `json:"vantage"`
	TimeoutMS int    `json:"timeout_ms"`
}

type apiHandler struct {
	registry       *locus.Registry
	providers      *locus.Providers
	statePath      string
	defaultFrom    string
	defaultVantage string
	context        contextResponse
}

func newHandler(config Config) (http.Handler, error) {
	registry, err := locus.LoadActiveRegistry(config.Registry)
	if err != nil {
		return nil, err
	}
	currentEntity := ""
	if config.From != "" {
		currentEntity, err = registry.ResolveEntity(config.From)
		if err != nil {
			return nil, err
		}
	}
	vantage, err := locus.ObservationVantage(config.Vantage)
	if err != nil {
		return nil, err
	}
	statePath, err := locus.DefaultStatePath()
	if err != nil {
		return nil, err
	}
	imports := make([]importResponse, 0, len(registry.Aliases))
	for alias, scopeID := range registry.Aliases {
		imports = append(imports, importResponse{Alias: alias, ScopeID: scopeID})
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Alias < imports[j].Alias })
	api := &apiHandler{
		registry: registry, providers: locus.NewProviders(), statePath: statePath,
		defaultFrom: currentEntity, defaultVantage: vantage,
		context: contextResponse{
			ActiveScope: registry.Manifest.Scope, Imports: imports, Bindings: registry.Bindings,
			Runtime: runtimeResponse{CurrentEntity: currentEntity, Vantage: vantage},
		},
	}
	ui, err := uiHandler()
	if err != nil {
		return nil, err
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
	vantage, err := locus.ObservationVantage(firstNonEmpty(request.URL.Query().Get("vantage"), a.defaultVantage))
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
	result, err := a.registry.Status(request.Context(), vantage, store)
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
	writeJSON(response, http.StatusOK, map[string]any{
		"valid": true, "active_scope": a.registry.Manifest.Scope.ID,
		"entities": len(a.registry.Entities), "links": len(a.registry.Links), "routes": len(a.registry.Routes),
	})
}

func (a *apiHandler) getResolve(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	target, capability := query.Get("target"), query.Get("capability")
	if target == "" || capability == "" {
		writeAPIError(response, http.StatusBadRequest, errors.New("target and capability are required"))
		return
	}
	runtime, err := locus.RequiredRuntime(a.registry, firstNonEmpty(query.Get("from"), a.defaultFrom), firstNonEmpty(query.Get("vantage"), a.defaultVantage))
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
	runtime, err := locus.RequiredRuntime(a.registry, firstNonEmpty(input.From, a.defaultFrom), firstNonEmpty(input.Vantage, a.defaultVantage))
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
