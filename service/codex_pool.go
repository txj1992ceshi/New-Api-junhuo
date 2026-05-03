package service

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type CodexKeySelection struct {
	Key      string
	KeyIndex int
	Meta     *model.ChannelKeyMeta
}

type codexKeyScrubStats struct {
	Inspected     int
	Normalized    int
	InvalidDead   int
	RateLimitDead int
}

func SelectCodexKey(channel *model.Channel, excluded map[int]bool, now time.Time) (*CodexKeySelection, *types.NewAPIError) {
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	workingExcluded := make(map[int]bool)
	for idx, skip := range excluded {
		workingExcluded[idx] = skip
	}

	maxAttempts := len(channel.GetKeys())
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		key, index, meta, err := channel.GetNextAvailableCodexKey(workingExcluded, now)
		if err != nil {
			if err.GetErrorCode() == types.ErrorCodeChannelNoAvailableKey {
				RecordCodexNoAvailable(channel.Id, now)
			}
			return nil, err
		}
		if invalidReason := validateCodexOAuthKeyPayload(key); invalidReason != "" {
			workingExcluded[index] = true
			_ = MarkCodexKeyInvalid(channel.Id, index, now, invalidReason)
			continue
		}
		normalizedKey, oauthKey, _, changed := normalizeCodexOAuthKeyRaw(key)
		if changed {
			key = normalizedKey
		}
		if meta != nil && oauthKey != nil {
			hydrated := hydrateCodexKeyMeta(key, *meta)
			hydrated = applyCodexOAuthKeyToMeta(hydrated, oauthKey)
			meta = &hydrated
		}
		return &CodexKeySelection{
			Key:      key,
			KeyIndex: index,
			Meta:     meta,
		}, nil
	}
	RecordCodexNoAvailable(channel.Id, now)
	return nil, types.NewError(fmt.Errorf("no available codex keys"), types.ErrorCodeChannelNoAvailableKey)
}

type CodexPoolHealth struct {
	Total          int
	Healthy        int
	New            int
	Cooldown       int
	Suspect        int
	Dead           int
	Refreshing     int
	HealthyRatio   float64
	CooldownRatio  float64
	RecentDead30m  int
	AvailableCount int
}

func ComputeCodexPoolHealth(channel *model.Channel, now time.Time) *CodexPoolHealth {
	health := &CodexPoolHealth{}
	if channel == nil || channel.Type != constant.ChannelTypeCodex {
		return health
	}
	keys := channel.GetKeys()
	health.Total = len(keys)
	for i := range keys {
		meta := normalizeServiceCodexKeyMeta(channel.GetKeyMeta(i))
		switch meta.State {
		case model.CodexKeyStateHealthy:
			health.Healthy++
			health.AvailableCount++
		case model.CodexKeyStateNew:
			health.New++
			health.AvailableCount++
		case model.CodexKeyStateCooldown:
			if meta.CooldownUntil > now.Unix() {
				health.Cooldown++
			} else {
				health.Suspect++
				health.AvailableCount++
			}
		case model.CodexKeyStateSuspect:
			health.Suspect++
			health.AvailableCount++
		case model.CodexKeyStateRefreshing:
			health.Refreshing++
		case model.CodexKeyStateDead:
			health.Dead++
			if meta.LastErrorAt > 0 && now.Sub(time.Unix(meta.LastErrorAt, 0)) <= 30*time.Minute {
				health.RecentDead30m++
			}
		default:
			health.New++
			health.AvailableCount++
		}
	}
	if health.Total > 0 {
		health.HealthyRatio = float64(health.Healthy+health.New) / float64(health.Total)
		health.CooldownRatio = float64(health.Cooldown) / float64(health.Total)
	}
	return health
}

func ApplyCodexSelectionToContext(c *gin.Context, channel *model.Channel, selection *CodexKeySelection) {
	if c == nil || channel == nil || selection == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyChannelKey, selection.Key)
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, selection.KeyIndex)
	if selection.Meta != nil {
		common.SetContextKey(c, constant.ContextKeyCodexKeyState, string(selection.Meta.State))
		common.SetContextKey(c, constant.ContextKeyCodexAccountID, selection.Meta.AccountID)
		common.SetContextKey(c, constant.ContextKeyCodexEmail, selection.Meta.Email)
	}
}

func loadCodexChannelForUpdate(channelID int) (*model.Channel, *syncGuard, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	lock := model.GetChannelPollingLock(channelID)
	lock.Lock()
	return channel, &syncGuard{unlock: lock.Unlock}, nil
}

type syncGuard struct {
	unlock func()
}

func (g *syncGuard) Done() {
	if g != nil && g.unlock != nil {
		g.unlock()
	}
}

func hydrateCodexKeyMeta(key string, meta model.ChannelKeyMeta) model.ChannelKeyMeta {
	if meta.State == "" {
		meta.State = model.CodexKeyStateNew
	}
	if meta.Source == "" {
		meta.Source = "cursorpro"
	}
	type codexOAuthKeyLite struct {
		AccountID string `json:"account_id,omitempty"`
		Email     string `json:"email,omitempty"`
		Expired   string `json:"expired,omitempty"`
	}
	var oauthKey codexOAuthKeyLite
	if strings.HasPrefix(strings.TrimSpace(key), "{") && common.Unmarshal([]byte(key), &oauthKey) == nil {
		if meta.AccountID == "" {
			meta.AccountID = oauthKey.AccountID
		}
		if meta.Email == "" {
			meta.Email = oauthKey.Email
		}
		if meta.ExpiresAt == "" {
			meta.ExpiresAt = oauthKey.Expired
		}
	}
	return meta
}

func applyCodexOAuthKeyToMeta(meta model.ChannelKeyMeta, oauthKey *CodexOAuthKey) model.ChannelKeyMeta {
	if oauthKey == nil {
		return meta
	}
	if meta.AccountID == "" {
		meta.AccountID = strings.TrimSpace(oauthKey.AccountID)
	}
	if meta.Email == "" {
		meta.Email = strings.TrimSpace(oauthKey.Email)
	}
	if meta.ExpiresAt == "" {
		meta.ExpiresAt = strings.TrimSpace(oauthKey.Expired)
	}
	if meta.Source == "" {
		meta.Source = "cursorpro"
	}
	return meta
}

func normalizeServiceCodexKeyMeta(meta model.ChannelKeyMeta) model.ChannelKeyMeta {
	if meta.State == "" {
		meta.State = model.CodexKeyStateNew
	}
	return meta
}

func updateCodexKeyMeta(channelID int, keyIndex int, updater func(*model.Channel, *model.ChannelKeyMeta, time.Time)) error {
	channel, guard, err := loadCodexChannelForUpdate(channelID)
	if err != nil {
		return err
	}
	defer guard.Done()
	if channel == nil {
		return fmt.Errorf("channel %d not found", channelID)
	}
	if channel.Type != constant.ChannelTypeCodex {
		return fmt.Errorf("channel %d is not codex", channelID)
	}
	keys := channel.GetKeys()
	if keyIndex < 0 || keyIndex >= len(keys) {
		return fmt.Errorf("codex key index out of range")
	}
	meta := hydrateCodexKeyMeta(keys[keyIndex], channel.GetKeyMeta(keyIndex))
	updater(channel, &meta, time.Now())
	channel.SetKeyMeta(keyIndex, meta)
	if err := channel.SaveChannelInfo(); err != nil {
		return err
	}
	if common.MemoryCacheEnabled {
		model.CacheUpdateChannel(channel)
	}
	return nil
}

func MarkCodexKeySuccess(channelID int, keyIndex int, now time.Time) error {
	return updateCodexKeyMeta(channelID, keyIndex, func(channel *model.Channel, meta *model.ChannelKeyMeta, _ time.Time) {
		meta.LastSuccessAt = now.Unix()
		meta.LastErrorKind = ""
		meta.Consecutive429 = 0
		meta.Consecutive5xx = 0
		meta.ConsecutiveAuthFail = 0
		meta.SoftFailCount = 0
		meta.TotalSuccess++
		meta.CooldownUntil = 0
		if meta.State == model.CodexKeyStateNew {
			meta.NewSuccessCount++
			if meta.NewSuccessCount >= 3 {
				meta.State = model.CodexKeyStateHealthy
			}
		} else {
			meta.State = model.CodexKeyStateHealthy
		}
	})
}

func MarkCodexKeyRateLimited(channelID int, keyIndex int, now time.Time) error {
	return updateCodexKeyMeta(channelID, keyIndex, func(channel *model.Channel, meta *model.ChannelKeyMeta, _ time.Time) {
		meta.LastErrorAt = now.Unix()
		meta.LastErrorKind = string(CodexErrorKindRateLimit)
		meta.TotalFail++
		meta.Consecutive429++
		meta.Consecutive5xx = 0
		meta.ConsecutiveAuthFail = 0
		meta.CooldownUntil = 0
		meta.State = model.CodexKeyStateDead
	})
}

func MarkCodexKeyServerError(channelID int, keyIndex int, now time.Time) error {
	return updateCodexKeyMeta(channelID, keyIndex, func(channel *model.Channel, meta *model.ChannelKeyMeta, _ time.Time) {
		meta.LastErrorAt = now.Unix()
		meta.LastErrorKind = string(CodexErrorKindServer)
		meta.TotalFail++
		meta.Consecutive5xx++
		switch meta.Consecutive5xx {
		case 1:
			// keep current state for immediate failover
		case 2:
			meta.State = model.CodexKeyStateSuspect
		case 3, 4:
			meta.State = model.CodexKeyStateCooldown
			meta.CooldownUntil = now.Add(15 * time.Minute).Unix()
		default:
			meta.State = model.CodexKeyStateDead
		}
	})
}

func MarkCodexKeyAuthFail(channelID int, keyIndex int, now time.Time) error {
	return updateCodexKeyMeta(channelID, keyIndex, func(channel *model.Channel, meta *model.ChannelKeyMeta, _ time.Time) {
		meta.LastErrorAt = now.Unix()
		meta.LastErrorKind = string(CodexErrorKindAuth)
		meta.TotalFail++
		meta.ConsecutiveAuthFail++
		if meta.ConsecutiveAuthFail >= 2 {
			meta.State = model.CodexKeyStateDead
			return
		}
		meta.State = model.CodexKeyStateRefreshing
	})
}

func MarkCodexKeySoftFail(channelID int, keyIndex int, now time.Time) error {
	return updateCodexKeyMeta(channelID, keyIndex, func(channel *model.Channel, meta *model.ChannelKeyMeta, _ time.Time) {
		meta.LastErrorAt = now.Unix()
		meta.LastErrorKind = string(CodexErrorKindSoftFail)
		meta.TotalFail++
		meta.SoftFailCount++
		if meta.SoftFailCount >= 5 {
			meta.State = model.CodexKeyStateSuspect
		}
	})
}

func MarkCodexKeyInvalid(channelID int, keyIndex int, now time.Time, reason string) error {
	recordCodexInvalidKey(channelID, now)
	return updateCodexKeyMeta(channelID, keyIndex, func(channel *model.Channel, meta *model.ChannelKeyMeta, _ time.Time) {
		meta.State = model.CodexKeyStateDead
		meta.LastErrorAt = now.Unix()
		meta.LastErrorKind = string(CodexErrorKindInvalid)
		if strings.TrimSpace(reason) != "" {
			meta.LastErrorKind = strings.TrimSpace(reason)
		}
		meta.TotalFail++
		meta.CooldownUntil = 0
	})
}

func normalizeCodexOAuthKeyRaw(raw string) (string, *CodexOAuthKey, string, bool) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		return "", nil, "invalid_key", false
	}
	key, err := parseCodexOAuthKey(trimmed)
	if err != nil {
		return "", nil, "invalid_key_json", false
	}
	if strings.TrimSpace(key.AccessToken) == "" {
		return "", key, "invalid_key_missing_access_token", false
	}
	if strings.TrimSpace(key.AccountID) == "" {
		return "", key, "invalid_key_missing_account_id", false
	}
	encoded, err := common.Marshal(key)
	if err != nil {
		return trimmed, key, "", false
	}
	normalized := string(encoded)
	return normalized, key, "", normalized != trimmed
}

func shouldAggressivelyEvictRateLimitedKey(meta model.ChannelKeyMeta, now time.Time) bool {
	meta = normalizeServiceCodexKeyMeta(meta)
	if meta.State != model.CodexKeyStateCooldown && meta.State != model.CodexKeyStateSuspect {
		return false
	}
	if meta.Consecutive429 < 2 {
		return false
	}
	if meta.LastSuccessAt <= 0 {
		return true
	}
	return now.Sub(time.Unix(meta.LastSuccessAt, 0)) >= 30*time.Minute
}

func ScrubCodexChannelKeys(channelID int, now time.Time) (*codexKeyScrubStats, error) {
	channel, guard, err := loadCodexChannelForUpdate(channelID)
	if err != nil {
		return nil, err
	}
	defer guard.Done()
	if channel == nil {
		return nil, fmt.Errorf("channel %d not found", channelID)
	}
	if channel.Type != constant.ChannelTypeCodex {
		return nil, fmt.Errorf("channel %d is not codex", channelID)
	}

	keys := channel.GetKeys()
	stats := &codexKeyScrubStats{}
	updated := false
	for i, rawKey := range keys {
		stats.Inspected++
		meta := hydrateCodexKeyMeta(rawKey, channel.GetKeyMeta(i))
		normalizedKey, oauthKey, invalidReason, changed := normalizeCodexOAuthKeyRaw(rawKey)
		if invalidReason != "" {
			if meta.State != model.CodexKeyStateDead || meta.LastErrorKind != invalidReason || meta.CooldownUntil != 0 {
				meta.State = model.CodexKeyStateDead
				meta.LastErrorAt = now.Unix()
				meta.LastErrorKind = invalidReason
				meta.TotalFail++
				meta.CooldownUntil = 0
				channel.SetKeyMeta(i, meta)
				updated = true
			}
			recordCodexInvalidKey(channelID, now)
			stats.InvalidDead++
			continue
		}
		if changed {
			keys[i] = normalizedKey
			rawKey = normalizedKey
			stats.Normalized++
			updated = true
		}
		nextMeta := applyCodexOAuthKeyToMeta(hydrateCodexKeyMeta(rawKey, meta), oauthKey)
		if shouldAggressivelyEvictRateLimitedKey(nextMeta, now) {
			if nextMeta.State != model.CodexKeyStateDead || nextMeta.LastErrorKind != "rate_limit_exhausted" || nextMeta.CooldownUntil != 0 {
				nextMeta.State = model.CodexKeyStateDead
				nextMeta.LastErrorAt = now.Unix()
				nextMeta.LastErrorKind = "rate_limit_exhausted"
				nextMeta.CooldownUntil = 0
				nextMeta.TotalFail++
				updated = true
			}
			stats.RateLimitDead++
		}
		if !reflect.DeepEqual(nextMeta, meta) {
			channel.SetKeyMeta(i, nextMeta)
			updated = true
		}
	}
	if !updated {
		return stats, nil
	}
	channel.Key = strings.Join(keys, "\n")
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"key":          channel.Key,
		"channel_info": channel.ChannelInfo,
	}).Error; err != nil {
		return nil, err
	}
	if common.MemoryCacheEnabled {
		model.CacheUpdateChannel(channel)
	}
	return stats, nil
}

func validateCodexOAuthKeyPayload(raw string) string {
	_, _, invalidReason, _ := normalizeCodexOAuthKeyRaw(raw)
	return invalidReason
}
