package common

import (
	"context"
	"sync"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	viamutils "go.viam.com/utils"
)

// Primary defines the primary sensor for the failover.
type Primary struct {
	workers         *viamutils.StoppableWorkers
	logger          logging.Logger
	primarySensor   resource.Sensor
	pollPrimaryChan chan bool
	timeout         int

	mu         sync.Mutex
	usePrimary bool

	calls []Call
}

func CreatePrimary(ctx context.Context,
	timeout int,
	logger logging.Logger,
	primarySensor resource.Sensor,
	calls []Call,
) *Primary {
	primary := &Primary{
		workers:         viamutils.NewBackgroundStoppableWorkers(),
		pollPrimaryChan: make(chan bool, 1),
		usePrimary:      true,
		timeout:         timeout,
		primarySensor:   primarySensor,
		logger:          logger,
		calls:           calls,
	}

	// Start goroutine to check health of the primary sensor
	primary.PollPrimaryForHealth()

	// TryAllReadings to determine the health of the primary sensor and set the usePrimary flag accordingly.
	primary.TryAllReadings(ctx)

	return primary
}

func (p *Primary) UsePrimary() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.usePrimary
}

func (p *Primary) setUsePrimary(val bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.usePrimary = val
}

// signalPoll asks the health-poll worker to start checking the primary.
// Non-blocking so we never deadlock if the worker is already polling.
func (p *Primary) signalPoll() {
	select {
	case p.pollPrimaryChan <- true:
	default:
	}
}

// TryAllReadings checks that all functions on primary are working,
// if not tell the goroutine to start polling for health and don't use the primary.
func (p *Primary) TryAllReadings(ctx context.Context) {
	err := CallAllFunctions(ctx, p.primarySensor, p.timeout, nil, p.calls)
	if err != nil {
		p.logger.Warnf("primary sensor failed: %s", err.Error())
		p.setUsePrimary(false)
		p.signalPoll()
	}
}

// TryPrimary is a helper function to call a reading from the primary sensor and start polling if it fails.
func TryPrimary[T any](ctx context.Context,
	s *Primary,
	extra map[string]any,
	call Call,
) (T, error) {
	readings, err := TryReadingOrFail(ctx, s.timeout, s.primarySensor, call, extra)
	if err == nil {
		reading := any(readings).(T)
		return reading, nil
	}

	var zero T

	// upon error of the last working sensor, log the error.
	s.logger.Warnf("primary sensor failed: %s", err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	s.signalPoll()
	s.setUsePrimary(false)

	return zero, err
}

// PollPrimaryForHealth starts a goroutine and waits for data to come into the pollPrimaryChan.
// Then, it calls all APIs on the primary sensor until they are all successful and updates the
// UsePrimary flag.
func (p *Primary) PollPrimaryForHealth() {
	p.workers.Add(func(ctx context.Context) {
		// poll every 100 ms.
		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()

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
						p.setUsePrimary(true)
						break L
					}
				}
			}
		}
	})
}

func (p *Primary) Close() {
	// Stop workers first so the poll goroutine can exit via ctx.Done().
	// Closing the channel before Stop can spuriously wake the worker and race
	// with shutdown, leaving briefly-lived goroutines that flake leak checks.
	if p.workers != nil {
		p.workers.Stop()
	}
}
