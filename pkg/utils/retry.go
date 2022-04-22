package utils

import (
	"context"
	"errors"
	"time"

	"github.com/jpillora/backoff"
)

var (
	// ErrCancelled indicates the operation was cancelled
	ErrCancelled = errors.New("operation cancelled")
	// ErrReachMaxAttempts indicates we hit the limit
	ErrReachMaxAttempts = errors.New("reached max attempts")
)

const (
	// MaxAttempts is the max attempts
	MaxAttempts = 99999999
)

// RetryFunc performs the operation. It should return true if the operation is complete, false if
// it should be retried, and an error if an error which should NOT be retried has occurred.
type RetryFunc func() (bool, error)

// RetryWithTimeout creates a retry with a specific timeout
func RetryWithTimeout(ctx context.Context, timeout, interval time.Duration, retryFn RetryFunc) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return Retry(ctx, 0, true, interval, retryFn)
}

// Retry is used to retry an operation multiple times under a context. If the retryFn returns false
// with no error, the operation will be retried.
func Retry(ctx context.Context, attempts int, jitter bool, minInterval time.Duration, retryFn RetryFunc) error {
	// @hack: quick way to do this for now
	if attempts == 0 {
		attempts = MaxAttempts
	}

	backoff := &backoff.Backoff{
		Min:    minInterval,
		Max:    minInterval * 2,
		Factor: 1.5,
		Jitter: jitter,
	}

	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return ErrCancelled
		default:
		}

		finished, err := retryFn()
		if err != nil {
			return err
		}
		if finished {
			return nil
		}

		Sleep(ctx, backoff.Duration())
	}

	return ErrReachMaxAttempts
}

// Sleep provides a default sleep but with a cancellable context
func Sleep(ctx context.Context, sleep time.Duration) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(sleep):
	}

	return false
}
