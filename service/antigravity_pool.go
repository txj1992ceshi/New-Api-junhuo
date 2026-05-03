package service

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type AntigravityErrorClass string

const (
	AntigravityErrorClassUnknown              AntigravityErrorClass = "unknown"
	AntigravityErrorClassAuth                 AntigravityErrorClass = "auth"
	AntigravityErrorClassProjectDenied        AntigravityErrorClass = "project_denied"
	AntigravityErrorClassRateLimited          AntigravityErrorClass = "rate_limited"
	AntigravityErrorClassModelUnavailable     AntigravityErrorClass = "model_unavailable"
	AntigravityErrorClassProtocolIncompatible AntigravityErrorClass = "protocol_incompatible"
	AntigravityErrorClassServer               AntigravityErrorClass = "server_error"
)

type AntigravityKeySelection struct {
	Key      string
	KeyIndex int
	Meta     *model.ChannelKeyMeta
	OAuthKey *AntigravityOAuthKey
}

var antigravityRestrictedProjects = map[string]struct{}{
	"rising-fact-p41fc": {},
}

func SelectAntigravityKey(channel *model.Channel, relayMode int, modelName string, excluded map[int]bool, now time.Time) (*AntigravityKeySelection, *types.NewAPIError) {
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	keys := channel.GetKeys()
	if len(keys) == 0 {
		return nil, types.NewError(fmt.Errorf("no antigravity keys available"), types.ErrorCodeChannelNoAvailableKey)
	}

	lock := model.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	channel.EnsureMultiKeyMeta()

	type candidate struct {
		index int
		key   string
		meta  model.ChannelKeyMeta
		oauth *AntigravityOAuthKey
		w     int
	}
	candidates := make([]candidate, 0, len(keys))
	for idx, rawKey := range keys {
		if excluded != nil && excluded[idx] {
			continue
		}
		if channel.ChannelInfo.MultiKeyStatusList != nil {
			if status, ok := channel.ChannelInfo.MultiKeyStatusList[idx]; ok && status != common.ChannelStatusEnabled {
				continue
			}
		}
		oauthKey, err := ParseAntigravityOAuthKey(strings.TrimSpace(rawKey))
		if err != nil {
			continue
		}
		meta := hydrateAntigravityKeyMeta(rawKey, channel.GetKeyMeta(idx))
		meta = applyAntigravityOAuthKeyToMeta(meta, oauthKey)
		if meta.State == model.CodexKeyStateDead || meta.State == model.CodexKeyStateRefreshing {
			continue
		}
		if meta.CooldownUntil > now.Unix() {
			continue
		}
		if strings.TrimSpace(meta.EffectiveProjectID) == "" {
			continue
		}
		if _, restricted := antigravityRestrictedProjects[strings.TrimSpace(meta.EffectiveProjectID)]; restricted {
			continue
		}
		if !antigravityKeySupportsModel(&meta, relayMode, modelName) {
			continue
		}
		weight := antigravityKeyWeight(meta, now)
		if weight <= 0 {
			continue
		}
		candidates = append(candidates, candidate{
			index: idx,
			key:   rawKey,
			meta:  meta,
			oauth: oauthKey,
			w:     weight,
		})
	}
	if len(candidates) == 0 {
		return nil, types.NewError(fmt.Errorf("no available antigravity keys"), types.ErrorCodeChannelNoAvailableKey)
	}
	totalWeight := 0
	for _, c := range candidates {
		totalWeight += c.w
	}
	selected := candidates[0]
	if totalWeight > 0 {
		r := rand.Intn(totalWeight)
		for _, c := range candidates {
			r -= c.w
			if r < 0 {
				selected = c
				break
			}
		}
	}
	selected.meta.LastSelectedAt = now.Unix()
	channel.SetKeyMeta(selected.index, selected.meta)
	channel.ChannelInfo.MultiKeyPollingIndex = (selected.index + 1) % len(keys)
	meta := selected.meta
	return &AntigravityKeySelection{
		Key:      selected.key,
		KeyIndex: selected.index,
		Meta:     &meta,
		OAuthKey: selected.oauth,
	}, nil
}

func ApplyAntigravitySelectionToContext(c *gin.Context, selection *AntigravityKeySelection) {
	if c == nil || selection == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyChannelKey, selection.Key)
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, selection.KeyIndex)
	if selection.Meta != nil {
		common.SetContextKey(c, constant.ContextKeyAntigravityEmail, selection.Meta.Email)
		common.SetContextKey(c, constant.ContextKeyAntigravityEffectiveProjectID, selection.Meta.EffectiveProjectID)
		common.SetContextKey(c, constant.ContextKeyAntigravityKeyState, string(selection.Meta.State))
	}
}

func MarkAntigravityKeySuccess(channelID int, keyIndex int, now time.Time) error {
	return updateAntigravityKeyMeta(channelID, keyIndex, func(ch *model.Channel, meta *model.ChannelKeyMeta) {
		meta.LastSuccessAt = now.Unix()
		meta.LastErrorAt = 0
		meta.LastErrorKind = ""
		meta.LastProjectError = ""
		meta.Consecutive429 = 0
		meta.Consecutive5xx = 0
		meta.ConsecutiveAuthFail = 0
		meta.SoftFailCount = 0
		meta.TotalSuccess++
		meta.CooldownUntil = 0
		if meta.State == "" || meta.State == model.CodexKeyStateNew || meta.State == model.CodexKeyStateSuspect || meta.State == model.CodexKeyStateCooldown {
			meta.State = model.CodexKeyStateHealthy
		}
		clearAntigravityDisabledReason(ch, keyIndex)
	})
}

func MarkAntigravityKeyFailure(channelID int, keyIndex int, relayMode int, modelName string, class AntigravityErrorClass, now time.Time, detail string) error {
	return updateAntigravityKeyMeta(channelID, keyIndex, func(ch *model.Channel, meta *model.ChannelKeyMeta) {
		meta.LastErrorAt = now.Unix()
		meta.LastErrorKind = string(class)
		meta.TotalFail++
		switch class {
		case AntigravityErrorClassAuth:
			meta.ConsecutiveAuthFail++
			meta.State = model.CodexKeyStateDead
			setAntigravityDisabledReason(ch, keyIndex, "auth_failed")
		case AntigravityErrorClassProjectDenied:
			meta.LastProjectCheckAt = now.Unix()
			meta.LastProjectError = strings.TrimSpace(detail)
			meta.State = model.CodexKeyStateDead
			setAntigravityDisabledReason(ch, keyIndex, "project_denied")
		case AntigravityErrorClassRateLimited:
			meta.Consecutive429++
			meta.State = model.CodexKeyStateCooldown
			meta.CooldownUntil = now.Add(5 * time.Minute).Unix()
		case AntigravityErrorClassServer:
			meta.Consecutive5xx++
			meta.State = model.CodexKeyStateSuspect
		case AntigravityErrorClassModelUnavailable:
			removeAntigravityModelCapability(meta, relayMode, modelName)
			meta.State = model.CodexKeyStateSuspect
		case AntigravityErrorClassProtocolIncompatible:
			removeAntigravityModelCapability(meta, relayMode, modelName)
			meta.SoftFailCount++
			meta.State = model.CodexKeyStateSuspect
		default:
			meta.SoftFailCount++
			if meta.State == "" {
				meta.State = model.CodexKeyStateSuspect
			}
		}
	})
}

func ClassifyAntigravityError(statusCode int, message string) AntigravityErrorClass {
	msg := strings.ToLower(strings.TrimSpace(message))
	switch {
	case statusCode == 401 || strings.Contains(msg, "invalid_grant") || strings.Contains(msg, "unauthorized"):
		return AntigravityErrorClassAuth
	case statusCode == 403 || strings.Contains(msg, "lacks the required iam permission") || strings.Contains(msg, "permission denied"):
		return AntigravityErrorClassProjectDenied
	case statusCode == 429 || strings.Contains(msg, "too many requests") || strings.Contains(msg, "rate limit"):
		return AntigravityErrorClassRateLimited
	case statusCode == 404 && strings.Contains(msg, "model") && strings.Contains(msg, "not found"):
		return AntigravityErrorClassModelUnavailable
	case statusCode == 404 && strings.Contains(msg, "requested entity was not found"):
		return AntigravityErrorClassProtocolIncompatible
	case statusCode >= 500:
		return AntigravityErrorClassServer
	default:
		return AntigravityErrorClassUnknown
	}
}

func RefreshAntigravityChannelKeyCredential(ctx context.Context, channelID int, keyIndex int, resetCaches bool) (*AntigravityOAuthKey, *model.Channel, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if channel == nil {
		return nil, nil, fmt.Errorf("channel not found")
	}
	if channel.Type != constant.ChannelTypeAntigravity {
		return nil, nil, fmt.Errorf("channel type is not Antigravity")
	}
	keys := channel.GetKeys()
	if keyIndex < 0 || keyIndex >= len(keys) {
		return nil, nil, fmt.Errorf("antigravity key index out of range")
	}
	oauthKey, err := RefreshAntigravityCredentialWithProxy(ctx, strings.TrimSpace(keys[keyIndex]), channel.GetSetting().Proxy)
	if err != nil {
		return nil, nil, err
	}
	encoded, err := common.Marshal(oauthKey)
	if err != nil {
		return nil, nil, err
	}
	keys[keyIndex] = string(encoded)
	channel.Key = strings.Join(keys, "\n")
	meta := applyAntigravityOAuthKeyToMeta(hydrateAntigravityKeyMeta(keys[keyIndex], channel.GetKeyMeta(keyIndex)), oauthKey)
	meta.LastRefreshAt = time.Now().Unix()
	channel.SetKeyMeta(keyIndex, meta)
	channel.ChannelInfo.IsMultiKey = len(keys) > 1
	channel.ChannelInfo.MultiKeySize = len(keys)
	if err := channel.Update(); err != nil {
		return nil, nil, err
	}
	if resetCaches {
		model.InitChannelCache()
		ResetProxyClientCache()
	}
	return oauthKey, channel, nil
}

func UpsertAntigravityOAuthKeyToChannel(channel *model.Channel, encoded string, oauthKey *AntigravityOAuthKey) error {
	if channel == nil {
		return fmt.Errorf("channel is nil")
	}
	keys := channel.GetKeys()
	replaced := false
	for idx, raw := range keys {
		existing, err := ParseAntigravityOAuthKey(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		if oauthKey != nil && oauthKey.Email != "" && strings.EqualFold(strings.TrimSpace(existing.Email), strings.TrimSpace(oauthKey.Email)) {
			keys[idx] = encoded
			replaced = true
			break
		}
	}
	if !replaced {
		if len(keys) == 1 && strings.TrimSpace(keys[0]) == "" {
			keys[0] = encoded
		} else {
			keys = append(keys, encoded)
		}
	}
	channel.Key = strings.Join(keys, "\n")
	channel.ChannelInfo.IsMultiKey = len(keys) > 1
	channel.ChannelInfo.MultiKeySize = len(keys)
	if err := channel.Update(); err != nil {
		return err
	}
	return nil
}

func antigravityKeyWeight(meta model.ChannelKeyMeta, now time.Time) int {
	weight := 100
	if meta.LastSuccessAt > 0 && now.Unix()-meta.LastSuccessAt <= 900 {
		weight += 20
	}
	if meta.LastSelectedAt > 0 && now.Unix()-meta.LastSelectedAt >= 300 {
		weight += 10
	}
	weight -= meta.Consecutive429 * 15
	weight -= meta.ConsecutiveAuthFail * 40
	weight -= meta.Consecutive5xx * 10
	weight -= meta.SoftFailCount * 5
	if strings.TrimSpace(meta.LastProjectError) != "" {
		weight -= 100
	}
	if weight < 0 {
		return 0
	}
	return weight
}

func hydrateAntigravityKeyMeta(key string, meta model.ChannelKeyMeta) model.ChannelKeyMeta {
	if meta.State == "" {
		meta.State = model.CodexKeyStateNew
	}
	if meta.Source == "" {
		meta.Source = "antigravity"
	}
	var oauthKey AntigravityOAuthKey
	if strings.HasPrefix(strings.TrimSpace(key), "{") && common.Unmarshal([]byte(key), &oauthKey) == nil {
		meta = applyAntigravityOAuthKeyToMeta(meta, &oauthKey)
	}
	return meta
}

func applyAntigravityOAuthKeyToMeta(meta model.ChannelKeyMeta, oauthKey *AntigravityOAuthKey) model.ChannelKeyMeta {
	if oauthKey == nil {
		return meta
	}
	if meta.Email == "" {
		meta.Email = strings.TrimSpace(oauthKey.Email)
	}
	if meta.ExpiresAt == "" {
		meta.ExpiresAt = strings.TrimSpace(oauthKey.Expired)
	}
	meta.ProjectID = strings.TrimSpace(oauthKey.ProjectID)
	meta.ManagedProjectID = strings.TrimSpace(oauthKey.ManagedProjectID)
	meta.EffectiveProjectID = strings.TrimSpace(oauthKey.EffectiveProjectID())
	if meta.Source == "" {
		meta.Source = "antigravity"
	}
	return meta
}

func antigravityKeySupportsModel(meta *model.ChannelKeyMeta, relayMode int, modelName string) bool {
	if meta == nil {
		return false
	}
	switch relayMode {
	case relayconstant.RelayModeResponses:
		if len(meta.ResponsesModels) == 0 {
			return true
		}
		return slices.Contains(meta.ResponsesModels, modelName)
	case relayconstant.RelayModeResponsesCompact:
		if len(meta.ResponsesCompactModels) == 0 {
			return true
		}
		return slices.Contains(meta.ResponsesCompactModels, modelName)
	default:
		if len(meta.ChatModels) == 0 {
			return true
		}
		return slices.Contains(meta.ChatModels, modelName)
	}
}

func removeAntigravityModelCapability(meta *model.ChannelKeyMeta, relayMode int, modelName string) {
	if meta == nil || strings.TrimSpace(modelName) == "" {
		return
	}
	switch relayMode {
	case relayconstant.RelayModeResponses:
		if len(meta.ResponsesModels) == 0 {
			return
		}
		meta.ResponsesModels = slices.DeleteFunc(meta.ResponsesModels, func(item string) bool { return item == modelName })
	case relayconstant.RelayModeResponsesCompact:
		if len(meta.ResponsesCompactModels) == 0 {
			return
		}
		meta.ResponsesCompactModels = slices.DeleteFunc(meta.ResponsesCompactModels, func(item string) bool { return item == modelName })
	default:
		if len(meta.ChatModels) == 0 {
			return
		}
		meta.ChatModels = slices.DeleteFunc(meta.ChatModels, func(item string) bool { return item == modelName })
	}
}

func updateAntigravityKeyMeta(channelID int, keyIndex int, updater func(*model.Channel, *model.ChannelKeyMeta)) error {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return err
	}
	if channel == nil {
		return fmt.Errorf("channel %d not found", channelID)
	}
	if channel.Type != constant.ChannelTypeAntigravity {
		return fmt.Errorf("channel %d is not antigravity", channelID)
	}
	lock := model.GetChannelPollingLock(channelID)
	lock.Lock()
	defer lock.Unlock()
	keys := channel.GetKeys()
	if keyIndex < 0 || keyIndex >= len(keys) {
		return fmt.Errorf("antigravity key index out of range")
	}
	meta := hydrateAntigravityKeyMeta(keys[keyIndex], channel.GetKeyMeta(keyIndex))
	updater(channel, &meta)
	channel.SetKeyMeta(keyIndex, meta)
	if err := channel.SaveChannelInfo(); err != nil {
		return err
	}
	if common.MemoryCacheEnabled {
		model.CacheUpdateChannel(channel)
	}
	return nil
}

func setAntigravityDisabledReason(channel *model.Channel, keyIndex int, reason string) {
	if channel.ChannelInfo.MultiKeyStatusList == nil {
		channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
	}
	if channel.ChannelInfo.MultiKeyDisabledReason == nil {
		channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)
	}
	if channel.ChannelInfo.MultiKeyDisabledTime == nil {
		channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
	}
	channel.ChannelInfo.MultiKeyStatusList[keyIndex] = 3
	channel.ChannelInfo.MultiKeyDisabledReason[keyIndex] = reason
	channel.ChannelInfo.MultiKeyDisabledTime[keyIndex] = time.Now().Unix()
}

func clearAntigravityDisabledReason(channel *model.Channel, keyIndex int) {
	if channel.ChannelInfo.MultiKeyStatusList != nil {
		delete(channel.ChannelInfo.MultiKeyStatusList, keyIndex)
	}
	if channel.ChannelInfo.MultiKeyDisabledReason != nil {
		delete(channel.ChannelInfo.MultiKeyDisabledReason, keyIndex)
	}
	if channel.ChannelInfo.MultiKeyDisabledTime != nil {
		delete(channel.ChannelInfo.MultiKeyDisabledTime, keyIndex)
	}
}
