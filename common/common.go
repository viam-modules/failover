// package common
package common

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
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
		fmt.Println("last working had no errors")
		return nil
	}

	// If the lastWorkingsensor failed, we need to find a replacement, loop through the backups until one of them succeeds for all API calls.
	for _, backup := range b.BackupList {
		// Already tried it.
		if backup == b.LastWorkingSensor {
			continue
		}
		fmt.Println("hereeeee calling the backups")
		// Loop through all API calls and record the errors
		err := CallAllFunctions(ctx, backup, b.Timeout, extra, calls)
		if err != nil {
			continue
		}
		// all calls were successful, replace lastworkingsensor
		b.LastWorkingSensor = backup
		return nil
	}
	return errors.New("all sensors failed")

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

type Primary struct {
	Logger          logging.Logger
	S               resource.Sensor
	Workers         rdkutils.StoppableWorkers
	PollPrimaryChan chan bool
	Timeout         int
	UsePrimary      bool
	Calls           []func(context.Context, resource.Sensor, map[string]any) (any, error)
}

func CreatePrimary() Primary {
	primary := Primary{
		Workers:         rdkutils.NewStoppableWorkers(),
		PollPrimaryChan: make(chan bool),
		UsePrimary:      true,
	}

	// Start goroutine to check health of the primary sensor
	primary.PollPrimaryForHealth()
	return primary
}

func TryPrimary[T any](ctx context.Context, s *Primary, extra map[string]any, call func(context.Context, resource.Sensor, map[string]any) (any, error)) (T, error) {
	readings, err := TryReadingOrFail(ctx, s.Timeout, s.S, call, extra)
	if err == nil {
		reading := any(readings).(T)
		return reading, nil
	}
	var zero T

	// upon error of the last working sensor, log the error.
	s.Logger.Warnf("primary sensor failed: %s", err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	s.PollPrimaryChan <- true
	s.UsePrimary = false
	return zero, err
}

func (p *Primary) PollPrimaryForHealth() {
	// poll every 10 ms.
	ticker := time.NewTicker(time.Millisecond * 10)
	p.Workers.AddWorkers(func(ctx context.Context) {
		for {
			select {
			// wait for data to come into the channel before polling.
			case <-ctx.Done():
				return
			case <-p.PollPrimaryChan:
				fmt.Println("polling primary!!")
			}
			// label for loop so we can break out of it later.
		L:
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					err := CallAllFunctions(ctx, p.S, p.Timeout, nil, p.Calls)
					if err == nil {
						//TODO: mutex
						p.UsePrimary = true
						break L
					}
				}
			}
		}
	})
}

func (p *Primary) Close() {
	close(p.PollPrimaryChan)
	if p.Workers != nil {
		p.Workers.Stop()
	}
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
