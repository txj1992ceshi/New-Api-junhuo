package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func replaceChannelKeyAtIndex(channel *model.Channel, keyIndex int, encoded string) error {
	keys := channel.GetKeys()
	if keyIndex < 0 || keyIndex >= len(keys) {
		return fmt.Errorf("codex key index out of range")
	}
	keys[keyIndex] = encoded
	channel.Key = strings.Join(keys, "\n")
	return nil
}

func RefreshCodexChannelKeyCredential(ctx context.Context, channelID int, keyIndex int, resetCaches bool) (*CodexOAuthKey, *model.Channel, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if channel == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if channel.Type != constant.ChannelTypeCodex {
		return nil, nil, fmt.Errorf("channel type is not Codex")
	}
	keys := channel.GetKeys()
	if keyIndex < 0 || keyIndex >= len(keys) {
		return nil, nil, fmt.Errorf("codex key index out of range")
	}

	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(keys[keyIndex]))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(oauthKey.RefreshToken) == "" {
		return nil, nil, fmt.Errorf("codex channel key: refresh_token is required")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := RefreshCodexOAuthTokenWithProxy(refreshCtx, oauthKey.RefreshToken, channel.GetSetting().Proxy)
	if err != nil {
		return nil, nil, err
	}

	oauthKey.AccessToken = res.AccessToken
	oauthKey.RefreshToken = res.RefreshToken
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
		if email, ok := ExtractEmailFromJWT(oauthKey.AccessToken); ok {
			oauthKey.Email = email
		}
	}

	encoded, err := common.Marshal(oauthKey)
	if err != nil {
		return nil, nil, err
	}

	lock := model.GetChannelPollingLock(channelID)
	lock.Lock()
	defer lock.Unlock()

	if err := replaceChannelKeyAtIndex(channel, keyIndex, string(encoded)); err != nil {
		return nil, nil, err
	}
	meta := hydrateCodexKeyMeta(string(encoded), channel.GetKeyMeta(keyIndex))
	meta.LastRefreshAt = time.Now().Unix()
	meta.ExpiresAt = oauthKey.Expired
	meta.AccountID = oauthKey.AccountID
	meta.Email = oauthKey.Email
	meta.ConsecutiveAuthFail = 0
	meta.CooldownUntil = 0
	meta.LastErrorKind = ""
	switch meta.State {
	case model.CodexKeyStateRefreshing:
		meta.State = model.CodexKeyStateHealthy
	case model.CodexKeyStateDead:
		meta.State = model.CodexKeyStateHealthy
	}
	channel.SetKeyMeta(keyIndex, meta)
	finalizeCodexMultiKeyChannel(channel)

	if err := model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"key":          channel.Key,
		"channel_info": channel.ChannelInfo,
	}).Error; err != nil {
		return nil, nil, err
	}

	if resetCaches {
		model.InitChannelCache()
		ResetProxyClientCache()
	} else if common.MemoryCacheEnabled {
		model.CacheUpdateChannel(channel)
	}

	return oauthKey, channel, nil
}

func markCodexKeyDead(channelID int, keyIndex int, reason string) error {
	return updateCodexKeyMeta(channelID, keyIndex, func(channel *model.Channel, meta *model.ChannelKeyMeta, now time.Time) {
		meta.State = model.CodexKeyStateDead
		meta.LastErrorAt = now.Unix()
		meta.LastErrorKind = reason
		meta.TotalFail++
	})
}
