package service

import (
	"fmt"
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

func SelectCodexKey(channel *model.Channel, excluded map[int]bool, now time.Time) (*CodexKeySelection, *types.NewAPIError) {
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	key, index, meta, err := channel.GetNextAvailableCodexKey(excluded, now)
	if err != nil {
		if err.GetErrorCode() == types.ErrorCodeChannelNoAvailableKey {
			RecordCodexNoAvailable(channel.Id, now)
		}
		return nil, err
	}
	return &CodexKeySelection{
		Key:      key,
		KeyIndex: index,
		Meta:     meta,
	}, nil
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
				health.Healthy++
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
		switch meta.Consecutive429 {
		case 1:
			meta.CooldownUntil = now.Add(2 * time.Minute).Unix()
		case 2:
			meta.CooldownUntil = now.Add(10 * time.Minute).Unix()
		case 3:
			meta.CooldownUntil = now.Add(30 * time.Minute).Unix()
		default:
			meta.CooldownUntil = now.Add(2 * time.Hour).Unix()
		}
		meta.State = model.CodexKeyStateCooldown
		if meta.Consecutive429 >= 4 {
			meta.State = model.CodexKeyStateSuspect
		}
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
