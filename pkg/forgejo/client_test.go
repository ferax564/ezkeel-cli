package forgejo_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/pkg/forgejo"
)

func TestCreateRepo(t *testing.T) {
	var gotBody map[string]interface{}
	var srv *httptest.Server

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/user/repos" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "token test-token" {
			t.Errorf("expected Authorization header 'token test-token', got %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":        1,
			"name":      gotBody["name"],
			"clone_url": srv.URL + "/user/test-repo.git",
		})
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	repo, err := client.CreateRepo("test-repo", "desc", false)
	if err != nil {
		t.Fatalf("CreateRepo returned error: %v", err)
	}
	if repo.Name != "test-repo" {
		t.Errorf("expected repo name 'test-repo', got %q", repo.Name)
	}
	if private, ok := gotBody["private"].(bool); !ok || private {
		t.Errorf("expected private=false in request body, got %v", gotBody["private"])
	}
}

func TestCreateRepoPrivate(t *testing.T) {
	var gotBody map[string]interface{}
	var srv *httptest.Server

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/user/repos" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "token test-token" {
			t.Errorf("expected Authorization header 'token test-token', got %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":        2,
			"name":      gotBody["name"],
			"clone_url": srv.URL + "/user/private-repo.git",
		})
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	repo, err := client.CreateRepo("private-repo", "a private repo", true)
	if err != nil {
		t.Fatalf("CreateRepo returned error: %v", err)
	}
	if repo.Name != "private-repo" {
		t.Errorf("expected repo name 'private-repo', got %q", repo.Name)
	}
	if private, ok := gotBody["private"].(bool); !ok || !private {
		t.Errorf("expected private=true in request body, got %v", gotBody["private"])
	}
}

func TestCreateWebhook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/repos/user/test-repo/hooks" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "token test-token" {
			t.Errorf("expected Authorization header 'token test-token', got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	err := client.CreateWebhook("user", "test-repo", "https://example.com/hook", []string{"push"})
	if err != nil {
		t.Fatalf("CreateWebhook returned error: %v", err)
	}
}

func TestGetDefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/repos/owner/repo" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"default_branch":"trunk"}`))
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	branch, err := client.GetDefaultBranch("owner", "repo")
	if err != nil {
		t.Fatalf("GetDefaultBranch returned error: %v", err)
	}
	if branch != "trunk" {
		t.Errorf("expected branch 'trunk', got %q", branch)
	}
}

func TestGetTree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/branches/main":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"main","commit":{"id":"abc123"}}`))
		case "/api/v1/repos/owner/repo/git/trees/abc123":
			if r.URL.Query().Get("recursive") != "true" {
				t.Errorf("expected recursive=true, got %q", r.URL.Query().Get("recursive"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"sha":"abc123",
				"tree":[
					{"path":"README.md","type":"blob","size":42},
					{"path":"src","type":"tree"},
					{"path":"src/main.go","type":"blob","size":120}
				],
				"truncated":false
			}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	tree, err := client.GetTree("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetTree returned error: %v", err)
	}
	if tree.SHA != "abc123" {
		t.Errorf("expected SHA 'abc123', got %q", tree.SHA)
	}
	if tree.Ref != "main" {
		t.Errorf("expected ref 'main', got %q", tree.Ref)
	}
	if len(tree.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(tree.Entries))
	}
	if tree.Entries[0].Path != "README.md" || tree.Entries[0].Type != "blob" {
		t.Errorf("unexpected first entry: %+v", tree.Entries[0])
	}
}

func TestGetTreeFallsBackToDefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/branches/main":
			http.Error(w, "not found", http.StatusNotFound)
		case "/api/v1/repos/owner/repo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"default_branch":"trunk"}`))
		case "/api/v1/repos/owner/repo/branches/trunk":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"trunk","commit":{"id":"deadbeef"}}`))
		case "/api/v1/repos/owner/repo/git/trees/deadbeef":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sha":"deadbeef","tree":[],"truncated":false}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	tree, err := client.GetTree("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetTree returned error: %v", err)
	}
	if tree.Ref != "trunk" {
		t.Errorf("expected fallback ref 'trunk', got %q", tree.Ref)
	}
}

func TestGetRawFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/api/v1/repos/owner/repo/raw/main/api/app/main.py"
		if r.URL.Path != expected {
			t.Errorf("expected path %q, got %q", expected, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("missing Authorization header")
		}
		w.Header().Set("Last-Modified", "Mon, 10 Apr 2026 18:12:09 GMT")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("from fastapi import FastAPI\n"))
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	body, modified, err := client.GetRawFile("owner", "repo", "main", "api/app/main.py", 0)
	if err != nil {
		t.Fatalf("GetRawFile returned error: %v", err)
	}
	if string(body) != "from fastapi import FastAPI\n" {
		t.Errorf("unexpected body: %q", string(body))
	}
	if modified.IsZero() {
		t.Errorf("expected non-zero modified time")
	}
}

func TestGetRawFileTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), 100))
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	_, _, err := client.GetRawFile("owner", "repo", "main", "big.bin", 50)
	if !errors.Is(err, forgejo.ErrBlobTooLarge) {
		t.Errorf("expected ErrBlobTooLarge, got %v", err)
	}
}

// TestGetRawFileEscapesReservedCharsInPath pins the url.PathEscape fix
// for codex P2 finding #1 — filenames containing `?` or `#` must not
// end up as the query string or fragment of the upstream request.
// Asserts via RequestURI (the raw request line) because net/http
// decodes r.URL.Path before the handler sees it.
func TestGetRawFileEscapesReservedCharsInPath(t *testing.T) {
	var gotURI, gotRawQuery, gotDecodedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		gotRawQuery = r.URL.RawQuery
		gotDecodedPath = r.URL.Path
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	_, _, err := client.GetRawFile("owner", "repo", "feature/v2", "docs/what's new?.md", 0)
	if err != nil {
		t.Fatalf("GetRawFile returned error: %v", err)
	}
	if gotRawQuery != "" {
		t.Errorf("`?` in filename leaked into the query string: %q", gotRawQuery)
	}
	// Raw wire form must have the reserved chars escaped.
	wantEscaped := "what%27s%20new%3F.md"
	if !strings.Contains(gotURI, wantEscaped) {
		t.Errorf("request-uri missing escaped filename: got %q", gotURI)
	}
	// Decoded path must round-trip to the original filename.
	wantDecoded := "/api/v1/repos/owner/repo/raw/feature/v2/docs/what's new?.md"
	if gotDecodedPath != wantDecoded {
		t.Errorf("decoded path mismatch:\n  got:  %q\n  want: %q", gotDecodedPath, wantDecoded)
	}
}

// TestGetTreePreservesTruncatedFlag pins codex P3 finding #3 — the
// recursive tree endpoint's `truncated` boolean must survive to the
// Tree value so callers can surface it.
func TestGetTreePreservesTruncatedFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/owner/repo/branches/main":
			_, _ = w.Write([]byte(`{"name":"main","commit":{"id":"abc"}}`))
		case "/api/v1/repos/owner/repo/git/trees/abc":
			_, _ = w.Write([]byte(`{"sha":"abc","tree":[{"path":"x","type":"blob","size":1}],"truncated":true}`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := forgejo.NewClient(srv.URL, "test-token")
	tree, err := client.GetTree("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetTree returned error: %v", err)
	}
	if !tree.Truncated {
		t.Errorf("expected Truncated=true, got false")
	}
}
