package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"locus-link/internal/locus"
)

func TestContextAndEmbeddedUI(t *testing.T) {
	registry := writeTestRegistry(t)
	t.Setenv("LOCUS_STATE_PATH", filepath.Join(registry, "state.db"))
	t.Setenv("LOCUS_HOME", filepath.Join(registry, "home"))
	server, err := New(Config{Registry: registry, From: "workstation", Vantage: "office-lan"})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://localhost/api/v0/context", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("context status = %d: %s", response.Code, response.Body.String())
	}
	var value contextResponse
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	if value.ActiveScope.ID != "project.web" || value.Runtime.CurrentEntity != "project.web::workstation" || value.Runtime.Vantage != "office-lan" {
		t.Fatalf("unexpected context: %#v", value)
	}

	request = httptest.NewRequest(http.MethodGet, "http://localhost/api/v0/graph", nil)
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var graph locus.GraphView
	if response.Code != http.StatusOK {
		t.Fatalf("graph status = %d: %s", response.Code, response.Body.String())
	}
	if err := json.NewDecoder(response.Body).Decode(&graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Entities) != 2 || len(graph.Links) != 1 || len(graph.Routes) != 1 || len(graph.Entities[0].DocumentationIDs) != 1 {
		t.Fatalf("unexpected graph: %#v", graph)
	}

	request = httptest.NewRequest(http.MethodGet, "http://localhost/api/v0/status?vantage=office-lan", nil)
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var status locus.StatusView
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || status.Summary["unknown"] != 1 || len(status.Routes) != 1 || status.Routes[0].Evidence.Status != "unknown" {
		t.Fatalf("unexpected status response %d: %#v", response.Code, status)
	}

	request = httptest.NewRequest(http.MethodGet, "http://localhost/api/v0/resolve?target=target&capability=salt.ping&from=workstation&vantage=office-lan", nil)
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var resolved locus.ResolveResult
	if err := json.NewDecoder(response.Body).Decode(&resolved); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || resolved.Status != "resolved" || resolved.Target != "project.web::target" {
		t.Fatalf("unexpected resolve response %d: %#v", response.Code, resolved)
	}

	request = httptest.NewRequest(http.MethodPost, "http://localhost/api/v0/probes", strings.NewReader(`{\"subject\":\"route.safe\"} {}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("trailing probe JSON status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	request = httptest.NewRequest(http.MethodGet, "http://localhost/api/v0/missing", nil)
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unknown API response = %d %q", response.Code, response.Header().Get("Content-Type"))
	}

	request = httptest.NewRequest(http.MethodGet, "http://localhost/api/v0/knowledge", nil)
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var knowledge struct {
		Documents []locus.DocumentView `json:"documents"`
	}
	if err := json.NewDecoder(response.Body).Decode(&knowledge); err != nil {
		t.Fatal(err)
	}
	if len(knowledge.Documents) != 1 {
		t.Fatalf("unexpected knowledge index: %#v", knowledge)
	}
	request = httptest.NewRequest(http.MethodGet, "http://localhost/api/v0/knowledge/"+knowledge.Documents[0].ID, nil)
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	var document locus.DocumentContent
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		t.Fatal(err)
	}
	if document.Format != "markdown" || !strings.Contains(document.Body, "Web runbook") {
		t.Fatalf("unexpected document: %#v", document)
	}

	request = httptest.NewRequest(http.MethodGet, "http://localhost/graph", nil)
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `<div id="app"></div>`) {
		t.Fatalf("embedded UI response = %d: %s", response.Code, response.Body.String())
	}
}

func TestServerRejectsNonLocalAccess(t *testing.T) {
	registry := writeTestRegistry(t)
	t.Setenv("LOCUS_STATE_PATH", filepath.Join(registry, "state.db"))
	server, err := New(Config{Registry: registry, Vantage: "office-lan"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/v0/context", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if _, err := listenLoopback("0.0.0.0:0"); err == nil {
		t.Fatal("non-loopback listener was accepted")
	}
}

func writeTestRegistry(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "temp", "unit-tests", "web-registry"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"entities", "links", "routes", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"scope.yaml": "api_version: locus/v1\nscope_id: project.web\n",
		filepath.Join("entities", "workstation.yaml"): "api_version: locus/v1\ntype: entity\nid: workstation\nkind: workstation\nname: Workstation\n",
		filepath.Join("entities", "target.yaml"):      "api_version: locus/v1\ntype: entity\nid: target\nkind: host\nname: Target\ndocumentation:\n  - ref: ../docs/runbook.md\n    title: Web runbook\n",
		filepath.Join("links", "safe.yaml"):           "api_version: locus/v1\ntype: link\nid: link.safe\nfrom: workstation\nto: target\nprovider: salt\nprovides: [salt.ping]\nprovider_data:\n  minion_id: test-target\n",
		filepath.Join("routes", "safe.yaml"):          "api_version: locus/v1\ntype: route\nid: route.safe\nsteps:\n  - link: link.safe\n",
		filepath.Join("docs", "runbook.md"):           "# Web runbook\n\nUse the validated workspace context.\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
