package hetzner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.hetzner.cloud"

// defaultHTTPTimeout bounds individual Hetzner API calls so a stalled
// or unreachable Hetzner can't hang an EZKeel HTTP handler indefinitely.
// 30s is generous — Hetzner's documented p99 for create-server-class
// calls is ~5s; anything past 30s is realistically a network failure.
const defaultHTTPTimeout = 30 * time.Second

// Client calls the Hetzner Cloud API.
type Client struct {
	Token   string
	BaseURL string // override for testing

	// HTTPClient is the underlying transport. Optional — when nil,
	// the package falls back to a shared client with a 30s timeout
	// so callers don't have to construct their own. Tests override
	// to inject httptest-backed clients with custom timeouts.
	HTTPClient *http.Client
}

// NewClient returns a Hetzner API client with the given token.
func NewClient(token string) *Client {
	return &Client{Token: token}
}

// sharedHTTPClient is the default-timeout client used when Client.HTTPClient
// is nil. Defined as a package-level var so all client instances share
// the same connection pool.
var sharedHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}

// httpClient returns the configured HTTP client, falling back to the
// shared default-timeout one. Codex flagged the previous unbounded
// http.DefaultClient usage on the consuming repo's PR review — a
// stalled Hetzner request would otherwise block the EZKeel HTTP handler
// past the request context's expectations.
func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return sharedHTTPClient
}

type CreateServerRequest struct {
	Name       string   `json:"name"`
	ServerType string   `json:"server_type"`
	Location   string   `json:"location"`
	Image      string   `json:"image"`
	SSHKeys    []string `json:"ssh_keys"`
}

type CreateServerResponse struct {
	Server ServerInfo `json:"server"`
}

type GetServerResponse struct {
	Server ServerInfo `json:"server"`
}

type ServerInfo struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	PublicNet PublicNet `json:"public_net"`
}

type PublicNet struct {
	IPv4 IPv4 `json:"ipv4"`
}

type IPv4 struct {
	IP string `json:"ip"`
}

// CreateServer provisions a new Hetzner Cloud VPS.
func (c *Client) CreateServer(name, serverType, location, sshKeyName string) (*CreateServerResponse, error) {
	body := CreateServerRequest{
		Name:       name,
		ServerType: serverType,
		Location:   location,
		Image:      "ubuntu-24.04",
		SSHKeys:    []string{sshKeyName},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL()+"/v1/servers", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hetzner API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result CreateServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// UploadSSHKeyRequest is the body for POST /v1/ssh_keys.
type UploadSSHKeyRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

// UploadSSHKeyResponse wraps Hetzner's `ssh_key` envelope.
type UploadSSHKeyResponse struct {
	SSHKey SSHKeyInfo `json:"ssh_key"`
}

// SSHKeyInfo is the subset of Hetzner's SSH-key resource we care about.
type SSHKeyInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
}

// UploadSSHKey registers an SSH public key with the user's Hetzner
// project. Used by the auto-provisioning flow to inject a fresh
// per-server keypair before calling CreateServer. The `name` must be
// unique within the project — callers should namespace by ezkeel
// server ID to avoid collisions across re-provisions.
func (c *Client) UploadSSHKey(name, publicKey string) (*UploadSSHKeyResponse, error) {
	body := UploadSSHKeyRequest{Name: name, PublicKey: publicKey}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL()+"/v1/ssh_keys", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hetzner API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result UploadSSHKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// DeleteServer removes a server from the user's Hetzner project. Used
// by the auto-provisioning flow to clean up when EZKeel-side bootstrap
// fails after Hetzner-side provision succeeded — without this, the user
// pays Hetzner for a half-broken VPS we couldn't finish setting up.
//
// 204 No Content is success. 404 is treated as success because it means
// the server is already gone (idempotent — safe to retry).
func (c *Client) DeleteServer(id int) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/servers/%d", c.baseURL(), id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("hetzner API error (HTTP %d): %s", resp.StatusCode, string(respBody))
}

// GetServer fetches the current status of a server by ID.
func (c *Client) GetServer(id int) (*GetServerResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/servers/%d", c.baseURL(), id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hetzner API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result GetServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBaseURL
}
