package model

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertJunhuoLinkUser(t *testing.T, id int, username string, quota int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id: id, Username: username, Status: common.UserStatusEnabled, Quota: quota, AffCode: fmt.Sprintf("jl%d", id),
	}).Error)
}

func TestJunhuoLinkDeviceKeyIssueIsIdempotentAndRawKeyIsReturnedOnlyForCreation(t *testing.T) {
	truncateTables(t)
	insertJunhuoLinkUser(t, 701, "link_issue_user", 9000)
	_, err := UpsertJunhuoLinkUserMapping("station-user-a", 701, 1000)
	require.NoError(t, err)

	first, err := IssueJunhuoLinkDeviceKey("station-user-a", "device-a", "stored-key-first", "sha256:first", 1001)
	require.NoError(t, err)
	assert.True(t, first.Created)
	assert.Equal(t, "stored-key-first", first.RawKey)
	assert.NotZero(t, first.Token.Id)

	second, err := IssueJunhuoLinkDeviceKey("station-user-a", "device-a", "must-not-be-stored", "sha256:second", 1002)
	require.NoError(t, err)
	assert.False(t, second.Created)
	assert.Empty(t, second.RawKey)
	assert.Equal(t, first.Token.Id, second.Token.Id)
	assert.Equal(t, "sha256:first", second.Binding.KeyFingerprint)

	var tokenCount int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 701).Count(&tokenCount).Error)
	assert.EqualValues(t, 1, tokenCount)

	var leaked Token
	err = DB.Where("key = ?", "must-not-be-stored").First(&leaked).Error
	assert.Error(t, err)
}

func TestJunhuoLinkMappingIsOneToOneAndCannotBeGuessedAcrossUsers(t *testing.T) {
	truncateTables(t)
	insertJunhuoLinkUser(t, 702, "link_map_user_a", 100)
	insertJunhuoLinkUser(t, 703, "link_map_user_b", 100)

	_, err := UpsertJunhuoLinkUserMapping("station-user-a", 702, 2000)
	require.NoError(t, err)
	_, err = UpsertJunhuoLinkUserMapping("station-user-b", 702, 2001)
	assert.ErrorIs(t, err, ErrJunhuoLinkMappingConflict)
	_, err = UpsertJunhuoLinkUserMapping("station-user-a", 703, 2002)
	assert.ErrorIs(t, err, ErrJunhuoLinkMappingConflict)

	mapping, err := GetActiveJunhuoLinkUserMapping("station-user-a")
	require.NoError(t, err)
	assert.Equal(t, 702, mapping.NewAPIUserId)
}

func TestJunhuoLinkRevocationImmediatelyInvalidatesTheExistingNewAPIToken(t *testing.T) {
	truncateTables(t)
	insertJunhuoLinkUser(t, 704, "link_revoke_user", 1000)
	_, err := UpsertJunhuoLinkUserMapping("station-user-revoke", 704, 3000)
	require.NoError(t, err)
	issued, err := IssueJunhuoLinkDeviceKey("station-user-revoke", "device-revoke", "revoke-key", "sha256:revoke", 3001)
	require.NoError(t, err)

	validated, err := ValidateUserToken("revoke-key")
	require.NoError(t, err)
	assert.Equal(t, issued.Token.Id, validated.Id)

	require.NoError(t, RevokeJunhuoLinkDeviceKey("station-user-revoke", "device-revoke", issued.Token.Id, 3002))
	_, err = ValidateUserToken("revoke-key")
	assert.Error(t, err)

	binding, active, err := GetJunhuoLinkDeviceKeyStatus("station-user-revoke", "device-revoke", issued.Token.Id)
	require.NoError(t, err)
	assert.False(t, active)
	assert.EqualValues(t, 3002, binding.RevokedAt)
}

func TestJunhuoLinkUsageReceiptLinksAuthoritativeConsumeLogAndNeverChargesAgain(t *testing.T) {
	truncateTables(t)
	insertJunhuoLinkUser(t, 705, "link_usage_user", 7777)
	_, err := UpsertJunhuoLinkUserMapping("station-user-usage", 705, 4000)
	require.NoError(t, err)
	issued, err := IssueJunhuoLinkDeviceKey("station-user-usage", "device-usage", "usage-key", "sha256:usage", 4001)
	require.NoError(t, err)

	consume := &Log{
		UserId: 705, CreatedAt: 4010, Type: LogTypeConsume,
		PromptTokens: 11, CompletionTokens: 7, Quota: 123,
		TokenId: issued.Token.Id, RequestId: "newapi-request-1",
	}
	require.NoError(t, LOG_DB.Create(consume).Error)

	beforeQuota, err := GetUserQuota(705, true)
	require.NoError(t, err)
	first, duplicate, err := RecordJunhuoLinkUsageReceipt(
		"station-user-usage", "device-usage", issued.Token.Id,
		"turn-usage-1", "task-usage-1", 11, 7, 4020,
	)
	require.NoError(t, err)
	assert.False(t, duplicate)
	assert.Equal(t, consume.Id, first.LogId)
	assert.Equal(t, 123, first.UsedQuota)

	second, duplicate, err := RecordJunhuoLinkUsageReceipt(
		"station-user-usage", "device-usage", issued.Token.Id,
		"turn-usage-1", "task-usage-1", 11, 7, 4021,
	)
	require.NoError(t, err)
	assert.True(t, duplicate)
	assert.Equal(t, first.LogId, second.LogId)

	afterQuota, err := GetUserQuota(705, true)
	require.NoError(t, err)
	assert.Equal(t, beforeQuota, afterQuota, "receipt attribution must not charge quota again")

	usage, err := GetJunhuoLinkDeviceUsage("station-user-usage", "device-usage", issued.Token.Id)
	require.NoError(t, err)
	assert.Equal(t, JunhuoLinkUsage{InputTokens: 11, OutputTokens: 7, TotalTokens: 18, UsedQuota: 123}, *usage)

	var receiptCount int64
	require.NoError(t, DB.Model(&JunhuoLinkUsageReceipt{}).Count(&receiptCount).Error)
	assert.EqualValues(t, 1, receiptCount)
}

func TestJunhuoLinkUsageRejectsCrossTaskDeviceAndAmbiguousClaims(t *testing.T) {
	truncateTables(t)
	insertJunhuoLinkUser(t, 706, "link_conflict_user", 500)
	_, err := UpsertJunhuoLinkUserMapping("station-user-conflict", 706, 5000)
	require.NoError(t, err)
	issued, err := IssueJunhuoLinkDeviceKey("station-user-conflict", "device-conflict", "conflict-key", "sha256:conflict", 5001)
	require.NoError(t, err)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId: 706, CreatedAt: 5010, Type: LogTypeConsume,
		PromptTokens: 3, CompletionTokens: 4, Quota: 25,
		TokenId: issued.Token.Id, RequestId: "req-conflict",
	}).Error)

	_, _, err = RecordJunhuoLinkUsageReceipt("station-user-conflict", "device-conflict", issued.Token.Id, "usage-conflict", "task-a", 3, 4, 5020)
	require.NoError(t, err)
	_, _, err = RecordJunhuoLinkUsageReceipt("station-user-conflict", "device-conflict", issued.Token.Id, "usage-conflict", "task-b", 3, 4, 5021)
	assert.ErrorIs(t, err, ErrJunhuoLinkUsageConflict)

	_, err = GetJunhuoLinkDeviceUsage("station-user-conflict", "other-device", issued.Token.Id)
	assert.ErrorIs(t, err, ErrJunhuoLinkBindingNotFound)

	require.NoError(t, LOG_DB.Create(&Log{UserId: 706, CreatedAt: 5030, Type: LogTypeConsume, PromptTokens: 8, CompletionTokens: 9, Quota: 30, TokenId: issued.Token.Id}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 706, CreatedAt: 5031, Type: LogTypeConsume, PromptTokens: 8, CompletionTokens: 9, Quota: 31, TokenId: issued.Token.Id}).Error)
	_, _, err = RecordJunhuoLinkUsageReceipt("station-user-conflict", "device-conflict", issued.Token.Id, "usage-ambiguous", "task-c", 8, 9, 5040)
	assert.ErrorIs(t, err, ErrJunhuoLinkUsageAmbiguous)
}

func TestJunhuoLinkUserRevocationDisablesAllDeviceKeys(t *testing.T) {
	truncateTables(t)
	insertJunhuoLinkUser(t, 707, "link_user_revoke_all", 500)
	_, err := UpsertJunhuoLinkUserMapping("station-user-all", 707, 6000)
	require.NoError(t, err)
	first, err := IssueJunhuoLinkDeviceKey("station-user-all", "device-one", "all-key-one", "sha256:one", 6001)
	require.NoError(t, err)

	require.NoError(t, RevokeJunhuoLinkUserMapping("station-user-all", 6002))
	_, err = GetActiveJunhuoLinkUserMapping("station-user-all")
	assert.ErrorIs(t, err, ErrJunhuoLinkMappingNotFound)
	_, err = ValidateUserToken("all-key-one")
	assert.Error(t, err)
	binding, active, err := GetJunhuoLinkDeviceKeyStatus("station-user-all", "device-one", first.Token.Id)
	require.NoError(t, err)
	assert.False(t, active)
	assert.EqualValues(t, 6002, binding.RevokedAt)
}

func TestJunhuoLinkTablesNeverDuplicateRawAPIKeyMaterial(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(JunhuoLinkUserMapping{}),
		reflect.TypeOf(JunhuoLinkDeviceKey{}),
		reflect.TypeOf(JunhuoLinkUsageReceipt{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			assert.NotContains(t, name, "rawkey")
			assert.NotEqual(t, "key", name)
		}
	}
}

func TestJunhuoLinkUsageIdCannotBeReusedByAnotherBinding(t *testing.T) {
	truncateTables(t)
	insertJunhuoLinkUser(t, 708, "link_usage_owner_a", 500)
	insertJunhuoLinkUser(t, 709, "link_usage_owner_b", 500)
	_, err := UpsertJunhuoLinkUserMapping("station-owner-a", 708, 7000)
	require.NoError(t, err)
	_, err = UpsertJunhuoLinkUserMapping("station-owner-b", 709, 7000)
	require.NoError(t, err)
	a, err := IssueJunhuoLinkDeviceKey("station-owner-a", "device-a", "owner-key-a", "sha256:a", 7001)
	require.NoError(t, err)
	b, err := IssueJunhuoLinkDeviceKey("station-owner-b", "device-b", "owner-key-b", "sha256:b", 7001)
	require.NoError(t, err)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 708, CreatedAt: 7010, Type: LogTypeConsume, PromptTokens: 1, CompletionTokens: 2, Quota: 10, TokenId: a.Token.Id}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 709, CreatedAt: 7010, Type: LogTypeConsume, PromptTokens: 1, CompletionTokens: 2, Quota: 10, TokenId: b.Token.Id}).Error)
	_, _, err = RecordJunhuoLinkUsageReceipt("station-owner-a", "device-a", a.Token.Id, "shared-usage-id", "task-a", 1, 2, 7020)
	require.NoError(t, err)
	_, _, err = RecordJunhuoLinkUsageReceipt("station-owner-b", "device-b", b.Token.Id, "shared-usage-id", "task-b", 1, 2, 7020)
	assert.True(t, errors.Is(err, ErrJunhuoLinkUsageConflict))
}
