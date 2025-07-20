package retry

import (
    "context"
    "time"
)

type Operation func() error

func Do(ctx context.Context, attempts int, baseDelay time.Duration, op Operation) error {
    delay := baseDelay
	var lastErr error

    for i := 0; i < attempts; i++ {
        if err := op(); err != nil {
			lastErr = err
			
            if ctx.Err() != nil {
                return ctx.Err()
            }
            time.Sleep(delay)
            delay *= 2
            continue
        }
        return nil
    }
    return lastErr
}
