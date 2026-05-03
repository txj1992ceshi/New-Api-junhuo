package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetRandomSatisfiedChannelFilteredSkipsUnsupportedHigherPriority(t *testing.T) {
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
	})

	common.MemoryCacheEnabled = true
	group2model2channels = map[string]map[string][]int{
		"default": {
			"gpt-5.4": {1, 2, 3},
		},
	}
	channelsIDM = map[int]*Channel{
		1: {Id: 1, Name: "caowo", Priority: int64Ptr(300)},
		2: {Id: 2, Name: "codex-e2e-temp", Priority: int64Ptr(200)},
		3: {Id: 3, Name: "antigravity-openclaw", Priority: int64Ptr(100)},
	}

	channel, err := GetRandomSatisfiedChannelFiltered("default", "gpt-5.4", 0, func(ch *Channel) bool {
		return ch.Id != 1
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id)

	channel, err = GetRandomSatisfiedChannelFiltered("default", "gpt-5.4", 1, func(ch *Channel) bool {
		return ch.Id != 1
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 3, channel.Id)
}

func int64Ptr(v int64) *int64 {
	return &v
}
