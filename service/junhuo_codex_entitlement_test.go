package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSignJunhuoCodexEntitlementEmitsVerifiableEd25519Token(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	t.Setenv("JUNHUO_CODEX_ENTITLEMENT_PRIVATE_KEY", base64.RawURLEncoding.EncodeToString(privateKey.Seed()))

	token, err := SignJunhuoCodexEntitlement(JunhuoCodexEntitlementClaims{
		Issuer:     "test",
		Audience:   JunhuoCodexAudience,
		Subject:    "user_1",
		Account:    "tester@example.com",
		DeviceId:   "device_1",
		SessionId:  "session_1",
		Features:   []string{"local_agent"},
		AppVersion: "test",
		IssuedAt:   1_780_000_000,
		NotBefore:  1_780_000_000,
		ExpiresAt:  1_780_001_800,
		Jti:        "jti_1",
	})
	require.NoError(t, err)
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	require.True(t, ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), mustBase64URLDecode(t, parts[2])))

	var header junhuoCodexEntitlementHeader
	require.NoError(t, common.Unmarshal(mustBase64URLDecode(t, parts[0]), &header))
	require.Equal(t, "EdDSA", header.Alg)

	var claims JunhuoCodexEntitlementClaims
	require.NoError(t, common.Unmarshal(mustBase64URLDecode(t, parts[1]), &claims))
	require.Equal(t, JunhuoCodexAudience, claims.Audience)
	require.Equal(t, "device_1", claims.DeviceId)
	require.Equal(t, int64(1_780_001_800), claims.ExpiresAt)
}

func mustBase64URLDecode(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	require.NoError(t, err)
	return decoded
}
