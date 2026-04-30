package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type CodexPoolHealthStatus struct {
	ChannelID             int              `json:"channel_id"`
	ChannelName           string           `json:"channel_name"`
	AutoImportEnabled     bool             `json:"auto_import_enabled"`
	Health                *CodexPoolHealth `json:"health"`
	RecentNoAvailable5m   int              `json:"recent_no_available_5m"`
	TriggerRecommended    bool             `json:"trigger_recommended"`
	TriggerReason         string           `json:"trigger_reason,omitempty"`
	MinHealthyWatermark   int              `json:"min_healthy_watermark"`
	CursorProExportDir    string           `json:"cursorpro_export_dir"`
	CodexKeyStateCounters map[string]int   `json:"key_state_counters"`
}

type CursorProReplacementStatusView struct {
	ChannelID               int                      `json:"channel_id"`
	ChannelName             string                   `json:"channel_name"`
	AutoImportEnabled       bool                     `json:"auto_import_enabled"`
	TriggerAllowed          bool                     `json:"trigger_allowed"`
	BlockReason             string                   `json:"block_reason,omitempty"`
	LastTriggerAt           *time.Time               `json:"last_trigger_at,omitempty"`
	RecentTriggers30m       int                      `json:"recent_triggers_30m"`
	CircuitOpenUntil        *time.Time               `json:"circuit_open_until,omitempty"`
	ConsecutiveNoYield      int                      `json:"consecutive_no_yield"`
	LastTaskID              string                   `json:"last_task_id,omitempty"`
	LastTaskFinishedAt      string                   `json:"last_task_finished_at,omitempty"`
	LastResultStatus        string                   `json:"last_result_status,omitempty"`
	LastResultCode          string                   `json:"last_result_code,omitempty"`
	LastResultMessage       string                   `json:"last_result_message,omitempty"`
	ControlBaseURL          string                   `json:"control_base_url"`
	RegisterStatus          *cursorProRegisterStatus `json:"register_status,omitempty"`
	RegisterStatusError     string                   `json:"register_status_error,omitempty"`
	Health                  *CodexPoolHealth         `json:"health,omitempty"`
	RecentNoAvailable5m     int                      `json:"recent_no_available_5m,omitempty"`
	TriggerRecommended      bool                     `json:"trigger_recommended"`
	TriggerReason           string                   `json:"trigger_reason,omitempty"`
	MinTriggerIntervalSec   int                      `json:"min_trigger_interval_sec"`
	MaxTriggersPer30Min     int                      `json:"max_triggers_per_30m"`
	OpenCircuitAfterNoYield int                      `json:"open_circuit_after_no_yield"`
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
	triggerRecommended, triggerReason := ShouldTriggerCursorProReplacement(channel, health, now)
	stateCounters := map[string]int{
		string(model.CodexKeyStateHealthy):    health.Healthy,
		string(model.CodexKeyStateNew):        health.New,
		string(model.CodexKeyStateCooldown):   health.Cooldown,
		string(model.CodexKeyStateSuspect):    health.Suspect,
		string(model.CodexKeyStateDead):       health.Dead,
		string(model.CodexKeyStateRefreshing): health.Refreshing,
	}

	return &CodexPoolHealthStatus{
		ChannelID:             channel.Id,
		ChannelName:           channel.Name,
		AutoImportEnabled:     isCursorProAutoImportEnabled(channel),
		Health:                health,
		RecentNoAvailable5m:   recentNoAvailable,
		TriggerRecommended:    triggerRecommended,
		TriggerReason:         triggerReason,
		MinHealthyWatermark:   codexMinHealthyWatermark(channel),
		CursorProExportDir:    cursorProCodexExportDir(),
		CodexKeyStateCounters: stateCounters,
	}, nil
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

func evaluateCursorProTriggerAllowance(state *cursorProTriggerState, registerStatus *cursorProRegisterStatus, now time.Time) (bool, string) {
	if state == nil {
		return true, ""
	}
	if !state.CircuitOpenUntil.IsZero() && state.CircuitOpenUntil.After(now) {
		return false, "circuit_open"
	}
	if !state.LastTriggerAt.IsZero() && now.Sub(state.LastTriggerAt) < 3*time.Minute {
		return false, "cooldown"
	}
	if len(state.RecentTriggerTimes) >= 5 {
		return false, "rate_limited"
	}
	if registerStatus != nil && registerStatus.Status == "running" {
		return false, "already_running"
	}
	return true, ""
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
	triggerRecommended, triggerReason := ShouldTriggerCursorProReplacement(channel, health, now)

	var (
		registerStatus    *cursorProRegisterStatus
		registerStatusErr string
	)
	registerStatus, err = readCursorProRegisterStatus(ctx)
	if err != nil {
		registerStatusErr = err.Error()
	}

	state := getCursorProTriggerSnapshot(channelID, now)
	triggerAllowed, blockReason := evaluateCursorProTriggerAllowance(state, registerStatus, now)

	view := &CursorProReplacementStatusView{
		ChannelID:               channel.Id,
		ChannelName:             channel.Name,
		AutoImportEnabled:       isCursorProAutoImportEnabled(channel),
		TriggerAllowed:          triggerAllowed,
		BlockReason:             blockReason,
		RecentTriggers30m:       len(state.RecentTriggerTimes),
		ConsecutiveNoYield:      state.ConsecutiveNoYield,
		LastTaskID:              state.LastTaskID,
		LastTaskFinishedAt:      state.LastTaskFinishedAt,
		LastResultStatus:        state.LastResultStatus,
		LastResultCode:          state.LastErrorCode,
		LastResultMessage:       state.LastErrorMessage,
		ControlBaseURL:          cursorProControlBaseURL(),
		RegisterStatus:          registerStatus,
		RegisterStatusError:     registerStatusErr,
		Health:                  health,
		RecentNoAvailable5m:     recentNoAvailable,
		TriggerRecommended:      triggerRecommended,
		TriggerReason:           triggerReason,
		MinTriggerIntervalSec:   int((3 * time.Minute).Seconds()),
		MaxTriggersPer30Min:     5,
		OpenCircuitAfterNoYield: 3,
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
