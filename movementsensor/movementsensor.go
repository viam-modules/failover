package failovermovementsensor

import (
	"context"
	"errors"
	"failover/common"
	"fmt"
	"sync"
	"time"

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
		resource.Registration[movementsensor.MovementSensor, common.Config]{
			Constructor: newFailoverMovementSensor,
		},
	)
}

type failoverMovementSensor struct {
	resource.AlwaysRebuild
	resource.Named

	primary movementsensor.MovementSensor

	positionBackups           []movementsensor.MovementSensor
	linearVelocityBackups     []movementsensor.MovementSensor
	angularVelocityBackups    []movementsensor.MovementSensor
	orientationBackups        []movementsensor.MovementSensor
	accuracyBackups           []movementsensor.MovementSensor
	compassHeadingBackups     []movementsensor.MovementSensor
	linearAccelerationBackups []movementsensor.MovementSensor

	logger  logging.Logger
	workers rdkutils.StoppableWorkers

	timeout int

	mu                sync.Mutex
	lastWorkingSensor movementsensor.MovementSensor

	positionChan        chan bool
	linearVelocityChan  chan bool
	angularVelocityChan chan bool
	orientationChan     chan bool
	accuracyChan        chan bool
	compassHeadingChan  chan bool
	linearAccChan       chan bool
}

func newFailoverMovementSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (movementsensor.MovementSensor, error) {
	conf, err := resource.NativeConfig[common.Config](rawConf)
	if err != nil {
		return nil, err
	}

	s := &failoverMovementSensor{
		Named:   rawConf.ResourceName().AsNamed(),
		logger:  logger,
		workers: rdkutils.NewStoppableWorkers(),
	}

	primary, err := movementsensor.FromDependencies(deps, conf.Primary)
	if err != nil {
		return nil, err
	}
	s.primary = primary
	s.angularVelocityBackups = []movementsensor.MovementSensor{}
	s.positionBackups = []movementsensor.MovementSensor{}
	s.linearVelocityBackups = []movementsensor.MovementSensor{}
	s.linearAccelerationBackups = []movementsensor.MovementSensor{}
	s.compassHeadingBackups = []movementsensor.MovementSensor{}

	for _, backup := range conf.Backups {
		backup, err := movementsensor.FromDependencies(deps, backup)
		props, err := backup.Properties(ctx, nil)
		if err != nil {
			s.logger.Errorf(err.Error())
		} else {
			if props.AngularVelocitySupported {
				s.angularVelocityBackups = append(s.angularVelocityBackups, backup)
			}
			if props.PositionSupported {
				s.positionBackups = append(s.positionBackups, backup)
			}
			if props.LinearAccelerationSupported {
				s.linearAccelerationBackups = append(s.linearAccelerationBackups, backup)
			}
			if props.LinearVelocitySupported {
				s.linearVelocityBackups = append(s.linearVelocityBackups, backup)
			}
			if props.OrientationSupported {
				s.orientationBackups = append(s.orientationBackups, backup)
			}
			if props.CompassHeadingSupported {
				s.compassHeadingBackups = append(s.compassHeadingBackups, backup)
			}
		}
	}

	s.lastWorkingSensor = primary

	// default timeout is 1 second.
	s.timeout = 1000
	if conf.Timeout > 0 {
		s.timeout = conf.Timeout
	}

	s.positionChan = make(chan bool)
	s.orientationChan = make(chan bool)
	s.compassHeadingChan = make(chan bool)
	s.linearAccChan = make(chan bool)
	s.linearVelocityChan = make(chan bool)
	s.angularVelocityChan = make(chan bool)

	PollPrimaryForHealth(s, s.positionChan, positionWrapper)
	PollPrimaryForHealth(s, s.orientationChan, orientationWrapper)
	PollPrimaryForHealth(s, s.compassHeadingChan, compassHeadingWrapper)
	PollPrimaryForHealth(s, s.linearAccChan, linearAccelerationWrapper)
	PollPrimaryForHealth(s, s.linearVelocityChan, linearVelocityWrapper)
	PollPrimaryForHealth(s, s.angularVelocityChan, angularVelocityWrapper)

	return s, nil
}

func (ms *failoverMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	// Poll the last sensor we know is working
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, positionWrapper, extra)
	if err == nil {
		pos, alt, err := common.Get2ReadingsFromMap[*geo.Point, float64](reading, "position", "altitude")
		if err == nil {
			return pos, alt, nil
		}
	}
	// upon error of the last working sensor, log returned error.
	ms.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.positionChan <- true
	default:
	}

	// Start reading from the list of backup sensors until one succeeds.
	reading, err = tryBackups(ctx, ms, ms.positionBackups, positionWrapper, extra)
	if err != nil {
		return nil, 0, errors.New("all movement sensors failed to get position")
	}
	pos, alt, err := common.Get2ReadingsFromMap[*geo.Point, float64](reading, "position", "altitude")
	if err != nil {
		return nil, 0, fmt.Errorf("all movement sensors failed to get position: %w", err)
	}
	return pos, alt, nil
}

func (ms *failoverMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	// Poll the last sensor we know is working
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, linearVelocityWrapper, extra)
	if err == nil {
		vel, err := common.GetReadingFromMap[r3.Vector](reading, "velocity")
		if err == nil {
			return vel, nil
		}
	}
	// upon error of the last working sensor, log returned error.
	ms.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.linearVelocityChan <- true
	default:
	}

	// Start reading from the list of backup sensors until one succeeds.
	reading, err = tryBackups(ctx, ms, ms.linearVelocityBackups, linearVelocityWrapper, extra)
	if err != nil {
		return r3.Vector{}, errors.New("all movement sensors failed to get linear velocity")
	}
	vel, err := common.GetReadingFromMap[r3.Vector](reading, "velocity")
	if err != nil {
		return r3.Vector{}, fmt.Errorf("all movement sensors failed to get linear velocity: %w", err)
	}
	return vel, nil
}

func (ms *failoverMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, angularVelocityWrapper, extra)
	if err == nil {
		vel, err := common.GetReadingFromMap[spatialmath.AngularVelocity](reading, "velocity")
		if err == nil {
			return vel, nil
		}
	}
	// upon error of the last working sensor, log returned error.
	ms.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.angularVelocityChan <- true
	default:
	}

	// Start reading from the list of backup sensors until one succeeds.
	reading, err = tryBackups(ctx, ms, ms.angularVelocityBackups, angularVelocityWrapper, extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, errors.New("all movement sensors failed to get angular velocity")
	}
	vel, err := common.GetReadingFromMap[spatialmath.AngularVelocity](reading, "velocity")
	if err != nil {
		return spatialmath.AngularVelocity{}, fmt.Errorf("all movement sensors failed to get angular velocity: %w", err)
	}
	return vel, nil
}

func (ms *failoverMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, linearAccelerationWrapper, extra)
	if err == nil {
		acc, err := common.GetReadingFromMap[r3.Vector](reading, "acceleration")
		if err == nil {
			return acc, nil
		}
	}

	fmt.Println(err)
	// upon error of the last working sensor, log returned error.
	ms.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.linearAccChan <- true
	default:
	}

	// Start reading from the list of backup sensors until one succeeds.
	reading, err = tryBackups(ctx, ms, ms.linearAccelerationBackups, linearAccelerationWrapper, extra)
	if err != nil {
		return r3.Vector{}, errors.New("all movement sensors failed to get linear acceleration")
	}
	acc, err := common.GetReadingFromMap[r3.Vector](reading, "velocity")
	if err != nil {
		return r3.Vector{}, fmt.Errorf("all movement sensors failed to get linear acceleration: %w", err)
	}

	return acc, nil

}

func (ms *failoverMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, compassHeadingWrapper, extra)
	if err == nil {
		heading, newErr := common.GetReadingFromMap[float64](reading, "heading")
		if newErr == nil {
			return heading, nil
		}
		err = newErr
	}

	fmt.Println("ERROR HERE:")
	fmt.Println(err)
	// upon error of the last working sensor, log returned error.
	ms.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.compassHeadingChan <- true
	default:
	}

	// Start reading from the list of backup sensors until one succeeds.
	reading, err = tryBackups(ctx, ms, ms.compassHeadingBackups, compassHeadingWrapper, extra)
	if err != nil {
		return 0, errors.New("all movement sensors failed to get compass heading")
	}
	heading, err := common.GetReadingFromMap[float64](reading, "heading")
	if err != nil {
		return 0, fmt.Errorf("all movement sensors failed to get compass heading: %w", err)
	}
	return heading, nil
}

func (ms *failoverMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, orientationWrapper, extra)
	if err == nil {
		ori, err := common.GetReadingFromMap[spatialmath.Orientation](reading, "orientation")
		if err == nil {
			return ori, nil
		}
	}
	// upon error of the last working sensor, log returned error.
	ms.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.orientationChan <- true
	default:
	}

	// Start reading from the list of backup sensors until one succeeds.
	reading, err = tryBackups(ctx, ms, ms.orientationBackups, orientationWrapper, extra)
	if err != nil {
		return nil, errors.New("all movement sensors failed to get orientation")
	}
	ori, err := common.GetReadingFromMap[spatialmath.Orientation](reading, "orientation")
	if err != nil {
		return nil, fmt.Errorf("all movement sensors failed to get orientation: %w", err)
	}
	return ori, nil
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
	props, err := s.primary.Properties(ctx, extra)
	if err != nil {
		return nil, errors.New("failed to get properties")
	}

	return props, nil
}

func tryBackups[T any](ctx context.Context,
	ps *failoverMovementSensor,
	backups []movementsensor.MovementSensor,
	call func(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (T, error),
	extra map[string]interface{}) (
	T, error) {
	// Lock the mutex to protect lastWorkingSensor from changing before the readings call finishes.
	ps.mu.Lock()
	defer ps.mu.Unlock()
	var zero T
	for _, backup := range backups {
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

// pollPrimaryForHealth starts a background routine that waits for data to come into pollPrimary channel,
// then continuously polls the primary sensor until it returns a reading, and replaces lastWorkingSensor.
func PollPrimaryForHealth[K any](s *failoverMovementSensor,
	startChan chan bool,
	call func(context.Context, resource.Sensor, map[string]interface{}) (K, error)) {
	// poll every 10 ms.
	ticker := time.NewTicker(time.Millisecond * 10)
	s.workers.AddWorkers(func(ctx context.Context) {
		for {
			select {
			// wait for data to come into the channel before polling.
			case <-ctx.Done():
				return
			case <-startChan:
			}
			// label for loop so we can break out of it later.
		L:
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_, err := common.TryReadingOrFail(ctx, s.timeout, s.primary, call, nil)
					if err == nil {
						s.logger.Infof("successfully got reading from primary sensor")
						s.mu.Lock()
						s.lastWorkingSensor = s.primary
						s.mu.Unlock()
						break L
					}
				}
			}
		}
	})
}

func (s *failoverMovementSensor) Close(context.Context) error {
	if s.workers != nil {
		s.workers.Stop()
	}
	close(s.positionChan)
	close(s.orientationChan)
	close(s.compassHeadingChan)
	close(s.linearAccChan)
	close(s.linearVelocityChan)
	close(s.angularVelocityChan)
	return nil
}
