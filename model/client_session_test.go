package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestClientSessionValidatesHashDeviceAndExpiry(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	session, err := CreateClientSession(42, "device_1", "session-token", now+60)
	require.NoError(t, err)
	require.NotEmpty(t, session.Id)
	require.NotEqual(t, "session-token", session.TokenHash)
	require.Equal(t, HashClientSessionToken("session-token"), session.TokenHash)

	loaded, err := GetValidClientSession("session-token", "device_1", now)
	require.NoError(t, err)
	require.Equal(t, 42, loaded.UserId)

	_, err = GetValidClientSession("session-token", "device_2", now)
	require.Error(t, err)
	_, err = GetValidClientSession("session-token", "device_1", now+61)
	require.Error(t, err)
	require.NoError(t, DB.Model(&ClientSession{}).Where("id = ?", session.Id).Update("revoked_at", now).Error)
	_, err = GetValidClientSession("session-token", "device_1", now)
	require.Error(t, err)
}

func TestHasJunhuoCodexEntitlementByGroupAndSubscription(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	groupUser := &User{Id: 1001, Username: "group-user", Status: common.UserStatusEnabled, Group: "codex"}
	entitled, features, err := HasJunhuoCodexEntitlement(groupUser, []string{"codex"}, now)
	require.NoError(t, err)
	require.True(t, entitled)
	require.Contains(t, features, "local_agent")

	subUser := &User{Id: 1002, Username: "sub-user", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, DB.Create(subUser).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:       subUser.Id,
		Status:       "active",
		EndTime:      now + 3600,
		UpgradeGroup: "junhuo-codex",
	}).Error)
	entitled, features, err = HasJunhuoCodexEntitlement(subUser, []string{"junhuo-codex"}, now)
	require.NoError(t, err)
	require.True(t, entitled)
	require.Contains(t, features, "task_graph")

	disabledUser := &User{Id: 1003, Username: "disabled-user", Status: common.UserStatusDisabled, Group: "codex"}
	entitled, _, err = HasJunhuoCodexEntitlement(disabledUser, []string{"codex"}, now)
	require.NoError(t, err)
	require.False(t, entitled)
}
