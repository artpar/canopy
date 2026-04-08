package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OAuth App client ID for Canopy. Users authorize via device flow.
// To create your own: https://github.com/settings/applications/new
// Set "Device flow" enabled, callback URL can be anything.
const oauthClientID = "CANOPY_GITHUB_CLIENT_ID"

var (
	deviceCodeURL = "https://github.com/login/device/code"
	tokenURL      = "https://github.com/login/oauth/access_token"
)

const oauthScope = "public_repo"

// authPath returns ~/.canopy/auth.json, creating the directory if needed.
func authPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".canopy")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "auth.json"), nil
}

// LoadToken reads the stored GitHub token from disk. Returns nil if not found.
func LoadToken() (*StoredToken, error) {
	p, err := authPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var auth StoredAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, err
	}
	return auth.GitHub, nil
}

// SaveToken writes the GitHub token to disk with mode 0600.
func SaveToken(tok *StoredToken) error {
	p, err := authPath()
	if err != nil {
		return err
	}
	auth := StoredAuth{GitHub: tok}
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// DeviceFlowLogin initiates the GitHub OAuth device flow.
// It returns the verification URI and user code for display, then polls until
// the user authorizes. The callback is invoked with (userCode, verificationURI)
// so the caller can display instructions.
func DeviceFlowLogin(onPrompt func(userCode, verificationURI string)) (*StoredToken, error) {
	clientID := os.Getenv("CANOPY_GITHUB_CLIENT_ID")
	if clientID == "" {
		clientID = oauthClientID
	}

	// Step 1: Request device code
	resp, err := http.PostForm(deviceCodeURL, url.Values{
		"client_id": {clientID},
		"scope":     {oauthScope},
	})
	if err != nil {
		return nil, fmt.Errorf("device code request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// GitHub returns form-encoded by default, but we request JSON
	var dcr DeviceCodeResponse
	if resp.Header.Get("Content-Type") == "application/json" || strings.HasPrefix(string(body), "{") {
		if err := json.Unmarshal(body, &dcr); err != nil {
			return nil, fmt.Errorf("parse device code response: %w", err)
		}
	} else {
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, fmt.Errorf("parse device code form: %w", err)
		}
		dcr.DeviceCode = vals.Get("device_code")
		dcr.UserCode = vals.Get("user_code")
		dcr.VerificationURI = vals.Get("verification_uri")
		if v := vals.Get("interval"); v != "" {
			fmt.Sscanf(v, "%d", &dcr.Interval)
		}
		if v := vals.Get("expires_in"); v != "" {
			fmt.Sscanf(v, "%d", &dcr.ExpiresIn)
		}
	}

	if dcr.DeviceCode == "" {
		return nil, fmt.Errorf("empty device code in response: %s", string(body))
	}
	if dcr.Interval < 5 {
		dcr.Interval = 5
	}

	// Step 2: Show user the code
	onPrompt(dcr.UserCode, dcr.VerificationURI)

	// Step 3: Poll for token
	deadline := time.Now().Add(time.Duration(dcr.ExpiresIn) * time.Second)
	interval := time.Duration(dcr.Interval) * time.Second

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		tok, done, err := pollToken(clientID, dcr.DeviceCode)
		if err != nil {
			return nil, err
		}
		if done {
			stored := &StoredToken{
				AccessToken: tok.AccessToken,
				TokenType:   tok.TokenType,
				Scope:       tok.Scope,
				CreatedAt:   time.Now().UTC(),
			}
			if err := SaveToken(stored); err != nil {
				return stored, fmt.Errorf("token obtained but failed to save: %w", err)
			}
			return stored, nil
		}
	}
	return nil, fmt.Errorf("device flow timed out after %ds", dcr.ExpiresIn)
}

func pollToken(clientID, deviceCode string) (*TokenResponse, bool, error) {
	resp, err := http.PostForm(tokenURL, url.Values{
		"client_id":   {clientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	})
	if err != nil {
		return nil, false, fmt.Errorf("token poll: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tr TokenResponse
	if strings.HasPrefix(string(body), "{") {
		json.Unmarshal(body, &tr)
	} else {
		vals, _ := url.ParseQuery(string(body))
		tr.AccessToken = vals.Get("access_token")
		tr.TokenType = vals.Get("token_type")
		tr.Scope = vals.Get("scope")
		tr.Error = vals.Get("error")
	}

	switch tr.Error {
	case "authorization_pending":
		return nil, false, nil
	case "slow_down":
		return nil, false, nil
	case "expired_token":
		return nil, false, fmt.Errorf("device code expired")
	case "access_denied":
		return nil, false, fmt.Errorf("user denied authorization")
	case "":
		if tr.AccessToken != "" {
			return &tr, true, nil
		}
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("oauth error: %s", tr.Error)
	}
}
