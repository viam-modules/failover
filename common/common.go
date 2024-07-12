// package common
package common

import (
	"context"
	"errors"
	"fmt"
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

// Validate performs config validation.
func (cfg Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.Primary == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "primary")
	}
	deps = append(deps, cfg.Primary)

	if len(cfg.Backups) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "backups")
	}

	deps = append(deps, cfg.Backups...)

	return deps, nil
}

// CallAllFunctions is a helper to call all the inputted functions and return if one errors.
func CallAllFunctions(ctx context.Context, s resource.Sensor, timeout int, extra map[string]interface{}, calls []func(context.Context, resource.Sensor, map[string]any) (any, error)) error {
	for _, call := range calls {
		_, err := TryReadingOrFail(ctx, timeout, s, call, extra)
		// one of them errored, return
		if err != nil {
			return err
		}
	}
	return nil

}

// Go does not allow channels containing a tuple,
// so defining the struct with readings and error
// to send through a channel.
type ReadingsResult struct {
	readings any
	err      error
}

// getReading calls the inputted API call and returns the reading and error as a ReadingsResult struct.
func getReading[K any](ctx context.Context, call func(context.Context, resource.Sensor, map[string]any) (K, error), s resource.Sensor, extra map[string]any) ReadingsResult {
	reading, err := call(ctx, s, extra)

	result := ReadingsResult{
		readings: reading,
		err:      err,
	}

	return result

}

// TryReadingorFail will call the inputted API and either error, timeout, or return the reading.
func TryReadingOrFail[K any](ctx context.Context,
	timeout int,
	s resource.Sensor,
	call func(context.Context, resource.Sensor, map[string]any) (K, error),
	extra map[string]any) (
	K, error,
) {
	cancelCtx, cancel := context.WithCancel(ctx)
	resultChan := make(chan ReadingsResult)
	var zero K
	go func() {
		select {
		case <-cancelCtx.Done():
			return
		case resultChan <- getReading(cancelCtx, call, s, extra):
		}
	}()
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
		// timed out - cancel the context given to the API call and return
		cancel()
		return zero, errors.New("sensor timed out")
	case result := <-resultChan:
		if result.err != nil {
			return zero, fmt.Errorf("failed to get readings: %w", result.err)
		} else {
			return result.readings.(K), nil
		}
	}
}

// Since all sensors implement readings we can reuse the same wrapper for all models.
func ReadingsWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	readings, err := s.Readings(ctx, extra)
	if err != nil {
		return nil, err
	}
	return readings, err
}
