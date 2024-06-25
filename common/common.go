// package common
package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.viam.com/rdk/components/sensor"
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

// PowerSensor has special case since some of the functions returns 3 values.
func getPSReading[K any, R any](ctx context.Context, call func(context.Context, map[string]interface{}) (K, R, error), extra map[string]interface{}) ReadingsResult {
	reading1, reading2, err := call(ctx, extra)

	readings := map[string]interface{}{"1": reading1, "2": reading2}

	return ReadingsResult{
		readings: readings,
		err:      err,
	}
}

func TryReadingOrFail2Returns[K any, R any](ctx context.Context,
	timeout int,
	s sensor.Sensor,
	call func(context.Context, map[string]interface{}) (K, R, error),
	extra map[string]interface{}) (
	K, R, error) {

	resultChan := make(chan ReadingsResult, 1)
	var zeroK K
	var zeroR R
	go func() {
		resultChan <- getPSReading(ctx, call, extra)
	}()
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
		return zeroK, zeroR, fmt.Errorf("%s timed out", s.Name())
	case result := <-resultChan:
		if result.err != nil {
			return zeroK, zeroR, fmt.Errorf("sensor %s failed to get readings: %w", s.Name(), result.err)
		} else {
			readings := result.readings.(map[string]interface{})
			return readings["1"].(K), readings["2"].(R), nil
		}
	}
}

func getReading[K any](ctx context.Context, call func(context.Context, resource.Sensor, map[string]interface{}) (K, error), sensor resource.Sensor, extra map[string]interface{}) ReadingsResult {
	reading, err := call(ctx, sensor, extra)

	return ReadingsResult{
		readings: reading,
		err:      err,
	}
}

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
		return zero, fmt.Errorf("%s timed out", "name")
	case result := <-resultChan:
		if result.err != nil {
			return zero, fmt.Errorf("sensor %s failed to get readings: %w", "name", result.err)
		} else {
			return result.readings.(K), nil
		}
	}
}

// Since all sensors implement sensor readings can reuse the same wrapper.
func ReadingsWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	sensor, ok := ps.(sensor.Sensor)
	if !ok {
		return nil, errors.New("failed")
	}

	readings, err := sensor.Readings(ctx, extra)
	if err != nil {
		return nil, err
	}

	return readings, err
}
