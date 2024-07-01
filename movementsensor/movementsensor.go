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
	allBackups                []movementsensor.MovementSensor

	logger  logging.Logger
	workers rdkutils.StoppableWorkers

	timeout int

	mu                sync.Mutex
	lastWorkingSensor movementsensor.MovementSensor

	pollingChans []chan bool

	positionChan        chan bool
	linearVelocityChan  chan bool
	angularVelocityChan chan bool
	orientationChan     chan bool
	compassHeadingChan  chan bool
	linearAccChan       chan bool
	pollReadingsChan    chan bool
	pollAccuracyChan    chan bool
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
	s.allBackups = []movementsensor.MovementSensor{}

	primaryProps, err := s.primary.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	for _, backup := range conf.Backups {
		backup, err := movementsensor.FromDependencies(deps, backup)
		props, err := backup.Properties(ctx, nil)
		if err != nil {
			s.logger.Errorf(err.Error())
		} else {
			s.allBackups = append(s.allBackups, backup)
			if props.AngularVelocitySupported {
				if primaryProps.AngularVelocitySupported {
					s.angularVelocityBackups = append(s.angularVelocityBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support angular velocity but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
			if props.PositionSupported {
				if primaryProps.PositionSupported {
					s.positionBackups = append(s.positionBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support position but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
			if props.LinearAccelerationSupported {
				if primaryProps.LinearAccelerationSupported {
					s.linearAccelerationBackups = append(s.linearAccelerationBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support linear acceleration but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
			if props.LinearVelocitySupported {
				if primaryProps.LinearVelocitySupported {
					s.linearVelocityBackups = append(s.linearVelocityBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support linear acceleration but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}

			}
			if props.OrientationSupported {
				if primaryProps.OrientationSupported {
					s.orientationBackups = append(s.orientationBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support orientation but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
			if props.CompassHeadingSupported {
				if primaryProps.CompassHeadingSupported {
					s.compassHeadingBackups = append(s.compassHeadingBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support compass heading but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
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
	s.pollReadingsChan = make(chan bool)

	PollPrimaryForHealth(s, s.positionChan, positionWrapper)
	PollPrimaryForHealth(s, s.orientationChan, orientationWrapper)
	PollPrimaryForHealth(s, s.compassHeadingChan, compassHeadingWrapper)
	PollPrimaryForHealth(s, s.linearAccChan, linearAccelerationWrapper)
	PollPrimaryForHealth(s, s.linearVelocityChan, linearVelocityWrapper)
	PollPrimaryForHealth(s, s.angularVelocityChan, angularVelocityWrapper)
	PollPrimaryForHealth(s, s.pollReadingsChan, common.ReadingsWrapper)

	return s, nil
}

func (ms *failoverMovementSensor) Position(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {

	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return nil, 0, err
	}
	if !props.PositionSupported {
		return nil, 0, movementsensor.ErrMethodUnimplementedPosition
	}

	// Poll the last sensor we know is working
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, positionWrapper, extra)
	if err == nil {
		return reading.position, reading.altitiude, nil
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
	return reading.position, reading.altitiude, nil
}

func (ms *failoverMovementSensor) LinearVelocity(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return r3.Vector{}, err
	}
	if !props.LinearVelocitySupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
	}

	// Poll the last sensor we know is working
	vel, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, linearVelocityWrapper, extra)
	if err == nil {
		return vel, nil
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
	vel, err = tryBackups(ctx, ms, ms.linearVelocityBackups, linearVelocityWrapper, extra)
	if err != nil {
		return r3.Vector{}, errors.New("all movement sensors failed to get linear velocity")
	}
	return vel, nil
}

func (ms *failoverMovementSensor) AngularVelocity(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	if !props.AngularVelocitySupported {
		return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
	}

	vel, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, angularVelocityWrapper, extra)
	if err == nil {
		return vel, nil
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
	vel, err = tryBackups(ctx, ms, ms.angularVelocityBackups, angularVelocityWrapper, extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, errors.New("all movement sensors failed to get angular velocity")
	}
	return vel, nil
}

func (ms *failoverMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return r3.Vector{}, err
	}
	if !props.LinearAccelerationSupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
	}

	acc, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, linearAccelerationWrapper, extra)
	if err == nil {
		return acc, nil
	}

	// upon error of the last working sensor, log returned error.
	ms.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.linearAccChan <- true
	default:
	}

	// Start reading from the list of backup sensors until one succeeds.
	acc, err = tryBackups(ctx, ms, ms.linearAccelerationBackups, linearAccelerationWrapper, extra)
	if err != nil {
		return r3.Vector{}, errors.New("all movement sensors failed to get linear acceleration")
	}
	return acc, nil

}

func (ms *failoverMovementSensor) CompassHeading(ctx context.Context, extra map[string]any) (float64, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return 0, err
	}
	if !props.CompassHeadingSupported {
		return 0, movementsensor.ErrMethodUnimplementedCompassHeading
	}

	heading, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, compassHeadingWrapper, extra)
	if err == nil {
		return heading, nil
	}
	// upon error of the last working sensor, log returned error.
	ms.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.compassHeadingChan <- true
	default:
	}

	// Start reading from the list of backup sensors until one succeeds.
	heading, err = tryBackups(ctx, ms, ms.compassHeadingBackups, compassHeadingWrapper, extra)
	if err != nil {
		return 0, errors.New("all movement sensors failed to get compass heading")
	}
	return heading, nil
}

func (ms *failoverMovementSensor) Orientation(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return nil, err
	}
	if !props.OrientationSupported {
		return nil, movementsensor.ErrMethodUnimplementedOrientation
	}

	ori, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, orientationWrapper, extra)
	if err == nil {
		return ori, nil
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
	ori, err = tryBackups(ctx, ms, ms.orientationBackups, orientationWrapper, extra)
	if err != nil {
		return nil, errors.New("all movement sensors failed to get orientation")
	}
	return ori, nil
}

func (ms *failoverMovementSensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Poll the last sensor we know is working
	readings, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, common.ReadingsWrapper, extra)
	if err == nil {
		return readings, nil
	}

	// upon error of the last working sensor, log the error.
	ms.logger.Warnf("movement sensor %s failed to get readings: %s", ms.lastWorkingSensor.Name().ShortName(), err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.pollReadingsChan <- true
	default:
	}

	readings, err = tryBackups(ctx, ms, ms.allBackups, common.ReadingsWrapper, extra)
	if err != nil {
		return nil, errors.New("all movement sensors failed to get readings")
	}

	return readings, nil
}

func (ms *failoverMovementSensor) Accuracy(ctx context.Context, extra map[string]any) (*movementsensor.Accuracy, error,
) {
	// Poll the last sensor we know is working
	acc, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, accuracyWrapper, extra)
	if err == nil {
		return acc, nil
	}

	// upon error of the last working sensor, log the error.
	ms.logger.Warnf("movement sensor %s failed to get accuracy: %s", ms.lastWorkingSensor.Name().ShortName(), err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	switch ms.lastWorkingSensor {
	case ms.primary:
		ms.pollReadingsChan <- true
	default:
	}

	acc, err = tryBackups(ctx, ms, ms.allBackups, accuracyWrapper, extra)
	if err != nil {
		return nil, fmt.Errorf("all movement sensors failed to get accuracy: %w", err)
	}

	return acc, nil
}

func (s *failoverMovementSensor) Properties(ctx context.Context, extra map[string]any) (*movementsensor.Properties, error) {
	props, err := s.primary.Properties(ctx, extra)
	if err != nil {
		return nil, err
	}

	return props, nil
}

func tryBackups[T any](ctx context.Context,
	ms *failoverMovementSensor,
	backups []movementsensor.MovementSensor,
	call func(ctx context.Context, ps resource.Sensor, extra map[string]any) (T, error),
	extra map[string]any) (
	T, error) {
	var zero T
	for _, backup := range backups {
		// if the last working sensor is a backup, it was already tried above.
		if ms.lastWorkingSensor == backup {
			continue
		}
		ms.logger.Infof("calling backup %s", backup.Name())
		reading, err := common.TryReadingOrFail[T](ctx, ms.timeout, backup, call, extra)
		if err != nil {
			ms.logger.Warn(err.Error())
		} else {
			ms.logger.Infof("successfully got reading from %s", backup.Name())
			ms.lastWorkingSensor = backup
			return reading, nil
		}
	}
	return zero, fmt.Errorf("all movement sensors failed")
}

// pollPrimaryForHealth starts a background routine that waits for data to come into pollPrimary channel,
// then continuously polls the primary sensor until it returns a reading, and replaces lastWorkingSensor.
func PollPrimaryForHealth[K any](s *failoverMovementSensor,
	startChan chan bool,
	call func(context.Context, resource.Sensor, map[string]any) (K, error)) {
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
	close(s.pollAccuracyChan)
	close(s.pollReadingsChan)
	return nil
}
