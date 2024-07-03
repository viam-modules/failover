// Package failoversensor implements a sensor that specifies primary and backup sensors in case of failure.
package failoversensor

import (
	"context"
	"fmt"

	"failover/common"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var (
	// Model defines triplet name.
	Model = resource.NewModel("viam", "failover", "sensor")
)

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

	s.primary = common.CreatePrimary()
	s.primary.Logger = logger
	s.primary.S = primary
	s.backups = common.Backups{}

	for _, backup := range config.Backups {
		backup, err := sensor.FromDependencies(deps, backup)
		if err != nil {
			s.logger.Errorf(err.Error())
		} else {
			s.backups.BackupList = append(s.backups.BackupList, backup)
		}
	}

	s.backups.LastWorkingSensor = s.backups.BackupList[0]

	// default timeout is 1 second.
	s.timeout = 1000
	s.primary.Timeout = 1000
	s.backups.Timeout = 1000
	if config.Timeout > 0 {
		s.timeout = config.Timeout
	}
	return s, nil
}

type failoverSensor struct {
	resource.AlwaysRebuild
	resource.Named

	logger logging.Logger

	primary common.Primary
	backups common.Backups
	timeout int
}

func (s *failoverSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	if s.primary.UsePrimary {
		readings, err := common.TryPrimary[map[string]any](ctx, &s.primary, extra, common.ReadingsWrapper)
		if err == nil {
			return readings, nil
		}
	}

	fmt.Println("here")

	err := s.backups.GetWorkingSensor(ctx, extra, common.ReadingsWrapper)
	if err != nil {
		return nil, err
	}

	fmt.Println("updated last working")
	readings, err := common.TryReadingOrFail(ctx, s.timeout, s.backups.LastWorkingSensor, common.ReadingsWrapper, extra)
	if err != nil {
		return nil, err
	}

	fmt.Println("here -2 ")

	reading := readings.(map[string]interface{})
	return reading, nil
}

// Close closes the sensor.
func (s *failoverSensor) Close(ctx context.Context) error {
	s.primary.Close()
	return nil
}
