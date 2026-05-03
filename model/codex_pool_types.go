package model

type CodexKeyState string

const (
	CodexKeyStateHealthy    CodexKeyState = "healthy"
	CodexKeyStateNew        CodexKeyState = "new"
	CodexKeyStateCooldown   CodexKeyState = "cooldown"
	CodexKeyStateSuspect    CodexKeyState = "suspect"
	CodexKeyStateDead       CodexKeyState = "dead"
	CodexKeyStateRefreshing CodexKeyState = "refreshing"
)

type ChannelKeyMeta struct {
	State                  CodexKeyState `json:"state,omitempty"`
	Source                 string        `json:"source,omitempty"`
	AccountID              string        `json:"account_id,omitempty"`
	Email                  string        `json:"email,omitempty"`
	ExpiresAt              string        `json:"expires_at,omitempty"`
	ProjectID              string        `json:"project_id,omitempty"`
	ManagedProjectID       string        `json:"managed_project_id,omitempty"`
	EffectiveProjectID     string        `json:"effective_project_id,omitempty"`
	LastProjectCheckAt     int64         `json:"last_project_check_at,omitempty"`
	LastProjectError       string        `json:"last_project_error,omitempty"`
	ChatModels             []string      `json:"chat_models,omitempty"`
	ResponsesModels        []string      `json:"responses_models,omitempty"`
	ResponsesCompactModels []string      `json:"responses_compact_models,omitempty"`
	LastSuccessAt          int64         `json:"last_success_at,omitempty"`
	LastErrorAt            int64         `json:"last_error_at,omitempty"`
	LastErrorKind          string        `json:"last_error_kind,omitempty"`
	LastSelectedAt         int64         `json:"last_selected_at,omitempty"`
	LastRefreshAt          int64         `json:"last_refresh_at,omitempty"`
	Consecutive429         int           `json:"consecutive_429,omitempty"`
	Consecutive5xx         int           `json:"consecutive_5xx,omitempty"`
	ConsecutiveAuthFail    int           `json:"consecutive_auth_fail,omitempty"`
	SoftFailCount          int           `json:"soft_fail_count,omitempty"`
	TotalSuccess           int64         `json:"total_success,omitempty"`
	TotalFail              int64         `json:"total_fail,omitempty"`
	CooldownUntil          int64         `json:"cooldown_until,omitempty"`
	NewSuccessCount        int           `json:"new_success_count,omitempty"`
}
