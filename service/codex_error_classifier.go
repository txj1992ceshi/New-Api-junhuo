package service

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/types"
)

type CodexErrorKind string

const (
	CodexErrorKindSuccess   CodexErrorKind = "success"
	CodexErrorKindRateLimit CodexErrorKind = "rate_limit"
	CodexErrorKindServer    CodexErrorKind = "server"
	CodexErrorKindAuth      CodexErrorKind = "auth"
	CodexErrorKindSoftFail  CodexErrorKind = "soft_fail"
	CodexErrorKindFatal     CodexErrorKind = "fatal"
)

func ClassifyCodexError(resp *http.Response, reqErr error, apiErr *types.NewAPIError) CodexErrorKind {
	if reqErr == nil && apiErr == nil {
		return CodexErrorKindSuccess
	}
	if apiErr != nil {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests:
			return CodexErrorKindRateLimit
		case http.StatusUnauthorized, http.StatusForbidden:
			return CodexErrorKindAuth
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return CodexErrorKindServer
		}
		errMsg := strings.ToLower(apiErr.Error())
		switch {
		case strings.Contains(errMsg, "rate limit"), strings.Contains(errMsg, "too many requests"):
			return CodexErrorKindRateLimit
		case strings.Contains(errMsg, "unauthorized"), strings.Contains(errMsg, "forbidden"), strings.Contains(errMsg, "invalid token"):
			return CodexErrorKindAuth
		case strings.Contains(errMsg, "timeout"), strings.Contains(errMsg, "eof"), strings.Contains(errMsg, "connection reset"), strings.Contains(errMsg, "stream closed"):
			return CodexErrorKindSoftFail
		}
	}
	if reqErr != nil {
		errMsg := strings.ToLower(reqErr.Error())
		if strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "eof") ||
			strings.Contains(errMsg, "connection reset") ||
			strings.Contains(errMsg, "stream closed") ||
			strings.Contains(errMsg, "broken pipe") {
			return CodexErrorKindSoftFail
		}
		return CodexErrorKindServer
	}
	if resp != nil && resp.StatusCode >= 500 {
		return CodexErrorKindServer
	}
	return CodexErrorKindFatal
}
