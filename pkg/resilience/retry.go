package resilience

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
)

func WithBackoff(ctx context.Context, fn func() error) error {
	return retry.Do(fn,
		retry.Context(ctx),
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
		retry.DelayType(retry.BackOffDelay),
	)
}
