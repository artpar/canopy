package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListTagsError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("error"))
	})
	_, err := client.ListTags("owner/repo")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetLatestReleaseError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	})
	_, err := client.GetLatestRelease("owner/repo")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetReleaseByTagError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	})
	_, err := client.GetReleaseByTag("owner/repo", "v1.0.0")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetFileContentError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	})
	_, err := client.GetFileContent("owner/repo", "file.txt", "main")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCreateReleaseError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte("already exists"))
	})
	_, err := client.CreateRelease("owner/repo", CreateReleaseRequest{TagName: "v1.0.0"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetDefaultBranchSHARepoError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	})
	_, err := client.GetDefaultBranchSHA("owner/repo")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetDefaultBranchSHARefError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo" {
			json.NewEncoder(w).Encode(Repo{DefaultBranch: "main"})
			return
		}
		w.WriteHeader(404)
		w.Write([]byte("ref not found"))
	})
	_, err := client.GetDefaultBranchSHA("owner/repo")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetTopicsError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	})
	_, err := client.GetTopics("owner/repo")
	if err == nil {
		t.Error("expected error")
	}
}

func TestSearchReposError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})
	_, err := client.SearchRepos("test", "", 0)
	if err == nil {
		t.Error("expected error")
	}
}

func TestBrowseReposError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})
	_, err := client.BrowseRepos("", "", 0)
	if err == nil {
		t.Error("expected error")
	}
}

func TestDoWithBody(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type for POST with body")
		}
		w.Write([]byte("{}"))
	})

	err := client.do("POST", "/test", map[string]string{"key": "val"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDoAbsoluteURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	client := NewTestClient("", "http://unused")
	// Pass absolute URL — should not prepend baseURL
	err := client.do("GET", srv.URL+"/test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetTopicsError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("forbidden"))
	})
	err := client.SetTopics("owner/repo", []string{"test"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestNewTestClient(t *testing.T) {
	c := NewTestClient("tok", "http://localhost:1234")
	if c.baseURL != "http://localhost:1234" {
		t.Errorf("got %q", c.baseURL)
	}
	if !c.IsAuthenticated() {
		t.Error("should be authenticated")
	}
}
