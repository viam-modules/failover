package common

import (
	"context"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// Primary defines the primary sensor for the failover.
type Primary struct {
	workers         rdkutils.StoppableWorkers
	logger          logging.Logger
	primarySensor   resource.Sensor
	pollPrimaryChan chan bool
	timeout         int
	UsePrimary      bool
	calls           []func(context.Context, resource.Sensor, map[string]any) (any, error)
}

func CreatePrimary(ctx context.Context, timeout int, logger logging.Logger, primarySensor resource.Sensor, calls []func(context.Context, resource.Sensor, map[string]any) (any, error)) Primary {
	primary := Primary{
		workers:         rdkutils.NewStoppableWorkers(),
		pollPrimaryChan: make(chan bool),
		UsePrimary:      true,
		timeout:         timeout,
		primarySensor:   primarySensor,
		logger:          logger,
		calls:           calls,
	}

	// Start goroutine to check health of the primary sensor
	primary.PollPrimaryForHealth()

	// TryAllReadings to determine the health of the primary sensor.
	primary.TryAllReadings(ctx)

	return primary
}

// Check that all functions on primary are working, if not tell the goroutine to start polling for health and don't use the primary.
func (primary *Primary) TryAllReadings(ctx context.Context) {
	err := CallAllFunctions(ctx, primary.primarySensor, primary.timeout, nil, primary.calls)
	if err != nil {
		primary.UsePrimary = false
		primary.pollPrimaryChan <- true
	}
}

// TryPrimary is a helper function to call a reading from the primary sensor and start polling if it fails.
func TryPrimary[T any](ctx context.Context, s *Primary, extra map[string]any, call func(context.Context, resource.Sensor, map[string]any) (any, error)) (T, error) {
	readings, err := TryReadingOrFail(ctx, s.timeout, s.primarySensor, call, extra)
	if err == nil {
		reading := any(readings).(T)
		return reading, nil
	}
	var zero T

	// upon error of the last working sensor, log the error.
	s.logger.Warnf("primary sensor failed: %s", err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	s.pollPrimaryChan <- true
	s.UsePrimary = false
	return zero, err
}

// PollPrimaryForHealth starts a goroutine and waits for data to come into the pollPrimaryChan.
// Then, it calls all APIs on the primary sensor until they are all successfull and updates the
// the UsePrimary flag.
func (p *Primary) PollPrimaryForHealth() {
	// poll every 10 ms.
	ticker := time.NewTicker(time.Millisecond * 10)
	p.workers.AddWorkers(func(ctx context.Context) {
		for {
			select {
			// wait for data to come into the channel before polling.
			case <-ctx.Done():
				return
			case <-p.pollPrimaryChan:
			}
			// label for loop so we can break out of it later.
		L:
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					err := CallAllFunctions(ctx, p.primarySensor, p.timeout, nil, p.calls)
					// Primary succeeded, set flag to true
					if err == nil {
						p.UsePrimary = true
						break L
					}
				}
			}
		}
	})
}

func (p *Primary) Close() {
	close(p.pollPrimaryChan)
	if p.workers != nil {
		p.workers.Stop()
	}
}
