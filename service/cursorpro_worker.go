package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

const defaultCursorProControlURL = "http://127.0.0.1:18765"

type cursorProExportFile struct {
	Filename   string `json:"filename"`
	Provider   string `json:"provider"`
	AccountID  string `json:"account_id"`
	Email      string `json:"email"`
	ExpiresAt  string `json:"expires_at"`
	Source     string `json:"source"`
	Status     string `json:"status"`
	ExportedAt string `json:"exported_at"`
	Raw        struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
	} `json:"raw"`
}

type CursorProImportResult struct {
	Imported int `json:"imported"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
	Total    int `json:"total"`
}

type CursorProReplacementResult struct {
	Triggered bool   `json:"triggered"`
	Reason    string `json:"reason"`
	Status    string `json:"status,omitempty"`
}

type cursorProRegisterStatus struct {
	TaskID        string `json:"task_id"`
	Status        string `json:"status"`
	CreatedCount  int    `json:"created_count"`
	UpdatedCount  int    `json:"updated_count"`
	ErrorCode     string `json:"error_code"`
	ErrorMessage  string `json:"error_message"`
	FinishedAt    string `json:"finished_at"`
	StartedAt     string `json:"started_at"`
	LastExportAt  string `json:"last_export_at"`
	LastExportCnt int    `json:"last_export_count"`
}

type cursorProTriggerState struct {
	RecentTriggerTimes []time.Time
	LastTriggerAt      time.Time
	CircuitOpenUntil   time.Time
	ConsecutiveNoYield int
	LastTaskID         string
	LastTaskFinishedAt string
	LastResultStatus   string
}

var cursorProTriggerStateMap sync.Map

func defaultCursorProCodexExportDirForGOOS(goos string, homeDir string, localAppData string) string {
	switch goos {
	case "windows":
		base := strings.TrimSpace(localAppData)
		if base == "" && strings.TrimSpace(homeDir) != "" {
			base = filepath.Join(homeDir, "AppData", "Local")
		}
		if base != "" {
			return filepath.Join(base, "CursorPro3", "exports", "codex")
		}
	case "darwin":
		if strings.TrimSpace(homeDir) != "" {
			return filepath.Join(homeDir, "Library", "Application Support", "CursorPro3", "exports", "codex")
		}
	default:
		if strings.TrimSpace(homeDir) != "" {
			return filepath.Join(homeDir, ".local", "share", "CursorPro3", "exports", "codex")
		}
	}
	return "CursorPro3/exports/codex"
}

func cursorProCodexExportDir() string {
	dir := strings.TrimSpace(os.Getenv("CURSORPRO_CODEX_EXPORT_DIR"))
	if dir == "" {
		homeDir, _ := os.UserHomeDir()
		dir = defaultCursorProCodexExportDirForGOOS(runtime.GOOS, homeDir, os.Getenv("LOCALAPPDATA"))
	} else if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	return dir
}

func cursorProControlBaseURL() string {
	base := strings.TrimSpace(os.Getenv("CURSORPRO_CONTROL_URL"))
	if base == "" {
		base = defaultCursorProControlURL
	}
	return strings.TrimRight(base, "/")
}

func cursorProStateForChannel(channelID int) *cursorProTriggerState {
	actual, _ := cursorProTriggerStateMap.LoadOrStore(channelID, &cursorProTriggerState{})
	return actual.(*cursorProTriggerState)
}

func filterRecentTimes(times []time.Time, cutoff time.Time) []time.Time {
	out := times[:0]
	for _, ts := range times {
		if ts.After(cutoff) {
			out = append(out, ts)
		}
	}
	return out
}

func readCursorProRegisterStatus(ctx context.Context) (*cursorProRegisterStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cursorProControlBaseURL()+"/v1/register/status", nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cursorpro register status failed: status=%d", resp.StatusCode)
	}
	var status cursorProRegisterStatus
	if err := common.DecodeJson(resp.Body, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func TriggerCursorProReplacement(ctx context.Context, channelID int, reason string) (*CursorProReplacementResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	state := cursorProStateForChannel(channelID)
	now := time.Now()
	state.RecentTriggerTimes = filterRecentTimes(state.RecentTriggerTimes, now.Add(-30*time.Minute))
	if !state.CircuitOpenUntil.IsZero() && state.CircuitOpenUntil.After(now) {
		return &CursorProReplacementResult{
			Triggered: false,
			Reason:    reason,
			Status:    "circuit_open",
		}, nil
	}
	if !state.LastTriggerAt.IsZero() && now.Sub(state.LastTriggerAt) < 3*time.Minute {
		return &CursorProReplacementResult{
			Triggered: false,
			Reason:    reason,
			Status:    "cooldown",
		}, nil
	}
	if len(state.RecentTriggerTimes) >= 5 {
		return &CursorProReplacementResult{
			Triggered: false,
			Reason:    reason,
			Status:    "rate_limited",
		}, nil
	}

	status, err := readCursorProRegisterStatus(ctx)
	if err == nil && status != nil && status.Status == "running" {
		return &CursorProReplacementResult{
			Triggered: false,
			Reason:    reason,
			Status:    "already_running",
		}, nil
	}

	triggerPayload := map[string]any{
		"channel_id": channelID,
		"reason":     reason,
	}
	body, _ := common.Marshal(triggerPayload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cursorProControlBaseURL()+"/v1/register/trigger", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusConflict {
		return nil, fmt.Errorf("cursorpro register trigger failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var statusPayload cursorProRegisterStatus
	_ = common.Unmarshal(respBody, &statusPayload)
	state.LastTriggerAt = now
	state.RecentTriggerTimes = append(state.RecentTriggerTimes, now)
	if statusPayload.TaskID != "" {
		state.LastTaskID = statusPayload.TaskID
	}
	return &CursorProReplacementResult{
		Triggered: resp.StatusCode == http.StatusAccepted,
		Reason:    reason,
		Status:    map[bool]string{true: "triggered", false: "already_running"}[resp.StatusCode == http.StatusAccepted],
	}, nil
}

func buildCodexOAuthKeyFromCursorProExport(item cursorProExportFile) (string, error) {
	key := CodexOAuthKey{
		IDToken:      strings.TrimSpace(item.Raw.IDToken),
		AccessToken:  strings.TrimSpace(item.Raw.AccessToken),
		RefreshToken: strings.TrimSpace(item.Raw.RefreshToken),
		AccountID:    strings.TrimSpace(item.AccountID),
		Email:        strings.TrimSpace(item.Email),
		Expired:      strings.TrimSpace(item.ExpiresAt),
		Type:         "codex",
	}
	if key.AccessToken == "" || key.AccountID == "" {
		return "", fmt.Errorf("cursorpro export missing access_token/account_id")
	}
	raw, err := common.Marshal(key)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func parseCursorProExportFile(path string) (*cursorProExportFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var item cursorProExportFile
	if err := common.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func upsertCursorProToken(channel *model.Channel, key string, item *cursorProExportFile) (bool, bool) {
	keys := channel.GetKeys()
	accountID := strings.TrimSpace(item.AccountID)
	email := strings.TrimSpace(item.Email)
	for i, existingKey := range keys {
		existingOAuth, err := parseCodexOAuthKey(strings.TrimSpace(existingKey))
		if err != nil {
			continue
		}
		if (accountID != "" && strings.TrimSpace(existingOAuth.AccountID) == accountID) ||
			(email != "" && strings.EqualFold(strings.TrimSpace(existingOAuth.Email), email)) {
			updated := existingKey != key
			keys[i] = key
			channel.Key = strings.Join(keys, "\n")
			meta := hydrateCodexKeyMeta(key, channel.GetKeyMeta(i))
			channel.SetKeyMeta(i, meta)
			return false, updated
		}
	}

	keys = append(keys, key)
	channel.Key = strings.Join(keys, "\n")
	idx := len(keys) - 1
	meta := hydrateCodexKeyMeta(key, channel.GetKeyMeta(idx))
	meta.State = model.CodexKeyStateNew
	meta.NewSuccessCount = 0
	meta.Consecutive429 = 0
	meta.Consecutive5xx = 0
	meta.ConsecutiveAuthFail = 0
	meta.SoftFailCount = 0
	meta.CooldownUntil = 0
	meta.LastSelectedAt = 0
	meta.LastSuccessAt = 0
	meta.LastErrorAt = 0
	if item != nil {
		if strings.TrimSpace(item.Source) != "" {
			meta.Source = strings.TrimSpace(item.Source)
		} else {
			meta.Source = "cursorpro3"
		}
		meta.AccountID = strings.TrimSpace(item.AccountID)
		meta.Email = strings.TrimSpace(item.Email)
		meta.ExpiresAt = strings.TrimSpace(item.ExpiresAt)
	}
	channel.SetKeyMeta(idx, meta)
	return true, false
}

func finalizeCodexMultiKeyChannel(channel *model.Channel) {
	keys := channel.GetKeys()
	channel.ChannelInfo.IsMultiKey = len(keys) > 1
	channel.ChannelInfo.MultiKeySize = len(keys)
	if channel.ChannelInfo.MultiKeyMode == "" {
		channel.ChannelInfo.MultiKeyMode = constant.MultiKeyModeRandom
	}
}

func ImportCursorProExports(ctx context.Context, channelID int) (*CursorProImportResult, error) {
	_ = ctx
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("channel not found")
	}
	if channel.Type != constant.ChannelTypeCodex {
		return nil, fmt.Errorf("channel type is not Codex")
	}

	exportDir := cursorProCodexExportDir()
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		return nil, err
	}

	lock := model.GetChannelPollingLock(channelID)
	lock.Lock()
	defer lock.Unlock()

	result := &CursorProImportResult{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		result.Total++
		item, err := parseCursorProExportFile(filepath.Join(exportDir, entry.Name()))
		if err != nil || item == nil {
			result.Skipped++
			continue
		}
		if strings.TrimSpace(item.Provider) != "" && !strings.EqualFold(strings.TrimSpace(item.Provider), "codex") {
			result.Skipped++
			continue
		}
		key, err := buildCodexOAuthKeyFromCursorProExport(*item)
		if err != nil {
			result.Skipped++
			continue
		}
		imported, updated := upsertCursorProToken(channel, key, item)
		if imported {
			result.Imported++
		} else if updated {
			result.Updated++
		} else {
			result.Skipped++
		}
	}

	finalizeCodexMultiKeyChannel(channel)
	if err := model.DB.Model(&model.Channel{}).
		Where("id = ?", channel.Id).
		Updates(map[string]any{
			"key":          channel.Key,
			"channel_info": channel.ChannelInfo,
		}).Error; err != nil {
		return nil, err
	}

	if common.MemoryCacheEnabled {
		model.InitChannelCache()
	}
	ResetProxyClientCache()
	return result, nil
}

func isCursorProAutoImportEnabled(channel *model.Channel) bool {
	if channel == nil || channel.Type != constant.ChannelTypeCodex {
		return false
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(channel.GetTag())), "cursorpro") {
		return true
	}
	info := channel.GetOtherInfo()
	if raw, ok := info["cursorpro_auto_import"]; ok {
		if enabled, ok := raw.(bool); ok {
			return enabled
		}
	}
	return false
}

func ListCursorProAutoImportChannelIDs() ([]int, error) {
	var channels []*model.Channel
	if err := model.DB.Select("id", "type", "tag", "other_info", "status").Where("type = ? AND status = ?", constant.ChannelTypeCodex, common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		if isCursorProAutoImportEnabled(channel) {
			ids = append(ids, channel.Id)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

func RunCursorProAutoImportOnce(ctx context.Context) {
	channelIDs, err := ListCursorProAutoImportChannelIDs()
	if err != nil {
		return
	}
	for _, channelID := range channelIDs {
		_, _ = ImportCursorProExports(ctx, channelID)
	}
}

func ReconcileCursorProReplacementState(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	status, err := readCursorProRegisterStatus(ctx)
	if err != nil || status == nil {
		return
	}
	now := time.Now()
	cursorProTriggerStateMap.Range(func(key, value any) bool {
		state, ok := value.(*cursorProTriggerState)
		if !ok || state == nil {
			return true
		}
		if status.TaskID == "" || status.TaskID == state.LastTaskID && status.FinishedAt == state.LastTaskFinishedAt {
			return true
		}
		if status.Status == "succeeded" || status.Status == "failed" {
			state.LastTaskID = status.TaskID
			state.LastTaskFinishedAt = status.FinishedAt
			state.LastResultStatus = status.Status
			if status.CreatedCount+status.UpdatedCount > 0 {
				state.ConsecutiveNoYield = 0
			} else {
				state.ConsecutiveNoYield++
			}
			if state.ConsecutiveNoYield >= 3 {
				state.CircuitOpenUntil = now.Add(30 * time.Minute)
			}
		}
		return true
	})
}

func StartCursorProAutoImportTask() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		RunCursorProAutoImportOnce(context.Background())
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			RunCursorProAutoImportOnce(context.Background())
		}
	}()
}
