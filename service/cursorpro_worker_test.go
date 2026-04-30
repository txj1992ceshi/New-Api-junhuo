package service

import (
	"path/filepath"
	"testing"

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

	imported, updated := upsertCursorProToken(channel, key, item)
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
