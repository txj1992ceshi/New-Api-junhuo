package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestFilterHiddenPricingGroupsForNonAdminLeavesAdminUntouched(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "gpt-5", EnableGroup: []string{"default", "antigravity-chat"}},
	}
	usableGroup := map[string]string{
		"default":           "default",
		"antigravity-chat":  "antigravity-chat",
	}

	filteredPricing, filteredGroups := filterHiddenPricingGroupsForNonAdmin(pricing, usableGroup, true)

	require.Equal(t, pricing, filteredPricing)
	require.Equal(t, usableGroup, filteredGroups)
}

func TestFilterHiddenPricingGroupsForNonAdminHidesInternalGroups(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "gpt-5", EnableGroup: []string{"default", "antigravity-chat"}},
		{ModelName: "gpt-5.5", EnableGroup: []string{"antigravity-chat"}},
	}
	usableGroup := map[string]string{
		"default":          "default",
		"vip":              "vip",
		"antigravity-chat": "antigravity-chat",
	}

	filteredPricing, filteredGroups := filterHiddenPricingGroupsForNonAdmin(pricing, usableGroup, false)

	require.Equal(t, map[string]string{
		"default": "default",
		"vip":     "vip",
	}, filteredGroups)
	require.Equal(t, []string{"default"}, filteredPricing[0].EnableGroup)
	require.Empty(t, filteredPricing[1].EnableGroup)
	require.Equal(t, []string{"default", "antigravity-chat"}, pricing[0].EnableGroup)
}
