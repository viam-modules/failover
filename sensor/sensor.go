// Package failoversensor implements a sensor that specifies primary and backup sensors in case of failure.
package failoversensor

import (
	"context"
	"errors"
	"failover/common"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// Model defines triplet name.
var Model = resource.NewModel("viam", "failover", "sensor")

func init() {
	resource.RegisterComponent(sensor.API, Model,
		resource.Registration[sensor.Sensor, common.Config]{
			Constructor: newFailoverSensor,
		},
	)
}

func newFailoverSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (
	sensor.Sensor, error,
) {
	config, err := resource.NativeConfig[common.Config](conf)
	if err != nil {
		return nil, err
	}

	s := &failoverSensor{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	primary, err := sensor.FromDependencies(deps, config.Primary)
	if err != nil {
		return nil, err
	}

	s.timeout = 1000
	if config.Timeout > 0 {
		s.timeout = config.Timeout
	}

	backups := []resource.Sensor{}

	for _, backup := range config.Backups {
		backup, err := sensor.FromDependencies(deps, backup)
		if err != nil {
			s.logger.Errorf(err.Error())
		}
		backups = append(backups, backup)
	}

	calls := []common.Call{common.ReadingsWrapper}

	s.primary = common.CreatePrimary(ctx, s.timeout, logger, primary, calls)
	s.backups = common.CreateBackup(s.timeout, backups, calls)
	return s, nil
}

type failoverSensor struct {
	resource.AlwaysRebuild
	resource.Named

	logger logging.Logger

	primary *common.Primary
	backups *common.Backups
	timeout int
}

func (s *failoverSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	// If UsePrimary flag is set, call readings on primary sensor and return if no error.
	if s.primary.UsePrimary() {
		readings, err := common.TryPrimary[map[string]any](ctx, s.primary, extra, common.ReadingsWrapper)
		if err == nil {
			return readings, nil
		}
	}

	// if primary failed, update the backups lastworkingsensor
	workingSensor, err := s.backups.GetWorkingSensor(ctx, extra)
	if err != nil {
		return nil, fmt.Errorf("all sensors failed to get readings: %w", err)
	}

	// Call readings on the lastworkingsensor
	readings, err := common.TryReadingOrFail(ctx, s.timeout, workingSensor, common.ReadingsWrapper, extra)
	if err != nil {
		return nil, fmt.Errorf("all sensors failed to get readings: %w", err)
	}

	reading, ok := readings.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to get readings: type assertion failed")
	}
	return reading, nil
}

// Close closes the sensor.
func (s *failoverSensor) Close(ctx context.Context) error {
	s.primary.Close()
	return nil
}
