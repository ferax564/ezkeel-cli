package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// --- ResolveAuth tests ---

func TestResolveAuth_EnvVar(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token-123")

	// Env var is checked before gh CLI, so no need to skip CLI.
	token, method, err := ResolveAuth("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "env-token-123" {
		t.Errorf("token = %q, want %q", token, "env-token-123")
	}
	if method != AuthEnvToken {
		t.Errorf("method = %v, want AuthEnvToken", method)
	}
}

func TestResolveAuth_ExplicitToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token-123")

	token, method, err := ResolveAuth("flag-token-456", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "flag-token-456" {
		t.Errorf("token = %q, want flag-token-456", token)
	}
	if method != AuthFlagToken {
		t.Errorf("method = %v, want AuthFlagToken", method)
	}
}

func TestResolveAuth_NoAuth(t *testing.T) {
	// Unset env var and pass a non-existent gh path so no auth is available.
	old := os.Getenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	defer os.Setenv("GITHUB_TOKEN", old)

	_, _, err := ResolveAuth("", "/nonexistent/gh")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Client / Repo Operation tests ---

func TestAuthenticatedUser(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/user" {
			t.Errorf("path = %s, want /user", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}

		json.NewEncoder(w).Encode(map[string]string{"login": "ferax564"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	login, err := c.AuthenticatedUser()
	if err != nil {
		t.Fatalf("AuthenticatedUser error: %v", err)
	}
	if login != "ferax564" {
		t.Errorf("login = %q, want %q", login, "ferax564")
	}
}

func TestRepoExists_True(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/ferax564/myrepo" {
			t.Errorf("path = %s, want /repos/ferax564/myrepo", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Repo{Name: "myrepo"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	exists, err := c.RepoExists("ferax564", "myrepo")
	if err != nil {
		t.Fatalf("RepoExists error: %v", err)
	}
	if !exists {
		t.Error("expected exists=true, got false")
	}
}

func TestRepoExists_False(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	exists, err := c.RepoExists("ferax564", "nonexistent")
	if err != nil {
		t.Fatalf("RepoExists error: %v", err)
	}
	if exists {
		t.Error("expected exists=false, got true")
	}
}

func TestCreateRepo(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/user/repos" {
			t.Errorf("path = %s, want /user/repos", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "new-repo" {
			t.Errorf("name = %v, want %q", body["name"], "new-repo")
		}
		if body["description"] != "a test repo" {
			t.Errorf("description = %v, want %q", body["description"], "a test repo")
		}
		if body["private"] != false {
			t.Errorf("private = %v, want false", body["private"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Repo{
			Name:     "new-repo",
			CloneURL: "https://github.com/ferax564/new-repo.git",
			HTMLURL:  "https://github.com/ferax564/new-repo",
			Private:  false,
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	repo, err := c.CreateRepo("new-repo", "a test repo")
	if err != nil {
		t.Fatalf("CreateRepo error: %v", err)
	}
	if repo.Name != "new-repo" {
		t.Errorf("Name = %q, want %q", repo.Name, "new-repo")
	}
	if repo.CloneURL != "https://github.com/ferax564/new-repo.git" {
		t.Errorf("CloneURL = %q", repo.CloneURL)
	}
}

func TestCreateOrgRepo(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/orgs/my-org/repos" {
			t.Errorf("path = %s, want /orgs/my-org/repos", r.URL.Path)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "project-public" {
			t.Errorf("name = %v", body["name"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Repo{
			Name:     "project-public",
			CloneURL: "https://github.com/my-org/project-public.git",
			HTMLURL:  "https://github.com/my-org/project-public",
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	repo, err := c.CreateOrgRepo("my-org", "project-public", "Public mirror")
	if err != nil {
		t.Fatalf("CreateOrgRepo error: %v", err)
	}
	if repo.Name != "project-public" {
		t.Errorf("name = %q", repo.Name)
	}
	if repo.CloneURL != "https://github.com/my-org/project-public.git" {
		t.Errorf("CloneURL = %q", repo.CloneURL)
	}
}

func TestCreateRepo_AlreadyExists(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Repository creation failed.",
			"errors":  []map[string]string{{"message": "name already exists on this account"}},
		})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	_, err := c.CreateRepo("existing-repo", "desc")
	if err == nil {
		t.Fatal("expected error for 422 response, got nil")
	}
}
