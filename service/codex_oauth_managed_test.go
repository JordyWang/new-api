package service

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCodexOAuthAuthorizationFlowUsesPKCEAndManagedCallback(t *testing.T) {
	flow, err := CreateCodexOAuthAuthorizationFlow()
	require.NoError(t, err)
	assert.NotEmpty(t, flow.State)
	assert.NotEmpty(t, flow.Verifier)
	assert.NotEmpty(t, flow.Challenge)

	authorizeURL, err := url.Parse(flow.AuthorizeURL)
	require.NoError(t, err)
	assert.Equal(t, "https", authorizeURL.Scheme)
	assert.Equal(t, "auth.openai.com", authorizeURL.Host)
	assert.Equal(t, "/oauth/authorize", authorizeURL.Path)
	assert.Equal(t, "http://localhost:1455/auth/callback", authorizeURL.Query().Get("redirect_uri"))
	assert.Equal(t, flow.State, authorizeURL.Query().Get("state"))
	assert.Equal(t, flow.Challenge, authorizeURL.Query().Get("code_challenge"))
	assert.Equal(t, "S256", authorizeURL.Query().Get("code_challenge_method"))
	assert.Contains(t, strings.Fields(authorizeURL.Query().Get("scope")), "offline_access")
}

func TestExtractCodexManagedAccountMetadataFromJWT(t *testing.T) {
	claims := map[string]any{
		"email": "owner@example.com",
		codexJWTClaimPath: map[string]any{
			"chatgpt_account_id": "acc_123",
			"chatgpt_plan_type":  "pro",
		},
	}
	payload, err := common.Marshal(claims)
	require.NoError(t, err)
	token := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"

	accountId, ok := ExtractCodexAccountIDFromJWT(token)
	require.True(t, ok)
	assert.Equal(t, "acc_123", accountId)
	email, ok := ExtractEmailFromJWT(token)
	require.True(t, ok)
	assert.Equal(t, "owner@example.com", email)
	planType, ok := ExtractCodexPlanTypeFromJWT(token)
	require.True(t, ok)
	assert.Equal(t, "pro", planType)
}

func TestCodexBrowserOAuthFlowViewSeparatesFlowAndCredentialExpiry(t *testing.T) {
	now := time.Now().Unix()
	flow := &model.CodexOAuthFlow{
		Id:                  "flow_123",
		Status:              model.CodexOAuthFlowStatusCompleted,
		ProfileId:           11,
		AgentId:             22,
		ProxyId:             33,
		CredentialExpiresAt: now + 3600,
		ExpiresAt:           now + 600,
		CreatedAt:           now,
		CompletedAt:         now + 5,
	}
	profile := &model.BrowserProfile{Id: 11, Name: "primary"}
	agent := &model.BrowserAgent{Id: 22, Name: "desktop", Enabled: true, LastSeenAt: now}

	view := codexBrowserOAuthFlowView(flow, profile, agent, now)

	assert.Equal(t, flow.CredentialExpiresAt, view.CredentialExpiresAt)
	assert.Equal(t, flow.ExpiresAt, view.ExpiresAt)
	assert.NotEqual(t, view.ExpiresAt, view.CredentialExpiresAt)
}
