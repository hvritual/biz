package bizruntime

import (
	"errors"
	"net/http"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

func verificationHTTPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, domain.ErrVerificationRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, domain.ErrVerificationConsumed), errors.Is(err, domain.ErrVerificationConflict):
		return http.StatusConflict
	case errors.Is(err, domain.ErrVerificationExpired):
		return http.StatusGone
	case errors.Is(err, domain.ErrNotificationUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

func verificationRetryAfter(err error) time.Duration {
	var rate domain.RateLimitError
	if errors.As(err, &rate) && rate.RetryAfter > 0 {
		return rate.RetryAfter
	}
	return 0
}
