package failovermovementsensor

import (
	"context"
	"errors"
	"failover/common"
	"fmt"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"

	rdkutils "go.viam.com/rdk/utils"
)

var Model = resource.NewModel("viam", "failover", "movementsensor")

func init() {
	resource.RegisterComponent(movementsensor.API, Model,
		resource.Registration[movementsensor.MovementSensor, *common.Config]{
			Constructor: newFailoverMovementSensor,
		},
	)
}

type failoverMovementSensor struct {
	resource.AlwaysRebuild
	resource.Named

	primary movementsensor.MovementSensor
	backups []movementsensor.MovementSensor

	logger  logging.Logger
	workers rdkutils.StoppableWorkers

	timeout int

	mu                sync.Mutex
	lastWorkingSensor movementsensor.MovementSensor

	pollVoltageChan  chan bool
	pollCurrentChan  chan bool
	pollPowerChan    chan bool
	pollReadingsChan chan bool
}

func newFailoverMovementSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (movementsensor.MovementSensor, error) {
	conf, err := resource.NativeConfig[common.Config](rawConf)
	if err != nil {
		return nil, err
	}

	s := &failoverMovementSensor{
		logger: logger,
	}

	primary, err := movementsensor.FromDependencies(deps, conf.Primary)
	if err != nil {
		return nil, err
	}
	s.primary = primary
	s.backups = []movementsensor.MovementSensor{}

	for _, backup := range conf.Backups {
		backup, err := movementsensor.FromDependencies(deps, backup)
		if err != nil {
			s.logger.Errorf(err.Error())
		} else {
			s.backups = append(s.backups, backup)
		}
	}

	s.lastWorkingSensor = primary

	// default timeout is 1 second.
	s.timeout = 1000
	if conf.Timeout > 0 {
		s.timeout = conf.Timeout
	}

	return s, nil
}

func (ms *failoverMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return nil, 0, nil
}

func (s *failoverMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	// Poll the last sensor we know is working
	reading, err := common.TryReadingOrFail(ctx, s.timeout, s.lastWorkingSensor, LinearVelocityWrapper, extra)
	if err == nil {
		vel, err := common.GetReadingFromMap[r3.Vector](reading, "velocity")
		if err == nil {
			return vel, nil
		}
	}
	// upon error of the last working sensor, log returned error.
	s.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.

	// Start reading from the list of backup sensors until one succeeds.
	reading, err = tryBackups(ctx, s, LinearVelocityWrapper, extra)
	if err != nil {
		return r3.Vector{}, errors.New("all movement sensors failed to get linear velocity")
	}
	return reading["velocity"].(r3.Vector), nil
}

func (s *failoverMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, nil
}

func (s *failoverMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (s *failoverMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, nil
}

func (s *failoverMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return nil, nil
}

func (s *failoverMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	l, _ := s.LinearVelocity(ctx, extra)
	return map[string]interface{}{"linear": l}, nil
}

func (s *failoverMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error,
) {
	return nil, nil
}

func (s *failoverMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		PositionSupported:       true,
		OrientationSupported:    true,
		CompassHeadingSupported: true,
		LinearVelocitySupported: true,
	}, nil
}

func tryBackups[T any](ctx context.Context,
	ps *failoverMovementSensor,
	call func(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (T, error),
	extra map[string]interface{}) (
	T, error) {
	// Lock the mutex to protect lastWorkingSensor from changing before the readings call finishes.
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var zero T
	for _, backup := range ps.backups {
		// if the last working sensor is a backup, it was already tried above.
		if ps.lastWorkingSensor == backup {
			continue
		}
		ps.logger.Infof("calling backup %s", backup.Name())
		reading, err := common.TryReadingOrFail[T](ctx, ps.timeout, backup, call, extra)
		if err != nil {
			ps.logger.Warn(err.Error())
		} else {
			ps.logger.Infof("successfully got reading from %s", backup.Name())
			ps.lastWorkingSensor = backup
			return reading, nil
		}
	}
	return zero, fmt.Errorf("all power sensors failed")
}

func (s *failoverMovementSensor) Close(context.Context) error {
	return nil
}
