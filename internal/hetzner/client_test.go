package hetzner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestUploadSSHKey(t *testing.T) {
	var captured UploadSSHKeyRequest
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/ssh_keys" {
			t.Errorf("path = %s, want /v1/ssh_keys", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"ssh_key":{"id":42,"name":"test-key","fingerprint":"ab:cd"}}`))
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	resp, err := c.UploadSSHKey("test-key", "ssh-ed25519 AAAA...")
	if err != nil {
		t.Fatalf("UploadSSHKey: %v", err)
	}
	if resp.SSHKey.ID != 42 {
		t.Errorf("ID = %d, want 42", resp.SSHKey.ID)
	}
	if captured.Name != "test-key" {
		t.Errorf("captured name = %q, want test-key", captured.Name)
	}
	if captured.PublicKey != "ssh-ed25519 AAAA..." {
		t.Errorf("captured public_key = %q", captured.PublicKey)
	}
}

func TestUploadSSHKey_Unauthorized(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"unauthorized","message":"invalid token"}}`))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "bad-token", BaseURL: srv.URL}
	_, err := c.UploadSSHKey("k", "ssh-ed25519 ...")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
}

// TestUploadSSHKey_DuplicateName covers the realistic case where a
// previous provision uploaded a key with the same name (e.g. the
// caller's namespace collision). Hetzner returns 422 with `code:
// uniqueness_error`. The orchestrator needs the error to surface so it
// can append a disambiguator and retry.
func TestUploadSSHKey_DuplicateName(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error":{"code":"uniqueness_error","message":"name already in use"}}`))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	_, err := c.UploadSSHKey("dup", "ssh-ed25519 ...")
	if err == nil {
		t.Fatal("expected error on 422, got nil")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error should carry HTTP code: %v", err)
	}
}

// TestUploadSSHKey_NetworkError simulates the server closing the
// connection mid-flight. The client must surface a clean error rather
// than hang or panic. Eng-review test gap #1.
func TestUploadSSHKey_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("response writer not hijackable")
		}
		conn, _, _ := hj.Hijack()
		_ = conn.Close() // drop the connection mid-response
	}))
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	_, err := c.UploadSSHKey("k", "ssh-ed25519 ...")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestDeleteServer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/v1/servers/123" {
			t.Errorf("path = %s, want /v1/servers/123", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	if err := c.DeleteServer(123); err != nil {
		t.Errorf("DeleteServer: %v", err)
	}
}

// TestDeleteServer_AlreadyGone — 404 is treated as success because the
// caller's intent ("make sure server N is deleted") is satisfied either
// way. Idempotency matters because the orchestrator may retry on
// transient errors and shouldn't fail the second attempt.
func TestDeleteServer_AlreadyGone(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "test-token", BaseURL: srv.URL}
	if err := c.DeleteServer(999); err != nil {
		t.Errorf("404 should be success, got %v", err)
	}
}

func TestDeleteServer_Unauthorized(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := &Client{Token: "bad-token", BaseURL: srv.URL}
	err := c.DeleteServer(1)
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
}
