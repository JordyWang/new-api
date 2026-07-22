package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/codexoauth"
)

const (
	codexOAuthClientID     = codexoauth.ClientID
	codexOAuthAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	codexOAuthTokenURL     = codexoauth.TokenURL
	codexOAuthRedirectURI  = codexoauth.RedirectURI
	codexOAuthScope        = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	codexJWTClaimPath      = "https://api.openai.com/auth"
	defaultHTTPTimeout     = 20 * time.Second
)

type CodexOAuthTokenResult = codexoauth.TokenResult

type CodexOAuthAuthorizationFlow struct {
	State        string
	Verifier     string
	Challenge    string
	AuthorizeURL string
}

func CreateCodexOAuthAuthorizationFlow() (*CodexOAuthAuthorizationFlow, error) {
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, err
	}
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, err
	}

	state := fmt.Sprintf("%x", stateBytes)
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])

	u, err := url.Parse(codexOAuthAuthorizeURL)
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Set("response_type", "code")
	query.Set("client_id", codexOAuthClientID)
	query.Set("redirect_uri", codexOAuthRedirectURI)
	query.Set("scope", codexOAuthScope)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	query.Set("id_token_add_organizations", "true")
	query.Set("codex_cli_simplified_flow", "true")
	query.Set("originator", "codex-tui")
	u.RawQuery = query.Encode()

	return &CodexOAuthAuthorizationFlow{
		State:        state,
		Verifier:     verifier,
		Challenge:    challenge,
		AuthorizeURL: u.String(),
	}, nil
}

func RefreshCodexOAuthToken(ctx context.Context, refreshToken string) (*CodexOAuthTokenResult, error) {
	return RefreshCodexOAuthTokenWithProxy(ctx, refreshToken, "")
}

func RefreshCodexOAuthTokenWithProxy(ctx context.Context, refreshToken string, proxyURL string) (*CodexOAuthTokenResult, error) {
	client, err := getCodexOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	return refreshCodexOAuthToken(ctx, client, codexOAuthTokenURL, codexOAuthClientID, refreshToken)
}

func ExchangeCodexAuthorizationCodeWithProxy(ctx context.Context, code string, verifier string, proxyURL string) (*CodexOAuthTokenResult, error) {
	client, err := getCodexOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	return codexoauth.ExchangeAuthorizationCode(ctx, client, code, verifier)
}

func refreshCodexOAuthToken(
	ctx context.Context,
	client *http.Client,
	tokenURL string,
	clientID string,
	refreshToken string,
) (*CodexOAuthTokenResult, error) {
	rt := strings.TrimSpace(refreshToken)
	if rt == "" {
		return nil, errors.New("empty refresh_token")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", rt)
	form.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
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
		return nil, fmt.Errorf("codex oauth refresh failed: status=%d", resp.StatusCode)
	}

	if strings.TrimSpace(payload.AccessToken) == "" || strings.TrimSpace(payload.RefreshToken) == "" || payload.ExpiresIn <= 0 {
		return nil, errors.New("codex oauth refresh response missing fields")
	}

	return &CodexOAuthTokenResult{
		IDToken:      strings.TrimSpace(payload.IDToken),
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		ExpiresAt:    time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

func getCodexOAuthHTTPClient(proxyURL string) (*http.Client, error) {
	baseClient, err := GetHttpClientWithProxy(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	if baseClient == nil {
		return &http.Client{Timeout: defaultHTTPTimeout}, nil
	}
	clientCopy := *baseClient
	clientCopy.Timeout = defaultHTTPTimeout
	return &clientCopy, nil
}

func ExtractCodexAccountIDFromJWT(token string) (string, bool) {
	claims, ok := decodeJWTClaims(token)
	if !ok {
		return "", false
	}
	raw, ok := claims[codexJWTClaimPath]
	if !ok {
		return "", false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return "", false
	}
	v, ok := obj["chatgpt_account_id"]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func ExtractEmailFromJWT(token string) (string, bool) {
	claims, ok := decodeJWTClaims(token)
	if !ok {
		return "", false
	}
	v, ok := claims["email"]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func ExtractCodexPlanTypeFromJWT(token string) (string, bool) {
	claims, ok := decodeJWTClaims(token)
	if !ok {
		return "", false
	}
	raw, ok := claims[codexJWTClaimPath]
	if !ok {
		return "", false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return "", false
	}
	value, ok := obj["chatgpt_plan_type"]
	if !ok {
		return "", false
	}
	planType, ok := value.(string)
	if !ok {
		return "", false
	}
	planType = strings.TrimSpace(planType)
	if planType == "" {
		return "", false
	}
	return planType, true
}

func decodeJWTClaims(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}
	var claims map[string]any
	if err := common.Unmarshal(payloadRaw, &claims); err != nil {
		return nil, false
	}
	return claims, true
}
