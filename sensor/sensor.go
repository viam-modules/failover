// Package failoversensor implements a sensor that specifies primary and backup sensors in case of failure.
package failoversensor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var (
	// Model defines triplet name.
	Model = resource.NewModel("viam", "failover", "sensor")
)

func init() {
	resource.RegisterComponent(sensor.API, Model,
		resource.Registration[sensor.Sensor, Config]{
			Constructor: newFailoverSensor,
		},
	)
}

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
	deps = append(deps, cfg.Backups...)

	return deps, nil
}

func newFailoverSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (
	sensor.Sensor, error,
) {
	config, err := resource.NativeConfig[Config](conf)
	if err != nil {
		return nil, err
	}

	s := &failoverSensor{
		name:   conf.ResourceName(),
		logger: logger,
	}

	primary, err := sensor.FromDependencies(deps, config.Primary)
	if err != nil {
		return nil, err
	}
	s.primary = primary
	s.backups = []sensor.Sensor{}

	for _, backup := range config.Backups {
		backup, err := sensor.FromDependencies(deps, backup)
		if err != nil {
			return nil, err
		}
		s.backups = append(s.backups, backup)
	}

	s.lastWorkingSensor = primary

	// default timeout is 1 second.
	s.timeout = 1000
	if config.Timeout > 0 {
		s.timeout = config.Timeout
	}

	return s, nil
}

type failoverSensor struct {
	resource.AlwaysRebuild

	logger logging.Logger
	name   resource.Name

	workers rdkutils.StoppableWorkers

	primary sensor.Sensor
	backups []sensor.Sensor
	timeout int

	mu                sync.Mutex
	lastWorkingSensor sensor.Sensor
}

// Go does not allow channels containing a tuple,
// so defining the struct with readings and error
// to send through a channel.
type readingsResult struct {
	readings map[string]interface{}
	err      error
}

func getReading(ctx context.Context, sensor resource.Sensor, extra map[string]interface{}) readingsResult {
	readings, err := sensor.Readings(ctx, extra)
	return readingsResult{
		readings: readings,
		err:      err,
	}
}

func (s *failoverSensor) tryReadingOrFail(ctx context.Context, sensor sensor.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	result := make(chan readingsResult, 1)
	go func() {
		result <- getReading(ctx, sensor, extra)
	}()
	select {
	case <-time.After(time.Duration(s.timeout * 1e6)):
		return nil, fmt.Errorf("%s timed out", sensor.Name())
	case result := <-result:
		if result.err != nil {
			return nil, fmt.Errorf("sensor %s failed to get readings: %s", sensor.Name(), result.err.Error())
		} else {
			return result.readings, nil
		}
	}
}

func (s *failoverSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	reading, err := s.tryReadingOrFail(ctx, s.lastWorkingSensor, extra)
	if reading != nil {
		return reading, nil
	}
	s.logger.Warnf(err.Error())

	// if the primary failed, start goroutine to check for it to get readings again.
	switch s.lastWorkingSensor {
	case s.primary:
		s.pollPrimaryForHealth(extra)
	default:
	}

	for _, backup := range s.backups {
		// Lock the mutex to protect lastWorkingSensor from changing before the readings call finishes.
		s.mu.Lock()
		// if the last working sensor is a backup, it was already tried above.
		if s.lastWorkingSensor == backup {
			continue
		}
		s.mu.Unlock()
		s.logger.Infof("calling backup %s", backup.Name())
		reading, err := s.tryReadingOrFail(ctx, backup, extra)
		if err != nil {
			s.logger.Warnf(err.Error())
		} else {
			s.logger.Infof("successfully got reading from %s", backup.Name())
			s.mu.Lock()
			s.lastWorkingSensor = backup
			s.mu.Unlock()
			return reading, nil
		}
	}
	// couldn't get reading from any sensors.
	return nil, fmt.Errorf("failover %s: all sensors failed to get readings", s.name)
}

func (s *failoverSensor) Name() resource.Name {
	return s.name
}

// Close closes the sensor.
func (s *failoverSensor) Close(ctx context.Context) error {
	if s.workers != nil {
		s.workers.Stop()
	}
	return nil
}

func (s *failoverSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.ErrUnsupported
}

// pollPrimaryForHealth starts a background routine that continuously polls the primary sensor until it returns a reading.
func (s *failoverSensor) pollPrimaryForHealth(extra map[string]interface{}) {
	// poll every 10 ms.
	ticker := time.NewTicker(time.Millisecond * 10)
	s.workers = rdkutils.NewStoppableWorkers(func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := s.tryReadingOrFail(ctx, s.primary, extra)
				if err == nil {
					s.logger.Infof("successfully got reading from primary sensor")
					s.mu.Lock()
					s.lastWorkingSensor = s.primary
					s.mu.Unlock()
					return
				}
			}
		}
	})
}
