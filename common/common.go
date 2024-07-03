// package common
package common

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
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

type Backups struct {
	mu                sync.Mutex
	BackupList        []resource.Sensor
	LastWorkingSensor resource.Sensor
	Timeout           int
	Calls             []func(context.Context, resource.Sensor, map[string]any) (any, error)
}

func (b *Backups) GetWorkingSensor(ctx context.Context, extra map[string]interface{}, calls ...func(context.Context, resource.Sensor, map[string]any) (any, error)) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// First call all APIs for the lastworkingsensor, if it works can use the same lastworkingsensor, return
	err := CallAllFunctions(ctx, b.LastWorkingSensor, b.Timeout, extra, calls)
	if err == nil {
		return nil
	}

	// If the lastWorkingsensor failed, we need to find a replacement, loop through the backups until one of them succeeds for all API calls.
	for _, backup := range b.BackupList {
		// Already tried it.
		if backup == b.LastWorkingSensor {
			continue
		}
		// Loop through all API calls and record the errors
		err := CallAllFunctions(ctx, backup, b.Timeout, extra, calls)
		if err != nil {
			return err
		}
		// all calls were successful, replace lastworkingsensor
		b.LastWorkingSensor = backup
		break
	}
	return nil

}

func CallAllFunctions(ctx context.Context, s resource.Sensor, timeout int, extra map[string]interface{}, calls []func(context.Context, resource.Sensor, map[string]any) (any, error)) error {
	var errors *multierror.Error

	for _, call := range calls {
		_, err := TryReadingOrFail(ctx, timeout, s, call, extra)
		errors = multierror.Append(errors, err)
	}
	if errors.ErrorOrNil() == nil {
		return nil
	}
	return errors

}

type primary struct {
	resource.Sensor
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

	return ReadingsResult{
		readings: reading,
		err:      err,
	}
}

// TryReadingorFail will call the inputted API and either error, timeout, or return the reading.
func TryReadingOrFail[K any](ctx context.Context,
	timeout int,
	s resource.Sensor,
	call func(context.Context, resource.Sensor, map[string]any) (K, error),
	extra map[string]any) (
	K, error) {

	resultChan := make(chan ReadingsResult, 1)
	var zero K
	go func() {
		resultChan <- getReading(ctx, call, s, extra)
	}()
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
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
