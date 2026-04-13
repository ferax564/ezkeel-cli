package forgejo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRunner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/actions/runners/registration-token" {
			t.Errorf("expected /api/v1/user/actions/runners/registration-token, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "token admin-tok" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token": "runner-reg-token-123",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin-tok")
	token, err := client.CreateRunnerRegistrationToken()
	if err != nil {
		t.Fatalf("CreateRunnerRegistrationToken() error: %v", err)
	}
	if token != "runner-reg-token-123" {
		t.Errorf("token = %q, want %q", token, "runner-reg-token-123")
	}
}
