package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBase = "https://api.github.com"

// Client is an authenticated GitHub API client.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a client. Token may be empty for unauthenticated (read-only) use.
func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: apiBase,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewTestClient creates a client pointing at a custom base URL (for httptest).
func NewTestClient(token, baseURL string) *Client {
	c := NewClient(token)
	c.baseURL = baseURL
	return c
}

// NewClientFromStored loads the token from disk and creates a client.
// Returns an unauthenticated client if no token is stored.
func NewClientFromStored() (*Client, error) {
	tok, err := LoadToken()
	if err != nil {
		return nil, err
	}
	token := ""
	if tok != nil {
		token = tok.AccessToken
	}
	return NewClient(token), nil
}

// IsAuthenticated returns true if the client has a token.
func (c *Client) IsAuthenticated() bool {
	return c.token != ""
}

func (c *Client) do(method, path string, body any, result any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	u := path
	if !strings.HasPrefix(path, "https://") && !strings.HasPrefix(path, "http://") {
		u = c.baseURL + path
	}

	req, err := http.NewRequest(method, u, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("github api %s %s: %d %s", method, path, resp.StatusCode, string(respBody))
	}

	if result != nil {
		return json.Unmarshal(respBody, result)
	}
	return nil
}

// GetRepo fetches a repository by owner/name.
func (c *Client) GetRepo(ownerRepo string) (*Repo, error) {
	var repo Repo
	if err := c.do("GET", "/repos/"+ownerRepo, nil, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// SearchRepos searches GitHub repositories by query. Adds the canopy-package topic filter.
func (c *Client) SearchRepos(query string, pkgType string, limit int) (*SearchResult, error) {
	q := query + " topic:canopy-package"
	if pkgType != "" {
		q += " topic:canopy-" + pkgType
	}
	if limit <= 0 {
		limit = 20
	}

	params := url.Values{
		"q":        {q},
		"per_page": {fmt.Sprintf("%d", limit)},
		"sort":     {"stars"},
		"order":    {"desc"},
	}
	var result SearchResult
	if err := c.do("GET", "/search/repositories?"+params.Encode(), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BrowseRepos lists canopy packages sorted by the given field.
func (c *Client) BrowseRepos(pkgType string, sort string, limit int) (*SearchResult, error) {
	q := "topic:canopy-package"
	if pkgType != "" {
		q += " topic:canopy-" + pkgType
	}
	if sort == "" {
		sort = "stars"
	}
	if limit <= 0 {
		limit = 20
	}

	params := url.Values{
		"q":        {q},
		"per_page": {fmt.Sprintf("%d", limit)},
		"sort":     {sort},
		"order":    {"desc"},
	}
	var result SearchResult
	if err := c.do("GET", "/search/repositories?"+params.Encode(), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTags returns all tags for a repo, most recent first.
func (c *Client) ListTags(ownerRepo string) ([]Tag, error) {
	var tags []Tag
	if err := c.do("GET", "/repos/"+ownerRepo+"/tags?per_page=100", nil, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// GetLatestRelease returns the latest release for a repo.
func (c *Client) GetLatestRelease(ownerRepo string) (*Release, error) {
	var rel Release
	if err := c.do("GET", "/repos/"+ownerRepo+"/releases/latest", nil, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// GetReleaseByTag returns a release for a specific tag.
func (c *Client) GetReleaseByTag(ownerRepo, tag string) (*Release, error) {
	var rel Release
	if err := c.do("GET", "/repos/"+ownerRepo+"/releases/tags/"+tag, nil, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// GetFileContent reads a file from a repo at a given ref (tag, branch, or commit).
func (c *Client) GetFileContent(ownerRepo, path, ref string) ([]byte, error) {
	u := "/repos/" + ownerRepo + "/contents/" + path
	if ref != "" {
		u += "?ref=" + url.QueryEscape(ref)
	}
	var fc FileContent
	if err := c.do("GET", u, nil, &fc); err != nil {
		return nil, err
	}
	if fc.Encoding == "base64" {
		clean := strings.ReplaceAll(fc.Content, "\n", "")
		return base64.StdEncoding.DecodeString(clean)
	}
	return []byte(fc.Content), nil
}

// DownloadTarball downloads a tarball for the given ref and returns the bytes.
func (c *Client) DownloadTarball(ownerRepo, ref string) ([]byte, error) {
	u := c.baseURL + "/repos/" + ownerRepo + "/tarball"
	if ref != "" {
		u += "/" + ref
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download tarball: %d %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// CreateRelease creates a new release on a repo.
func (c *Client) CreateRelease(ownerRepo string, req CreateReleaseRequest) (*Release, error) {
	var rel Release
	if err := c.do("POST", "/repos/"+ownerRepo+"/releases", req, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// CreateTag creates a lightweight tag ref pointing to a commit SHA.
func (c *Client) CreateTag(ownerRepo, tag, sha string) error {
	body := map[string]string{
		"ref": "refs/tags/" + tag,
		"sha": sha,
	}
	return c.do("POST", "/repos/"+ownerRepo+"/git/refs", body, nil)
}

// GetDefaultBranchSHA returns the HEAD commit SHA of the default branch.
func (c *Client) GetDefaultBranchSHA(ownerRepo string) (string, error) {
	repo, err := c.GetRepo(ownerRepo)
	if err != nil {
		return "", err
	}
	branch := repo.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	var ref GitRef
	if err := c.do("GET", "/repos/"+ownerRepo+"/git/ref/heads/"+branch, nil, &ref); err != nil {
		return "", err
	}
	return ref.Object.SHA, nil
}

// SetTopics replaces the repo's topics.
func (c *Client) SetTopics(ownerRepo string, topics []string) error {
	return c.do("PUT", "/repos/"+ownerRepo+"/topics", TopicsRequest{Names: topics}, nil)
}

// GetTopics returns the repo's current topics.
func (c *Client) GetTopics(ownerRepo string) ([]string, error) {
	var result struct {
		Names []string `json:"names"`
	}
	if err := c.do("GET", "/repos/"+ownerRepo+"/topics", nil, &result); err != nil {
		return nil, err
	}
	return result.Names, nil
}
