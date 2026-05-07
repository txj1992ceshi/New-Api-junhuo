package controller

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestInjectExternalPoolSummaryMarksUpstreamErrorOnFailure(t *testing.T) {
	baseURL := "http://127.0.0.1:1"

	cursorChannel := &model.Channel{
		Id:        101,
		Name:      "cursor-pool-proxy",
		Key:       "proxy-key",
		BaseURL:   &baseURL,
		OtherInfo: `{"cursor_pool_proxy":true}`,
	}
	injectCursorPoolSummary(context.Background(), cursorChannel)
	require.NotNil(t, cursorChannel.CursorPoolSummary)
	require.Equal(t, "upstream_error", cursorChannel.CursorPoolSummary.PoolState)
	require.Equal(t, "unavailable", cursorChannel.CursorPoolSummary.Availability)
	require.Equal(t, "upstream_unreachable", cursorChannel.CursorPoolSummary.Diagnosis)
	require.True(t, strings.TrimSpace(cursorChannel.CursorPoolSummary.UpstreamError) != "")

	windsurfChannel := &model.Channel{
		Id:        102,
		Name:      "windsurf-pool-proxy",
		Key:       "proxy-key",
		BaseURL:   &baseURL,
		OtherInfo: `{"windsurf_pool_proxy":true}`,
	}
	injectWindsurfPoolSummary(context.Background(), windsurfChannel)
	require.NotNil(t, windsurfChannel.WindsurfPoolSummary)
	require.Equal(t, "upstream_error", windsurfChannel.WindsurfPoolSummary.PoolState)
	require.Equal(t, "unavailable", windsurfChannel.WindsurfPoolSummary.Availability)
	require.Equal(t, "upstream_unreachable", windsurfChannel.WindsurfPoolSummary.Diagnosis)
	require.True(t, strings.TrimSpace(windsurfChannel.WindsurfPoolSummary.UpstreamError) != "")

	kiroChannel := &model.Channel{
		Id:        103,
		Name:      "kiro-pool-proxy",
		Key:       "proxy-key",
		BaseURL:   &baseURL,
		OtherInfo: `{"kiro_pool_proxy":true}`,
	}
	injectKiroPoolSummary(context.Background(), kiroChannel)
	require.NotNil(t, kiroChannel.KiroPoolSummary)
	require.Equal(t, "upstream_error", kiroChannel.KiroPoolSummary.PoolState)
	require.Equal(t, "unavailable", kiroChannel.KiroPoolSummary.Availability)
	require.Equal(t, "upstream_unreachable", kiroChannel.KiroPoolSummary.Diagnosis)
	require.True(t, strings.TrimSpace(kiroChannel.KiroPoolSummary.UpstreamError) != "")
}
