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

// Go does not allow channels containing a tuple,
// so defining the struct with readings and error
// to send through a channel.
type ReadingsResult struct {
	readings any
	err      error
}

// getReading calls the inputted API call and returns the reading and error as a ReadingsResult struct.
func getReading[K any](ctx context.Context, call func(context.Context, resource.Sensor, map[string]interface{}) (K, error), sensor resource.Sensor, extra map[string]interface{}) ReadingsResult {
	reading, err := call(ctx, sensor, extra)

	return ReadingsResult{
		readings: reading,
		err:      err,
	}
}

// TryReadingorFail will call the inputted API and either error, timeout, or return the reading.
func TryReadingOrFail[K any](ctx context.Context,
	timeout int,
	s resource.Sensor,
	call func(context.Context, resource.Sensor, map[string]interface{}) (K, error),
	extra map[string]interface{}) (
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
func ReadingsWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (any, error) {
	readings, err := s.Readings(ctx, extra)
	if err != nil {
		return nil, err
	}
	return readings, err
}

// Helper function to get the reading from the map and convert is type.
func GetReadingFromMap[T any](m map[string]interface{}, reading string) (T, error) {
	var zero T
	r, ok := m[reading]
	if !ok {
		return zero, errors.New("failed to get reading from map")
	}
	ret, ok := any(r).(T)
	if !ok {
		return zero, errors.New("reading failed type assertion")
	}

	return ret, nil
}

func Get2ReadingsFromMap[T any, R any](m map[string]interface{}, reading1 string, reading2 string) (T, R, error) {
	var zeroT T
	var zeroR R

	ret1, err := GetReadingFromMap[T](m, reading1)
	if err != nil {
		return zeroT, zeroR, err
	}

	ret2, err := GetReadingFromMap[R](m, reading2)
	if err != nil {
		return zeroT, zeroR, err
	}

	return ret1, ret2, nil
}
