package github

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewTestClient("test-token", srv.URL), srv
}

func TestNewClient(t *testing.T) {
	c := NewClient("tok123")
	if !c.IsAuthenticated() {
		t.Error("expected authenticated")
	}
	c2 := NewClient("")
	if c2.IsAuthenticated() {
		t.Error("expected unauthenticated")
	}
}

func TestGetRepo(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth header")
		}
		json.NewEncoder(w).Encode(Repo{
			FullName:    "owner/repo",
			Description: "test repo",
			Stars:       42,
			DefaultBranch: "main",
		})
	})

	repo, err := client.GetRepo("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if repo.FullName != "owner/repo" {
		t.Errorf("got %q", repo.FullName)
	}
	if repo.Stars != 42 {
		t.Errorf("got %d stars", repo.Stars)
	}
}

func TestGetRepoError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"Not Found"}`))
	})

	_, err := client.GetRepo("owner/missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error: %v", err)
	}
}

func TestSearchRepos(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "topic:canopy-package") {
			t.Errorf("missing canopy-package topic in query: %s", q)
		}
		if !strings.Contains(q, "topic:canopy-app") {
			t.Errorf("missing type topic: %s", q)
		}
		json.NewEncoder(w).Encode(SearchResult{
			TotalCount: 1,
			Items:      []Repo{{FullName: "owner/app1", Stars: 10}},
		})
	})

	result, err := client.SearchRepos("calculator", "app", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 1 {
		t.Errorf("got %d results", result.TotalCount)
	}
}

func TestSearchReposDefaultLimit(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		pp := r.URL.Query().Get("per_page")
		if pp != "20" {
			t.Errorf("expected default limit 20, got %s", pp)
		}
		json.NewEncoder(w).Encode(SearchResult{})
	})

	client.SearchRepos("test", "", 0)
}

func TestBrowseRepos(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "topic:canopy-package") {
			t.Errorf("missing topic: %s", q)
		}
		s := r.URL.Query().Get("sort")
		if s != "stars" {
			t.Errorf("expected sort=stars, got %s", s)
		}
		json.NewEncoder(w).Encode(SearchResult{TotalCount: 2})
	})

	result, err := client.BrowseRepos("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 2 {
		t.Errorf("got %d", result.TotalCount)
	}
}

func TestBrowseReposWithType(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "topic:canopy-theme") {
			t.Errorf("missing type topic: %s", q)
		}
		s := r.URL.Query().Get("sort")
		if s != "updated" {
			t.Errorf("expected sort=updated, got %s", s)
		}
		json.NewEncoder(w).Encode(SearchResult{})
	})

	client.BrowseRepos("theme", "updated", 10)
}

func TestListTags(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/owner/repo/tags") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]Tag{
			{Name: "v1.0.0"},
			{Name: "v1.1.0"},
		})
	})

	tags, err := client.ListTags("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Errorf("got %d tags", len(tags))
	}
}

func TestGetLatestRelease(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Release{TagName: "v2.0.0", HTMLURL: "https://example.com"})
	})

	rel, err := client.GetLatestRelease("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "v2.0.0" {
		t.Errorf("got %q", rel.TagName)
	}
}

func TestGetReleaseByTag(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/tags/v1.0.0") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Release{TagName: "v1.0.0"})
	})

	rel, err := client.GetReleaseByTag("owner/repo", "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "v1.0.0" {
		t.Errorf("got %q", rel.TagName)
	}
}

func TestGetFileContent(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/contents/canopy.json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ref := r.URL.Query().Get("ref")
		if ref != "v1.0.0" {
			t.Errorf("expected ref=v1.0.0, got %s", ref)
		}
		content := base64.StdEncoding.EncodeToString([]byte(`{"name":"test"}`))
		json.NewEncoder(w).Encode(FileContent{Content: content, Encoding: "base64"})
	})

	data, err := client.GetFileContent("owner/repo", "canopy.json", "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"name":"test"}` {
		t.Errorf("got %q", string(data))
	}
}

func TestGetFileContentRaw(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(FileContent{Content: "raw content", Encoding: ""})
	})

	data, err := client.GetFileContent("owner/repo", "file.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "raw content" {
		t.Errorf("got %q", string(data))
	}
}

func TestDownloadTarball(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/tarball/v1.0.0") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte("fake-tarball-data"))
	})

	data, err := client.DownloadTarball("owner/repo", "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fake-tarball-data" {
		t.Errorf("got %q", string(data))
	}
}

func TestDownloadTarballNoRef(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tarball/") {
			t.Errorf("should not have ref in path: %s", r.URL.Path)
		}
		w.Write([]byte("data"))
	})

	client.DownloadTarball("owner/repo", "")
}

func TestDownloadTarballError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("forbidden"))
	})

	_, err := client.DownloadTarball("owner/repo", "v1.0.0")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateRelease(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req CreateReleaseRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.TagName != "v1.0.0" {
			t.Errorf("got tag %q", req.TagName)
		}
		json.NewEncoder(w).Encode(Release{TagName: "v1.0.0", HTMLURL: "https://release"})
	})

	rel, err := client.CreateRelease("owner/repo", CreateReleaseRequest{TagName: "v1.0.0", Name: "Release"})
	if err != nil {
		t.Fatal(err)
	}
	if rel.HTMLURL != "https://release" {
		t.Errorf("got %q", rel.HTMLURL)
	}
}

func TestCreateTag(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["ref"] != "refs/tags/v1.0.0" {
			t.Errorf("got ref %q", body["ref"])
		}
		w.WriteHeader(201)
	})

	err := client.CreateTag("owner/repo", "v1.0.0", "abc123")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDefaultBranchSHA(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/ref/") {
			json.NewEncoder(w).Encode(GitRef{Object: struct {
				SHA string `json:"sha"`
			}{SHA: "abc123"}})
		} else {
			json.NewEncoder(w).Encode(Repo{DefaultBranch: "main"})
		}
	})

	sha, err := client.GetDefaultBranchSHA("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if sha != "abc123" {
		t.Errorf("got %q", sha)
	}
}

func TestGetDefaultBranchSHAEmpty(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/ref/heads/main") {
			json.NewEncoder(w).Encode(GitRef{Object: struct {
				SHA string `json:"sha"`
			}{SHA: "def456"}})
		} else {
			json.NewEncoder(w).Encode(Repo{DefaultBranch: ""})
		}
	})

	sha, err := client.GetDefaultBranchSHA("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if sha != "def456" {
		t.Errorf("got %q", sha)
	}
}

func TestSetTopics(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		var req TopicsRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Names) != 2 {
			t.Errorf("got %d topics", len(req.Names))
		}
		w.WriteHeader(200)
	})

	err := client.SetTopics("owner/repo", []string{"canopy-package", "canopy-app"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetTopics(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(struct {
			Names []string `json:"names"`
		}{Names: []string{"go", "canopy-package"}})
	})

	topics, err := client.GetTopics("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 2 {
		t.Errorf("got %d topics", len(topics))
	}
}

func TestDoWithoutBody(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "" {
			t.Error("should not have Content-Type for GET")
		}
		w.Write([]byte("{}"))
	})

	err := client.do("GET", "/test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDoUnauthenticated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("should not have auth header")
		}
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	client := NewTestClient("", srv.URL)
	err := client.do("GET", "/test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}
