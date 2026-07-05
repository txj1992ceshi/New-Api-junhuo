package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const JunhuoCodexAudience = "junhuo-Codex-claw"

type JunhuoCodexEntitlementClaims struct {
	Issuer     string   `json:"iss"`
	Audience   string   `json:"aud"`
	Subject    string   `json:"sub"`
	Account    string   `json:"account"`
	DeviceId   string   `json:"deviceId"`
	SessionId  string   `json:"sessionId"`
	Features   []string `json:"features"`
	AppVersion string   `json:"appVersion"`
	IssuedAt   int64    `json:"iat"`
	NotBefore  int64    `json:"nbf"`
	ExpiresAt  int64    `json:"exp"`
	Jti        string   `json:"jti"`
}

type junhuoCodexEntitlementHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

func JunhuoCodexEntitlementGroups() []string {
	raw := common.GetEnvOrDefaultString("JUNHUO_CODEX_ENTITLEMENT_GROUPS", "codex,codex-claw,junhuo-codex")
	parts := strings.Split(raw, ",")
	groups := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			groups = append(groups, part)
		}
	}
	return groups
}

func JunhuoCodexClientSessionTTLSeconds() int {
	return common.GetEnvOrDefault("JUNHUO_CODEX_CLIENT_SESSION_TTL_SECONDS", 30*24*60*60)
}

func JunhuoCodexEntitlementTTLSeconds() int {
	return common.GetEnvOrDefault("JUNHUO_CODEX_ENTITLEMENT_TTL_SECONDS", 30*60)
}

func GenerateClientSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func SignJunhuoCodexEntitlement(claims JunhuoCodexEntitlementClaims) (string, error) {
	privateKey, err := loadJunhuoCodexEntitlementPrivateKey()
	if err != nil {
		return "", err
	}
	headerBytes, err := common.Marshal(junhuoCodexEntitlementHeader{Alg: "EdDSA", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	claimsBytes, err := common.Marshal(claims)
	if err != nil {
		return "", err
	}
	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := header + "." + payload
	signature := ed25519.Sign(privateKey, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func loadJunhuoCodexEntitlementPrivateKey() (ed25519.PrivateKey, error) {
	raw := strings.TrimSpace(common.GetEnvOrDefaultString("JUNHUO_CODEX_ENTITLEMENT_PRIVATE_KEY", ""))
	if raw == "" {
		return nil, errors.New("JUNHUO_CODEX_ENTITLEMENT_PRIVATE_KEY is required")
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			if privateKey, ok := key.(ed25519.PrivateKey); ok {
				return privateKey, nil
			}
		}
		if privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
			if typed, ok := privateKey.(ed25519.PrivateKey); ok {
				return typed, nil
			}
		}
		return nil, errors.New("JUNHUO_CODEX_ENTITLEMENT_PRIVATE_KEY PEM is not an Ed25519 private key")
	}
	decoded, err := decodeBase64Key(raw)
	if err != nil {
		return nil, err
	}
	switch len(decoded) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	default:
		return nil, errors.New("JUNHUO_CODEX_ENTITLEMENT_PRIVATE_KEY must decode to 32-byte seed or 64-byte private key")
	}
}

func decodeBase64Key(raw string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(raw)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
