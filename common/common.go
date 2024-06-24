// package common
package common

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/components/sensor"
)

// Go does not allow channels containing a tuple,
// so defining the struct with readings and error
// to send through a channel.
type ReadingsResult struct {
	readings interface{}
	err      error
}

func getReading[K interface{}](ctx context.Context, call func(context.Context, map[string]interface{}) (K, error), extra map[string]interface{}) ReadingsResult {
	readings, err := call(ctx, extra)

	return ReadingsResult{
		readings: readings,
		err:      err,
	}
}

func TryReadingOrFail[K interface{}](ctx context.Context,
	timeout int,
	s sensor.Sensor,
	call func(context.Context, map[string]interface{}) (K, error),
	extra map[string]interface{}) (
	K, error) {

	resultChan := make(chan ReadingsResult, 1)
	var zero K
	go func() {
		resultChan <- getReading(ctx, call, extra)
	}()
	select {
	case <-time.After(time.Duration(timeout) * time.Millisecond):
		return zero, fmt.Errorf("%s timed out", s.Name())
	case result := <-resultChan:
		if result.err != nil {
			return zero, fmt.Errorf("sensor %s failed to get readings: %w", s.Name(), result.err)
		} else {
			return result.readings.(K), nil
		}
	}
}
