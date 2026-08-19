package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	ErrJunhuoLinkMappingNotFound = errors.New("junhuo_link_mapping_not_found")
	ErrJunhuoLinkMappingConflict = errors.New("junhuo_link_mapping_conflict")
	ErrJunhuoLinkBindingNotFound = errors.New("junhuo_link_binding_not_found")
	ErrJunhuoLinkBindingConflict = errors.New("junhuo_link_binding_conflict")
	ErrJunhuoLinkUsageConflict   = errors.New("junhuo_link_usage_id_conflict")
	ErrJunhuoLinkUsageNotFound   = errors.New("junhuo_link_authoritative_usage_not_found")
	ErrJunhuoLinkUsageAmbiguous  = errors.New("junhuo_link_authoritative_usage_ambiguous")
	ErrJunhuoLinkUsageMismatch   = errors.New("junhuo_link_authoritative_usage_mismatch")
)

const junhuoLinkUsageMatchWindowSeconds int64 = 30 * 60

type JunhuoLinkUserMapping struct {
	LinkUserId   string `json:"link_user_id" gorm:"primaryKey;size:128"`
	NewAPIUserId int    `json:"newapi_user_id" gorm:"not null;uniqueIndex"`
	Status       string `json:"status" gorm:"size:16;not null;default:'active';index"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;not null"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint;not null"`
	RevokedAt    int64  `json:"revoked_at" gorm:"bigint;not null;default:0;index"`
}

type JunhuoLinkDeviceKey struct {
	Id             int    `json:"id"`
	LinkUserId     string `json:"link_user_id" gorm:"size:128;not null;uniqueIndex:idx_junhuo_link_device,priority:1;index"`
	DeviceId       string `json:"device_id" gorm:"size:128;not null;uniqueIndex:idx_junhuo_link_device,priority:2;index"`
	NewAPIUserId   int    `json:"newapi_user_id" gorm:"not null;index"`
	TokenId        int    `json:"token_id" gorm:"not null;uniqueIndex"`
	KeyFingerprint string `json:"key_fingerprint" gorm:"size:128;not null"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;not null"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;not null"`
	RevokedAt      int64  `json:"revoked_at" gorm:"bigint;not null;default:0;index"`
}

type JunhuoLinkUsageReceipt struct {
	UsageId      string `json:"usage_id" gorm:"primaryKey;size:256"`
	BindingId    int    `json:"binding_id" gorm:"not null;index"`
	LinkUserId   string `json:"link_user_id" gorm:"size:128;not null;index"`
	DeviceId     string `json:"device_id" gorm:"size:128;not null;index"`
	NewAPIUserId int    `json:"newapi_user_id" gorm:"not null;index"`
	TokenId      int    `json:"token_id" gorm:"not null;index"`
	TaskId       string `json:"task_id" gorm:"size:256;not null;default:'';index"`
	LogId        int    `json:"log_id" gorm:"not null;uniqueIndex"`
	RequestId    string `json:"request_id" gorm:"size:128;not null;default:'';index"`
	InputTokens  int    `json:"input_tokens" gorm:"not null;default:0"`
	OutputTokens int    `json:"output_tokens" gorm:"not null;default:0"`
	TotalTokens  int    `json:"total_tokens" gorm:"not null;default:0"`
	UsedQuota    int    `json:"used_quota" gorm:"not null;default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;not null"`
}

type JunhuoLinkIssuedDeviceKey struct {
	Binding JunhuoLinkDeviceKey
	Token   Token
	Created bool
	RawKey  string
}

type JunhuoLinkUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
	UsedQuota    int `json:"usedQuota"`
}

type JunhuoLinkUserUsage struct {
	JunhuoLinkUsage
	Balance int `json:"balance"`
}

func UpsertJunhuoLinkUserMapping(linkUserId string, newAPIUserId int, now int64) (*JunhuoLinkUserMapping, error) {
	linkUserId = strings.TrimSpace(linkUserId)
	if linkUserId == "" || len(linkUserId) > 128 || newAPIUserId <= 0 {
		return nil, ErrJunhuoLinkMappingConflict
	}
	user, err := GetUserById(newAPIUserId, false)
	if err != nil || user.Status != common.UserStatusEnabled {
		return nil, ErrJunhuoLinkMappingNotFound
	}
	var result JunhuoLinkUserMapping
	err = DB.Transaction(func(tx *gorm.DB) error {
		var byLink JunhuoLinkUserMapping
		linkErr := tx.Where("link_user_id = ?", linkUserId).First(&byLink).Error
		if linkErr == nil {
			if byLink.NewAPIUserId != newAPIUserId {
				return ErrJunhuoLinkMappingConflict
			}
			byLink.Status = "active"
			byLink.RevokedAt = 0
			byLink.UpdatedAt = now
			if err := tx.Save(&byLink).Error; err != nil {
				return err
			}
			result = byLink
			return nil
		}
		if !errors.Is(linkErr, gorm.ErrRecordNotFound) {
			return linkErr
		}
		var byNewAPI JunhuoLinkUserMapping
		otherErr := tx.Where("new_api_user_id = ?", newAPIUserId).First(&byNewAPI).Error
		if otherErr == nil && byNewAPI.LinkUserId != linkUserId {
			return ErrJunhuoLinkMappingConflict
		}
		if otherErr != nil && !errors.Is(otherErr, gorm.ErrRecordNotFound) {
			return otherErr
		}
		result = JunhuoLinkUserMapping{
			LinkUserId: linkUserId, NewAPIUserId: newAPIUserId, Status: "active", CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&result).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func GetActiveJunhuoLinkUserMapping(linkUserId string) (*JunhuoLinkUserMapping, error) {
	var mapping JunhuoLinkUserMapping
	err := DB.Where("link_user_id = ? AND status = ? AND revoked_at = 0", strings.TrimSpace(linkUserId), "active").First(&mapping).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrJunhuoLinkMappingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

func RevokeJunhuoLinkUserMapping(linkUserId string, now int64) error {
	linkUserId = strings.TrimSpace(linkUserId)
	var mapping JunhuoLinkUserMapping
	err := DB.Where("link_user_id = ?", linkUserId).First(&mapping).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrJunhuoLinkMappingNotFound
	}
	if err != nil {
		return err
	}
	if mapping.Status != "active" || mapping.RevokedAt != 0 {
		return nil
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var bindings []JunhuoLinkDeviceKey
		if err := tx.Where("link_user_id = ? AND revoked_at = 0", mapping.LinkUserId).Find(&bindings).Error; err != nil {
			return err
		}
		for _, binding := range bindings {
			if err := tx.Model(&Token{}).Where("id = ? AND user_id = ?", binding.TokenId, binding.NewAPIUserId).Update("status", common.TokenStatusDisabled).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&JunhuoLinkDeviceKey{}).Where("link_user_id = ? AND revoked_at = 0", mapping.LinkUserId).Updates(map[string]interface{}{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&JunhuoLinkUserMapping{}).Where("link_user_id = ?", mapping.LinkUserId).Updates(map[string]interface{}{"status": "revoked", "revoked_at": now, "updated_at": now}).Error
	})
	if err == nil {
		_ = InvalidateUserTokensCache(mapping.NewAPIUserId)
	}
	return err
}

func IssueJunhuoLinkDeviceKey(linkUserId string, deviceId string, rawStoredKey string, keyFingerprint string, now int64) (*JunhuoLinkIssuedDeviceKey, error) {
	linkUserId = strings.TrimSpace(linkUserId)
	deviceId = strings.TrimSpace(deviceId)
	keyFingerprint = strings.TrimSpace(keyFingerprint)
	if linkUserId == "" || deviceId == "" || len(deviceId) > 128 || rawStoredKey == "" || keyFingerprint == "" {
		return nil, ErrJunhuoLinkBindingConflict
	}
	mapping, err := GetActiveJunhuoLinkUserMapping(linkUserId)
	if err != nil {
		return nil, err
	}
	result := &JunhuoLinkIssuedDeviceKey{}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var existing JunhuoLinkDeviceKey
		existingErr := tx.Where("link_user_id = ? AND device_id = ?", linkUserId, deviceId).First(&existing).Error
		if existingErr == nil && existing.RevokedAt == 0 {
			var token Token
			if err := tx.Unscoped().Where("id = ? AND user_id = ?", existing.TokenId, mapping.NewAPIUserId).First(&token).Error; err != nil {
				return ErrJunhuoLinkBindingConflict
			}
			if token.Status != common.TokenStatusEnabled || token.DeletedAt.Valid {
				return ErrJunhuoLinkBindingConflict
			}
			result.Binding = existing
			result.Token = token
			result.Created = false
			return nil
		}
		if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}
		if existingErr == nil && existing.NewAPIUserId != mapping.NewAPIUserId {
			return ErrJunhuoLinkBindingConflict
		}

		token := Token{
			UserId: mapping.NewAPIUserId, Key: rawStoredKey, Status: common.TokenStatusEnabled,
			Name:        fmt.Sprintf("Junhuo Link %s", truncateJunhuoLinkLabel(deviceId, 32)),
			CreatedTime: now, AccessedTime: now, ExpiredTime: -1,
			RemainQuota: 0, UnlimitedQuota: true,
		}
		if err := tx.Create(&token).Error; err != nil {
			return err
		}
		if existingErr == nil {
			existing.NewAPIUserId = mapping.NewAPIUserId
			existing.TokenId = token.Id
			existing.KeyFingerprint = keyFingerprint
			existing.RevokedAt = 0
			existing.UpdatedAt = now
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			result.Binding = existing
		} else {
			binding := JunhuoLinkDeviceKey{
				LinkUserId: linkUserId, DeviceId: deviceId, NewAPIUserId: mapping.NewAPIUserId,
				TokenId: token.Id, KeyFingerprint: keyFingerprint, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&binding).Error; err != nil {
				return err
			}
			result.Binding = binding
		}
		result.Token = token
		result.Created = true
		result.RawKey = rawStoredKey
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetJunhuoLinkDeviceKey(linkUserId string, deviceId string, tokenId int) (*JunhuoLinkDeviceKey, error) {
	var binding JunhuoLinkDeviceKey
	err := DB.Where("token_id = ? AND link_user_id = ? AND device_id = ?", tokenId, strings.TrimSpace(linkUserId), strings.TrimSpace(deviceId)).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrJunhuoLinkBindingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func RevokeJunhuoLinkDeviceKey(linkUserId string, deviceId string, tokenId int, now int64) error {
	binding, err := GetJunhuoLinkDeviceKey(linkUserId, deviceId, tokenId)
	if err != nil {
		return err
	}
	if binding.RevokedAt != 0 {
		return nil
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Token{}).Where("id = ? AND user_id = ?", binding.TokenId, binding.NewAPIUserId).Update("status", common.TokenStatusDisabled).Error; err != nil {
			return err
		}
		return tx.Model(&JunhuoLinkDeviceKey{}).Where("id = ? AND revoked_at = 0", binding.Id).Updates(map[string]interface{}{"revoked_at": now, "updated_at": now}).Error
	})
	if err == nil {
		_ = InvalidateUserTokensCache(binding.NewAPIUserId)
	}
	return err
}

func GetJunhuoLinkDeviceKeyStatus(linkUserId string, deviceId string, tokenId int) (*JunhuoLinkDeviceKey, bool, error) {
	binding, err := GetJunhuoLinkDeviceKey(linkUserId, deviceId, tokenId)
	if err != nil {
		return nil, false, err
	}
	var token Token
	err = DB.Unscoped().Where("id = ? AND user_id = ?", binding.TokenId, binding.NewAPIUserId).First(&token).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	active := err == nil && binding.RevokedAt == 0 && !token.DeletedAt.Valid && token.Status == common.TokenStatusEnabled
	return binding, active, nil
}

func GetJunhuoLinkDeviceUsage(linkUserId string, deviceId string, tokenId int) (*JunhuoLinkUsage, error) {
	binding, err := GetJunhuoLinkDeviceKey(linkUserId, deviceId, tokenId)
	if err != nil {
		return nil, err
	}
	var totals struct {
		InputTokens  int
		OutputTokens int
		UsedQuota    int
	}
	err = LOG_DB.Model(&Log{}).
		Select("COALESCE(SUM(prompt_tokens), 0) AS input_tokens, COALESCE(SUM(completion_tokens), 0) AS output_tokens, COALESCE(SUM(quota), 0) AS used_quota").
		Where("token_id = ? AND type = ?", binding.TokenId, LogTypeConsume).
		Scan(&totals).Error
	if err != nil {
		return nil, err
	}
	return &JunhuoLinkUsage{InputTokens: totals.InputTokens, OutputTokens: totals.OutputTokens, TotalTokens: totals.InputTokens + totals.OutputTokens, UsedQuota: totals.UsedQuota}, nil
}

func GetJunhuoLinkUserUsage(linkUserId string) (*JunhuoLinkUserUsage, error) {
	mapping, err := GetActiveJunhuoLinkUserMapping(linkUserId)
	if err != nil {
		return nil, err
	}
	user, err := GetUserById(mapping.NewAPIUserId, false)
	if err != nil {
		return nil, err
	}
	var totals struct {
		InputTokens  int
		OutputTokens int
		UsedQuota    int
	}
	err = LOG_DB.Model(&Log{}).
		Select("COALESCE(SUM(prompt_tokens), 0) AS input_tokens, COALESCE(SUM(completion_tokens), 0) AS output_tokens, COALESCE(SUM(quota), 0) AS used_quota").
		Where("user_id = ? AND type = ?", mapping.NewAPIUserId, LogTypeConsume).
		Scan(&totals).Error
	if err != nil {
		return nil, err
	}
	return &JunhuoLinkUserUsage{JunhuoLinkUsage: JunhuoLinkUsage{InputTokens: totals.InputTokens, OutputTokens: totals.OutputTokens, TotalTokens: totals.InputTokens + totals.OutputTokens, UsedQuota: totals.UsedQuota}, Balance: user.Quota}, nil
}

func RecordJunhuoLinkUsageReceipt(linkUserId string, deviceId string, tokenId int, usageId string, taskId string, inputTokens int, outputTokens int, now int64) (*JunhuoLinkUsageReceipt, bool, error) {
	usageId = strings.TrimSpace(usageId)
	taskId = strings.TrimSpace(taskId)
	if usageId == "" || len(usageId) > 256 || len(taskId) > 256 || inputTokens < 0 || outputTokens < 0 {
		return nil, false, ErrJunhuoLinkUsageMismatch
	}
	binding, err := GetJunhuoLinkDeviceKey(linkUserId, deviceId, tokenId)
	if err != nil {
		return nil, false, err
	}
	if binding.RevokedAt != 0 {
		return nil, false, ErrJunhuoLinkBindingNotFound
	}

	var existing JunhuoLinkUsageReceipt
	err = DB.Where("usage_id = ?", usageId).First(&existing).Error
	if err == nil {
		if existing.BindingId != binding.Id || existing.LinkUserId != binding.LinkUserId || existing.DeviceId != binding.DeviceId || existing.TokenId != binding.TokenId || existing.TaskId != taskId {
			return nil, false, ErrJunhuoLinkUsageConflict
		}
		return &existing, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	log, err := findAuthoritativeJunhuoLinkUsageLog(binding.TokenId, usageId, inputTokens, outputTokens, now)
	if err != nil {
		return nil, false, err
	}
	receipt := &JunhuoLinkUsageReceipt{
		UsageId: usageId, BindingId: binding.Id, LinkUserId: binding.LinkUserId, DeviceId: binding.DeviceId,
		NewAPIUserId: binding.NewAPIUserId, TokenId: binding.TokenId, TaskId: taskId,
		LogId: log.Id, RequestId: log.RequestId, InputTokens: log.PromptTokens, OutputTokens: log.CompletionTokens,
		TotalTokens: log.PromptTokens + log.CompletionTokens, UsedQuota: log.Quota, CreatedAt: now,
	}
	if err := DB.Create(receipt).Error; err != nil {
		var raced JunhuoLinkUsageReceipt
		if readErr := DB.Where("usage_id = ?", usageId).First(&raced).Error; readErr == nil {
			if raced.BindingId == binding.Id && raced.TaskId == taskId {
				return &raced, true, nil
			}
			return nil, false, ErrJunhuoLinkUsageConflict
		}
		return nil, false, err
	}
	return receipt, false, nil
}

func findAuthoritativeJunhuoLinkUsageLog(tokenId int, usageId string, inputTokens int, outputTokens int, now int64) (*Log, error) {
	var exact []Log
	if usageId != "" {
		if err := LOG_DB.Where("token_id = ? AND type = ? AND request_id = ?", tokenId, LogTypeConsume, usageId).Order("id DESC").Limit(2).Find(&exact).Error; err != nil {
			return nil, err
		}
		if len(exact) == 1 {
			if exact[0].PromptTokens != inputTokens || exact[0].CompletionTokens != outputTokens {
				return nil, ErrJunhuoLinkUsageMismatch
			}
			if isJunhuoLinkLogClaimed(exact[0].Id) {
				return nil, ErrJunhuoLinkUsageConflict
			}
			return &exact[0], nil
		}
		if len(exact) > 1 {
			return nil, ErrJunhuoLinkUsageAmbiguous
		}
	}

	cutoff := now - junhuoLinkUsageMatchWindowSeconds
	var candidates []Log
	if err := LOG_DB.Where(
		"token_id = ? AND type = ? AND prompt_tokens = ? AND completion_tokens = ? AND created_at >= ? AND created_at <= ?",
		tokenId, LogTypeConsume, inputTokens, outputTokens, cutoff, now+60,
	).Order("id DESC").Limit(8).Find(&candidates).Error; err != nil {
		return nil, err
	}
	unclaimed := make([]Log, 0, len(candidates))
	for _, candidate := range candidates {
		if !isJunhuoLinkLogClaimed(candidate.Id) {
			unclaimed = append(unclaimed, candidate)
		}
	}
	if len(unclaimed) == 0 {
		return nil, ErrJunhuoLinkUsageNotFound
	}
	if len(unclaimed) != 1 {
		return nil, ErrJunhuoLinkUsageAmbiguous
	}
	return &unclaimed[0], nil
}

func isJunhuoLinkLogClaimed(logId int) bool {
	var count int64
	if err := DB.Model(&JunhuoLinkUsageReceipt{}).Where("log_id = ?", logId).Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

func truncateJunhuoLinkLabel(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func JunhuoLinkNow() int64 { return time.Now().Unix() }
