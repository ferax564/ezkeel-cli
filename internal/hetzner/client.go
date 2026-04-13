package hetzner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultBaseURL = "https://api.hetzner.cloud"

// Client calls the Hetzner Cloud API.
type Client struct {
	Token   string
	BaseURL string // override for testing
}

// NewClient returns a Hetzner API client with the given token.
func NewClient(token string) *Client {
	return &Client{Token: token}
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

	resp, err := http.DefaultClient.Do(req)
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

// GetServer fetches the current status of a server by ID.
func (c *Client) GetServer(id int) (*GetServerResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/servers/%d", c.baseURL(), id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := http.DefaultClient.Do(req)
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
