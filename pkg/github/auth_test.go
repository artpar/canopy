package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestSaveAndLoadToken(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	tok := &StoredToken{
		AccessToken: "gho_test123",
		TokenType:   "bearer",
		Scope:       "public_repo",
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := SaveToken(tok); err != nil {
		t.Fatal(err)
	}

	// Verify file permissions
	p := filepath.Join(tmpDir, ".canopy", "auth.json")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600, got %o", info.Mode().Perm())
	}

	loaded, err := LoadToken()
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("loaded nil")
	}
	if loaded.AccessToken != "gho_test123" {
		t.Errorf("got %q", loaded.AccessToken)
	}
	if loaded.Scope != "public_repo" {
		t.Errorf("got scope %q", loaded.Scope)
	}
}

func TestLoadTokenNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	tok, err := LoadToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != nil {
		t.Error("expected nil for missing token")
	}
}

func TestLoadTokenInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	dir := filepath.Join(tmpDir, ".canopy")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "auth.json"), []byte("not json"), 0600)

	_, err := LoadToken()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadTokenNoGitHub(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	dir := filepath.Join(tmpDir, ".canopy")
	os.MkdirAll(dir, 0700)
	data, _ := json.Marshal(StoredAuth{})
	os.WriteFile(filepath.Join(dir, "auth.json"), data, 0600)

	tok, err := LoadToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != nil {
		t.Error("expected nil for empty auth")
	}
}

func TestAuthPath(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	p, err := authPath()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(tmpDir, ".canopy", "auth.json")
	if p != expected {
		t.Errorf("got %q, want %q", p, expected)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".canopy")); err != nil {
		t.Error("directory not created")
	}
}

func TestDeviceFlowLoginJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	var pollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DeviceCodeResponse{
				DeviceCode:      "DEVCODE",
				UserCode:        "USER-CODE",
				VerificationURI: "https://github.com/login/device",
				ExpiresIn:       10,
				Interval:        1,
			})
			return
		}
		if r.URL.Path == "/login/oauth/access_token" {
			n := pollCount.Add(1)
			if n < 2 {
				// First poll: pending
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(TokenResponse{Error: "authorization_pending"})
				return
			}
			// Second poll: success
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: "gho_success",
				TokenType:   "bearer",
				Scope:       "public_repo",
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	// Override URLs
	origDeviceURL := deviceCodeURL
	origTokenURL := tokenURL
	deviceCodeURL = srv.URL + "/login/device/code"
	tokenURL = srv.URL + "/login/oauth/access_token"
	defer func() {
		deviceCodeURL = origDeviceURL
		tokenURL = origTokenURL
	}()

	var gotCode, gotURI string
	tok, err := DeviceFlowLogin(func(code, uri string) {
		gotCode = code
		gotURI = uri
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotCode != "USER-CODE" {
		t.Errorf("got code %q", gotCode)
	}
	if gotURI != "https://github.com/login/device" {
		t.Errorf("got uri %q", gotURI)
	}
	if tok.AccessToken != "gho_success" {
		t.Errorf("got token %q", tok.AccessToken)
	}

	// Token should be saved to disk
	loaded, _ := LoadToken()
	if loaded == nil || loaded.AccessToken != "gho_success" {
		t.Error("token not saved")
	}
}

func TestDeviceFlowLoginFormEncoded(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			// Form-encoded response (GitHub default)
			w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			w.Write([]byte("device_code=DC123&user_code=UC-456&verification_uri=https://verify&interval=1&expires_in=10"))
			return
		}
		if r.URL.Path == "/login/oauth/access_token" {
			// Form-encoded token
			w.Write([]byte("access_token=gho_form&token_type=bearer&scope=public_repo"))
			return
		}
	}))
	defer srv.Close()

	origDeviceURL := deviceCodeURL
	origTokenURL := tokenURL
	deviceCodeURL = srv.URL + "/login/device/code"
	tokenURL = srv.URL + "/login/oauth/access_token"
	defer func() {
		deviceCodeURL = origDeviceURL
		tokenURL = origTokenURL
	}()

	tok, err := DeviceFlowLogin(func(code, uri string) {})
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "gho_form" {
		t.Errorf("got %q", tok.AccessToken)
	}
}

func TestDeviceFlowLoginEmptyDeviceCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCodeResponse{})
	}))
	defer srv.Close()

	origDeviceURL := deviceCodeURL
	deviceCodeURL = srv.URL + "/login/device/code"
	defer func() { deviceCodeURL = origDeviceURL }()

	_, err := DeviceFlowLogin(func(code, uri string) {})
	if err == nil {
		t.Error("expected error for empty device code")
	}
}

func TestDeviceFlowLoginAccessDenied(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DeviceCodeResponse{
				DeviceCode: "DC", UserCode: "UC",
				VerificationURI: "https://v", ExpiresIn: 10, Interval: 1,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{Error: "access_denied"})
	}))
	defer srv.Close()

	origDeviceURL := deviceCodeURL
	origTokenURL := tokenURL
	deviceCodeURL = srv.URL + "/login/device/code"
	tokenURL = srv.URL + "/login/oauth/access_token"
	defer func() {
		deviceCodeURL = origDeviceURL
		tokenURL = origTokenURL
	}()

	_, err := DeviceFlowLogin(func(code, uri string) {})
	if err == nil {
		t.Error("expected error for access denied")
	}
}

func TestDeviceFlowLoginExpired(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DeviceCodeResponse{
				DeviceCode: "DC", UserCode: "UC",
				VerificationURI: "https://v", ExpiresIn: 10, Interval: 1,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{Error: "expired_token"})
	}))
	defer srv.Close()

	origDeviceURL := deviceCodeURL
	origTokenURL := tokenURL
	deviceCodeURL = srv.URL + "/login/device/code"
	tokenURL = srv.URL + "/login/oauth/access_token"
	defer func() {
		deviceCodeURL = origDeviceURL
		tokenURL = origTokenURL
	}()

	_, err := DeviceFlowLogin(func(code, uri string) {})
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestDeviceFlowLoginSlowDown(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DeviceCodeResponse{
				DeviceCode: "DC", UserCode: "UC",
				VerificationURI: "https://v", ExpiresIn: 10, Interval: 1,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		n := count.Add(1)
		if n == 1 {
			json.NewEncoder(w).Encode(TokenResponse{Error: "slow_down"})
			return
		}
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "gho_after_slow",
			TokenType:   "bearer",
			Scope:       "public_repo",
		})
	}))
	defer srv.Close()

	origDeviceURL := deviceCodeURL
	origTokenURL := tokenURL
	deviceCodeURL = srv.URL + "/login/device/code"
	tokenURL = srv.URL + "/login/oauth/access_token"
	defer func() {
		deviceCodeURL = origDeviceURL
		tokenURL = origTokenURL
	}()

	tok, err := DeviceFlowLogin(func(code, uri string) {})
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "gho_after_slow" {
		t.Errorf("got %q", tok.AccessToken)
	}
}

func TestDeviceFlowLoginUnknownError(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DeviceCodeResponse{
				DeviceCode: "DC", UserCode: "UC",
				VerificationURI: "https://v", ExpiresIn: 10, Interval: 1,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{Error: "server_error"})
	}))
	defer srv.Close()

	origDeviceURL := deviceCodeURL
	origTokenURL := tokenURL
	deviceCodeURL = srv.URL + "/login/device/code"
	tokenURL = srv.URL + "/login/oauth/access_token"
	defer func() {
		deviceCodeURL = origDeviceURL
		tokenURL = origTokenURL
	}()

	_, err := DeviceFlowLogin(func(code, uri string) {})
	if err == nil {
		t.Error("expected error")
	}
}

func TestNewClientFromStored(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// No token stored
	c, err := NewClientFromStored()
	if err != nil {
		t.Fatal(err)
	}
	if c.IsAuthenticated() {
		t.Error("should be unauthenticated")
	}

	// Save a token
	SaveToken(&StoredToken{AccessToken: "tok123"})
	c, err = NewClientFromStored()
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsAuthenticated() {
		t.Error("should be authenticated")
	}
}
