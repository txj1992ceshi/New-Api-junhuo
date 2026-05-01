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
	CodexErrorKindInvalid   CodexErrorKind = "invalid_key"
	CodexErrorKindFatal     CodexErrorKind = "fatal"
)

func ClassifyCodexError(resp *http.Response, reqErr error, apiErr *types.NewAPIError) CodexErrorKind {
	if reqErr == nil && apiErr == nil {
		return CodexErrorKindSuccess
	}
	if apiErr != nil {
		errMsg := strings.ToLower(apiErr.Error())
		switch {
		case strings.Contains(errMsg, "key must be a json object"),
			strings.Contains(errMsg, "invalid oauth key json"),
			strings.Contains(errMsg, "access_token is required"),
			strings.Contains(errMsg, "account_id is required"):
			return CodexErrorKindInvalid
		}
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests:
			return CodexErrorKindRateLimit
		case http.StatusUnauthorized, http.StatusForbidden:
			return CodexErrorKindAuth
		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return CodexErrorKindServer
		}
		switch {
		case strings.Contains(errMsg, "rate limit"),
			strings.Contains(errMsg, "rate limited"),
			strings.Contains(errMsg, "too many requests"),
			strings.Contains(errMsg, "usage limit has been reached"),
			strings.Contains(errMsg, "you've hit your usage limit"),
			strings.Contains(errMsg, "usage_limit"):
			return CodexErrorKindRateLimit
		case strings.Contains(errMsg, "unauthorized"), strings.Contains(errMsg, "forbidden"), strings.Contains(errMsg, "invalid token"):
			return CodexErrorKindAuth
		case strings.Contains(errMsg, "timeout"), strings.Contains(errMsg, "eof"), strings.Contains(errMsg, "connection reset"), strings.Contains(errMsg, "stream closed"):
			return CodexErrorKindSoftFail
		}
	}
	if reqErr != nil {
		errMsg := strings.ToLower(reqErr.Error())
		if strings.Contains(errMsg, "invalid oauth key json") ||
			strings.Contains(errMsg, "key must be a json object") ||
			strings.Contains(errMsg, "access_token is required") ||
			strings.Contains(errMsg, "account_id is required") {
			return CodexErrorKindInvalid
		}
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
