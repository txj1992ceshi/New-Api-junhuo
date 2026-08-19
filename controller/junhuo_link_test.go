package controller

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJunhuoLinkSecurityLogCallsitesDoNotReferenceKeyMaterial(t *testing.T) {
	data, err := os.ReadFile("junhuo_link.go")
	require.NoError(t, err)

	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "SysLog(") {
			continue
		}
		count++
		lower := strings.ToLower(line)
		assert.NotContains(t, lower, "key")
		assert.NotContains(t, lower, "secret")
		assert.NotContains(t, lower, "body")
	}
	assert.Greater(t, count, 0, "security logging should remain explicit and reviewable")
}
