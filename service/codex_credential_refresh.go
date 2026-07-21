package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type CodexCredentialRefreshOptions struct {
	ResetCaches bool
}

type CodexOAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	AccountID   string `json:"account_id,omitempty"`
	LastRefresh string `json:"last_refresh,omitempty"`
	Email       string `json:"email,omitempty"`
	PlanType    string `json:"plan_type,omitempty"`
	Type        string `json:"type,omitempty"`
	Expired     string `json:"expired,omitempty"`
}

func parseCodexOAuthKey(raw string) (*CodexOAuthKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("codex channel: empty oauth key")
	}
	var key CodexOAuthKey
	if err := common.Unmarshal([]byte(raw), &key); err != nil {
		return nil, errors.New("codex channel: invalid oauth key json")
	}
	return &key, nil
}

func RefreshCodexChannelCredential(ctx context.Context, channelID int, opts CodexCredentialRefreshOptions) (*CodexOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeCodex {
		return nil, nil, fmt.Errorf("channel type is not Codex")
	}

	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("codex channel: refresh_token is required to refresh credential")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	proxyURL, err := ResolveChannelProxyURL(ch)
	if err != nil {
		return nil, nil, err
	}
	res, err := RefreshCodexOAuthTokenWithProxy(refreshCtx, oauthKey.RefreshToken, proxyURL)
	if err != nil {
		return nil, nil, err
	}

	oauthKey.AccessToken = res.AccessToken
	oauthKey.RefreshToken = res.RefreshToken
	if strings.TrimSpace(res.IDToken) != "" {
		oauthKey.IDToken = res.IDToken
	}
	oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
	oauthKey.Expired = res.ExpiresAt.Format(time.RFC3339)
	if strings.TrimSpace(oauthKey.Type) == "" {
		oauthKey.Type = "codex"
	}

	if strings.TrimSpace(oauthKey.AccountID) == "" {
		if accountID, ok := ExtractCodexAccountIDFromJWT(oauthKey.AccessToken); ok {
			oauthKey.AccountID = accountID
		}
	}
	if strings.TrimSpace(oauthKey.Email) == "" {
		emailToken := oauthKey.IDToken
		if strings.TrimSpace(emailToken) == "" {
			emailToken = oauthKey.AccessToken
		}
		if email, ok := ExtractEmailFromJWT(emailToken); ok {
			oauthKey.Email = email
		}
	}
	if strings.TrimSpace(oauthKey.PlanType) == "" {
		if planType, ok := ExtractCodexPlanTypeFromJWT(oauthKey.AccessToken); ok {
			oauthKey.PlanType = planType
		}
	}

	encoded, err := common.Marshal(oauthKey)
	if err != nil {
		return nil, nil, err
	}

	if err := model.DB.Model(&model.Channel{}).Where("id = ?", ch.Id).Update("key", string(encoded)).Error; err != nil {
		return nil, nil, err
	}

	if opts.ResetCaches {
		model.InitChannelCache()
		ResetProxyClientCache()
	}

	return oauthKey, ch, nil
}
