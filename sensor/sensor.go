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
)

var (
	// Model defines triplet name.
	Model            = resource.NewModel("viam", "failover", "sensor")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.RegisterComponent(sensor.API, Model,
		resource.Registration[sensor.Sensor, *Config]{
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
func (cfg *Config) Validate(path string) ([]string, error) {
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

func newFailoverSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (
	sensor.Sensor, error,
) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &failoverSensor{
		name:       rawConf.ResourceName(),
		logger:     logger,
		cfg:        conf,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	primary, err := sensor.FromDependencies(deps, conf.Primary)
	if err != nil {
		return nil, err
	}
	s.primary = primary
	s.backups = []sensor.Sensor{}

	for _, backup := range conf.Backups {
		backup, err := sensor.FromDependencies(deps, backup)
		if err != nil {
			return nil, err
		}
		s.backups = append(s.backups, backup)
	}

	s.lastWorkingSensor = primary

	// default timeout is 1 second.
	s.timeout = 1000
	if conf.Timeout > 0 {
		s.timeout = conf.Timeout
	}

	return s, nil
}

type failoverSensor struct {
	name   resource.Name
	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()

	primary           sensor.Sensor
	backups           []sensor.Sensor
	timeout           int
	lastWorkingSensor sensor.Sensor
	mu                sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
	resource.AlwaysRebuild
}

func (s *failoverSensor) Name() resource.Name {
	return s.name
}

type readingsResult struct {
	reading map[string]interface{}
	Error   error
}

func getReading(ctx context.Context, sensor resource.Sensor, extra map[string]interface{}) readingsResult {
	readings, err := sensor.Readings(ctx, extra)
	return readingsResult{
		reading: readings,
		Error:   err,
	}
}

func (s *failoverSensor) tryReadingOrFail(ctx context.Context, sensor sensor.Sensor, extra map[string]interface{}) map[string]interface{} {
	result := make(chan readingsResult, 1)
	go func() {
		result <- getReading(ctx, sensor, extra)
	}()
	select {
	case <-time.After(time.Duration(s.timeout * 1e6)):
		s.logger.Infof("%s timed out", sensor.Name())
		return nil
	case result := <-result:
		if result.Error != nil {
			s.logger.Infof("sensor %s failed to get readings: %s", sensor.Name(), result.Error.Error())
			return nil
		} else {
			return result.reading
		}
	}
}

func (s *failoverSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	reading := s.tryReadingOrFail(ctx, s.lastWorkingSensor, extra)
	if reading != nil {
		return reading, nil
	}

	// primary failed, start goroutine to check for it to get readings again.
	s.mu.Lock()
	if s.lastWorkingSensor == s.primary {
		s.activeBackgroundWorkers.Add(1)
		s.pollForHealth(s.cancelCtx, extra)
	}
	s.mu.Unlock()

	// try the backups.
	for _, backup := range s.backups {
		// already tried this one.
		s.mu.Lock()
		if backup == s.lastWorkingSensor {
			continue
		}
		s.mu.Unlock()
		s.logger.Infof("calling backup %s", backup.Name())
		reading := s.tryReadingOrFail(ctx, backup, extra)
		if reading != nil {
			s.mu.Lock()
			s.lastWorkingSensor = backup
			s.mu.Unlock()
			return reading, nil
		}
	}
	// couldnt get reading from any sensors.
	return nil, fmt.Errorf("failover %s: all sensors failed to get readings", s.Name())
}

func (s *failoverSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	s.logger.Error("DoCommand method unimplemented")
	return nil, errUnimplemented
}

// Close closes the sensor.
func (s *failoverSensor) Close(ctx context.Context) error {
	s.cancelFunc()
	return nil
}

func (s *failoverSensor) pollForHealth(ctx context.Context, extra map[string]interface{}) {
	utils.ManagedGo(func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				result := make(chan readingsResult, 1)
				go func() {
					result <- getReading(ctx, s.primary, extra)
				}()
				select {
				case <-time.After(time.Duration(s.timeout * 1e6)):
				case result := <-result:
					if result.reading != nil {
						s.logger.Infof("primary sensor successfully got reading")
						s.mu.Lock()
						s.lastWorkingSensor = s.primary
						s.mu.Unlock()
						return
					}
				}
			}
		}
	}, s.activeBackgroundWorkers.Done)
}
