package infisical

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/workspace" {
			t.Errorf("expected /api/v2/workspace, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer auth, got %s", r.Header.Get("Authorization"))
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["projectName"] != "my-project" {
			t.Errorf("expected projectName 'my-project', got %v", body["projectName"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"project": map[string]interface{}{
				"id":   "ws-123",
				"name": "my-project",
				"slug": "my-project",
			},
		})
	}))
	defer server.Close()

	client := NewAdminClient(server.URL, "test-token")
	project, err := client.CreateProject("my-project", "org-123")
	if err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}
	if project.Slug != "my-project" {
		t.Errorf("project.Slug = %q, want %q", project.Slug, "my-project")
	}
}

func TestLoginUniversalAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/universal-auth/login" {
			t.Errorf("expected /api/v1/auth/universal-auth/login, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["clientId"] != "cid-123" {
			t.Errorf("expected clientId 'cid-123', got %v", body["clientId"])
		}
		if body["clientSecret"] != "csecret-456" {
			t.Errorf("expected clientSecret 'csecret-456', got %v", body["clientSecret"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"accessToken": "jwt-token-abc",
		})
	}))
	defer server.Close()

	client, err := LoginUniversalAuth(server.URL, "cid-123", "csecret-456")
	if err != nil {
		t.Fatalf("LoginUniversalAuth() error: %v", err)
	}
	if client.token != "jwt-token-abc" {
		t.Errorf("client.token = %q, want %q", client.token, "jwt-token-abc")
	}
}

func TestLoginUniversalAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Invalid credentials"}`))
	}))
	defer server.Close()

	_, err := LoginUniversalAuth(server.URL, "bad-id", "bad-secret")
	if err == nil {
		t.Fatal("expected error for bad credentials, got nil")
	}
}

func TestCreateEnvironment(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/api/v1/workspace/ws-123/environments" {
			t.Errorf("expected /api/v1/workspace/ws-123/environments, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	client := NewAdminClient(server.URL, "test-token")
	err := client.CreateEnvironment("ws-123", "staging", "staging")
	if err != nil {
		t.Fatalf("CreateEnvironment() error: %v", err)
	}
	if !called {
		t.Error("expected API call to be made")
	}
}
