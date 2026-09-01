package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var embeddedUI embed.FS

func Handler() (http.Handler, error) {
	dist, err := fs.Sub(embeddedUI, "dist")
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		if info, statErr := fs.Stat(dist, name); statErr == nil && !info.IsDir() {
			http.ServeFileFS(response, request, dist, name)
			return
		}
		if path.Ext(name) != "" {
			http.NotFound(response, request)
			return
		}
		http.ServeFileFS(response, request, dist, "index.html")
	}), nil
}
