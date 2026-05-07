package service

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

type KiroPoolProxy = ExternalPoolProxy
type KiroPoolStatus = ExternalPoolStatus
type KiroPoolAccount = ExternalPoolAccount
type KiroPoolStatusView = ExternalPoolStatusView
type KiroPoolAccountsView = ExternalPoolAccountsView

func ResolveKiroPoolProxy(channel *model.Channel) (*KiroPoolProxy, bool) {
	return resolveExternalPoolProxy(channel, ExternalPoolKindKiro)
}

func GetKiroPoolSummary(ctx context.Context, channel *model.Channel) (*model.KiroPoolSummary, error) {
	status, err := FetchKiroPoolStatus(ctx, channel)
	if err != nil {
		return nil, err
	}
	diagnosis := classifyExternalPoolSummary(status, nil, nil)
	probed, inferenceOK, inferenceErr := ProbeExternalPoolInference(ctx, channel, ExternalPoolKindKiro, status)
	if probed && status != nil && status.Authenticated && status.Active > 0 && !inferenceOK {
		diagnosis = ExternalPoolDiagnosis{Availability: "degraded", Diagnosis: "auth_only"}
	}
	return &model.KiroPoolSummary{
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

func FetchKiroPoolStatus(ctx context.Context, channel *model.Channel) (*KiroPoolStatus, error) {
	return fetchExternalPoolStatus(ctx, channel, ExternalPoolKindKiro)
}

func FetchKiroPoolAccounts(ctx context.Context, channel *model.Channel) ([]KiroPoolAccount, error) {
	return fetchExternalPoolAccounts(ctx, channel, ExternalPoolKindKiro)
}

func GetKiroPoolStatusView(ctx context.Context, channelID int) (*KiroPoolStatusView, error) {
	return getExternalPoolStatusView(ctx, channelID, ExternalPoolKindKiro)
}

func GetKiroPoolAccountsView(ctx context.Context, channelID int) (*KiroPoolAccountsView, error) {
	return getExternalPoolAccountsView(ctx, channelID, ExternalPoolKindKiro)
}

func GetKiroPoolAuthView(ctx context.Context, channelID int) (*ExternalPoolAuthView, error) {
	return getExternalPoolAuthView(ctx, channelID, ExternalPoolKindKiro, "")
}

func StartKiroPoolAuth(ctx context.Context, channelID int) (interface{}, error) {
	return startExternalPoolAuth(ctx, channelID, ExternalPoolKindKiro, "")
}

func CompleteKiroPoolAuth(ctx context.Context, channelID int, input string) (interface{}, error) {
	return completeExternalPoolAuth(ctx, channelID, ExternalPoolKindKiro, input, "")
}
