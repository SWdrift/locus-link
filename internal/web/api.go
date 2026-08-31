package web

import (
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"

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
	imports := make([]importResponse, 0, len(registry.Aliases))
	for alias, scopeID := range registry.Aliases {
		imports = append(imports, importResponse{Alias: alias, ScopeID: scopeID})
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Alias < imports[j].Alias })
	contextValue := contextResponse{
		ActiveScope: registry.Manifest.Scope,
		Imports:     imports,
		Bindings:    registry.Bindings,
		Runtime:     runtimeResponse{CurrentEntity: currentEntity, Vantage: vantage},
	}

	ui, err := uiHandler()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/context", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(response).Encode(contextValue); err != nil {
			http.Error(response, "encode context", http.StatusInternalServerError)
		}
	})
	mux.Handle("/", ui)
	return secureLocalHandler(mux), nil
}

func secureLocalHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !localHost(request.Host) {
			http.Error(response, "invalid host", http.StatusForbidden)
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
