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

const (
	defaultCursorProControlURL  = "http://127.0.0.1:18765"
	cursorProResultCodeNoYield  = "register_no_yield"
	cursorProResultCodeControl  = "control_unreachable"
	cursorProBlockReasonNoYield = "recent_no_yield"
	cursorProManagedChannelID   = 2
	cursorProManagedPoolCap     = 100
)

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

type cursorProTokenStatus struct {
	SourceTokenCount         int     `json:"source_token_count"`
	SourceLatestFile         string  `json:"source_latest_file"`
	SourceLatestMtime        string  `json:"source_latest_mtime"`
	ExportTokenCount         int     `json:"export_token_count"`
	ExportLatestFile         string  `json:"export_latest_file"`
	ExportLatestMtime        string  `json:"export_latest_mtime"`
	LastSyncAt               string  `json:"last_sync_at"`
	LastSyncResult           string  `json:"last_sync_result"`
	LastSourceToExportReason string  `json:"last_source_to_export_reason"`
	SyncLagSeconds           float64 `json:"sync_lag_seconds"`
}

type cursorProTokenSyncResponse struct {
	Changed bool                  `json:"changed"`
	Forced  bool                  `json:"forced"`
	Reason  string                `json:"reason"`
	Result  string                `json:"result"`
	State   *cursorProTokenStatus `json:"state"`
}

type cursorProTriggerState struct {
	RecentTriggerTimes           []time.Time
	LastTriggerAt                time.Time
	LastTriggerReason            string
	CircuitOpenUntil             time.Time
	ConsecutiveNoYield           int
	LastTaskID                   string
	LastTaskFinishedAt           string
	LastResultStatus             string
	LastErrorCode                string
	LastErrorMessage             string
	LastExportCount              int
	LastExportName               string
	LastExportMtime              time.Time
	LastProbeAt                  time.Time
	LastProbeModel               string
	LastProbeResult              string
	LastImportAt                 time.Time
	LastImportResult             string
	LastImportImported           int
	LastImportUpdated            int
	LastImportSkipped            int
	LastImportTotal              int
	LastSuccessfulRecoveryAt     time.Time
	LastSuccessfulRecoveryReason string
	SourceQuietSince             time.Time
}

type cursorProCooldownDecision struct {
	Allowed                  bool
	BlockReason              string
	CooldownUntil            time.Time
	CooldownSecondsRemaining int
	CooldownBaseSeconds      int
	CooldownMode             string
	CooldownBreakAllowed     bool
	CooldownBreakReason      string
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

func isManagedCursorProChannel(channel *model.Channel) bool {
	return channel != nil && channel.Id == cursorProManagedChannelID
}

func managedCursorProPoolCapacity(channel *model.Channel) int {
	if isManagedCursorProChannel(channel) {
		return cursorProManagedPoolCap
	}
	return 0
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

type cursorProExportSnapshot struct {
	Count       int
	LatestName  string
	LatestMtime time.Time
}

func readCursorProExportSnapshot() cursorProExportSnapshot {
	dir := cursorProCodexExportDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return cursorProExportSnapshot{}
	}
	snapshot := cursorProExportSnapshot{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		snapshot.Count++
		info, err := entry.Info()
		if err != nil {
			continue
		}
		modTime := info.ModTime()
		if modTime.After(snapshot.LatestMtime) {
			snapshot.LatestMtime = modTime
			snapshot.LatestName = entry.Name()
		}
	}
	return snapshot
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

func readCursorProTokenStatus(ctx context.Context) (*cursorProTokenStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cursorProControlBaseURL()+"/v1/tokens/status", nil)
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
		return nil, fmt.Errorf("cursorpro token status failed: status=%d", resp.StatusCode)
	}
	var status cursorProTokenStatus
	if err := common.DecodeJson(resp.Body, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func SyncCursorProTokens(ctx context.Context, force bool, reason string) (*cursorProTokenSyncResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	payload := map[string]any{
		"force":  force,
		"reason": reason,
	}
	body, _ := common.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cursorProControlBaseURL()+"/v1/tokens/sync", bytes.NewBuffer(body))
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cursorpro token sync failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result cursorProTokenSyncResponse
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func sourceFreshWindow() time.Duration {
	return 2 * time.Minute
}

func cursorProSourceRecentlyUpdated(tokenStatus *cursorProTokenStatus, now time.Time) bool {
	if tokenStatus == nil {
		return false
	}
	sourceLatest := parseRFC3339TimeOrZero(tokenStatus.SourceLatestMtime)
	if sourceLatest.IsZero() {
		return false
	}
	return now.Sub(sourceLatest) <= sourceFreshWindow()
}

func updateCursorProSourceQuietSince(state *cursorProTriggerState, tokenStatus *cursorProTokenStatus, now time.Time) {
	if state == nil {
		return
	}
	if cursorProSourceRecentlyUpdated(tokenStatus, now) {
		state.SourceQuietSince = time.Time{}
		return
	}
	if state.SourceQuietSince.IsZero() {
		state.SourceQuietSince = now
	}
}

func recordCursorProSuccessfulRecovery(state *cursorProTriggerState, when time.Time, reason string) {
	if state == nil {
		return
	}
	state.LastSuccessfulRecoveryAt = when
	state.LastSuccessfulRecoveryReason = reason
}

func loadCursorProCooldownContext(channelID int, now time.Time) (*CodexPoolHealth, int, int) {
	if channelID <= 0 {
		return nil, 0, 0
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil || channel == nil || channel.Type != constant.ChannelTypeCodex {
		return nil, RecentCodexNoAvailableCount(channelID, now), RecentCodexHotPathTriggerCount(channelID, now)
	}
	return ComputeCodexPoolHealth(channel, now), RecentCodexNoAvailableCount(channelID, now), RecentCodexHotPathTriggerCount(channelID, now)
}

func cursorProReplacementModeFromChannelID(channelID int) string {
	if channelID <= 0 {
		return cursorProReplacementModePoolHealth
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return cursorProReplacementModePoolHealth
	}
	return cursorProReplacementMode(channel)
}

func hasUsableRecoverySinceTrigger(state *cursorProTriggerState, health *CodexPoolHealth, triggerAt time.Time) bool {
	if state == nil || triggerAt.IsZero() {
		return false
	}
	if !state.LastSuccessfulRecoveryAt.IsZero() && !state.LastSuccessfulRecoveryAt.Before(triggerAt) {
		return true
	}
	if state.LastProbeResult == "probe_succeeded" && !state.LastProbeAt.IsZero() && !state.LastProbeAt.Before(triggerAt) {
		return true
	}
	return health != nil && health.AvailableCount > 0 && !state.LastSuccessfulRecoveryAt.IsZero() && !state.LastSuccessfulRecoveryAt.Before(triggerAt)
}

func deriveCursorProCooldownBaseSeconds(state *cursorProTriggerState, registerStatus *cursorProRegisterStatus, health *CodexPoolHealth) int {
	return deriveCursorProCooldownBaseSecondsWithMode(state, registerStatus, health, cursorProReplacementModePoolHealth)
}

func deriveCursorProCooldownBaseSecondsWithMode(state *cursorProTriggerState, registerStatus *cursorProRegisterStatus, health *CodexPoolHealth, mode string) int {
	if state == nil || state.LastTriggerAt.IsZero() {
		return 0
	}
	if mode == cursorProReplacementModeNearExhausted {
		if state.LastResultStatus == "failed" || (registerStatus != nil && registerStatus.Status == "failed") {
			return 300
		}
		if hasUsableRecoverySinceTrigger(state, health, state.LastTriggerAt) {
			return 120
		}
		return 180
	}
	if state.LastResultStatus == "failed" || (registerStatus != nil && registerStatus.Status == "failed") {
		return 180
	}
	if hasUsableRecoverySinceTrigger(state, health, state.LastTriggerAt) {
		return 45
	}
	return 90
}

func cursorProNoYieldCooldownSeconds(mode string) int {
	if mode == cursorProReplacementModeNearExhausted {
		return 300
	}
	return 600
}

func hasCursorProYieldSinceTrigger(state *cursorProTriggerState, tokenStatus *cursorProTokenStatus) bool {
	if state == nil || state.LastTriggerAt.IsZero() {
		return false
	}
	triggerAt := state.LastTriggerAt
	if !state.LastSuccessfulRecoveryAt.IsZero() && state.LastSuccessfulRecoveryAt.After(triggerAt) {
		return true
	}
	if !state.LastImportAt.IsZero() && state.LastImportAt.After(triggerAt) {
		return true
	}
	if !state.LastExportMtime.IsZero() && state.LastExportMtime.After(triggerAt) {
		return true
	}
	if tokenStatus == nil {
		return false
	}
	sourceLatest := parseRFC3339TimeOrZero(tokenStatus.SourceLatestMtime)
	if !sourceLatest.IsZero() && sourceLatest.After(triggerAt) {
		return true
	}
	exportLatest := parseRFC3339TimeOrZero(tokenStatus.ExportLatestMtime)
	if !exportLatest.IsZero() && exportLatest.After(triggerAt) {
		return true
	}
	return false
}

func deriveCursorProCooldownBreakReason(health *CodexPoolHealth, recentNoAvailable int, recentHotPath int) string {
	return deriveCursorProCooldownBreakReasonWithMode(health, recentNoAvailable, recentHotPath, cursorProReplacementModePoolHealth)
}

func deriveCursorProCooldownBreakReasonWithMode(health *CodexPoolHealth, recentNoAvailable int, recentHotPath int, mode string) string {
	if health != nil && (health.AvailableCount <= 0 || health.Healthy+health.New <= 0) {
		return "cooldown_break_available_count_zero"
	}
	if mode == cursorProReplacementModeNearExhausted {
		return ""
	}
	if recentNoAvailable >= 3 {
		return "cooldown_break_no_available_spike"
	}
	if recentHotPath >= 2 {
		return "cooldown_break_rate_limit_spike"
	}
	return ""
}

func evaluateCursorProTriggerCooldown(
	state *cursorProTriggerState,
	registerStatus *cursorProRegisterStatus,
	tokenStatus *cursorProTokenStatus,
	health *CodexPoolHealth,
	recentNoAvailable int,
	recentHotPath int,
	now time.Time,
) cursorProCooldownDecision {
	return evaluateCursorProTriggerCooldownWithMode(state, registerStatus, tokenStatus, health, recentNoAvailable, recentHotPath, now, cursorProReplacementModePoolHealth)
}

func evaluateCursorProTriggerCooldownWithMode(
	state *cursorProTriggerState,
	registerStatus *cursorProRegisterStatus,
	tokenStatus *cursorProTokenStatus,
	health *CodexPoolHealth,
	recentNoAvailable int,
	recentHotPath int,
	now time.Time,
	mode string,
) cursorProCooldownDecision {
	decision := cursorProCooldownDecision{
		Allowed:      true,
		CooldownMode: "result_aware",
	}
	if mode == cursorProReplacementModeNearExhausted {
		decision.CooldownMode = "near_exhausted_result_aware"
	}
	if state == nil {
		return decision
	}
	state.RecentTriggerTimes = filterRecentTimes(state.RecentTriggerTimes, now.Add(-30*time.Minute))
	if registerStatus != nil && registerStatus.Status == "running" {
		decision.Allowed = false
		decision.BlockReason = "already_running"
		return decision
	}
	if !state.CircuitOpenUntil.IsZero() && state.CircuitOpenUntil.After(now) {
		decision.Allowed = false
		decision.BlockReason = "circuit_open"
		decision.CooldownUntil = state.CircuitOpenUntil
		return decision
	}
	if len(state.RecentTriggerTimes) >= 5 {
		decision.Allowed = false
		decision.BlockReason = "rate_limited"
		return decision
	}
	if state.LastResultStatus == "failed" && state.LastErrorCode == cursorProResultCodeNoYield && !state.LastTriggerAt.IsZero() && !hasCursorProYieldSinceTrigger(state, tokenStatus) {
		baseSeconds := cursorProNoYieldCooldownSeconds(mode)
		decision.CooldownBaseSeconds = baseSeconds
		decision.CooldownUntil = state.LastTriggerAt.Add(time.Duration(baseSeconds) * time.Second)
		if decision.CooldownUntil.After(now) {
			decision.Allowed = false
			decision.BlockReason = cursorProBlockReasonNoYield
			decision.CooldownMode = "no_yield_backoff"
			decision.CooldownSecondsRemaining = int(decision.CooldownUntil.Sub(now).Seconds())
			if decision.CooldownSecondsRemaining < 0 {
				decision.CooldownSecondsRemaining = 0
			}
			return decision
		}
	}
	baseSeconds := deriveCursorProCooldownBaseSecondsWithMode(state, registerStatus, health, mode)
	decision.CooldownBaseSeconds = baseSeconds
	if state.LastTriggerAt.IsZero() || baseSeconds <= 0 {
		return decision
	}
	decision.CooldownUntil = state.LastTriggerAt.Add(time.Duration(baseSeconds) * time.Second)
	if !decision.CooldownUntil.After(now) {
		return decision
	}
	breakReason := deriveCursorProCooldownBreakReasonWithMode(health, recentNoAvailable, recentHotPath, mode)
	if breakReason != "" {
		decision.CooldownBreakAllowed = true
		decision.CooldownBreakReason = breakReason
		decision.CooldownMode = "broken_by_pool_critical"
		return decision
	}
	decision.Allowed = false
	decision.BlockReason = "cooldown"
	decision.CooldownSecondsRemaining = int(decision.CooldownUntil.Sub(now).Seconds())
	if decision.CooldownSecondsRemaining < 0 {
		decision.CooldownSecondsRemaining = 0
	}
	return decision
}

func TriggerCursorProReplacement(ctx context.Context, channelID int, reason string) (*CursorProReplacementResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	state := cursorProStateForChannel(channelID)
	now := time.Now()
	tokenStatus, tokenStatusErr := readCursorProTokenStatus(ctx)
	registerStatus, registerStatusErr := readCursorProRegisterStatus(ctx)
	if registerStatusErr != nil && tokenStatusErr != nil && state != nil {
		state.LastResultStatus = "failed"
		state.LastErrorCode = cursorProResultCodeControl
		state.LastErrorMessage = "CursorPro control service is unreachable."
	}
	health, recentNoAvailable, recentHotPath := loadCursorProCooldownContext(channelID, now)
	mode := cursorProReplacementModeFromChannelID(channelID)
	updateCursorProSourceQuietSince(state, tokenStatus, now)
	cooldownDecision := evaluateCursorProTriggerCooldownWithMode(state, registerStatus, tokenStatus, health, recentNoAvailable, recentHotPath, now, mode)
	if !cooldownDecision.Allowed {
		return &CursorProReplacementResult{
			Triggered: false,
			Reason:    reason,
			Status:    cooldownDecision.BlockReason,
		}, nil
	}
	if cursorProSourceRecentlyUpdated(tokenStatus, now) {
		return &CursorProReplacementResult{
			Triggered: false,
			Reason:    reason,
			Status:    "trigger_skipped_recent_source_update",
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
	exportSnapshot := readCursorProExportSnapshot()
	state.LastTriggerAt = now
	state.LastTriggerReason = reason
	state.RecentTriggerTimes = append(state.RecentTriggerTimes, now)
	if statusPayload.TaskID != "" {
		state.LastTaskID = statusPayload.TaskID
	}
	state.LastExportCount = exportSnapshot.Count
	state.LastExportName = exportSnapshot.LatestName
	state.LastExportMtime = exportSnapshot.LatestMtime
	return &CursorProReplacementResult{
		Triggered: resp.StatusCode == http.StatusAccepted,
		Reason:    reason,
		Status:    map[bool]string{true: "triggered", false: "already_running"}[resp.StatusCode == http.StatusAccepted],
	}, nil
}

func buildCodexOAuthKeyFromCursorProExport(item cursorProExportFile) (string, error) {
	accountID := strings.TrimSpace(item.AccountID)
	email := strings.TrimSpace(item.Email)
	accessToken := strings.TrimSpace(item.Raw.AccessToken)
	idToken := strings.TrimSpace(item.Raw.IDToken)
	if accountID == "" {
		if v, ok := ExtractCodexAccountIDFromJWT(accessToken); ok {
			accountID = v
		} else if v, ok := ExtractCodexAccountIDFromJWT(idToken); ok {
			accountID = v
		}
	}
	if email == "" {
		if v, ok := ExtractEmailFromJWT(accessToken); ok {
			email = v
		} else if v, ok := ExtractEmailFromJWT(idToken); ok {
			email = v
		}
	}
	key := CodexOAuthKey{
		IDToken:      idToken,
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(item.Raw.RefreshToken),
		AccountID:    accountID,
		Email:        email,
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

type cursorProUpsertResult struct {
	Index        int
	Imported     bool
	Updated      bool
	Replaced     bool
	CapacityFull bool
}

func resetImportedCursorProMeta(meta model.ChannelKeyMeta, item *cursorProExportFile) model.ChannelKeyMeta {
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
	meta.LastErrorKind = ""
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
	return meta
}

func replaceCursorProTokenAtIndex(channel *model.Channel, index int, key string, item *cursorProExportFile) cursorProUpsertResult {
	keys := channel.GetKeys()
	keys[index] = key
	channel.Key = strings.Join(keys, "\n")
	meta := hydrateCodexKeyMeta(key, channel.GetKeyMeta(index))
	meta = resetImportedCursorProMeta(meta, item)
	channel.SetKeyMeta(index, meta)
	return cursorProUpsertResult{
		Index:    index,
		Imported: true,
		Replaced: true,
	}
}

func cursorProDeadReplacePriority(meta model.ChannelKeyMeta) (int, bool) {
	if meta.State != model.CodexKeyStateDead {
		return 0, false
	}
	lastError := strings.ToLower(strings.TrimSpace(meta.LastErrorKind))
	switch {
	case lastError == "rate_limit_exhausted":
		return 1, true
	case strings.HasPrefix(lastError, "invalid"):
		return 2, true
	case lastError == "rate_limit":
		return 3, true
	default:
		return 0, false
	}
}

func findReplaceableCursorProDeadSlot(channel *model.Channel) int {
	if channel == nil {
		return -1
	}
	keys := channel.GetKeys()
	type candidate struct {
		index       int
		priority    int
		lastErrorAt int64
	}
	var best *candidate
	for i := range keys {
		meta := channel.GetKeyMeta(i)
		priority, ok := cursorProDeadReplacePriority(meta)
		if !ok {
			continue
		}
		current := candidate{
			index:       i,
			priority:    priority,
			lastErrorAt: meta.LastErrorAt,
		}
		if best == nil ||
			current.priority < best.priority ||
			(current.priority == best.priority && current.lastErrorAt < best.lastErrorAt) ||
			(current.priority == best.priority && current.lastErrorAt == best.lastErrorAt && current.index < best.index) {
			tmp := current
			best = &tmp
		}
	}
	if best == nil {
		return -1
	}
	return best.index
}

func upsertCursorProToken(channel *model.Channel, key string, item *cursorProExportFile) cursorProUpsertResult {
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
			if updated {
				meta = resetImportedCursorProMeta(meta, item)
			}
			channel.SetKeyMeta(i, meta)
			return cursorProUpsertResult{
				Index:   i,
				Updated: updated,
			}
		}
	}

	if replaceIndex := findReplaceableCursorProDeadSlot(channel); replaceIndex >= 0 {
		return replaceCursorProTokenAtIndex(channel, replaceIndex, key, item)
	}

	if capacity := managedCursorProPoolCapacity(channel); capacity > 0 && len(keys) >= capacity {
		return cursorProUpsertResult{
			Index:        -1,
			CapacityFull: true,
		}
	}

	keys = append(keys, key)
	channel.Key = strings.Join(keys, "\n")
	idx := len(keys) - 1
	meta := hydrateCodexKeyMeta(key, channel.GetKeyMeta(idx))
	meta = resetImportedCursorProMeta(meta, item)
	channel.SetKeyMeta(idx, meta)
	return cursorProUpsertResult{
		Index:    idx,
		Imported: true,
	}
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
	if ctx == nil {
		ctx = context.Background()
	}
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

	_, _ = SyncCursorProTokens(ctx, false, "new_api_import")

	exportDir := cursorProCodexExportDir()
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		return nil, err
	}

	lock := model.GetChannelPollingLock(channelID)
	lock.Lock()
	defer lock.Unlock()

	result := &CursorProImportResult{}
	importedIndexes := make([]int, 0)
	replacedCount := 0
	capacityFullCount := 0
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
		upsert := upsertCursorProToken(channel, key, item)
		if upsert.CapacityFull {
			capacityFullCount++
			result.Skipped++
		} else if upsert.Imported {
			result.Imported++
			if upsert.Replaced {
				replacedCount++
			}
			importedIndexes = append(importedIndexes, upsert.Index)
		} else if upsert.Updated {
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
	for _, index := range importedIndexes {
		EnqueueCodexNewKeyProbe(channel.Id, index, "probe_pending")
	}
	state := cursorProStateForChannel(channelID)
	if state != nil {
		state.LastImportAt = time.Now()
		state.LastImportImported = result.Imported
		state.LastImportUpdated = result.Updated
		state.LastImportSkipped = result.Skipped
		state.LastImportTotal = result.Total
		switch {
		case replacedCount > 0:
			state.LastImportResult = "replaced_dead_tokens"
			recordCursorProSuccessfulRecovery(state, state.LastImportAt, state.LastImportResult)
		case result.Imported > 0:
			state.LastImportResult = "imported_to_channel"
			recordCursorProSuccessfulRecovery(state, state.LastImportAt, state.LastImportResult)
		case result.Updated > 0:
			state.LastImportResult = "updated_existing_tokens"
			recordCursorProSuccessfulRecovery(state, state.LastImportAt, state.LastImportResult)
		case capacityFullCount > 0:
			state.LastImportResult = "capacity_full_no_replacement"
		case result.Total > 0:
			state.LastImportResult = "no_new_tokens_imported"
		default:
			state.LastImportResult = "no_export_files"
		}
	}
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
	_, _ = SyncCursorProTokens(ctx, false, "auto_import_tick")
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
			exportSnapshot := readCursorProExportSnapshot()
			applyCursorProRegisterOutcome(state, status, exportSnapshot, now)
		}
		return true
	})
}

func applyCursorProRegisterOutcome(state *cursorProTriggerState, status *cursorProRegisterStatus, exportSnapshot cursorProExportSnapshot, now time.Time) {
	if state == nil || status == nil {
		return
	}
	baselineKnown := state.LastExportCount > 0 || state.LastExportName != "" || !state.LastExportMtime.IsZero()
	exportYielded := false
	if baselineKnown {
		exportYielded = exportSnapshot.Count > state.LastExportCount
		if !exportYielded && !exportSnapshot.LatestMtime.IsZero() && exportSnapshot.LatestMtime.After(state.LastExportMtime) {
			exportYielded = true
		}
		if !exportYielded && exportSnapshot.LatestName != "" && exportSnapshot.LatestName != state.LastExportName {
			exportYielded = true
		}
	}
	state.LastTaskID = status.TaskID
	state.LastTaskFinishedAt = status.FinishedAt
	state.LastResultStatus = status.Status
	state.LastErrorCode = status.ErrorCode
	state.LastErrorMessage = status.ErrorMessage
	if exportYielded && status.Status == "failed" && status.ErrorCode == "register_timeout" {
		state.LastResultStatus = "succeeded"
		state.LastErrorCode = "export_detected_after_timeout"
		state.LastErrorMessage = "Detected new CursorPro export files after timeout; replacement produced new tokens."
	} else if !exportYielded && status.Status == "failed" && status.ErrorCode == "register_timeout" {
		state.LastResultStatus = "failed"
		state.LastErrorCode = cursorProResultCodeNoYield
		state.LastErrorMessage = "Register trigger completed without new source/export tokens."
	}
	if status.CreatedCount+status.UpdatedCount > 0 || exportYielded {
		state.ConsecutiveNoYield = 0
		recordCursorProSuccessfulRecovery(state, now, "source_sync_succeeded")
	} else {
		state.ConsecutiveNoYield++
	}
	state.LastExportCount = exportSnapshot.Count
	state.LastExportName = exportSnapshot.LatestName
	state.LastExportMtime = exportSnapshot.LatestMtime
	if state.ConsecutiveNoYield >= 3 {
		state.CircuitOpenUntil = now.Add(30 * time.Minute)
	}
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
