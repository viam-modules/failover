// Package failoversensor implements a sensor that specifies primary and backup sensors in case of failure.
package failoversensor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"failover/common"

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

	if len(cfg.Backups) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "backups")
	}

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
		Named:   conf.ResourceName().AsNamed(),
		logger:  logger,
		workers: rdkutils.NewStoppableWorkers(),
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
			s.logger.Errorf(err.Error())
		} else {
			s.backups = append(s.backups, backup)
		}
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
	resource.Named

	logger  logging.Logger
	workers rdkutils.StoppableWorkers

	primary sensor.Sensor
	backups []sensor.Sensor
	timeout int

	mu                sync.Mutex
	lastWorkingSensor sensor.Sensor
}

func (s *failoverSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {

	// Poll the last sensor we know is working
	readings, err := common.TryReadingOrFail(ctx, s.timeout, s.lastWorkingSensor, s.lastWorkingSensor.Readings, extra)
	if readings != nil {
		return readings, nil
	}
	// upon error of the last working sensor, logthe  error.
	s.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch s.lastWorkingSensor {
	case s.primary:
		// Check if the primary comes back online and replace the lastWorkingSensor
		// with primary when it does.
		s.pollPrimaryForHealth(extra)
	default:
	}

	readings, err = s.tryBackups(ctx, extra)
	if err != nil {
		return nil, err
	}
	return readings, nil
}

func (s *failoverSensor) tryBackups(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	// Lock the mutex to protect lastWorkingSensor from changing before the readings call finishes.
	s.mu.Lock()
	defer s.mu.Unlock()
	// Start reading from the list of backup sensors until one succeeds.
	for _, backup := range s.backups {
		// if the last working sensor is a backup, it was already tried above.
		if s.lastWorkingSensor == backup {
			continue
		}
		s.logger.Infof("calling backup %s", backup.Name())
		reading, err := common.TryReadingOrFail(ctx, s.timeout, backup, backup.Readings, extra)
		if err != nil {
			s.logger.Warn(err.Error())
		} else {
			s.logger.Infof("successfully got reading from %s", backup.Name())
			s.lastWorkingSensor = backup
			return reading, nil
		}
	}
	return nil, fmt.Errorf("all sensors failed to get readings")
}

// pollPrimaryForHealth starts a background routine that continuously polls the primary sensor until it returns a reading.
func (s *failoverSensor) pollPrimaryForHealth(extra map[string]interface{}) {
	// poll every 10 ms.
	ticker := time.NewTicker(time.Millisecond * 10)
	s.workers.AddWorkers(func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := common.TryReadingOrFail(ctx, s.timeout, s.primary, s.primary.Readings, extra)
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

// Close closes the sensor.
func (s *failoverSensor) Close(ctx context.Context) error {
	if s.workers != nil {
		s.workers.Stop()
	}
	return nil
}
