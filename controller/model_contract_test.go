package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterVisibleModelsByPublicContract(t *testing.T) {
	models := []string{"cursor-default", "gpt-5.5", "gpt-5.4", "kiro-sonnet"}
	publicModels := []string{"gpt-5.5", "gpt-5.4"}

	filtered := filterVisibleModelsByPublicContract(models, nil, publicModels)
	require.Equal(t, []string{"gpt-5.5", "gpt-5.4"}, filtered)
}

func TestFilterVisibleModelsByPublicContractFallsBackWhenTokenLacksPublicModels(t *testing.T) {
	models := []string{"cursor-default", "kiro-sonnet"}
	publicModels := []string{"gpt-5.5", "gpt-5.4"}
	allowed := map[string]bool{
		"cursor-default": true,
		"kiro-sonnet":    true,
	}

	filtered := filterVisibleModelsByPublicContract(models, allowed, publicModels)
	require.Equal(t, models, filtered)
}
