package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	antigravityCredentialRefreshTickInterval = 30 * time.Minute
	antigravityCredentialRefreshThreshold    = 2 * time.Hour
)

var (
	antigravityCredentialRefreshOnce    sync.Once
	antigravityCredentialRefreshRunning atomic.Bool
)

func StartAntigravityCredentialAutoRefreshTask() {
	antigravityCredentialRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		var count int64
		if err := model.DB.Model(&model.Channel{}).Where("type = ?", constant.ChannelTypeAntigravity).Count(&count).Error; err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("antigravity credential auto-refresh: preflight query failed: %v", err))
			return
		}
		if count == 0 {
			return
		}
		gopool.Go(func() {
			ticker := time.NewTicker(antigravityCredentialRefreshTickInterval)
			defer ticker.Stop()
			runAntigravityCredentialAutoRefreshOnce()
			for range ticker.C {
				runAntigravityCredentialAutoRefreshOnce()
			}
		})
	})
}

func runAntigravityCredentialAutoRefreshOnce() {
	if !antigravityCredentialRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer antigravityCredentialRefreshRunning.Store(false)

	ctx := context.Background()
	var channels []*model.Channel
	if err := model.DB.Select("id", "name", "key", "status").Where("type = ? AND status = 1", constant.ChannelTypeAntigravity).Find(&channels).Error; err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("antigravity credential auto-refresh: query channels failed: %v", err))
		return
	}
	if len(channels) == 0 {
		return
	}

	now := time.Now()
	refreshed := 0
	for _, ch := range channels {
		if ch == nil || strings.TrimSpace(ch.Key) == "" {
			continue
		}
		keys := ch.GetKeys()
		for keyIndex, rawKey := range keys {
			key, err := parseAntigravityOAuthKey(strings.TrimSpace(rawKey))
			if err != nil || strings.TrimSpace(key.RefreshToken) == "" {
				continue
			}
			exp := key.ExpiresAt()
			if !exp.IsZero() && exp.Sub(now) > antigravityCredentialRefreshThreshold {
				continue
			}
			refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			var newKey *AntigravityOAuthKey
			if ch.ChannelInfo.IsMultiKey {
				newKey, _, err = RefreshAntigravityChannelKeyCredential(refreshCtx, ch.Id, keyIndex, false)
			} else {
				newKey, _, err = RefreshAntigravityChannelCredential(refreshCtx, ch.Id, AntigravityCredentialRefreshOptions{ResetCaches: false})
			}
			cancel()
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("antigravity credential auto-refresh: channel_id=%d key_index=%d name=%s refresh failed: %v", ch.Id, keyIndex, ch.Name, err))
				continue
			}
			refreshed++
			logger.LogInfo(ctx, fmt.Sprintf("antigravity credential auto-refresh: channel_id=%d key_index=%d name=%s refreshed, expires_at=%s", ch.Id, keyIndex, ch.Name, newKey.Expired))
		}
	}
	if refreshed > 0 {
		model.InitChannelCache()
		ResetProxyClientCache()
	}
}
