package service

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

type WindsurfPoolProxy = ExternalPoolProxy
type WindsurfPoolStatus = ExternalPoolStatus
type WindsurfPoolAccount = ExternalPoolAccount
type WindsurfPoolStatusView = ExternalPoolStatusView
type WindsurfPoolAccountsView = ExternalPoolAccountsView

func ResolveWindsurfPoolProxy(channel *model.Channel) (*WindsurfPoolProxy, bool) {
	return resolveExternalPoolProxy(channel, ExternalPoolKindWindsurf)
}

func GetWindsurfPoolSummary(ctx context.Context, channel *model.Channel) (*model.WindsurfPoolSummary, error) {
	status, err := FetchWindsurfPoolStatus(ctx, channel)
	if err != nil {
		return nil, err
	}
	diagnosis := classifyExternalPoolSummary(status, nil, nil)
	probed, inferenceOK, inferenceErr := ProbeExternalPoolInference(ctx, channel, ExternalPoolKindWindsurf, status)
	if probed && status != nil && status.Authenticated && status.Active > 0 && !inferenceOK {
		diagnosis = ExternalPoolDiagnosis{Availability: "degraded", Diagnosis: "auth_only"}
	}
	return &model.WindsurfPoolSummary{
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

func FetchWindsurfPoolStatus(ctx context.Context, channel *model.Channel) (*WindsurfPoolStatus, error) {
	return fetchExternalPoolStatus(ctx, channel, ExternalPoolKindWindsurf)
}

func FetchWindsurfPoolAccounts(ctx context.Context, channel *model.Channel) ([]WindsurfPoolAccount, error) {
	return fetchExternalPoolAccounts(ctx, channel, ExternalPoolKindWindsurf)
}

func GetWindsurfPoolStatusView(ctx context.Context, channelID int) (*WindsurfPoolStatusView, error) {
	return getExternalPoolStatusView(ctx, channelID, ExternalPoolKindWindsurf)
}

func GetWindsurfPoolAccountsView(ctx context.Context, channelID int) (*WindsurfPoolAccountsView, error) {
	return getExternalPoolAccountsView(ctx, channelID, ExternalPoolKindWindsurf)
}

func GetWindsurfPoolAuthView(ctx context.Context, channelID int) (*ExternalPoolAuthView, error) {
	return getExternalPoolAuthView(ctx, channelID, ExternalPoolKindWindsurf, "")
}

func StartWindsurfPoolAuth(ctx context.Context, channelID int) (interface{}, error) {
	return startExternalPoolAuth(ctx, channelID, ExternalPoolKindWindsurf, "")
}

func CompleteWindsurfPoolAuth(ctx context.Context, channelID int, input string) (interface{}, error) {
	return completeExternalPoolAuth(ctx, channelID, ExternalPoolKindWindsurf, input, "")
}
