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

type AntigravityCredentialRefreshOptions struct {
	ResetCaches bool
}

type AntigravityOAuthKey struct {
	AccessToken      string `json:"access_token,omitempty"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	Expired          string `json:"expired,omitempty"`
	Email            string `json:"email,omitempty"`
	ProjectID        string `json:"project_id,omitempty"`
	ManagedProjectID string `json:"managed_project_id,omitempty"`
	Type             string `json:"type,omitempty"`
	LastRefresh      string `json:"last_refresh,omitempty"`
}

func ParseAntigravityOAuthKey(raw string) (*AntigravityOAuthKey, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("antigravity channel: empty oauth key")
	}
	var key AntigravityOAuthKey
	err := common.Unmarshal([]byte(raw), &key)
	if err != nil {
		return nil, errors.New("antigravity channel: invalid oauth key json")
	}
	if strings.TrimSpace(key.Type) == "" {
		key.Type = "antigravity"
	}
	return &key, nil
}

func (k *AntigravityOAuthKey) EffectiveProjectID() string {
	if k == nil {
		return ""
	}
	if strings.TrimSpace(k.ManagedProjectID) != "" {
		return strings.TrimSpace(k.ManagedProjectID)
	}
	return strings.TrimSpace(k.ProjectID)
}

func (k *AntigravityOAuthKey) ExpiresAt() time.Time {
	if k == nil {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339, strings.TrimSpace(k.Expired))
	if err != nil {
		return time.Time{}
	}
	return ts
}

func (k *AntigravityOAuthKey) IsAccessExpired(buffer time.Duration) bool {
	exp := k.ExpiresAt()
	if exp.IsZero() {
		return true
	}
	return time.Now().Add(buffer).After(exp)
}

func parseAntigravityOAuthKey(raw string) (*AntigravityOAuthKey, error) {
	return ParseAntigravityOAuthKey(raw)
}

func RefreshAntigravityCredentialWithProxy(ctx context.Context, raw string, proxy string) (*AntigravityOAuthKey, error) {
	oauthKey, err := ParseAntigravityOAuthKey(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, fmt.Errorf("antigravity channel: refresh_token is required to refresh credential")
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := RefreshAntigravityOAuthTokenWithProxy(refreshCtx, oauthKey.RefreshToken, proxy)
	if err != nil {
		return nil, err
	}
	oauthKey.AccessToken = res.AccessToken
	if strings.TrimSpace(res.RefreshToken) != "" {
		oauthKey.RefreshToken = res.RefreshToken
	}
	oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
	oauthKey.Expired = res.ExpiresAt.Format(time.RFC3339)
	if strings.TrimSpace(res.Email) != "" {
		oauthKey.Email = res.Email
	}
	if strings.TrimSpace(oauthKey.Type) == "" {
		oauthKey.Type = "antigravity"
	}
	return oauthKey, nil
}

func RefreshAntigravityChannelCredential(ctx context.Context, channelID int, opts AntigravityCredentialRefreshOptions) (*AntigravityOAuthKey, *model.Channel, error) {
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if ch.Type != constant.ChannelTypeAntigravity {
		return nil, nil, fmt.Errorf("channel type is not Antigravity")
	}
	if ch.ChannelInfo.IsMultiKey {
		return RefreshAntigravityChannelKeyCredential(ctx, channelID, 0, opts.ResetCaches)
	}
	oauthKey, err := RefreshAntigravityCredentialWithProxy(ctx, strings.TrimSpace(ch.Key), ch.GetSetting().Proxy)
	if err != nil {
		return nil, nil, err
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
