package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type CodexPoolHealthStatus struct {
	ChannelID                    int                        `json:"channel_id"`
	ChannelName                  string                     `json:"channel_name"`
	AutoImportEnabled            bool                       `json:"auto_import_enabled"`
	Health                       *CodexPoolHealth           `json:"health"`
	RecentNoAvailable5m          int                        `json:"recent_no_available_5m"`
	RecentHotPathTriggers        int                        `json:"recent_hot_path_triggers_5m"`
	RecentInvalidKey5m           int                        `json:"recent_invalid_key_5m"`
	RecentProbeSuccess5m         int                        `json:"recent_probe_success_5m"`
	RecentProbeFail5m            int                        `json:"recent_probe_fail_5m"`
	PendingProbeCount            int                        `json:"pending_probe_count"`
	LastProbeModel               string                     `json:"last_probe_model,omitempty"`
	LastProbeResult              string                     `json:"last_probe_result,omitempty"`
	TriggerRecommended           bool                       `json:"trigger_recommended"`
	TriggerReason                string                     `json:"trigger_reason,omitempty"`
	MinHealthyWatermark          int                        `json:"min_healthy_watermark"`
	CursorProExportDir           string                     `json:"cursorpro_export_dir"`
	CodexKeyStateCounters        map[string]int             `json:"key_state_counters"`
	TokenStatus                  *cursorProTokenStatus      `json:"token_status,omitempty"`
	TokenStatusError             string                     `json:"token_status_error,omitempty"`
	LastImportAt                 *time.Time                 `json:"last_import_at,omitempty"`
	LastImportResult             string                     `json:"last_import_result,omitempty"`
	SyncDiagnosis                string                     `json:"sync_diagnosis,omitempty"`
	PoolRiskLevel                string                     `json:"pool_risk_level,omitempty"`
	SourceQuietSince             *time.Time                 `json:"source_quiet_since,omitempty"`
	LastSuccessfulRecoveryAt     *time.Time                 `json:"last_successful_recovery_at,omitempty"`
	LastSuccessfulRecoveryReason string                     `json:"last_successful_recovery_reason,omitempty"`
	RecoveryTimeline             *CursorProRecoveryTimeline `json:"recovery_timeline,omitempty"`
	CooldownUntil                *time.Time                 `json:"cooldown_until,omitempty"`
	CooldownSecondsRemaining     int                        `json:"cooldown_seconds_remaining,omitempty"`
	CooldownBaseSeconds          int                        `json:"cooldown_base_seconds,omitempty"`
	CooldownMode                 string                     `json:"cooldown_mode,omitempty"`
	CooldownBreakAllowed         bool                       `json:"cooldown_break_allowed"`
	CooldownBreakReason          string                     `json:"cooldown_break_reason,omitempty"`
}

type CursorProReplacementStatusView struct {
	ChannelID                    int                        `json:"channel_id"`
	ChannelName                  string                     `json:"channel_name"`
	AutoImportEnabled            bool                       `json:"auto_import_enabled"`
	TriggerAllowed               bool                       `json:"trigger_allowed"`
	BlockReason                  string                     `json:"block_reason,omitempty"`
	LastTriggerReason            string                     `json:"last_trigger_reason,omitempty"`
	LastTriggerAt                *time.Time                 `json:"last_trigger_at,omitempty"`
	RecentTriggers30m            int                        `json:"recent_triggers_30m"`
	CircuitOpenUntil             *time.Time                 `json:"circuit_open_until,omitempty"`
	ConsecutiveNoYield           int                        `json:"consecutive_no_yield"`
	LastTaskID                   string                     `json:"last_task_id,omitempty"`
	LastTaskFinishedAt           string                     `json:"last_task_finished_at,omitempty"`
	LastResultStatus             string                     `json:"last_result_status,omitempty"`
	LastResultCode               string                     `json:"last_result_code,omitempty"`
	LastResultMessage            string                     `json:"last_result_message,omitempty"`
	ControlBaseURL               string                     `json:"control_base_url"`
	RegisterStatus               *cursorProRegisterStatus   `json:"register_status,omitempty"`
	RegisterStatusError          string                     `json:"register_status_error,omitempty"`
	Health                       *CodexPoolHealth           `json:"health,omitempty"`
	RecentNoAvailable5m          int                        `json:"recent_no_available_5m,omitempty"`
	RecentHotPathTriggers        int                        `json:"recent_hot_path_triggers_5m,omitempty"`
	RecentInvalidKey5m           int                        `json:"recent_invalid_key_5m"`
	RecentProbeSuccess5m         int                        `json:"recent_probe_success_5m"`
	RecentProbeFail5m            int                        `json:"recent_probe_fail_5m"`
	PendingProbeCount            int                        `json:"pending_probe_count"`
	LastProbeModel               string                     `json:"last_probe_model,omitempty"`
	LastProbeResult              string                     `json:"last_probe_result,omitempty"`
	TriggerRecommended           bool                       `json:"trigger_recommended"`
	TriggerReason                string                     `json:"trigger_reason,omitempty"`
	MinTriggerIntervalSec        int                        `json:"min_trigger_interval_sec"`
	MaxTriggersPer30Min          int                        `json:"max_triggers_per_30m"`
	OpenCircuitAfterNoYield      int                        `json:"open_circuit_after_no_yield"`
	TokenStatus                  *cursorProTokenStatus      `json:"token_status,omitempty"`
	TokenStatusError             string                     `json:"token_status_error,omitempty"`
	LastImportAt                 *time.Time                 `json:"last_import_at,omitempty"`
	LastImportResult             string                     `json:"last_import_result,omitempty"`
	SyncDiagnosis                string                     `json:"sync_diagnosis,omitempty"`
	TriggerResult                string                     `json:"trigger_result,omitempty"`
	RecoveryResult               string                     `json:"recovery_result,omitempty"`
	PoolRiskLevel                string                     `json:"pool_risk_level,omitempty"`
	SourceQuietSince             *time.Time                 `json:"source_quiet_since,omitempty"`
	LastSuccessfulRecoveryAt     *time.Time                 `json:"last_successful_recovery_at,omitempty"`
	LastSuccessfulRecoveryReason string                     `json:"last_successful_recovery_reason,omitempty"`
	RecoveryTimeline             *CursorProRecoveryTimeline `json:"recovery_timeline,omitempty"`
	CooldownUntil                *time.Time                 `json:"cooldown_until,omitempty"`
	CooldownSecondsRemaining     int                        `json:"cooldown_seconds_remaining,omitempty"`
	CooldownBaseSeconds          int                        `json:"cooldown_base_seconds,omitempty"`
	CooldownMode                 string                     `json:"cooldown_mode,omitempty"`
	CooldownBreakAllowed         bool                       `json:"cooldown_break_allowed"`
	CooldownBreakReason          string                     `json:"cooldown_break_reason,omitempty"`
}

type CursorProRecoveryTimeline struct {
	TriggerAt         *time.Time `json:"trigger_at,omitempty"`
	SourceDetectedAt  *time.Time `json:"source_detected_at,omitempty"`
	ExportWrittenAt   *time.Time `json:"export_written_at,omitempty"`
	ImportedAt        *time.Time `json:"imported_at,omitempty"`
	ProbedAt          *time.Time `json:"probed_at,omitempty"`
	CurrentStage      string     `json:"current_stage,omitempty"`
	TriggerToSourceMs *int64     `json:"trigger_to_source_ms,omitempty"`
	SourceToExportMs  *int64     `json:"source_to_export_ms,omitempty"`
	ExportToImportMs  *int64     `json:"export_to_import_ms,omitempty"`
	ImportToProbeMs   *int64     `json:"import_to_probe_ms,omitempty"`
	EndToEndMs        *int64     `json:"end_to_end_ms,omitempty"`
}

func codexMinHealthyWatermark(channel *model.Channel) int {
	minWatermark := 5
	if channel == nil {
		return minWatermark
	}
	info := channel.GetOtherInfo()
	if raw, ok := info["cursorpro_min_healthy_watermark"]; ok {
		if v, ok := raw.(float64); ok && int(v) > 0 {
			minWatermark = int(v)
		}
	}
	return minWatermark
}

func GetCodexPoolHealthStatus(channelID int, now time.Time) (*CodexPoolHealthStatus, error) {
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

	health := ComputeCodexPoolHealth(channel, now)
	recentNoAvailable := RecentCodexNoAvailableCount(channelID, now)
	recentHotPathTriggers := RecentCodexHotPathTriggerCount(channelID, now)
	recentInvalidKeys := RecentCodexInvalidKeyCount(channelID, now)
	recentProbeSuccess := RecentCodexProbeSuccessCount(channelID, now)
	recentProbeFail := RecentCodexProbeFailCount(channelID, now)
	triggerRecommended, triggerReason := ShouldTriggerCursorProReplacement(channel, health, now)
	var (
		tokenStatus    *cursorProTokenStatus
		tokenStatusErr string
	)
	tokenStatus, err = readCursorProTokenStatus(context.Background())
	if err != nil {
		tokenStatusErr = err.Error()
	}
	snapshot := getCursorProTriggerSnapshot(channelID, now)
	stateCounters := map[string]int{
		string(model.CodexKeyStateHealthy):    health.Healthy,
		string(model.CodexKeyStateNew):        health.New,
		string(model.CodexKeyStateCooldown):   health.Cooldown,
		string(model.CodexKeyStateSuspect):    health.Suspect,
		string(model.CodexKeyStateDead):       health.Dead,
		string(model.CodexKeyStateRefreshing): health.Refreshing,
	}

	return &CodexPoolHealthStatus{
		ChannelID:                    channel.Id,
		ChannelName:                  channel.Name,
		AutoImportEnabled:            isCursorProAutoImportEnabled(channel),
		Health:                       health,
		RecentNoAvailable5m:          recentNoAvailable,
		RecentHotPathTriggers:        recentHotPathTriggers,
		RecentInvalidKey5m:           recentInvalidKeys,
		RecentProbeSuccess5m:         recentProbeSuccess,
		RecentProbeFail5m:            recentProbeFail,
		PendingProbeCount:            PendingCodexProbeCount(channelID),
		LastProbeModel:               snapshot.LastProbeModel,
		LastProbeResult:              snapshot.LastProbeResult,
		TriggerRecommended:           triggerRecommended,
		TriggerReason:                triggerReason,
		MinHealthyWatermark:          codexMinHealthyWatermark(channel),
		CursorProExportDir:           cursorProCodexExportDir(),
		CodexKeyStateCounters:        stateCounters,
		TokenStatus:                  tokenStatus,
		TokenStatusError:             tokenStatusErr,
		LastImportAt:                 lastImportAtPtr(snapshot),
		LastImportResult:             snapshot.LastImportResult,
		SyncDiagnosis:                diagnoseCursorProSync(tokenStatus, snapshot, health),
		PoolRiskLevel:                derivePoolRiskLevel(tokenStatus, snapshot, health, now),
		SourceQuietSince:             timePtrIfNonZero(snapshot.SourceQuietSince),
		LastSuccessfulRecoveryAt:     timePtrIfNonZero(snapshot.LastSuccessfulRecoveryAt),
		LastSuccessfulRecoveryReason: snapshot.LastSuccessfulRecoveryReason,
		RecoveryTimeline:             buildCursorProRecoveryTimeline(snapshot, nil, tokenStatus),
	}, nil
}

func lastImportAtPtr(state *cursorProTriggerState) *time.Time {
	if state == nil || state.LastImportAt.IsZero() {
		return nil
	}
	at := state.LastImportAt
	return &at
}

func timePtrIfNonZero(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	v := value
	return &v
}

func parseRFC3339TimeOrZero(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func durationMillisPtr(start time.Time, end time.Time) *int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil
	}
	value := end.Sub(start).Milliseconds()
	return &value
}

func chooseTimelineTriggerAt(state *cursorProTriggerState, registerStatus *cursorProRegisterStatus) time.Time {
	candidate := time.Time{}
	if state != nil && !state.LastTriggerAt.IsZero() {
		candidate = state.LastTriggerAt
	}
	if registerStatus != nil {
		startedAt := parseRFC3339TimeOrZero(registerStatus.StartedAt)
		if !startedAt.IsZero() && startedAt.After(candidate) {
			candidate = startedAt
		}
	}
	return candidate
}

func buildCursorProRecoveryTimeline(state *cursorProTriggerState, registerStatus *cursorProRegisterStatus, tokenStatus *cursorProTokenStatus) *CursorProRecoveryTimeline {
	if state == nil {
		state = &cursorProTriggerState{}
	}
	triggerAt := chooseTimelineTriggerAt(state, registerStatus)
	sourceAt := parseRFC3339TimeOrZero("")
	exportAt := parseRFC3339TimeOrZero("")
	if tokenStatus != nil {
		sourceAt = parseRFC3339TimeOrZero(tokenStatus.SourceLatestMtime)
		exportAt = parseRFC3339TimeOrZero(tokenStatus.ExportLatestMtime)
	}
	importAt := state.LastImportAt
	probeAt := state.LastProbeAt

	// Filter out obviously stale stages from older rounds.
	if !triggerAt.IsZero() {
		if !sourceAt.IsZero() && sourceAt.Before(triggerAt) {
			sourceAt = time.Time{}
		}
		if !exportAt.IsZero() && exportAt.Before(triggerAt) {
			exportAt = time.Time{}
		}
		if !importAt.IsZero() && importAt.Before(triggerAt) {
			importAt = time.Time{}
		}
		if !probeAt.IsZero() && probeAt.Before(triggerAt) {
			probeAt = time.Time{}
		}
	}

	currentStage := ""
	switch {
	case !probeAt.IsZero():
		currentStage = "probed"
	case !importAt.IsZero():
		currentStage = "imported"
	case !exportAt.IsZero():
		currentStage = "exported"
	case !sourceAt.IsZero():
		currentStage = "source_detected"
	case !triggerAt.IsZero():
		currentStage = "triggered"
	}
	if currentStage == "" {
		return nil
	}

	endAt := probeAt
	if endAt.IsZero() {
		endAt = importAt
	}
	if endAt.IsZero() {
		endAt = exportAt
	}
	if endAt.IsZero() {
		endAt = sourceAt
	}

	return &CursorProRecoveryTimeline{
		TriggerAt:         timePtrIfNonZero(triggerAt),
		SourceDetectedAt:  timePtrIfNonZero(sourceAt),
		ExportWrittenAt:   timePtrIfNonZero(exportAt),
		ImportedAt:        timePtrIfNonZero(importAt),
		ProbedAt:          timePtrIfNonZero(probeAt),
		CurrentStage:      currentStage,
		TriggerToSourceMs: durationMillisPtr(triggerAt, sourceAt),
		SourceToExportMs:  durationMillisPtr(sourceAt, exportAt),
		ExportToImportMs:  durationMillisPtr(exportAt, importAt),
		ImportToProbeMs:   durationMillisPtr(importAt, probeAt),
		EndToEndMs:        durationMillisPtr(triggerAt, endAt),
	}
}

func diagnoseCursorProSync(tokenStatus *cursorProTokenStatus, state *cursorProTriggerState, health *CodexPoolHealth) string {
	if state == nil {
		state = &cursorProTriggerState{}
	}
	if tokenStatus == nil {
		if state.LastResultStatus == "failed" && state.LastErrorCode == "register_trigger_failed" {
			return "register_gui_blocked"
		}
		return ""
	}

	sourceLatest := parseRFC3339TimeOrZero(tokenStatus.SourceLatestMtime)
	exportLatest := parseRFC3339TimeOrZero(tokenStatus.ExportLatestMtime)

	switch {
	case state.LastResultStatus == "failed" && state.LastErrorCode == "register_trigger_failed" && state.LastProbeResult == "probe_succeeded":
		return "trigger_failed_but_sync_ok"
	case !sourceLatest.IsZero() && (exportLatest.IsZero() || sourceLatest.After(exportLatest)):
		if state.LastResultStatus == "failed" && state.LastErrorCode == "register_trigger_failed" {
			return "register_gui_blocked"
		}
		return "source_updated_not_exported"
	case !exportLatest.IsZero() && state.LastImportAt.IsZero():
		return "export_updated_not_imported"
	case !exportLatest.IsZero() && !state.LastImportAt.IsZero() && exportLatest.After(state.LastImportAt):
		return "export_updated_not_imported"
	case state.LastImportResult == "imported_to_channel" && state.LastProbeResult == "probe_pending":
		return "imported_pending_probe"
	case state.LastProbeResult == "probe_succeeded":
		return "probe_succeeded"
	case state.LastProbeResult == "probe_failed_rate_limit":
		return "probe_failed_rate_limit"
	}

	if health != nil && health.AvailableCount <= 0 && !state.SourceQuietSince.IsZero() {
		return "source_quiet_pool_low"
	}

	if health != nil && health.AvailableCount <= 0 && state.LastResultStatus == "failed" && state.LastErrorCode == "register_trigger_failed" {
		if !state.SourceQuietSince.IsZero() {
			return "trigger_failed_no_new_source"
		}
		return "register_gui_blocked"
	}
	return ""
}

func deriveTriggerResult(state *cursorProTriggerState, registerStatus *cursorProRegisterStatus) string {
	if state != nil {
		switch state.LastResultStatus {
		case "failed":
			return "trigger_failed"
		case "succeeded":
			return "trigger_succeeded"
		}
		if !state.LastTriggerAt.IsZero() {
			return "trigger_skipped"
		}
	}
	if registerStatus != nil {
		switch registerStatus.Status {
		case "failed":
			return "trigger_failed"
		case "succeeded":
			return "trigger_succeeded"
		}
	}
	return ""
}

func deriveRecoveryResult(state *cursorProTriggerState, registerStatus *cursorProRegisterStatus, tokenStatus *cursorProTokenStatus, health *CodexPoolHealth) string {
	if state == nil {
		state = &cursorProTriggerState{}
	}
	switch {
	case state.LastProbeResult == "probe_succeeded":
		return "probe_succeeded"
	case state.LastImportResult == "imported_to_channel" && state.LastProbeResult == "probe_pending":
		return "imported_pending_probe"
	case state.LastSuccessfulRecoveryReason != "":
		return state.LastSuccessfulRecoveryReason
	case state.LastResultStatus == "failed":
		return "recovery_failed"
	}
	if registerStatus != nil && registerStatus.Status == "failed" && registerStatus.ErrorCode == "register_trigger_failed" {
		if health != nil && health.AvailableCount > 0 && tokenStatus != nil && tokenStatus.ExportTokenCount > 0 {
			return "source_sync_succeeded"
		}
		return "recovery_failed"
	}
	return ""
}

func derivePoolRiskLevel(tokenStatus *cursorProTokenStatus, state *cursorProTriggerState, health *CodexPoolHealth, now time.Time) string {
	if health == nil {
		return ""
	}
	if state == nil {
		state = &cursorProTriggerState{}
	}
	if health.AvailableCount == 0 && !state.SourceQuietSince.IsZero() && state.LastResultStatus == "failed" {
		return "critical"
	}
	if health.AvailableCount > 0 || cursorProSourceRecentlyUpdated(tokenStatus, now) || !state.LastSuccessfulRecoveryAt.IsZero() {
		if state.LastResultStatus == "failed" {
			return "degraded"
		}
		return "ok"
	}
	if state.LastResultStatus == "failed" {
		return "degraded"
	}
	return "critical"
}

func getCursorProTriggerSnapshot(channelID int, now time.Time) *cursorProTriggerState {
	state := cursorProStateForChannel(channelID)
	if state == nil {
		return &cursorProTriggerState{}
	}
	state.RecentTriggerTimes = filterRecentTimes(state.RecentTriggerTimes, now.Add(-30*time.Minute))
	snapshot := *state
	snapshot.RecentTriggerTimes = append([]time.Time(nil), state.RecentTriggerTimes...)
	return &snapshot
}

func GetCursorProReplacementStatus(ctx context.Context, channelID int, now time.Time) (*CursorProReplacementStatusView, error) {
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

	health := ComputeCodexPoolHealth(channel, now)
	recentNoAvailable := RecentCodexNoAvailableCount(channelID, now)
	recentHotPathTriggers := RecentCodexHotPathTriggerCount(channelID, now)
	recentInvalidKeys := RecentCodexInvalidKeyCount(channelID, now)
	recentProbeSuccess := RecentCodexProbeSuccessCount(channelID, now)
	recentProbeFail := RecentCodexProbeFailCount(channelID, now)
	triggerRecommended, triggerReason := ShouldTriggerCursorProReplacement(channel, health, now)

	var (
		registerStatus    *cursorProRegisterStatus
		registerStatusErr string
		tokenStatus       *cursorProTokenStatus
		tokenStatusErr    string
	)
	registerStatus, err = readCursorProRegisterStatus(ctx)
	if err != nil {
		registerStatusErr = err.Error()
	}
	tokenStatus, err = readCursorProTokenStatus(ctx)
	if err != nil {
		tokenStatusErr = err.Error()
	}

	state := getCursorProTriggerSnapshot(channelID, now)
	cooldownDecision := evaluateCursorProTriggerCooldown(state, registerStatus, health, recentNoAvailable, recentHotPathTriggers, now)
	timeline := buildCursorProRecoveryTimeline(state, registerStatus, tokenStatus)

	view := &CursorProReplacementStatusView{
		ChannelID:                    channel.Id,
		ChannelName:                  channel.Name,
		AutoImportEnabled:            isCursorProAutoImportEnabled(channel),
		TriggerAllowed:               cooldownDecision.Allowed,
		BlockReason:                  cooldownDecision.BlockReason,
		LastTriggerReason:            state.LastTriggerReason,
		RecentTriggers30m:            len(state.RecentTriggerTimes),
		ConsecutiveNoYield:           state.ConsecutiveNoYield,
		LastTaskID:                   state.LastTaskID,
		LastTaskFinishedAt:           state.LastTaskFinishedAt,
		LastResultStatus:             state.LastResultStatus,
		LastResultCode:               state.LastErrorCode,
		LastResultMessage:            state.LastErrorMessage,
		ControlBaseURL:               cursorProControlBaseURL(),
		RegisterStatus:               registerStatus,
		RegisterStatusError:          registerStatusErr,
		Health:                       health,
		RecentNoAvailable5m:          recentNoAvailable,
		RecentHotPathTriggers:        recentHotPathTriggers,
		RecentInvalidKey5m:           recentInvalidKeys,
		RecentProbeSuccess5m:         recentProbeSuccess,
		RecentProbeFail5m:            recentProbeFail,
		PendingProbeCount:            PendingCodexProbeCount(channelID),
		LastProbeModel:               state.LastProbeModel,
		LastProbeResult:              state.LastProbeResult,
		TriggerRecommended:           triggerRecommended,
		TriggerReason:                triggerReason,
		MinTriggerIntervalSec:        int((3 * time.Minute).Seconds()),
		MaxTriggersPer30Min:          5,
		OpenCircuitAfterNoYield:      3,
		TokenStatus:                  tokenStatus,
		TokenStatusError:             tokenStatusErr,
		LastImportAt:                 lastImportAtPtr(state),
		LastImportResult:             state.LastImportResult,
		SyncDiagnosis:                diagnoseCursorProSync(tokenStatus, state, health),
		TriggerResult:                deriveTriggerResult(state, registerStatus),
		RecoveryResult:               deriveRecoveryResult(state, registerStatus, tokenStatus, health),
		PoolRiskLevel:                derivePoolRiskLevel(tokenStatus, state, health, now),
		SourceQuietSince:             timePtrIfNonZero(state.SourceQuietSince),
		LastSuccessfulRecoveryAt:     timePtrIfNonZero(state.LastSuccessfulRecoveryAt),
		LastSuccessfulRecoveryReason: state.LastSuccessfulRecoveryReason,
		RecoveryTimeline:             timeline,
		CooldownUntil:                timePtrIfNonZero(cooldownDecision.CooldownUntil),
		CooldownSecondsRemaining:     cooldownDecision.CooldownSecondsRemaining,
		CooldownBaseSeconds:          cooldownDecision.CooldownBaseSeconds,
		CooldownMode:                 cooldownDecision.CooldownMode,
		CooldownBreakAllowed:         cooldownDecision.CooldownBreakAllowed,
		CooldownBreakReason:          cooldownDecision.CooldownBreakReason,
	}
	if !state.LastTriggerAt.IsZero() {
		last := state.LastTriggerAt
		view.LastTriggerAt = &last
	}
	if !state.CircuitOpenUntil.IsZero() {
		until := state.CircuitOpenUntil
		view.CircuitOpenUntil = &until
	}
	return view, nil
}
