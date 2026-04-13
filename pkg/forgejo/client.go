package forgejo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// escapePath URL-escapes each slash-separated segment of a repo path,
// preserving the `/` separators so Forgejo's routing still treats them
// as path boundaries. Needed because git allows filenames and refs
// containing reserved URL characters (`?`, `#`, space, etc.) that
// otherwise end up as the query string or fragment of the request.
//
// NOTE: `owner` and `repo` values elsewhere in this file go through
// plain `url.PathEscape` instead, because they are single path segments
// that cannot contain `/`. Using `escapePath` on them would be wrong:
// a stray `/` would be preserved instead of escaped and forgejo would
// route the request to the wrong repository. Only `ref` and `filePath`
// legitimately contain `/` and need the multi-segment escaper.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// ErrBlobTooLarge is returned by GetRawFile when the caller passes a
// positive maxBytes and the upstream response exceeds it. The handler
// uses errors.Is to distinguish this from network or auth failures.
var ErrBlobTooLarge = errors.New("forgejo: blob exceeds max bytes")

// Client is a Forgejo API client.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// Repo represents a Forgejo repository.
type Repo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	CloneURL string `json:"clone_url"`
}

// NewClient returns a new Forgejo API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{},
	}
}

// Token returns the API token this client was constructed with. Used by
// the orchestrator's deploy tool to inject the token into a `git clone`
// URL for private repositories (see internal/web/orchestrator/executor.go).
func (c *Client) Token() string {
	return c.token
}

// do performs an HTTP request, JSON-marshaling body if non-nil, and returns the response body bytes.
func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
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

	req.Header.Set("Authorization", "token "+c.token)
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
		return nil, fmt.Errorf("API request %s %s returned status %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// CreateRepo creates a new repository for the authenticated user.
func (c *Client) CreateRepo(name, desc string, private bool) (*Repo, error) {
	payload := map[string]interface{}{
		"name":        name,
		"description": desc,
		"private":     private,
		"auto_init":   true,
	}

	data, err := c.do(http.MethodPost, "/api/v1/user/repos", payload)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}

	var repo Repo
	if err := json.Unmarshal(data, &repo); err != nil {
		return nil, fmt.Errorf("parsing create repo response: %w", err)
	}

	return &repo, nil
}

// CreateWebhook creates a webhook on the given repository.
func (c *Client) CreateWebhook(owner, repo, url string, events []string) error {
	payload := map[string]interface{}{
		"type": "forgejo",
		"config": map[string]string{
			"url":          url,
			"content_type": "json",
		},
		"events": events,
		"active": true,
	}

	path := fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner, repo)
	_, err := c.do(http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("creating webhook: %w", err)
	}

	return nil
}

// CreateOrgRepo creates a new repository under the given organization.
func (c *Client) CreateOrgRepo(org, name, desc string, private bool) (*Repo, error) {
	payload := map[string]interface{}{
		"name":        name,
		"description": desc,
		"private":     private,
		"auto_init":   true,
	}

	path := fmt.Sprintf("/api/v1/orgs/%s/repos", org)
	data, err := c.do(http.MethodPost, path, payload)
	if err != nil {
		return nil, fmt.Errorf("creating org repo: %w", err)
	}

	var repo Repo
	if err := json.Unmarshal(data, &repo); err != nil {
		return nil, fmt.Errorf("parsing create org repo response: %w", err)
	}

	return &repo, nil
}

// FileContent represents a file in a Forgejo repository.
type FileContent struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"` // base64 encoded
	SHA     string `json:"sha"`
}

// CreateFile creates a new file in the given repository.
// The content must be base64 encoded.
func (c *Client) CreateFile(owner, repo, filepath, content, message string) error {
	payload := map[string]interface{}{
		"content": content,
		"message": message,
	}
	path := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", owner, repo, filepath)
	_, err := c.do(http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", filepath, err)
	}
	return nil
}

// GetFileContent retrieves a file's content and metadata from the given repository.
func (c *Client) GetFileContent(owner, repo, filepath string) (*FileContent, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", owner, repo, filepath)
	data, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var fc FileContent
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parsing file content: %w", err)
	}
	return &fc, nil
}

// UpdateFile updates an existing file in the given repository.
// The content must be base64 encoded. The sha must match the current file's SHA.
func (c *Client) UpdateFile(owner, repo, filepath, content, sha, message string) error {
	payload := map[string]interface{}{
		"content": content,
		"sha":     sha,
		"message": message,
	}
	path := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", owner, repo, filepath)
	_, err := c.do(http.MethodPut, path, payload)
	if err != nil {
		return fmt.Errorf("updating file %s: %w", filepath, err)
	}
	return nil
}

// DirEntry represents a file or directory entry in a Forgejo repository listing.
type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "dir"
	Size int64  `json:"size"`
}

// ListFiles lists the contents of a directory in the given repository.
// Pass an empty dirPath to list the root directory.
func (c *Client) ListFiles(owner, repo, dirPath string) ([]DirEntry, error) {
	apiPath := fmt.Sprintf("/api/v1/repos/%s/%s/contents", owner, repo)
	if dirPath != "" {
		apiPath = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", owner, repo, dirPath)
	}
	data, err := c.do(http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, fmt.Errorf("listing files: %w", err)
	}
	var entries []DirEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing file listing: %w", err)
	}
	return entries, nil
}

// WorkflowRun represents a Forgejo Actions workflow run.
type WorkflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadBranch string `json:"head_branch"`
	Event      string `json:"event"`
	CreatedAt  string `json:"created_at"`
	HTMLURL    string `json:"url"`
}

// GetRawFile reads the full body into memory with an optional size cap.
// Pass 0 for maxBytes to disable the cap. Returns ErrBlobTooLarge when
// the cap is positive and the upstream response exceeds it.
func (c *Client) GetRawFile(owner, repo, ref, filePath string, maxBytes int64) ([]byte, time.Time, error) {
	rc, _, modified, err := c.openRawFile(owner, repo, ref, filePath)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rc.Close()

	var reader io.Reader = rc
	if maxBytes > 0 {
		reader = io.LimitReader(rc, maxBytes+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("reading raw file: %w", err)
	}
	if maxBytes > 0 && int64(len(body)) > maxBytes {
		return nil, time.Time{}, ErrBlobTooLarge
	}
	return body, modified, nil
}

// GetRawFileStream opens the raw endpoint and returns the live response
// body, the upstream Content-Length (-1 if unknown), and the Last-Modified
// time. The caller MUST close the returned ReadCloser. Use this for
// download paths where buffering the full body would blow the heap.
func (c *Client) GetRawFileStream(owner, repo, ref, filePath string) (io.ReadCloser, int64, time.Time, error) {
	return c.openRawFile(owner, repo, ref, filePath)
}

func (c *Client) openRawFile(owner, repo, ref, filePath string) (io.ReadCloser, int64, time.Time, error) {
	reqURL := c.baseURL + fmt.Sprintf("/api/v1/repos/%s/%s/raw/%s/%s",
		url.PathEscape(owner), url.PathEscape(repo), escapePath(ref), escapePath(filePath))
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, time.Time{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, time.Time{}, fmt.Errorf("executing request: %w", err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, 0, time.Time{}, fmt.Errorf("raw file request returned status %d: %s", resp.StatusCode, string(body))
	}

	var modified time.Time
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, perr := time.Parse(time.RFC1123, lm); perr == nil {
			modified = t
		}
	}
	return resp.Body, resp.ContentLength, modified, nil
}

// TreeEntry is one entry in a recursive Forgejo git-tree listing.
type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" or "tree"
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
}

// Tree is the resolved recursive listing of a Forgejo repository at a
// specific ref. Ref reflects the branch name actually used — it may
// differ from the requested ref if the requested branch was missing
// and the client fell back to the repository's default branch.
// Truncated is true when Forgejo capped the listing at per_page and
// additional entries exist upstream; callers must surface this to the
// user so the UI does not silently render an incomplete tree.
type Tree struct {
	Ref       string      `json:"ref"`
	SHA       string      `json:"sha"`
	Entries   []TreeEntry `json:"entries"`
	Truncated bool        `json:"truncated"`
}

// GetTree returns the recursive listing of a repository at the given ref.
// If the ref does not exist, it falls back to the repository's default
// branch and returns the listing for that instead.
func (c *Client) GetTree(owner, repo, ref string) (*Tree, error) {
	ownerEsc := url.PathEscape(owner)
	repoEsc := url.PathEscape(repo)
	resolvedRef := ref
	branchData, err := c.do(http.MethodGet,
		fmt.Sprintf("/api/v1/repos/%s/%s/branches/%s", ownerEsc, repoEsc, escapePath(ref)), nil)
	if err != nil {
		def, defErr := c.GetDefaultBranch(owner, repo)
		if defErr != nil {
			return nil, fmt.Errorf("resolving ref %q: %w (default branch fallback: %v)", ref, err, defErr)
		}
		resolvedRef = def
		branchData, err = c.do(http.MethodGet,
			fmt.Sprintf("/api/v1/repos/%s/%s/branches/%s", ownerEsc, repoEsc, escapePath(def)), nil)
		if err != nil {
			return nil, fmt.Errorf("resolving default branch %q: %w", def, err)
		}
	}
	var br struct {
		Name   string `json:"name"`
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(branchData, &br); err != nil {
		return nil, fmt.Errorf("parsing branch response: %w", err)
	}
	if br.Commit.ID == "" {
		return nil, fmt.Errorf("branch %q has empty commit id", resolvedRef)
	}
	treeData, err := c.do(http.MethodGet,
		fmt.Sprintf("/api/v1/repos/%s/%s/git/trees/%s?recursive=true&per_page=10000",
			ownerEsc, repoEsc, url.PathEscape(br.Commit.ID)), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching tree: %w", err)
	}
	var tr struct {
		SHA       string      `json:"sha"`
		Tree      []TreeEntry `json:"tree"`
		Truncated bool        `json:"truncated"`
	}
	if err := json.Unmarshal(treeData, &tr); err != nil {
		return nil, fmt.Errorf("parsing tree response: %w", err)
	}
	return &Tree{Ref: resolvedRef, SHA: tr.SHA, Entries: tr.Tree, Truncated: tr.Truncated}, nil
}

// GetDefaultBranch returns the configured default branch for a repository
// (e.g. "main" or "trunk"). Used by the file viewer to resolve HEAD when
// a hardcoded branch lookup is not possible.
func (c *Client) GetDefaultBranch(owner, repo string) (string, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	data, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return "", fmt.Errorf("getting repo: %w", err)
	}
	var resp struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parsing repo response: %w", err)
	}
	return resp.DefaultBranch, nil
}

// ListWorkflowRuns returns recent workflow runs for a repository.
func (c *Client) ListWorkflowRuns(owner, repo string, limit int) ([]WorkflowRun, error) {
	path := fmt.Sprintf("/api/v1/repos/%s/%s/actions/tasks?limit=%d", owner, repo, limit)
	data, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("listing workflow runs: %w", err)
	}

	var result struct {
		Runs []WorkflowRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing workflow runs: %w", err)
	}

	return result.Runs, nil
}
