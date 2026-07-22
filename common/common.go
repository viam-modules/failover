// Package common contains all common functions
package common

import (
	"context"
	"errors"
	"runtime"
	"time"

	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

// Config is used for converting config attributes.
type Config struct {
	Primary string   `json:"primary"`
	Backups []string `json:"backups"`
	Timeout int      `json:"timeout_ms,omitempty"`
}

// Call defines a general API call.
type Call = func(context.Context, resource.Sensor, map[string]any) (any, error)

// Validate performs config validation.
func (cfg Config) Validate(path string) ([]string, []string, error) {
	var deps []string
	if cfg.Primary == "" {
		return nil, nil, utils.NewConfigValidationFieldRequiredError(path, "primary")
	}
	deps = append(deps, cfg.Primary)

	if len(cfg.Backups) == 0 {
		return nil, nil, utils.NewConfigValidationFieldRequiredError(path, "backups")
	}

	deps = append(deps, cfg.Backups...)

	return deps, nil, nil
}

// CallAllFunctions is a helper to call all the inputted functions and return if one errors.
func CallAllFunctions(ctx context.Context,
	s resource.Sensor,
	timeout int,
	extra map[string]interface{},
	calls []Call,
) error {
	for _, call := range calls {
		_, err := TryReadingOrFail(ctx, timeout, s, call, extra)
		// one of them errored, return
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadingsResult struct to return readings and error
// Go does not allow channels containing a tuple,
// so defining the struct with readings and error
// to send through a channel.
type ReadingsResult struct {
	readings any
	err      error
}

// TryReadingOrFail will call the inputted API and either error, timeout, or return the reading.
func TryReadingOrFail[K any](ctx context.Context,
	timeout int,
	s resource.Sensor,
	call func(context.Context, resource.Sensor, map[string]any) (K, error),
	extra map[string]any) (
	K, error,
) {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Buffer so the worker can exit even if the caller has already timed out/canceled.
	resultChan := make(chan ReadingsResult, 1)
	var zero K
	go func() {
		reading, err := call(cancelCtx, s, extra)
		resultChan <- ReadingsResult{readings: reading, err: err}
	}()

	timer := time.NewTimer(time.Duration(timeout) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-timer.C:
		// timed out - the context passed into the API call will be canceled on return.
		return zero, errors.New("sensor timed out")
	case result := <-resultChan:
		if result.err != nil {
			return zero, result.err
		}
		return result.readings.(K), nil
	}
}

// ReadingsWrapper wraps Readings API.
// Since all sensors implement readings we can reuse the same wrapper for all models.
func ReadingsWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	readings, err := s.Readings(ctx, extra)
	if err != nil {
		return nil, err
	}
	return readings, err
}

// WaitForGoroutineCount waits until runtime.NumGoroutine() is at most want,
// or until d elapses. Timed-out sensor reads intentionally return before their
// background call goroutines exit; tests should use this instead of comparing
// NumGoroutine immediately after Close.
func WaitForGoroutineCount(want int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if runtime.NumGoroutine() <= want {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}
