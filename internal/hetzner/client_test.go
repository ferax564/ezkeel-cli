package hetzner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateServer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/servers" {
			t.Errorf("path = %s, want /v1/servers", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}

		var body CreateServerRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "ezkeel-vps" {
			t.Errorf("name = %q, want %q", body.Name, "ezkeel-vps")
		}
		if body.ServerType != "cx22" {
			t.Errorf("server_type = %q, want %q", body.ServerType, "cx22")
		}

		resp := CreateServerResponse{
			Server: ServerInfo{
				ID:     12345,
				Name:   body.Name,
				Status: "initializing",
				PublicNet: PublicNet{
					IPv4: IPv4{IP: "168.119.1.1"},
				},
			},
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	result, err := c.CreateServer("ezkeel-vps", "cx22", "fsn1", "my-ssh-key")
	if err != nil {
		t.Fatalf("CreateServer error: %v", err)
	}
	if result.Server.ID != 12345 {
		t.Errorf("ID = %d, want 12345", result.Server.ID)
	}
	if result.Server.PublicNet.IPv4.IP != "168.119.1.1" {
		t.Errorf("IP = %q, want %q", result.Server.PublicNet.IPv4.IP, "168.119.1.1")
	}
}

func TestGetServer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/servers/12345" {
			t.Errorf("path = %s, want /v1/servers/12345", r.URL.Path)
		}

		resp := GetServerResponse{
			Server: ServerInfo{
				ID:     12345,
				Name:   "ezkeel-vps",
				Status: "running",
				PublicNet: PublicNet{
					IPv4: IPv4{IP: "168.119.1.1"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	result, err := c.GetServer(12345)
	if err != nil {
		t.Fatalf("GetServer error: %v", err)
	}
	if result.Server.Status != "running" {
		t.Errorf("Status = %q, want %q", result.Server.Status, "running")
	}
}

func TestCreateServer_APIError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"code":"rate_limit_exceeded","message":"rate limit exceeded"}}`))
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	_, err := c.CreateServer("ezkeel-vps", "cx22", "fsn1", "my-ssh-key")
	if err == nil {
		t.Fatal("expected error for 429 response, got nil")
	}
}

func TestGetServer_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":{"code":"not_found","message":"server not found"}}`))
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	_, err := c.GetServer(99999)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}
