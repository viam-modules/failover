package failovermovementsensor

import (
	"context"
	"failover/common"
	"fmt"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Model triplet.
var Model = resource.NewModel("viam", "failover", "movementsensor")

func init() {
	resource.RegisterComponent(movementsensor.API, Model,
		resource.Registration[movementsensor.MovementSensor, common.Config]{
			Constructor: newFailoverMovementSensor,
		},
	)
}

type call = func(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error)

type failoverMovementSensor struct {
	resource.AlwaysRebuild
	resource.Named
	logger logging.Logger

	primary               *common.Primary
	primaryMovementSensor movementsensor.MovementSensor

	positionBackups           *common.Backups
	linearVelocityBackups     *common.Backups
	angularVelocityBackups    *common.Backups
	orientationBackups        *common.Backups
	accuracyBackups           *common.Backups
	compassHeadingBackups     *common.Backups
	linearAccelerationBackups *common.Backups
	readingsBackups           *common.Backups

	timeout int

	mu sync.Mutex
}

func constructMessage(function, name string) string {
	return fmt.Sprintf("primary doesn't support %s but backup %s does: consider using a merged movement sensor", function, name)
}

func newFailoverMovementSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (
	movementsensor.MovementSensor, error,
) {
	conf, err := resource.NativeConfig[common.Config](rawConf)
	if err != nil {
		return nil, err
	}

	s := &failoverMovementSensor{
		Named:  rawConf.ResourceName().AsNamed(),
		logger: logger,
	}

	// default timeout is 1 second.
	s.timeout = 1000
	if conf.Timeout > 0 {
		s.timeout = conf.Timeout
	}

	primary, err := movementsensor.FromDependencies(deps, conf.Primary)
	if err != nil {
		return nil, err
	}

	s.primaryMovementSensor = primary

	// get properties of the primary sensor and add all supported functions to calls
	primaryProps, err := primary.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	s.constructPrimary(ctx, primaryProps)

	// create list of backups for all APIs
	allBackups := []resource.Sensor{}
	angularVelocityBackups := []resource.Sensor{}
	positionBackups := []resource.Sensor{}
	linearVelocityBackups := []resource.Sensor{}
	linearAccelerationBackups := []resource.Sensor{}
	compassHeadingBackups := []resource.Sensor{}
	orientationBackups := []resource.Sensor{}

	// loop through list of backups and get properties to add to list.
	for _, backup := range conf.Backups {
		backup, err := movementsensor.FromDependencies(deps, backup)
		// if we couldnt get the backup, log the error and get the next one.
		if err != nil {
			s.logger.Errorf(err.Error())
			continue
		}
		props, err := backup.Properties(ctx, nil)
		if err != nil {
			s.logger.Errorf(err.Error())
			continue
		}
		allBackups = append(allBackups, backup)
		if props.AngularVelocitySupported {
			if primaryProps.AngularVelocitySupported {
				angularVelocityBackups = append(angularVelocityBackups, backup)
			} else {
				s.logger.Infof(constructMessage("angular velocity", backup.Name().ShortName()))
			}
		}
		if props.PositionSupported {
			if primaryProps.PositionSupported {
				positionBackups = append(positionBackups, backup)
			} else {
				s.logger.Infof(constructMessage("position", backup.Name().ShortName()))
			}
		}
		if props.LinearAccelerationSupported {
			if primaryProps.LinearAccelerationSupported {
				linearAccelerationBackups = append(linearAccelerationBackups, backup)
			} else {
				s.logger.Infof(constructMessage("linear acceleration", backup.Name().ShortName()))
			}
		}
		if props.LinearVelocitySupported {
			if primaryProps.LinearVelocitySupported {
				linearVelocityBackups = append(linearVelocityBackups, backup)
			} else {
				s.logger.Infof(constructMessage("linear velocity", backup.Name().ShortName()))
			}
		}
		if props.OrientationSupported {
			if primaryProps.OrientationSupported {
				orientationBackups = append(orientationBackups, backup)
			} else {
				s.logger.Infof(constructMessage("orientation", backup.Name().ShortName()))
			}
		}
		if props.CompassHeadingSupported {
			if primaryProps.CompassHeadingSupported {
				compassHeadingBackups = append(compassHeadingBackups, backup)
			} else {
				s.logger.Infof(constructMessage("compass heading", backup.Name().ShortName()))
			}
		}
	}

	if len(angularVelocityBackups) != 0 {
		s.angularVelocityBackups = common.CreateBackup(s.timeout, angularVelocityBackups, []call{angularVelocityWrapper})
	}

	if len(linearAccelerationBackups) != 0 {
		s.linearAccelerationBackups = common.CreateBackup(s.timeout, linearAccelerationBackups, []call{linearAccelerationWrapper})
	}

	if len(linearVelocityBackups) != 0 {
		s.linearVelocityBackups = common.CreateBackup(s.timeout, linearVelocityBackups, []call{linearVelocityWrapper})
	}

	if len(orientationBackups) != 0 {
		s.orientationBackups = common.CreateBackup(s.timeout, orientationBackups, []call{orientationWrapper})
	}
	if len(compassHeadingBackups) != 0 {
		s.compassHeadingBackups = common.CreateBackup(s.timeout, compassHeadingBackups, []call{compassHeadingWrapper})
	}

	if len(positionBackups) != 0 {
		s.positionBackups = common.CreateBackup(s.timeout, positionBackups, []call{positionWrapper})
	}

	s.readingsBackups = common.CreateBackup(s.timeout, allBackups, []call{common.ReadingsWrapper})
	s.accuracyBackups = common.CreateBackup(s.timeout, allBackups, []call{accuracyWrapper})

	return s, nil
}

func (ms *failoverMovementSensor) constructPrimary(ctx context.Context, primaryProps *movementsensor.Properties) {
	calls := []call{common.ReadingsWrapper}
	if primaryProps.AngularVelocitySupported {
		calls = append(calls, angularVelocityWrapper)
	}
	if primaryProps.CompassHeadingSupported {
		calls = append(calls, compassHeadingWrapper)
	}
	if primaryProps.LinearAccelerationSupported {
		calls = append(calls, linearAccelerationWrapper)
	}
	if primaryProps.OrientationSupported {
		calls = append(calls, orientationWrapper)
	}
	if primaryProps.PositionSupported {
		calls = append(calls, positionWrapper)
	}
	if primaryProps.LinearVelocitySupported {
		calls = append(calls, linearVelocityWrapper)
	}

	ms.primary = common.CreatePrimary(ctx, ms.timeout, ms.logger, ms.primaryMovementSensor, calls)
}

func (ms *failoverMovementSensor) Position(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return nil, 0, err
	}
	if !props.PositionSupported {
		return nil, 0, movementsensor.ErrMethodUnimplementedPosition
	}

	pos, err := getReading[postionVals](ctx, ms, positionWrapper, extra, ms.positionBackups)
	if err != nil {
		return nil, math.NaN(), fmt.Errorf("all movement sensors failed to get position: %w", err)
	}
	return pos.position, pos.altitiude, nil
}

func (ms *failoverMovementSensor) LinearVelocity(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return r3.Vector{}, err
	}
	if !props.LinearVelocitySupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
	}

	vel, err := getReading[r3.Vector](ctx, ms, linearVelocityWrapper, extra, ms.linearVelocityBackups)
	if err != nil {
		return r3.Vector{}, fmt.Errorf("all movement sensors failed to get linear velocity: %w", err)
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

	vel, err := getReading[spatialmath.AngularVelocity](ctx, ms, angularVelocityWrapper, extra, ms.angularVelocityBackups)
	if err != nil {
		return spatialmath.AngularVelocity{}, fmt.Errorf("all movement sensors failed to get angular velocity: %w", err)
	}

	return vel, nil
}

func (ms *failoverMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	// If this API is not supported on primary, return error
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return r3.Vector{}, err
	}
	if !props.LinearAccelerationSupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedAngularVelocity
	}

	acc, err := getReading[r3.Vector](ctx, ms, linearAccelerationWrapper, extra, ms.linearAccelerationBackups)
	if err != nil {
		return r3.Vector{}, fmt.Errorf("all movement sensors failed to get linear acceleration: %w", err)
	}
	return acc, nil
}

func (ms *failoverMovementSensor) CompassHeading(ctx context.Context, extra map[string]any) (float64, error) {
	// If this API is not supported on primary, return error
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return 0, err
	}
	if !props.CompassHeadingSupported {
		return 0, movementsensor.ErrMethodUnimplementedAngularVelocity
	}

	heading, err := getReading[float64](ctx, ms, compassHeadingWrapper, extra, ms.compassHeadingBackups)
	if err != nil {
		return 0, fmt.Errorf("all movement sensors failed to get compass heading: %w", err)
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

	ori, err := getReading[spatialmath.Orientation](ctx, ms, orientationWrapper, extra, ms.orientationBackups)
	if err != nil {
		return nil, fmt.Errorf("all movement sensors failed to get orientation: %w", err)
	}
	return ori, nil
}

func (ms *failoverMovementSensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	readings, err := getReading[map[string]any](ctx, ms, common.ReadingsWrapper, extra, ms.readingsBackups)
	if err != nil {
		return map[string]any{}, fmt.Errorf("all movement sensors failed to get readings: %w", err)
	}
	return readings, nil
}

func (ms *failoverMovementSensor) Accuracy(ctx context.Context, extra map[string]any) (*movementsensor.Accuracy, error,
) {
	// Accuracy isn't included in properties so trying all the backups.
	acc, err := getReading[*movementsensor.Accuracy](ctx, ms, accuracyWrapper, extra, ms.accuracyBackups)
	if err != nil {
		return nil, fmt.Errorf("all movement sensors failed to get accuracy: %w", err)
	}
	return acc, nil
}

func (ms *failoverMovementSensor) Properties(ctx context.Context, extra map[string]any) (*movementsensor.Properties, error) {
	// failover has same properties as primary
	props, err := ms.primaryMovementSensor.Properties(ctx, extra)
	if err != nil {
		return nil, err
	}

	return props, nil
}

func getReading[T any](ctx context.Context,
	ms *failoverMovementSensor,
	call call,
	extra map[string]any, backups *common.Backups,
) (T, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[T](ctx, ms.primary, extra, call)
		if err == nil {
			return reading, nil
		}
	}
	var zero T

	// Primary failed, find a working sensor
	workingSensor, err := backups.GetWorkingSensor(ctx, extra)
	if err != nil {
		return zero, err
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, workingSensor, call, extra)
	if err != nil {
		return zero, fmt.Errorf("all movement sensors failed to get compass heading: %w", err)
	}
	return any(reading).(T), nil
}

func (ms *failoverMovementSensor) Close(context.Context) error {
	if ms.primary != nil {
		ms.primary.Close()
	}
	return nil
}
