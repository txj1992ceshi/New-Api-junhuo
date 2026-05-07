package service

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

type CursorPoolProxy = ExternalPoolProxy
type CursorPoolStatus = ExternalPoolStatus
type CursorPoolAccount = ExternalPoolAccount
type CursorPoolStatusView = ExternalPoolStatusView
type CursorPoolAccountsView = ExternalPoolAccountsView

func ResolveCursorPoolProxy(channel *model.Channel) (*CursorPoolProxy, bool) {
	return resolveExternalPoolProxy(channel, ExternalPoolKindCursor)
}

func GetCursorPoolSummary(ctx context.Context, channel *model.Channel) (*model.CursorPoolSummary, error) {
	status, err := FetchCursorPoolStatus(ctx, channel)
	if err != nil {
		return nil, err
	}
	diagnosis := classifyExternalPoolSummary(status, nil, nil)
	probed, inferenceOK, inferenceErr := ProbeExternalPoolInference(ctx, channel, ExternalPoolKindCursor, status)
	if probed && status != nil && status.Authenticated && status.Active > 0 && !inferenceOK {
		diagnosis = ExternalPoolDiagnosis{Availability: "degraded", Diagnosis: "auth_only"}
	}
	return &model.CursorPoolSummary{
		AvailableCount:   status.Active,
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

func FetchCursorPoolStatus(ctx context.Context, channel *model.Channel) (*CursorPoolStatus, error) {
	return fetchExternalPoolStatus(ctx, channel, ExternalPoolKindCursor)
}

func FetchCursorPoolAccounts(ctx context.Context, channel *model.Channel) ([]CursorPoolAccount, error) {
	return fetchExternalPoolAccounts(ctx, channel, ExternalPoolKindCursor)
}

func GetCursorPoolStatusView(ctx context.Context, channelID int) (*CursorPoolStatusView, error) {
	return getExternalPoolStatusView(ctx, channelID, ExternalPoolKindCursor)
}

func GetCursorPoolAccountsView(ctx context.Context, channelID int) (*CursorPoolAccountsView, error) {
	return getExternalPoolAccountsView(ctx, channelID, ExternalPoolKindCursor)
}

func GetCursorPoolAuthView(ctx context.Context, channelID int, authStrategy string) (*ExternalPoolAuthView, error) {
	return getExternalPoolAuthView(ctx, channelID, ExternalPoolKindCursor, authStrategy)
}

func StartCursorPoolAuth(ctx context.Context, channelID int, authStrategy string) (interface{}, error) {
	return startExternalPoolAuth(ctx, channelID, ExternalPoolKindCursor, authStrategy)
}

func CompleteCursorPoolAuth(ctx context.Context, channelID int, input string, authStrategy string) (interface{}, error) {
	return completeExternalPoolAuth(ctx, channelID, ExternalPoolKindCursor, input, authStrategy)
}
