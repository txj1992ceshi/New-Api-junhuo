package service

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

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

	_, imported, updated := upsertCursorProToken(channel, key, item)
	if !imported || updated {
		t.Fatalf("expected imported=true updated=false, got imported=%v updated=%v", imported, updated)
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
