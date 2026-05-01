package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
)

func TestClassifyCodexError(t *testing.T) {
	t.Run("rate limit by status", func(t *testing.T) {
		err := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
		if got := ClassifyCodexError(nil, nil, err); got != CodexErrorKindRateLimit {
			t.Fatalf("expected rate limit, got %s", got)
		}
	})

	t.Run("auth by status", func(t *testing.T) {
		err := types.NewErrorWithStatusCode(errors.New("unauthorized"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized)
		if got := ClassifyCodexError(nil, nil, err); got != CodexErrorKindAuth {
			t.Fatalf("expected auth, got %s", got)
		}
	})

	t.Run("soft fail by transport error", func(t *testing.T) {
		if got := ClassifyCodexError(nil, errors.New("stream closed early"), nil); got != CodexErrorKindSoftFail {
			t.Fatalf("expected soft fail, got %s", got)
		}
	})

	t.Run("usage limit text is treated as rate limit", func(t *testing.T) {
		err := types.NewErrorWithStatusCode(errors.New("The usage limit has been reached"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
		if got := ClassifyCodexError(nil, nil, err); got != CodexErrorKindRateLimit {
			t.Fatalf("expected rate limit, got %s", got)
		}
	})

	t.Run("usage limit variant is treated as rate limit", func(t *testing.T) {
		err := types.NewErrorWithStatusCode(errors.New("You've hit your usage limit. Try again later."), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
		if got := ClassifyCodexError(nil, nil, err); got != CodexErrorKindRateLimit {
			t.Fatalf("expected rate limit, got %s", got)
		}
	})

	t.Run("invalid key text is treated as invalid", func(t *testing.T) {
		err := types.NewErrorWithStatusCode(errors.New("codex channel: key must be a JSON object"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		if got := ClassifyCodexError(nil, nil, err); got != CodexErrorKindInvalid {
			t.Fatalf("expected invalid key, got %s", got)
		}
	})
}
