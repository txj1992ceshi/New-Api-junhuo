package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestTriggerCursorProReplacementReturnsDisabled(t *testing.T) {
	state := cursorProStateForChannel(43210)
	state.LastResultStatus = ""
	state.LastErrorCode = ""
	state.LastErrorMessage = ""

	result, err := TriggerCursorProReplacement(nil, 43210, "test_reason")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected replacement result")
	}
	if result.Triggered {
		t.Fatalf("expected disabled trigger, got %+v", result)
	}
	if result.Status != cursorProResultCodeDisabled {
		t.Fatalf("unexpected status: %+v", result)
	}
	if state.LastResultStatus != "disabled" || state.LastErrorCode != cursorProResultCodeDisabled {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestUpsertCursorProTokenAppendsNewKeyAsNewState(t *testing.T) {
	channel := &model.Channel{
		Id:   1,
		Type: constant.ChannelTypeCodex,
		Key:  `{"access_token":"old","refresh_token":"old-rt","account_id":"old-account","email":"old@example.com","type":"codex"}`,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   false,
			MultiKeySize: 1,
		},
	}

	item := &cursorProExportFile{
		AccountID: "new-account",
		Email:     "new@example.com",
		ExpiresAt: "2026-05-09T00:00:00Z",
		Source:    "cursorpro3",
	}
	item.Raw.AccessToken = "new-at"
	item.Raw.RefreshToken = "new-rt"
	item.Raw.IDToken = "new-id"

	key, err := buildCodexOAuthKeyFromCursorProExport(*item)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	result := upsertCursorProToken(channel, key, item)
	if result.Status != cursorProUpsertStatusImported || result.Replaced || result.Index != 1 {
		t.Fatalf("expected imported=true updated=false, got %+v", result)
	}

	finalizeCodexMultiKeyChannel(channel)
	if !channel.ChannelInfo.IsMultiKey || channel.ChannelInfo.MultiKeySize != 2 {
		t.Fatalf("expected multi-key channel after import, got %+v", channel.ChannelInfo)
	}

	meta := channel.GetKeyMeta(1)
	if meta.State != model.CodexKeyStateNew {
		t.Fatalf("expected new state, got %s", meta.State)
	}
	if meta.AccountID != item.AccountID || meta.Email != item.Email {
		t.Fatalf("unexpected imported meta: %+v", meta)
	}
}

func TestUpsertCursorProTokenUpdatesExistingAccountAndResetsDeadMeta(t *testing.T) {
	channel := &model.Channel{
		Id:   cursorProManagedChannelID,
		Type: constant.ChannelTypeCodex,
		Key:  `{"access_token":"old","refresh_token":"old-rt","account_id":"acct-1","email":"old@example.com","type":"codex"}`,
		ChannelInfo: model.ChannelInfo{
			MultiKeyMeta: map[int]model.ChannelKeyMeta{
				0: {
					State:         model.CodexKeyStateDead,
					LastErrorKind: "rate_limit",
					LastErrorAt:   time.Now().Add(-time.Hour).Unix(),
					CooldownUntil: time.Now().Add(time.Hour).Unix(),
				},
			},
		},
	}
	item := &cursorProExportFile{
		AccountID: "acct-1",
		Email:     "new@example.com",
		ExpiresAt: "2026-05-09T00:00:00Z",
		Source:    "cursorpro3",
	}
	item.Raw.AccessToken = "new-at"
	item.Raw.RefreshToken = "new-rt"
	item.Raw.IDToken = "new-id"

	key, err := buildCodexOAuthKeyFromCursorProExport(*item)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	result := upsertCursorProToken(channel, key, item)
	if result.Status != cursorProUpsertStatusUpdated || result.Replaced {
		t.Fatalf("unexpected upsert result: %+v", result)
	}
	meta := channel.GetKeyMeta(0)
	if meta.State != model.CodexKeyStateNew || meta.LastErrorKind != "" || meta.LastErrorAt != 0 || meta.CooldownUntil != 0 {
		t.Fatalf("expected refreshed token meta to be reset as new, got %+v", meta)
	}
	if meta.Email != item.Email || meta.AccountID != item.AccountID {
		t.Fatalf("unexpected refreshed meta identity: %+v", meta)
	}
}

func TestUpsertCursorProTokenReplacesDeadSlotAtManagedCapacity(t *testing.T) {
	keys := make([]string, 0, cursorProManagedPoolCap)
	meta := make(map[int]model.ChannelKeyMeta, cursorProManagedPoolCap)
	for i := 0; i < cursorProManagedPoolCap; i++ {
		keys = append(keys, `{"access_token":"at-`+strconv.Itoa(i)+`","refresh_token":"rt-`+strconv.Itoa(i)+`","account_id":"acct-`+strconv.Itoa(i)+`","email":"user`+strconv.Itoa(i)+`@example.com","type":"codex"}`)
		meta[i] = model.ChannelKeyMeta{State: model.CodexKeyStateHealthy}
	}
	meta[17] = model.ChannelKeyMeta{
		State:         model.CodexKeyStateDead,
		LastErrorKind: "rate_limit_exhausted",
		LastErrorAt:   time.Now().Add(-2 * time.Hour).Unix(),
		CooldownUntil: time.Now().Add(time.Hour).Unix(),
	}
	channel := &model.Channel{
		Id:   cursorProManagedChannelID,
		Type: constant.ChannelTypeCodex,
		Key:  strings.Join(keys, "\n"),
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: len(keys),
			MultiKeyMeta: meta,
		},
	}
	item := &cursorProExportFile{
		AccountID: "fresh-account",
		Email:     "fresh@example.com",
		ExpiresAt: "2026-05-09T00:00:00Z",
		Source:    "cursorpro3",
	}
	item.Raw.AccessToken = "fresh-at"
	item.Raw.RefreshToken = "fresh-rt"

	key, err := buildCodexOAuthKeyFromCursorProExport(*item)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	result := upsertCursorProToken(channel, key, item)
	if result.Status != cursorProUpsertStatusImported || !result.Replaced {
		t.Fatalf("unexpected upsert result: %+v", result)
	}
	if result.Index != 17 {
		t.Fatalf("expected dead slot 17 to be replaced, got %d", result.Index)
	}
	slotMeta := channel.GetKeyMeta(17)
	if slotMeta.State != model.CodexKeyStateNew || slotMeta.LastErrorKind != "" || slotMeta.LastErrorAt != 0 {
		t.Fatalf("expected replaced slot to reset to new, got %+v", slotMeta)
	}
}

func TestUpsertCursorProTokenSkipsWhenManagedCapacityFullWithoutReplaceableDead(t *testing.T) {
	keys := make([]string, 0, cursorProManagedPoolCap)
	meta := make(map[int]model.ChannelKeyMeta, cursorProManagedPoolCap)
	for i := 0; i < cursorProManagedPoolCap; i++ {
		keys = append(keys, `{"access_token":"at-`+strconv.Itoa(i)+`","refresh_token":"rt-`+strconv.Itoa(i)+`","account_id":"acct-`+strconv.Itoa(i)+`","email":"user`+strconv.Itoa(i)+`@example.com","type":"codex"}`)
		meta[i] = model.ChannelKeyMeta{State: model.CodexKeyStateHealthy}
	}
	channel := &model.Channel{
		Id:   cursorProManagedChannelID,
		Type: constant.ChannelTypeCodex,
		Key:  strings.Join(keys, "\n"),
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: len(keys),
			MultiKeyMeta: meta,
		},
	}
	item := &cursorProExportFile{
		AccountID: "fresh-account",
		Email:     "fresh@example.com",
		Source:    "cursorpro3",
	}
	item.Raw.AccessToken = "fresh-at"
	item.Raw.RefreshToken = "fresh-rt"

	key, err := buildCodexOAuthKeyFromCursorProExport(*item)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	result := upsertCursorProToken(channel, key, item)
	if result.Status != cursorProUpsertStatusCapacityFull || result.Replaced {
		t.Fatalf("unexpected upsert result: %+v", result)
	}
}

func TestUpsertCursorProTokenMarksConsumedNoChangeWhenExistingKeyMatches(t *testing.T) {
	channel := &model.Channel{
		Id:   1,
		Type: constant.ChannelTypeCodex,
		ChannelInfo: model.ChannelInfo{
			MultiKeyMeta: map[int]model.ChannelKeyMeta{
				0: {
					State:     model.CodexKeyStateHealthy,
					AccountID: "acct-1",
					Email:     "same@example.com",
				},
			},
		},
	}
	item := &cursorProExportFile{
		AccountID: "acct-1",
		Email:     "same@example.com",
		ExpiresAt: "2026-05-09T00:00:00Z",
		Source:    "cursorpro3",
	}
	item.Raw.AccessToken = "same-at"
	item.Raw.RefreshToken = "same-rt"
	item.Raw.IDToken = "same-id"

	key, err := buildCodexOAuthKeyFromCursorProExport(*item)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	channel.Key = key
	originalKey := channel.Key
	originalMeta := channel.GetKeyMeta(0)

	result := upsertCursorProToken(channel, key, item)
	if result.Status != cursorProUpsertStatusConsumedNoChange || result.Replaced {
		t.Fatalf("unexpected upsert result: %+v", result)
	}
	if channel.Key != originalKey {
		t.Fatalf("expected channel key unchanged")
	}
	if got := channel.GetKeyMeta(0); got.State != originalMeta.State ||
		got.AccountID != originalMeta.AccountID ||
		got.Email != originalMeta.Email ||
		got.LastErrorKind != originalMeta.LastErrorKind ||
		got.LastErrorAt != originalMeta.LastErrorAt ||
		got.CooldownUntil != originalMeta.CooldownUntil {
		t.Fatalf("expected key meta unchanged, got %+v want %+v", got, originalMeta)
	}
}

func TestBuildCodexOAuthKeyFromCursorProExportDerivesAccountIDFromJWT(t *testing.T) {
	makeJWT := func(claims map[string]any) string {
		header, _ := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
		payload, _ := json.Marshal(claims)
		return base64.RawURLEncoding.EncodeToString(header) + "." +
			base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	}

	accessToken := makeJWT(map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "derived-account-id",
		},
		"email": "derived@example.com",
	})

	item := cursorProExportFile{
		Email: "",
	}
	item.Raw.AccessToken = accessToken
	item.Raw.RefreshToken = "rt"

	key, err := buildCodexOAuthKeyFromCursorProExport(item)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	oauthKey, err := parseCodexOAuthKey(key)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if oauthKey.AccountID != "derived-account-id" {
		t.Fatalf("expected derived account id, got %q", oauthKey.AccountID)
	}
	if oauthKey.Email != "derived@example.com" {
		t.Fatalf("expected derived email, got %q", oauthKey.Email)
	}
}

func TestDefaultCursorProCodexExportDirForGOOS(t *testing.T) {
	t.Run("darwin", func(t *testing.T) {
		got := defaultCursorProCodexExportDirForGOOS("darwin", "/Users/tester", "")
		want := "/Users/tester/Library/Application Support/CursorPro3/exports/codex"
		if got != want {
			t.Fatalf("unexpected darwin export dir: got=%q want=%q", got, want)
		}
	})

	t.Run("windows", func(t *testing.T) {
		got := defaultCursorProCodexExportDirForGOOS("windows", `C:\Users\tester`, `C:\Users\tester\AppData\Local`)
		want := filepath.Join(`C:\Users\tester\AppData\Local`, "CursorPro3", "exports", "codex")
		if got != want {
			t.Fatalf("unexpected windows export dir: got=%q want=%q", got, want)
		}
	})

	t.Run("linux fallback", func(t *testing.T) {
		got := defaultCursorProCodexExportDirForGOOS("linux", "/home/tester", "")
		want := "/home/tester/.local/share/CursorPro3/exports/codex"
		if got != want {
			t.Fatalf("unexpected linux export dir: got=%q want=%q", got, want)
		}
	})
}

func TestValidateCodexOAuthKeyPayload(t *testing.T) {
	makeJWT := func(claims map[string]any) string {
		header, _ := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
		payload, _ := json.Marshal(claims)
		return base64.RawURLEncoding.EncodeToString(header) + "." +
			base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	}

	t.Run("rejects plain text key", func(t *testing.T) {
		if got := validateCodexOAuthKeyPayload("not-json"); got == "" {
			t.Fatal("expected invalid payload for plain text key")
		}
	})

	t.Run("rejects missing account id", func(t *testing.T) {
		raw := `{"access_token":"at","refresh_token":"rt","type":"codex"}`
		if got := validateCodexOAuthKeyPayload(raw); got != "invalid_key_missing_account_id" {
			t.Fatalf("unexpected validation result: %s", got)
		}
	})

	t.Run("accepts missing account id when jwt can derive it", func(t *testing.T) {
		raw := `{"access_token":"` + makeJWT(map[string]any{
			"https://api.openai.com/auth": map[string]any{
				"chatgpt_account_id": "jwt-account-id",
			},
			"email": "jwt@example.com",
		}) + `","refresh_token":"rt","type":"codex"}`
		if got := validateCodexOAuthKeyPayload(raw); got != "" {
			t.Fatalf("expected derived payload to be valid, got %s", got)
		}
	})

	t.Run("accepts valid oauth key", func(t *testing.T) {
		raw := `{"access_token":"at","refresh_token":"rt","account_id":"acct","type":"codex"}`
		if got := validateCodexOAuthKeyPayload(raw); got != "" {
			t.Fatalf("expected valid payload, got %s", got)
		}
	})
}

func TestShouldAggressivelyEvictRateLimitedKey(t *testing.T) {
	now := time.Now()

	t.Run("kills stale 429 key with no success history", func(t *testing.T) {
		meta := model.ChannelKeyMeta{
			State:          model.CodexKeyStateCooldown,
			Consecutive429: 2,
		}
		if !shouldAggressivelyEvictRateLimitedKey(meta, now) {
			t.Fatal("expected stale cooldown key to be evicted")
		}
	})

	t.Run("kills suspect 429 key with old success", func(t *testing.T) {
		meta := model.ChannelKeyMeta{
			State:          model.CodexKeyStateSuspect,
			Consecutive429: 3,
			LastSuccessAt:  now.Add(-31 * time.Minute).Unix(),
		}
		if !shouldAggressivelyEvictRateLimitedKey(meta, now) {
			t.Fatal("expected old suspect key to be evicted")
		}
	})

	t.Run("keeps recently successful key", func(t *testing.T) {
		meta := model.ChannelKeyMeta{
			State:          model.CodexKeyStateCooldown,
			Consecutive429: 2,
			LastSuccessAt:  now.Add(-10 * time.Minute).Unix(),
		}
		if shouldAggressivelyEvictRateLimitedKey(meta, now) {
			t.Fatal("did not expect recently successful key to be evicted")
		}
	})
}

func TestMarkCodexKeyRateLimitedPolicy(t *testing.T) {
	now := time.Now()
	meta := model.ChannelKeyMeta{
		State:          model.CodexKeyStateHealthy,
		Consecutive429: 0,
	}

	meta.LastErrorAt = now.Unix()
	meta.LastErrorKind = string(CodexErrorKindRateLimit)
	meta.TotalFail++
	meta.Consecutive429++
	meta.Consecutive5xx = 0
	meta.ConsecutiveAuthFail = 0
	meta.CooldownUntil = 0
	meta.State = model.CodexKeyStateDead

	if meta.State != model.CodexKeyStateDead {
		t.Fatalf("expected rate-limited key to become dead immediately, got %s", meta.State)
	}
	if meta.CooldownUntil != 0 {
		t.Fatalf("expected no cooldown for rate-limited key, got %d", meta.CooldownUntil)
	}
	if meta.Consecutive429 != 1 {
		t.Fatalf("expected consecutive429=1, got %d", meta.Consecutive429)
	}
}

func TestApplyCursorProRegisterOutcomeMarksNoYieldTimeout(t *testing.T) {
	now := time.Now()
	state := &cursorProTriggerState{
		LastTriggerAt:   now.Add(-5 * time.Minute),
		LastExportCount: 3,
		LastExportName:  "existing.json",
		LastExportMtime: now.Add(-6 * time.Minute),
	}
	status := &cursorProRegisterStatus{
		TaskID:       "task-1",
		Status:       "failed",
		ErrorCode:    "register_timeout",
		ErrorMessage: "No token file changes were detected before timeout.",
		FinishedAt:   now.Format(time.RFC3339),
	}
	exportSnapshot := cursorProExportSnapshot{
		Count:       3,
		LatestName:  "existing.json",
		LatestMtime: now.Add(-6 * time.Minute),
	}

	applyCursorProRegisterOutcome(state, status, exportSnapshot, now)

	if state.LastResultStatus != "failed" {
		t.Fatalf("expected failed result status, got %s", state.LastResultStatus)
	}
	if state.LastErrorCode != cursorProResultCodeNoYield {
		t.Fatalf("expected no-yield code, got %s", state.LastErrorCode)
	}
	if state.LastErrorMessage != "Register trigger completed without new source/export tokens." {
		t.Fatalf("unexpected message: %s", state.LastErrorMessage)
	}
}

func TestApplyCursorProRegisterOutcomePreservesTimeoutSuccessWhenExportYielded(t *testing.T) {
	now := time.Now()
	state := &cursorProTriggerState{
		LastTriggerAt:   now.Add(-5 * time.Minute),
		LastExportCount: 3,
		LastExportName:  "existing.json",
		LastExportMtime: now.Add(-6 * time.Minute),
	}
	status := &cursorProRegisterStatus{
		TaskID:       "task-2",
		Status:       "failed",
		ErrorCode:    "register_timeout",
		ErrorMessage: "No token file changes were detected before timeout.",
		FinishedAt:   now.Format(time.RFC3339),
	}
	exportSnapshot := cursorProExportSnapshot{
		Count:       4,
		LatestName:  "new.json",
		LatestMtime: now.Add(-time.Minute),
	}

	applyCursorProRegisterOutcome(state, status, exportSnapshot, now)

	if state.LastResultStatus != "succeeded" {
		t.Fatalf("expected succeeded result status, got %s", state.LastResultStatus)
	}
	if state.LastErrorCode != "export_detected_after_timeout" {
		t.Fatalf("unexpected result code: %s", state.LastErrorCode)
	}
}

func TestDeleteConsumedExportFiles(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.json")
	fileB := filepath.Join(tmpDir, "b.json")
	if err := os.WriteFile(fileA, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write fileB: %v", err)
	}

	deleted, failed := deleteConsumedExportFiles([]cursorProConsumedExport{
		{path: fileA, name: "a.json"},
		{path: filepath.Join(tmpDir, "missing.json"), name: "missing.json"},
		{path: fileB, name: "b.json"},
	})
	if deleted != 2 || failed != 1 {
		t.Fatalf("unexpected delete counts: deleted=%d failed=%d", deleted, failed)
	}
	if _, err := os.Stat(fileA); !os.IsNotExist(err) {
		t.Fatalf("expected fileA removed, stat err=%v", err)
	}
	if _, err := os.Stat(fileB); !os.IsNotExist(err) {
		t.Fatalf("expected fileB removed, stat err=%v", err)
	}
}

func setupCursorProImportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	model.DB = db
	model.LOG_DB = db
	if err := db.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("migrate channel: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestImportCursorProExportsConsumedNoChangeDeletesExportAndSetsResult(t *testing.T) {
	db := setupCursorProImportTestDB(t)

	exportDir := t.TempDir()
	t.Setenv("CURSORPRO_CODEX_EXPORT_DIR", exportDir)

	controlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tokens/sync":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"changed":false,"forced":false,"reason":"test","result":"noop"}`))
		case "/v1/tokens/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"source_token_count":1,"export_token_count":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer controlServer.Close()
	t.Setenv("CURSORPRO_CONTROL_URL", controlServer.URL)

	item := cursorProExportFile{
		Provider:   "codex",
		AccountID:  "acct-consumed",
		Email:      "consumed@example.com",
		ExpiresAt:  "2026-05-09T00:00:00Z",
		Source:     "cursorpro3",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}
	item.Raw.AccessToken = "same-at"
	item.Raw.RefreshToken = "same-rt"
	item.Raw.IDToken = "same-id"

	key, err := buildCodexOAuthKeyFromCursorProExport(item)
	if err != nil {
		t.Fatalf("build key: %v", err)
	}

	channelID := 901
	cursorProTriggerStateMap.Delete(channelID)
	channel := &model.Channel{
		Id:     channelID,
		Type:   constant.ChannelTypeCodex,
		Key:    key,
		Status: common.ChannelStatusEnabled,
		Name:   "cursorpro-import-test",
		ChannelInfo: model.ChannelInfo{
			MultiKeyMeta: map[int]model.ChannelKeyMeta{
				0: {
					State:     model.CodexKeyStateHealthy,
					AccountID: item.AccountID,
					Email:     item.Email,
				},
			},
		},
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}

	exportPath := filepath.Join(exportDir, "same.json")
	raw, err := common.Marshal(item)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	if err := os.WriteFile(exportPath, raw, 0o644); err != nil {
		t.Fatalf("write export: %v", err)
	}

	result, err := ImportCursorProExports(context.Background(), channelID)
	if err != nil {
		t.Fatalf("import exports: %v", err)
	}
	if result.ConsumedNoChange != 1 || result.Imported != 0 || result.Updated != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if result.DeletedExports != 1 || result.FailedExportDeletes != 0 {
		t.Fatalf("unexpected delete counts: %+v", result)
	}
	if _, err := os.Stat(exportPath); !os.IsNotExist(err) {
		t.Fatalf("expected export file deleted, stat err=%v", err)
	}

	state := cursorProStateForChannel(channelID)
	if state.LastImportResult != "already_consumed_no_change" {
		t.Fatalf("unexpected last import result: %s", state.LastImportResult)
	}
}

func TestImportCursorProExportsConsumedNoChangeDeleteFailureSetsResult(t *testing.T) {
	db := setupCursorProImportTestDB(t)

	exportDir := t.TempDir()
	t.Setenv("CURSORPRO_CODEX_EXPORT_DIR", exportDir)

	controlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tokens/sync":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"changed":false,"forced":false,"reason":"test","result":"noop"}`))
		case "/v1/tokens/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"source_token_count":1,"export_token_count":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer controlServer.Close()
	t.Setenv("CURSORPRO_CONTROL_URL", controlServer.URL)

	item := cursorProExportFile{
		Provider:   "codex",
		AccountID:  "acct-consumed-fail",
		Email:      "consumed-fail@example.com",
		ExpiresAt:  "2026-05-09T00:00:00Z",
		Source:     "cursorpro3",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}
	item.Raw.AccessToken = "same-at-fail"
	item.Raw.RefreshToken = "same-rt-fail"
	item.Raw.IDToken = "same-id-fail"

	key, err := buildCodexOAuthKeyFromCursorProExport(item)
	if err != nil {
		t.Fatalf("build key: %v", err)
	}

	channelID := 902
	cursorProTriggerStateMap.Delete(channelID)
	channel := &model.Channel{
		Id:     channelID,
		Type:   constant.ChannelTypeCodex,
		Key:    key,
		Status: common.ChannelStatusEnabled,
		Name:   "cursorpro-import-test-delete-fail",
		ChannelInfo: model.ChannelInfo{
			MultiKeyMeta: map[int]model.ChannelKeyMeta{
				0: {
					State:     model.CodexKeyStateHealthy,
					AccountID: item.AccountID,
					Email:     item.Email,
				},
			},
		},
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}

	exportPath := filepath.Join(exportDir, "same-fail.json")
	raw, err := common.Marshal(item)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	if err := os.WriteFile(exportPath, raw, 0o644); err != nil {
		t.Fatalf("write export: %v", err)
	}

	if err := os.Chmod(exportDir, 0o555); err != nil {
		t.Fatalf("chmod export dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(exportDir, 0o755)
	})

	result, err := ImportCursorProExports(context.Background(), channelID)
	if err != nil {
		t.Fatalf("import exports: %v", err)
	}
	if result.ConsumedNoChange != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if result.DeletedExports != 0 || result.FailedExportDeletes != 1 {
		t.Fatalf("unexpected delete counts: %+v", result)
	}
	if _, err := os.Stat(exportPath); err != nil {
		t.Fatalf("expected export file to remain after delete failure, stat err=%v", err)
	}

	state := cursorProStateForChannel(channelID)
	if state.LastImportResult != "consumed_no_change_delete_failed" {
		t.Fatalf("unexpected last import result: %s", state.LastImportResult)
	}
}
