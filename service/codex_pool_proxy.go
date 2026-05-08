package service

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

type CodexPoolProxy = ExternalPoolProxy
type CodexPoolStatus = ExternalPoolStatus
type CodexPoolAccount = ExternalPoolAccount
type CodexPoolStatusView = ExternalPoolStatusView
type CodexPoolAccountsView = ExternalPoolAccountsView

func ResolveCodexPoolProxy(channel *model.Channel) (*CodexPoolProxy, bool) {
	return resolveExternalPoolProxy(channel, ExternalPoolKindCodex)
}

func GetCodexPoolSummary(ctx context.Context, channel *model.Channel) (*model.CodexPoolSummary, error) {
	status, err := FetchCodexPoolStatus(ctx, channel)
	if err != nil {
		return nil, err
	}
	diagnosis := classifyExternalPoolSummary(status, nil, nil)
	probed, inferenceOK, inferenceErr := ProbeExternalPoolInference(ctx, channel, ExternalPoolKindCodex, status)
	if probed && status != nil && status.Authenticated && status.Active > 0 && !inferenceOK {
		diagnosis = ExternalPoolDiagnosis{Availability: "degraded", Diagnosis: "auth_only"}
	}
	return &model.CodexPoolSummary{
		AvailableCount:   status.Active,
		HealthyCount:     status.Active,
		CooldownCount:    0,
		SuspectCount:     0,
		DeadCount:        status.Error,
		TotalCount:       status.Total,
		ErrorCount:       status.Error,
		Availability:     diagnosis.Availability,
		PoolState:        classifyExternalPoolState(status, nil),
		Diagnosis:        diagnosis.Diagnosis,
		AuthCapable:      status.Authenticated,
		InferenceProbed:  probed,
		InferenceCapable: probed && inferenceOK,
		InferenceError:   inferenceErr,
	}, nil
}

func FetchCodexPoolStatus(ctx context.Context, channel *model.Channel) (*CodexPoolStatus, error) {
	return fetchExternalPoolStatus(ctx, channel, ExternalPoolKindCodex)
}

func FetchCodexPoolAccounts(ctx context.Context, channel *model.Channel) ([]CodexPoolAccount, error) {
	return fetchExternalPoolAccounts(ctx, channel, ExternalPoolKindCodex)
}

func GetCodexPoolStatusView(ctx context.Context, channelID int) (*CodexPoolStatusView, error) {
	return getExternalPoolStatusView(ctx, channelID, ExternalPoolKindCodex)
}

func GetCodexPoolAccountsView(ctx context.Context, channelID int) (*CodexPoolAccountsView, error) {
	return getExternalPoolAccountsView(ctx, channelID, ExternalPoolKindCodex)
}

func GetCodexPoolAuthView(ctx context.Context, channelID int, authStrategy string) (*ExternalPoolAuthView, error) {
	return getExternalPoolAuthView(ctx, channelID, ExternalPoolKindCodex, authStrategy)
}

func StartCodexPoolAuth(ctx context.Context, channelID int, authStrategy string) (interface{}, error) {
	return startExternalPoolAuth(ctx, channelID, ExternalPoolKindCodex, authStrategy)
}

func CompleteCodexPoolAuth(ctx context.Context, channelID int, input string, authStrategy string) (interface{}, error) {
	return completeExternalPoolAuth(ctx, channelID, ExternalPoolKindCodex, input, authStrategy)
}
