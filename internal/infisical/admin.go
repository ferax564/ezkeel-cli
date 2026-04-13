package infisical

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AdminClient interacts with the Infisical REST API for admin operations.
type AdminClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// Project represents an Infisical project (workspace).
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// NewAdminClient returns a client for Infisical admin API operations.
// The token is a pre-existing bearer token (e.g. from LoginUniversalAuth).
func NewAdminClient(baseURL, token string) *AdminClient {
	return &AdminClient{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{},
	}
}

// LoginUniversalAuth authenticates a machine identity using Infisical's
// Universal Auth (client credentials) and returns an AdminClient with the
// resulting access token.
func LoginUniversalAuth(baseURL, clientID, clientSecret string) (*AdminClient, error) {
	payload := map[string]string{
		"clientId":     clientID,
		"clientSecret": clientSecret,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling login payload: %w", err)
	}

	resp, err := http.Post(baseURL+"/api/v1/auth/universal-auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("universal auth request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading login response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("universal auth login failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing login response: %w", err)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("universal auth login returned empty access token")
	}

	return &AdminClient{
		baseURL: baseURL,
		token:   result.AccessToken,
		http:    &http.Client{},
	}, nil
}

// do performs an authenticated HTTP request and returns the response body.
func (c *AdminClient) do(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API %s %s returned status %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// CreateProject creates a new Infisical project in the given organization.
func (c *AdminClient) CreateProject(name, orgID string) (*Project, error) {
	payload := map[string]string{
		"projectName":    name,
		"organizationId": orgID,
	}

	data, err := c.do(http.MethodPost, "/api/v2/workspace", payload)
	if err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	// The API may return the project under "project" (newer) or "workspace" (older).
	var result struct {
		Project   Project `json:"project"`
		Workspace Project `json:"workspace"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if result.Project.ID != "" {
		return &result.Project, nil
	}
	if result.Workspace.ID != "" {
		return &result.Workspace, nil
	}
	return nil, fmt.Errorf("unexpected API response: no project or workspace returned")
}

// CreateEnvironment adds a custom environment to an Infisical project.
func (c *AdminClient) CreateEnvironment(projectID, name, slug string) error {
	payload := map[string]string{
		"name": name,
		"slug": slug,
	}

	path := fmt.Sprintf("/api/v1/workspace/%s/environments", projectID)
	_, err := c.do(http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("creating environment %q: %w", name, err)
	}
	return nil
}
