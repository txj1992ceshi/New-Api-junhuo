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
}
