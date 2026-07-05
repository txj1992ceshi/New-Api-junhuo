package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ClientSession struct {
	Id         string `json:"id" gorm:"type:varchar(64);primaryKey"`
	UserId     int    `json:"user_id" gorm:"index;not null"`
	DeviceId   string `json:"device_id" gorm:"type:varchar(128);index;not null"`
	TokenHash  string `json:"-" gorm:"type:varchar(128);uniqueIndex;not null"`
	ExpiresAt  int64  `json:"expires_at" gorm:"index;not null"`
	RevokedAt  int64  `json:"revoked_at" gorm:"index;default:0"`
	LastSeenAt int64  `json:"last_seen_at" gorm:"bigint;default:0"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt  int64  `json:"updated_at" gorm:"bigint"`
}

func (s *ClientSession) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if s.Id == "" {
		s.Id = uuid.NewString()
	}
	s.CreatedAt = now
	s.UpdatedAt = now
	if s.LastSeenAt == 0 {
		s.LastSeenAt = now
	}
	return nil
}

func (s *ClientSession) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

func HashClientSessionToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func CreateClientSession(userId int, deviceId string, token string, expiresAt int64) (*ClientSession, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	deviceId = strings.TrimSpace(deviceId)
	token = strings.TrimSpace(token)
	if deviceId == "" || token == "" || expiresAt <= common.GetTimestamp() {
		return nil, errors.New("invalid client session")
	}
	session := &ClientSession{
		UserId:    userId,
		DeviceId:  deviceId,
		TokenHash: HashClientSessionToken(token),
		ExpiresAt: expiresAt,
	}
	if err := DB.Create(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

func GetValidClientSession(token string, deviceId string, now int64) (*ClientSession, error) {
	token = strings.TrimSpace(token)
	deviceId = strings.TrimSpace(deviceId)
	if token == "" || deviceId == "" {
		return nil, errors.New("client session is required")
	}
	var session ClientSession
	err := DB.Where("token_hash = ? AND device_id = ? AND revoked_at = ? AND expires_at > ?", HashClientSessionToken(token), deviceId, 0, now).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	if err := DB.Model(&ClientSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{
		"last_seen_at": now,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, err
	}
	session.LastSeenAt = now
	return &session, nil
}

func HasJunhuoCodexEntitlement(user *User, entitlementGroups []string, now int64) (bool, []string, error) {
	if user == nil || user.Id <= 0 {
		return false, nil, errors.New("invalid user")
	}
	if user.Status != common.UserStatusEnabled {
		return false, nil, nil
	}
	groupSet := map[string]bool{}
	for _, group := range entitlementGroups {
		group = strings.TrimSpace(group)
		if group != "" {
			groupSet[group] = true
		}
	}
	if len(groupSet) == 0 {
		return false, []string{}, nil
	}
	if groupSet[strings.TrimSpace(user.Group)] {
		return true, []string{"local_agent", "browser_bridge", "task_graph"}, nil
	}
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group IN ?", user.Id, "active", now, keysOfBoolMap(groupSet)).
		Count(&count).Error; err != nil {
		return false, nil, err
	}
	if count > 0 {
		return true, []string{"local_agent", "browser_bridge", "task_graph"}, nil
	}
	return false, []string{}, nil
}

func keysOfBoolMap(input map[string]bool) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	return keys
}
