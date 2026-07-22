package codexoauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	ClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	TokenURL    = "https://auth.openai.com/oauth/token"
	RedirectURI = "http://localhost:1455/auth/callback"
)

type TokenResult struct {
	IDToken      string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func ExchangeAuthorizationCode(ctx context.Context, client *http.Client, code string, verifier string) (*TokenResult, error) {
	if client == nil {
		return nil, errors.New("OAuth HTTP client is nil")
	}
	trimmedCode := strings.TrimSpace(code)
	trimmedVerifier := strings.TrimSpace(verifier)
	if trimmedCode == "" {
		return nil, errors.New("empty authorization code")
	}
	if trimmedVerifier == "" {
		return nil, errors.New("empty code_verifier")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", ClientID)
	form.Set("code", trimmedCode)
	form.Set("code_verifier", trimmedVerifier)
	form.Set("redirect_uri", RedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("codex oauth code exchange failed: status=%d", resp.StatusCode)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || strings.TrimSpace(payload.RefreshToken) == "" || payload.ExpiresIn <= 0 {
		return nil, errors.New("codex oauth token response missing fields")
	}

	return &TokenResult{
		IDToken:      strings.TrimSpace(payload.IDToken),
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		ExpiresAt:    time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}
