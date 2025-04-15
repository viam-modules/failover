// Package failovermovementsensor implements a failover movement sensor
package failovermovementsensor

import (
	"context"
	"errors"
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
var Model = resource.NewModel("viam", "failover", "movement_sensor")

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

	mu                    sync.Mutex
	primary               *common.Primary
	primaryMovementSensor movementsensor.MovementSensor
	supportedProps        *movementsensor.Properties

	backup            *common.Backups
	lastWorkingSensor movementsensor.MovementSensor
	timeoutMs         int
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
	s.timeoutMs = 1000
	if conf.Timeout > 0 {
		s.timeoutMs = conf.Timeout
	}

	backups := []resource.Sensor{}

	// supportedCallsMap specify which calls each backup supports
	supportedCallsMap := make(map[resource.Sensor][]common.Call)

	// supportedCalls is a lsit of all APIs the failover supports
	var supportedCalls []common.Call

	// Handle primary sensor if specified
	if conf.Primary != "" {
		primary, err := movementsensor.FromDependencies(deps, conf.Primary)
		if err != nil {
			return nil, err
		}
		s.primaryMovementSensor = primary
		s.lastWorkingSensor = primary

		// get properties of the primary sensor
		primaryProps, err := primary.Properties(ctx, nil)
		if err != nil {
			return nil, err
		}
		// If there is a primary, the failover's supported props will be the primary's props.
		s.supportedProps = primaryProps
		supportedCalls := createCalls(s.supportedProps)
		s.primary = common.CreatePrimary(ctx, s.timeoutMs, s.logger, s.primaryMovementSensor, supportedCalls)
	} else {
		// Initialize properties to track what's supported across backups
		s.supportedProps = &movementsensor.Properties{}
	}

	// Process backup sensors
	for _, backup := range conf.Backups {
		backup, err := movementsensor.FromDependencies(deps, backup)
		if err != nil {
			s.logger.Errorf(err.Error())
			continue
		}

		props, err := backup.Properties(ctx, nil)
		if err != nil {
			s.logger.Errorf(err.Error())
			continue
		}

		// Set first working backup as last working sensor if no primary
		if conf.Primary == "" && s.lastWorkingSensor == nil {
			s.lastWorkingSensor = backup
		}

		// If no primary, update supported properties based on backup capabilities
		if conf.Primary == "" {
			s.supportedProps.PositionSupported = s.supportedProps.PositionSupported || props.PositionSupported
			s.supportedProps.LinearVelocitySupported = s.supportedProps.LinearVelocitySupported || props.LinearVelocitySupported
			s.supportedProps.AngularVelocitySupported = s.supportedProps.AngularVelocitySupported || props.AngularVelocitySupported
			s.supportedProps.LinearAccelerationSupported = s.supportedProps.LinearAccelerationSupported || props.LinearAccelerationSupported
			s.supportedProps.CompassHeadingSupported = s.supportedProps.CompassHeadingSupported || props.CompassHeadingSupported
			s.supportedProps.OrientationSupported = s.supportedProps.OrientationSupported || props.OrientationSupported
		}

		backups = append(backups, backup)
		supportedCalls := createCalls(props)
		supportedCallsMap[backup] = supportedCalls
	}

	// If no primary, create supported calls based on aggregated properties
	if conf.Primary == "" {
		supportedCalls = createCalls(s.supportedProps)
	}
	if len(backups) == 0 {
		return nil, errors.New("no backups were successfully added")
	}
	s.backup = common.CreateBackup(s.timeoutMs, backups, supportedCalls)
	s.backup.SetCallsMap(supportedCallsMap)

	return s, nil
}

// createCalls is a helper function to create a list of API calls supported from the properties.
func createCalls(props *movementsensor.Properties) []common.Call {
	calls := []common.Call{common.ReadingsWrapper}

	if props.LinearVelocitySupported {
		calls = append(calls, linearVelocityWrapper)
	}
	if props.OrientationSupported {
		calls = append(calls, orientationWrapper)
	}
	if props.PositionSupported {
		calls = append(calls, positionWrapper)
	}
	if props.CompassHeadingSupported {
		calls = append(calls, compassHeadingWrapper)
	}
	if props.AngularVelocitySupported {
		calls = append(calls, angularVelocityWrapper)
	}
	if props.LinearAccelerationSupported {
		calls = append(calls, linearAccelerationWrapper)
	}
	return calls
}

// tryPrimaryThenFindWorking is a helper that tries the primary sensor first, then falls back to finding a working sensor
func tryPrimaryThenFindWorking[T any](
	ms *failoverMovementSensor,
	ctx context.Context,
	extra map[string]any,
	wrapper common.Call,
	propertySupported func(*movementsensor.Properties) bool,
	operation string,
) (T, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	var zero T

	// Try primary sensor first
	if ms.primary != nil && ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[T](ctx, ms.primary, extra, wrapper)
		if err == nil {
			ms.lastWorkingSensor = ms.primaryMovementSensor
			return reading, nil
		}
	}

	// Find a working sensor with the required property
	workingSensor, err := ms.findWorkingSensorWithProperty(ctx, extra, propertySupported)
	if err != nil {
		return zero, fmt.Errorf("failed to get %s: %w", operation, err)
	}

	// Read from the working sensor
	reading, err := common.TryReadingOrFail(ctx, ms.timeoutMs, workingSensor, wrapper, extra)
	if err != nil {
		return zero, fmt.Errorf("failed to get %s: %w", operation, err)
	}

	result, ok := reading.(T)
	if !ok {
		return zero, fmt.Errorf("failed to get %s: type assertion failed", operation)
	}

	return result, nil
}

func (ms *failoverMovementSensor) Position(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
	if !ms.supportedProps.PositionSupported {
		return nil, math.NaN(), movementsensor.ErrMethodUnimplementedPosition
	}

	pos, err := tryPrimaryThenFindWorking[positionVals](
		ms,
		ctx,
		extra,
		positionWrapper,
		func(props *movementsensor.Properties) bool { return props.PositionSupported },
		"position",
	)
	if err != nil {
		return nil, math.NaN(), err
	}

	return pos.position, pos.altitiude, nil
}

func (ms *failoverMovementSensor) LinearVelocity(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	if !ms.supportedProps.LinearVelocitySupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
	}

	return tryPrimaryThenFindWorking[r3.Vector](
		ms,
		ctx,
		extra,
		linearVelocityWrapper,
		func(props *movementsensor.Properties) bool { return props.LinearVelocitySupported },
		"linear velocity",
	)
}

func (ms *failoverMovementSensor) AngularVelocity(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
	if !ms.supportedProps.AngularVelocitySupported {
		return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
	}

	return tryPrimaryThenFindWorking[spatialmath.AngularVelocity](
		ms,
		ctx,
		extra,
		angularVelocityWrapper,
		func(props *movementsensor.Properties) bool { return props.AngularVelocitySupported },
		"angular velocity",
	)
}

func (ms *failoverMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	if !ms.supportedProps.LinearAccelerationSupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
	}

	return tryPrimaryThenFindWorking[r3.Vector](
		ms,
		ctx,
		extra,
		linearAccelerationWrapper,
		func(props *movementsensor.Properties) bool { return props.LinearAccelerationSupported },
		"linear acceleration",
	)
}

func (ms *failoverMovementSensor) CompassHeading(ctx context.Context, extra map[string]any) (float64, error) {
	if !ms.supportedProps.CompassHeadingSupported {
		return 0, movementsensor.ErrMethodUnimplementedCompassHeading
	}

	heading, err := tryPrimaryThenFindWorking[float64](
		ms,
		ctx,
		extra,
		compassHeadingWrapper,
		func(props *movementsensor.Properties) bool { return props.CompassHeadingSupported },
		"compass heading",
	)
	if err != nil {
		return math.NaN(), err
	}

	return heading, nil
}

func (ms *failoverMovementSensor) Orientation(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
	if !ms.supportedProps.OrientationSupported {
		return nil, movementsensor.ErrMethodUnimplementedOrientation
	}

	return tryPrimaryThenFindWorking[spatialmath.Orientation](
		ms,
		ctx,
		extra,
		orientationWrapper,
		func(props *movementsensor.Properties) bool { return props.OrientationSupported },
		"orientation",
	)
}

func (ms *failoverMovementSensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	readings, err := getReading[map[string]any](ctx, ms, common.ReadingsWrapper, extra, ms.backup)
	if err != nil {
		return map[string]any{}, fmt.Errorf("failed to get readings: %w", err)
	}
	return readings, nil
}

func (ms *failoverMovementSensor) Accuracy(ctx context.Context, extra map[string]any) (*movementsensor.Accuracy, error,
) {
	// Accuracy is a special case - return the lastworkingsensor's accuracy whether it errors or not.
	accuracy, err := ms.lastWorkingSensor.Accuracy(ctx, extra)
	if err != nil {
		return &movementsensor.Accuracy{}, fmt.Errorf("failed to get accuracy from last working sensor: %w", err)
	}
	return accuracy, nil
}

func (ms *failoverMovementSensor) Properties(ctx context.Context, extra map[string]any) (*movementsensor.Properties, error) {
	// Return the supported properties directly - these were determined during initialization
	return ms.supportedProps, nil
}

func getReading[T any](ctx context.Context,
	ms *failoverMovementSensor,
	call common.Call,
	extra map[string]any, backups *common.Backups,
) (T, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.primary != nil {
		if ms.primary.UsePrimary() {
			reading, err := common.TryPrimary[T](ctx, ms.primary, extra, call)
			if err == nil {
				ms.lastWorkingSensor = ms.primaryMovementSensor
				return reading, nil
			}
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
	reading, err := common.TryReadingOrFail(ctx, ms.timeoutMs, workingSensor, call, extra)
	if err != nil {
		return zero, fmt.Errorf("all movement sensors failed: %w", err)
	}
	return any(reading).(T), nil
}

func (ms *failoverMovementSensor) findWorkingSensorWithProperty(ctx context.Context, extra map[string]any, propertySupported func(*movementsensor.Properties) bool) (movementsensor.MovementSensor, error) {
	// Try each sensor until we find one that both works and supports the required property
	sensors := ms.backup.GetSensors()
	for _, sensor := range sensors {
		movs, ok := sensor.(movementsensor.MovementSensor)
		if !ok {
			continue
		}

		// Check if the sensor supports the required property
		props, err := movs.Properties(ctx, nil)
		if err != nil {
			continue
		}

		if !propertySupported(props) {
			continue
		}

		// Try to get readings from this sensor
		if err := ms.backup.TryReading(ctx, sensor, extra); err == nil {
			if ms.lastWorkingSensor != movs {
				ms.lastWorkingSensor = movs
			}
			return movs, nil
		}
	}

	return nil, errors.New("no working sensor found that supports the requested API")
}

func (ms *failoverMovementSensor) Close(context.Context) error {
	if ms.primary != nil {
		ms.primary.Close()
	}
	return nil
}
