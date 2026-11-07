package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestVersionMatchesPackageJSON(t *testing.T) {
	if version == "dev" {
		t.Skip("version is baked in at build time: run scripts/check-version.sh")
	}
	raw, err := os.ReadFile("../client/package.json")
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	versionHandler(rec, httptest.NewRequest(http.MethodGet, "/version", nil))
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if want := "v" + pkg.Version; body["version"] != want {
		t.Fatalf("/version = %q, want %q", body["version"], want)
	}
}
