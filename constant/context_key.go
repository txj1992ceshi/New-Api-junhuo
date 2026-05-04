package constant

type ContextKey string

const (
	ContextKeyTokenCountMeta  ContextKey = "token_count_meta"
	ContextKeyPromptTokens    ContextKey = "prompt_tokens"
	ContextKeyEstimatedTokens ContextKey = "estimated_tokens"

	ContextKeyOriginalModel    ContextKey = "original_model"
	ContextKeyRequestStartTime ContextKey = "request_start_time"

	/* token related keys */
	ContextKeyTokenUnlimited         ContextKey = "token_unlimited_quota"
	ContextKeyTokenKey               ContextKey = "token_key"
	ContextKeyTokenId                ContextKey = "token_id"
	ContextKeyTokenGroup             ContextKey = "token_group"
	ContextKeyTokenSpecificChannelId ContextKey = "specific_channel_id"
	ContextKeyTokenModelLimitEnabled ContextKey = "token_model_limit_enabled"
	ContextKeyTokenModelLimit        ContextKey = "token_model_limit"
	ContextKeyTokenCrossGroupRetry   ContextKey = "token_cross_group_retry"

	/* channel related keys */
	ContextKeyChannelId                           ContextKey = "channel_id"
	ContextKeyChannelName                         ContextKey = "channel_name"
	ContextKeyChannelCreateTime                   ContextKey = "channel_create_time"
	ContextKeyChannelBaseUrl                      ContextKey = "base_url"
	ContextKeyChannelType                         ContextKey = "channel_type"
	ContextKeyChannelSetting                      ContextKey = "channel_setting"
	ContextKeyChannelOtherSetting                 ContextKey = "channel_other_setting"
	ContextKeyChannelParamOverride                ContextKey = "param_override"
	ContextKeyChannelHeaderOverride               ContextKey = "header_override"
	ContextKeyChannelOrganization                 ContextKey = "channel_organization"
	ContextKeyChannelAutoBan                      ContextKey = "auto_ban"
	ContextKeyChannelModelMapping                 ContextKey = "model_mapping"
	ContextKeyChannelStatusCodeMapping            ContextKey = "status_code_mapping"
	ContextKeyResponsesModelMappingApplied        ContextKey = "responses_model_mapping_applied"
	ContextKeyResponsesCompatApplied              ContextKey = "responses_compat_applied"
	ContextKeyResponsesCompatProfile              ContextKey = "responses_compat_profile"
	ContextKeyResponsesCompatMode                 ContextKey = "responses_compat_mode"
	ContextKeyResponsesCompatRemovedFields        ContextKey = "responses_compat_removed_fields"
	ContextKeyResponsesCompatOriginModel          ContextKey = "responses_compat_origin_model"
	ContextKeyResponsesCompatChannelType          ContextKey = "responses_compat_channel_type"
	ContextKeyResponsesCompatNormalizedInput      ContextKey = "responses_compat_normalized_input"
	ContextKeyResponsesCompatRemovedToolItems     ContextKey = "responses_compat_removed_tool_items"
	ContextKeyResponsesCompatRemovedHistoryItems  ContextKey = "responses_compat_removed_history_items"
	ContextKeyResponsesCompatIncludeDropped       ContextKey = "responses_compat_include_dropped"
	ContextKeyResponsesCompatStoreDropped         ContextKey = "responses_compat_store_dropped"
	ContextKeyResponsesCompatParallelToolsDropped ContextKey = "responses_compat_parallel_tools_dropped"
	ContextKeyResponsesStateSanitized             ContextKey = "responses_state_sanitized"
	ContextKeyResponsesStateSanitizedFields       ContextKey = "responses_state_sanitized_fields"
	ContextKeyResponsesStateHadPreviousResponseID ContextKey = "responses_state_had_previous_response_id"
	ContextKeyResponsesStateHadConversation       ContextKey = "responses_state_had_conversation"
	ContextKeyResponsesStateHadContextManagement  ContextKey = "responses_state_had_context_management"
	ContextKeyResponsesStateHadPromptCacheKey     ContextKey = "responses_state_had_prompt_cache_key"
	ContextKeyChannelIsMultiKey                   ContextKey = "channel_is_multi_key"
	ContextKeyChannelMultiKeyIndex                ContextKey = "channel_multi_key_index"
	ContextKeyChannelKey                          ContextKey = "channel_key"
	ContextKeyAntigravityEmail                    ContextKey = "antigravity_email"
	ContextKeyAntigravityEffectiveProjectID       ContextKey = "antigravity_effective_project_id"
	ContextKeyAntigravityKeyState                 ContextKey = "antigravity_key_state"
	ContextKeyAntigravityErrorClass               ContextKey = "antigravity_error_class"
	ContextKeyAntigravityAccountSwitched          ContextKey = "antigravity_account_switched"
	ContextKeyAntigravityCredentialRefreshApplied ContextKey = "antigravity_credential_refresh_applied"
	ContextKeyAntigravityResponsesStreamCreated   ContextKey = "antigravity_responses_stream_created_emitted"
	ContextKeyAntigravityResponsesTextDelta       ContextKey = "antigravity_responses_text_delta_emitted"
	ContextKeyAntigravityResponsesItemDone        ContextKey = "antigravity_responses_item_done_emitted"
	ContextKeyAntigravityResponsesVisibleOutput   ContextKey = "antigravity_responses_visible_output_present"
	ContextKeyAntigravityResponsesEmptyStream     ContextKey = "antigravity_responses_empty_stream_detected"
	ContextKeyAntigravityResponsesEmptyReason     ContextKey = "antigravity_responses_empty_stream_reason"
	ContextKeyAntigravityResponsesFinishSummary   ContextKey = "antigravity_responses_finish_reason_summary"
	ContextKeyAntigravityResponsesCandidateCount  ContextKey = "antigravity_responses_candidate_count"
	ContextKeyAntigravityResponsesCompletionToken ContextKey = "antigravity_responses_completion_tokens"
	ContextKeyAntigravityResponsesFailedAsEmpty   ContextKey = "antigravity_responses_failed_as_empty"
	ContextKeyCodexAccountID                      ContextKey = "codex_account_id"
	ContextKeyCodexEmail                          ContextKey = "codex_email"
	ContextKeyCodexKeyState                       ContextKey = "codex_key_state"

	ContextKeyAutoGroup           ContextKey = "auto_group"
	ContextKeyAutoGroupIndex      ContextKey = "auto_group_index"
	ContextKeyAutoGroupRetryIndex ContextKey = "auto_group_retry_index"

	/* user related keys */
	ContextKeyUserId      ContextKey = "id"
	ContextKeyUserSetting ContextKey = "user_setting"
	ContextKeyUserQuota   ContextKey = "user_quota"
	ContextKeyUserStatus  ContextKey = "user_status"
	ContextKeyUserEmail   ContextKey = "user_email"
	ContextKeyUserGroup   ContextKey = "user_group"
	ContextKeyUsingGroup  ContextKey = "group"
	ContextKeyUserName    ContextKey = "username"

	ContextKeyLocalCountTokens ContextKey = "local_count_tokens"

	ContextKeySystemPromptOverride ContextKey = "system_prompt_override"

	// ContextKeyFileSourcesToCleanup stores file sources that need cleanup when request ends
	ContextKeyFileSourcesToCleanup ContextKey = "file_sources_to_cleanup"

	// ContextKeyAdminRejectReason stores an admin-only reject/block reason extracted from upstream responses.
	// It is not returned to end users, but can be persisted into consume/error logs for debugging.
	ContextKeyAdminRejectReason ContextKey = "admin_reject_reason"

	// ContextKeyLanguage stores the user's language preference for i18n
	ContextKeyLanguage ContextKey = "language"
	ContextKeyIsStream ContextKey = "is_stream"
)
