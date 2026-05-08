package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

const (
	externalPoolCooldownUntilKey      = "external_pool_cooldown_until"
	externalPoolCooldownReasonKey     = "external_pool_cooldown_reason"
	externalPoolCooldownKindKey       = "external_pool_cooldown_kind"
	externalPoolCooldownErrorKey      = "external_pool_cooldown_error"
	externalPoolLastSuccessAtKey      = "external_pool_last_success_at"
	defaultExternalPoolCooldownSecs   = int64(180)
	defaultExternalPoolRateLimitSecs  = int64(120)
	defaultExternalPoolModelErrorSecs = int64(600)
	defaultExternalPoolAuthErrorSecs  = int64(900)
)

func resolveExternalPoolKind(channel *model.Channel) string {
	if channel == nil {
		return ""
	}
	if _, ok := resolveExternalPoolProxy(channel, ExternalPoolKindCursor); ok {
		return ExternalPoolKindCursor
	}
	if _, ok := resolveExternalPoolProxy(channel, ExternalPoolKindCodex); ok {
		return ExternalPoolKindCodex
	}
	if _, ok := resolveExternalPoolProxy(channel, ExternalPoolKindWindsurf); ok {
		return ExternalPoolKindWindsurf
	}
	if _, ok := resolveExternalPoolProxy(channel, ExternalPoolKindKiro); ok {
		return ExternalPoolKindKiro
	}
	return ""
}

func IsExternalPoolChannelCoolingDown(channel *model.Channel, now time.Time) bool {
	if channel == nil {
		return false
	}
	if resolveExternalPoolKind(channel) == "" {
		return false
	}
	info := channel.GetOtherInfo()
	until := parseExternalPoolOtherInfoInt64(info, externalPoolCooldownUntilKey)
	return until > now.Unix()
}

func MarkExternalPoolChannelSuccess(channelID int) {
	_ = updateExternalPoolRuntimeState(channelID, func(info map[string]interface{}) bool {
		changed := false
		if _, ok := info[externalPoolCooldownUntilKey]; ok {
			delete(info, externalPoolCooldownUntilKey)
			changed = true
		}
		if _, ok := info[externalPoolCooldownReasonKey]; ok {
			delete(info, externalPoolCooldownReasonKey)
			changed = true
		}
		if _, ok := info[externalPoolCooldownKindKey]; ok {
			delete(info, externalPoolCooldownKindKey)
			changed = true
		}
		if _, ok := info[externalPoolCooldownErrorKey]; ok {
			delete(info, externalPoolCooldownErrorKey)
			changed = true
		}
		info[externalPoolLastSuccessAtKey] = nowUnix()
		return changed
	})
}

func ApplyExternalPoolSoftCooldown(channel *model.Channel, err *types.NewAPIError) bool {
	kind, cooldownSeconds, reason, summary := classifyExternalPoolCooldown(channel, err)
	if kind == "" || cooldownSeconds <= 0 {
		return false
	}
	until := nowUnix() + cooldownSeconds
	updateErr := updateExternalPoolRuntimeState(channel.Id, func(info map[string]interface{}) bool {
		info[externalPoolCooldownUntilKey] = until
		info[externalPoolCooldownReasonKey] = reason
		info[externalPoolCooldownKindKey] = kind
		info[externalPoolCooldownErrorKey] = summary
		return true
	})
	if updateErr != nil {
		common.SysLog(fmt.Sprintf("failed to apply external pool cooldown: channel_id=%d err=%v", channel.Id, updateErr))
		return false
	}
	common.SysLog(fmt.Sprintf(
		"applied external pool cooldown: channel_id=%d kind=%s seconds=%d reason=%s",
		channel.Id, kind, cooldownSeconds, reason,
	))
	return true
}

func classifyExternalPoolCooldown(channel *model.Channel, err *types.NewAPIError) (string, int64, string, string) {
	kind := resolveExternalPoolKind(channel)
	if kind == "" || err == nil {
		return "", 0, "", ""
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	statusCode := err.StatusCode

	switch {
	case strings.Contains(message, "too many requests"),
		strings.Contains(message, "rate limit"),
		strings.Contains(message, "rate_limited"),
		statusCode == 429:
		return kind, externalPoolCooldownSeconds(channel, kind, "rate_limit", defaultExternalPoolRateLimitSecs), "rate_limited", err.ErrorWithStatusCode()
	case strings.Contains(message, "model_not_entitled"),
		strings.Contains(message, "not entitled"),
		strings.Contains(message, "model_deprecated"),
		strings.Contains(message, "unsupported model"),
		strings.Contains(message, "model_not_found"),
		strings.Contains(message, "不可用（未订阅或已被封禁）"),
		strings.Contains(message, "已被 windsurf 上游废弃"):
		return kind, externalPoolCooldownSeconds(channel, kind, "model_error", defaultExternalPoolModelErrorSecs), "model_unavailable", err.ErrorWithStatusCode()
	case statusCode == 401, statusCode == 403,
		strings.Contains(message, "unauthorized"),
		strings.Contains(message, "forbidden"),
		strings.Contains(message, "auth"):
		return kind, externalPoolCooldownSeconds(channel, kind, "auth_error", defaultExternalPoolAuthErrorSecs), "auth_rejected", err.ErrorWithStatusCode()
	case statusCode >= 500,
		strings.Contains(message, "upstream_error"),
		strings.Contains(message, "timeout"),
		strings.Contains(message, "connection refused"),
		strings.Contains(message, "eof"):
		return kind, externalPoolCooldownSeconds(channel, kind, "server_error", defaultExternalPoolCooldownSecs), "upstream_unavailable", err.ErrorWithStatusCode()
	default:
		return "", 0, "", ""
	}
}

func externalPoolCooldownSeconds(channel *model.Channel, kind string, category string, fallback int64) int64 {
	info := channel.GetOtherInfo()
	for _, key := range []string{
		fmt.Sprintf("%s_pool_%s_cooldown_seconds", kind, category),
		fmt.Sprintf("%s_pool_cooldown_seconds", kind),
		fmt.Sprintf("external_pool_%s_cooldown_seconds", category),
		"external_pool_cooldown_seconds",
	} {
		if value := parseExternalPoolOtherInfoInt64(info, key); value > 0 {
			return value
		}
	}
	return fallback
}

func updateExternalPoolRuntimeState(channelID int, mutate func(map[string]interface{}) bool) error {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return err
	}
	info := channel.GetOtherInfo()
	if info == nil {
		info = make(map[string]interface{})
	}
	if !mutate(info) {
		return nil
	}
	payload, err := common.Marshal(info)
	if err != nil {
		return err
	}
	if err = model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("other_info", string(payload)).Error; err != nil {
		return err
	}
	if common.MemoryCacheEnabled {
		if cached, cacheErr := model.CacheGetChannel(channelID); cacheErr == nil && cached != nil {
			cloned := *cached
			cloned.OtherInfo = string(payload)
			model.CacheUpdateChannel(&cloned)
		}
	}
	return nil
}

func nowUnix() int64 {
	return time.Now().Unix()
}
