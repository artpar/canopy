package github

import "time"

// Repo represents a GitHub repository.
type Repo struct {
	FullName    string   `json:"full_name"`
	Description string   `json:"description"`
	HTMLURL     string   `json:"html_url"`
	CloneURL    string   `json:"clone_url"`
	Stars       int      `json:"stargazers_count"`
	Topics      []string `json:"topics"`
	UpdatedAt   time.Time `json:"updated_at"`
	Archived    bool     `json:"archived"`
	DefaultBranch string `json:"default_branch"`
}

// Release represents a GitHub release.
type Release struct {
	ID         int       `json:"id"`
	TagName    string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	CreatedAt  time.Time `json:"created_at"`
	TarballURL string    `json:"tarball_url"`
	HTMLURL    string    `json:"html_url"`
}

// Tag represents a git tag.
type Tag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// SearchResult is the response from GitHub's search API.
type SearchResult struct {
	TotalCount int    `json:"total_count"`
	Items      []Repo `json:"items"`
}

// FileContent represents a file fetched from a repo.
type FileContent struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
}

// DeviceCodeResponse is returned when initiating the device flow.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is returned when polling for the access token.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

// StoredAuth persists to ~/.canopy/auth.json.
type StoredAuth struct {
	GitHub *StoredToken `json:"github,omitempty"`
}

// StoredToken is a persisted OAuth token.
type StoredToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	CreatedAt   time.Time `json:"created_at"`
}

// GitRef is used to create a tag via the Git refs API.
type GitRef struct {
	Ref    string `json:"ref"`
	SHA    string `json:"sha"`
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

// CreateReleaseRequest is the payload for creating a release.
type CreateReleaseRequest struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Draft   bool   `json:"draft"`
}

// TopicsRequest is the payload for replacing repo topics.
type TopicsRequest struct {
	Names []string `json:"names"`
}
