package failovermovementsensor

import (
	"context"
	"failover/common"
	"fmt"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
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
	allBackups                *common.Backups

	timeout int

	mu sync.Mutex
}

func newFailoverMovementSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (movementsensor.MovementSensor, error) {
	conf, err := resource.NativeConfig[common.Config](rawConf)
	if err != nil {
		return nil, err
	}

	s := &failoverMovementSensor{
		Named:  rawConf.ResourceName().AsNamed(),
		logger: logger,
	}

	primary, err := movementsensor.FromDependencies(deps, conf.Primary)
	if err != nil {
		return nil, err
	}

	s.primaryMovementSensor = primary

	allBackups := []resource.Sensor{}
	angularVelocityBackups := []resource.Sensor{}
	positionBackups := []resource.Sensor{}
	linearVelocityBackups := []resource.Sensor{}
	linearAccelerationBackups := []resource.Sensor{}
	compassHeadingBackups := []resource.Sensor{}
	orientationBackups := []resource.Sensor{}

	primaryProps, err := primary.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	for _, backup := range conf.Backups {
		backup, err := movementsensor.FromDependencies(deps, backup)
		props, err := backup.Properties(ctx, nil)
		if err != nil {
			s.logger.Errorf(err.Error())
		} else {
			allBackups = append(allBackups, backup)
			if props.AngularVelocitySupported {
				if primaryProps.AngularVelocitySupported {
					angularVelocityBackups = append(angularVelocityBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support angular velocity but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
			if props.PositionSupported {
				if primaryProps.PositionSupported {
					positionBackups = append(positionBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support position but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
			if props.LinearAccelerationSupported {
				if primaryProps.LinearAccelerationSupported {
					linearAccelerationBackups = append(linearAccelerationBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support linear acceleration but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
			if props.LinearVelocitySupported {
				if primaryProps.LinearVelocitySupported {
					linearVelocityBackups = append(linearVelocityBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support linear acceleration but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}

			}
			if props.OrientationSupported {
				if primaryProps.OrientationSupported {
					orientationBackups = append(orientationBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support orientation but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
			if props.CompassHeadingSupported {
				if primaryProps.CompassHeadingSupported {
					compassHeadingBackups = append(compassHeadingBackups, backup)
				} else {
					s.logger.Infof("primary doesn't support compass heading but backup %s does: consider using a merged movement sensor", backup.Name().ShortName())
				}
			}
		}
	}

	// default timeout is 1 second.
	s.timeout = 1000
	if conf.Timeout > 0 {
		s.timeout = conf.Timeout
	}

	calls := []func(context.Context, resource.Sensor, map[string]any) (any, error){common.ReadingsWrapper}
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

	s.primary = common.CreatePrimary(ctx, s.timeout, s.logger, primary, calls)

	if len(angularVelocityBackups) != 0 {
		s.angularVelocityBackups = common.CreateBackup(s.timeout, angularVelocityBackups, calls)
	}

	if len(linearAccelerationBackups) != 0 {
		s.linearAccelerationBackups = common.CreateBackup(s.timeout, linearAccelerationBackups, calls)
	}

	if len(linearVelocityBackups) != 0 {
		s.linearVelocityBackups = common.CreateBackup(s.timeout, linearVelocityBackups, calls)
	}

	if len(orientationBackups) != 0 {
		s.orientationBackups = common.CreateBackup(s.timeout, orientationBackups, calls)
	}
	if len(compassHeadingBackups) != 0 {
		s.compassHeadingBackups = common.CreateBackup(s.timeout, compassHeadingBackups, calls)
	}

	if len(positionBackups) != 0 {
		s.positionBackups = common.CreateBackup(s.timeout, positionBackups, calls)
	}
	s.allBackups = common.CreateBackup(s.timeout, allBackups, calls)

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

	return nil, 0, nil
}

func (ms *failoverMovementSensor) LinearVelocity(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return r3.Vector{}, err
	}
	if !props.LinearVelocitySupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
	}
	return r3.Vector{}, nil
}

func (ms *failoverMovementSensor) AngularVelocity(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	if !props.AngularVelocitySupported {
		return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
	}

	return spatialmath.AngularVelocity{}, nil
}

func (ms *failoverMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	return r3.Vector{}, nil

}

func (ms *failoverMovementSensor) CompassHeading(ctx context.Context, extra map[string]any) (float64, error) {

	return 0, nil
}

func (ms *failoverMovementSensor) Orientation(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
	// props, err := ms.Properties(ctx, extra)
	// if err != nil {
	// 	return nil, err
	// }
	// if !props.OrientationSupported {
	// 	return nil, movementsensor.ErrMethodUnimplementedOrientation
	// }

	// ori, err := common.TryReadingOrFail(ctx, ms.timeout, ms.lastWorkingSensor, orientationWrapper, extra)
	// if err == nil {
	// 	return ori, nil
	// }
	// // upon error of the last working sensor, log returned error.
	// ms.logger.Warnf(err.Error())

	// // If the primary failed, start goroutine to check for it to get readings again.
	// switch ms.lastWorkingSensor {
	// case ms.primary:
	// 	ms.orientationChan <- true
	// default:
	// }

	// // Start reading from the list of backup sensors until one succeeds.
	// ori, err = tryBackups(ctx, ms, ms.orientationBackups, orientationWrapper, extra)
	// if err != nil {
	// 	return nil, errors.New("all movement sensors failed to get orientation")
	// }
	return nil, nil
}

func (ms *failoverMovementSensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.primary.UsePrimary() {
		readings, err := common.TryPrimary[map[string]any](ctx, ms.primary, extra, common.ReadingsWrapper)
		if err == nil {
			return readings, nil
		}
	}

	// Primary failed, find a working sensor
	workingSensor, err := ms.allBackups.GetWorkingSensor(ctx, extra)
	if err != nil {
		return nil, fmt.Errorf("all movement sensors failed to get readings: %w", err)
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	readings, err := common.TryReadingOrFail(ctx, ms.timeout, workingSensor, common.ReadingsWrapper, extra)
	if err != nil {
		return nil, fmt.Errorf("all movement sensors failed to get readings: %w", err)
	}

	reading := readings.(map[string]interface{})
	return reading, nil
}

func (ms *failoverMovementSensor) Accuracy(ctx context.Context, extra map[string]any) (*movementsensor.Accuracy, error,
) {
	return nil, nil
}

func (s *failoverMovementSensor) Properties(ctx context.Context, extra map[string]any) (*movementsensor.Properties, error) {
	props, err := s.primaryMovementSensor.Properties(ctx, extra)
	if err != nil {
		return nil, err
	}

	return props, nil
}

func (s *failoverMovementSensor) Close(context.Context) error {
	s.primary.Close()
	return nil
}
