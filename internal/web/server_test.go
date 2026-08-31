package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextAndEmbeddedUI(t *testing.T) {
	registry := writeTestRegistry(t)
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

	request = httptest.NewRequest(http.MethodGet, "http://localhost/graph", nil)
	response = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `<div id="app"></div>`) {
		t.Fatalf("embedded UI response = %d: %s", response.Code, response.Body.String())
	}
}

func TestServerRejectsNonLocalAccess(t *testing.T) {
	server, err := New(Config{Registry: writeTestRegistry(t), Vantage: "office-lan"})
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
	if err := os.MkdirAll(filepath.Join(root, "entities"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"scope.yaml": "api_version: locus/v0\nscope:\n  id: project.web\n  kind: project\n",
		filepath.Join("entities", "workstation.yaml"): "api_version: locus/v0\ntype: entity\nid: workstation\nkind: workstation\nname: Workstation\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
