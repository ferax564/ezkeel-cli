package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// AuthMethod indicates how the GitHub token was resolved.
type AuthMethod int

const (
	AuthGhCLI AuthMethod = iota
	AuthFlagToken
	AuthEnvToken
)

func (m AuthMethod) String() string {
	switch m {
	case AuthGhCLI:
		return "gh CLI"
	case AuthFlagToken:
		return "flag"
	case AuthEnvToken:
		return "GITHUB_TOKEN env"
	default:
		return "unknown"
	}
}

// Client calls the GitHub REST API.
type Client struct {
	Token   string
	BaseURL string // override for testing
	http    *http.Client
}

// Repo represents a GitHub repository.
type Repo struct {
	Name     string `json:"name"`
	CloneURL string `json:"clone_url"`
	HTMLURL  string `json:"html_url"`
	Private  bool   `json:"private"`
}

// NewClient returns a GitHub API client with the given token.
func NewClient(token string) *Client {
	return &Client{
		Token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ResolveAuth determines the GitHub token using this priority:
//  1. flagToken (from --github-token flag)
//  2. GITHUB_TOKEN environment variable (zero-cost, preferred in CI)
//  3. gh CLI (`gh auth token`)
func ResolveAuth(flagToken, ghPath string) (string, AuthMethod, error) {
	if flagToken != "" {
		return flagToken, AuthFlagToken, nil
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, AuthEnvToken, nil
	}

	if ghPath == "" {
		ghPath = "gh"
	}
	if out, err := exec.Command(ghPath, "auth", "token").Output(); err == nil {
		token := strings.TrimSpace(string(out))
		if token != "" {
			return token, AuthGhCLI, nil
		}
	}

	return "", 0, fmt.Errorf(
		"no GitHub token found; provide one via:\n" +
			"  --github-token flag\n" +
			"  GITHUB_TOKEN environment variable\n" +
			"  gh CLI (gh auth login)",
	)
}

// do performs an HTTP request, returning the raw response body bytes.
// It sets Authorization and Accept headers automatically.
// Non-2xx responses are returned as errors.
func (c *Client) do(method, path string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL()+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpClient := c.http
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// AuthenticatedUser returns the login of the authenticated user (GET /user).
func (c *Client) AuthenticatedUser() (string, error) {
	data, status, err := c.do("GET", "/user", nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("github API error (HTTP %d): %s", status, string(data))
	}

	var result struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.Login, nil
}

// RepoExists checks whether owner/name exists on GitHub.
func (c *Client) RepoExists(owner, name string) (bool, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, name)
	_, status, err := c.do("GET", path, nil)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status %d checking repo %s/%s", status, owner, name)
	}
}

// CreateRepo creates a public repository for the authenticated user.
func (c *Client) CreateRepo(name, description string) (*Repo, error) {
	payload := map[string]interface{}{
		"name":        name,
		"description": description,
		"private":     false,
	}
	return c.createRepo("/user/repos", payload)
}

// CreateOrgRepo creates a public repository under the given organization.
func (c *Client) CreateOrgRepo(org, name, description string) (*Repo, error) {
	payload := map[string]interface{}{
		"name":        name,
		"description": description,
		"private":     false,
	}
	return c.createRepo(fmt.Sprintf("/orgs/%s/repos", org), payload)
}

func (c *Client) createRepo(path string, payload interface{}) (*Repo, error) {
	data, status, err := c.do("POST", path, payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusCreated {
		return nil, fmt.Errorf("github API error (HTTP %d): %s", status, string(data))
	}

	var repo Repo
	if err := json.Unmarshal(data, &repo); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &repo, nil
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBaseURL
}
