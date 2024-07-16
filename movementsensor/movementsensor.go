// Package movementsensor implements movementsensor
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
	primaryProps          *movementsensor.Properties

	backup *common.Backups

	lastWorkingSensor movementsensor.MovementSensor
	timeout           int
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
	s.lastWorkingSensor = primary

	// get properties of the primary sensor and add all supported functions to supoortedCall
	primaryProps, err := primary.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}

	s.primaryProps = primaryProps

	supportedCalls := s.constructPrimary(ctx, primaryProps)

	// create list of backups for all APIs
	backups := []resource.Sensor{}

	callsMap := make(map[resource.Sensor][]func(context.Context, resource.Sensor, map[string]any) (any, error))
	// loop through list of backups and get properties.
	for _, backup := range conf.Backups {
		calls := []common.Call{common.ReadingsWrapper, accuracyWrapper}
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

		if primaryProps != props {
			s.logger.Infof("backup %s has different properties than primary - consider using a merged movement sensor", backup.Name().ShortName())
		}

		backups = append(backups, backup)

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

		callsMap[backup] = calls

	}

	s.backup = common.CreateBackup(s.timeout, backups, supportedCalls)
	s.backup.SetCallsMap(callsMap)

	return s, nil
}

func (ms *failoverMovementSensor) constructPrimary(ctx context.Context, primaryProps *movementsensor.Properties) []common.Call {
	calls := []common.Call{common.ReadingsWrapper}
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
	return calls
}

func (ms *failoverMovementSensor) Position(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.primaryProps.PositionSupported {
		return nil, math.NaN(), movementsensor.ErrMethodUnimplementedPosition
	}

	if ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[positionVals](ctx, ms.primary, extra, positionWrapper)
		if err == nil {
			ms.lastWorkingSensor = ms.primaryMovementSensor
			return reading.position, reading.altitiude, nil
		}
	}

	movs, err := ms.getLastWorkingBackup(ctx, extra)
	if err != nil {
		return nil, math.NaN(), fmt.Errorf("failed to get posiiton: %w", err)
	}

	// get properties to determine if this API is supported on the next working backup.
	props, err := movs.Properties(ctx, nil)
	if err != nil {
		return nil, math.NaN(), err
	}
	if !props.PositionSupported {
		return nil, math.NaN(), fmt.Errorf("next backup sensor %s does not support position", movs.Name().ShortName())
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, movs, positionWrapper, extra)
	if err != nil {
		return nil, math.NaN(), fmt.Errorf("failed to get posiiton: %w", err)
	}

	pos, ok := reading.(positionVals)
	if !ok {
		return nil, math.NaN(), errors.New("failed to get postion: type assertion failed")
	}
	return pos.position, pos.altitiude, nil
}

func (ms *failoverMovementSensor) LinearVelocity(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.primaryProps.LinearVelocitySupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
	}

	if ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[r3.Vector](ctx, ms.primary, extra, linearVelocityWrapper)
		if err == nil {
			ms.lastWorkingSensor = ms.primaryMovementSensor
			return reading, nil
		}
	}

	workingSensor, err := ms.getLastWorkingBackup(ctx, extra)
	if err != nil {
		return r3.Vector{}, fmt.Errorf("failed to get linear velocity: %w", err)
	}

	// get properties to determine if this API is supported on the next working backup.
	props, err := workingSensor.Properties(ctx, nil)
	if err != nil {
		return r3.Vector{}, err
	}
	if !props.LinearAccelerationSupported {
		return r3.Vector{}, fmt.Errorf("next backup sensor %s does not support linear velocity", workingSensor.Name().ShortName())
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, workingSensor, linearVelocityWrapper, extra)
	if err != nil {

		return r3.Vector{}, nil
	}

	vel, ok := reading.(r3.Vector)
	if !ok {
		return r3.Vector{}, errors.New("failed to get linear velocity: type assertion failed")
	}
	return vel, nil
}

func (ms *failoverMovementSensor) AngularVelocity(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	props, err := ms.Properties(ctx, extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	if !props.AngularVelocitySupported {
		return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
	}

	if ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[spatialmath.AngularVelocity](ctx, ms.primary, extra, angularVelocityWrapper)
		if err == nil {
			ms.lastWorkingSensor = ms.primaryMovementSensor
			return reading, nil
		}
	}

	// Primary failed, find a working sensor
	lastWorking, err := ms.getLastWorkingBackup(ctx, extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, fmt.Errorf("failed to get angular velocity: %w", err)
	}

	// get properties to determine if this API is supported on the next working backup.
	props, err = lastWorking.Properties(ctx, nil)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	if !props.LinearAccelerationSupported {
		return spatialmath.AngularVelocity{}, fmt.Errorf("next backup sensor %s does not support angular velocity", lastWorking.Name().ShortName())
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, lastWorking, angularVelocityWrapper, extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, fmt.Errorf("failed to get angular velocity: %w", err)
	}

	vel, ok := reading.(spatialmath.AngularVelocity)
	if !ok {
		return spatialmath.AngularVelocity{}, errors.New("all movement sensors failed to get angular velocity: type assertion failed")
	}
	return vel, nil

}

func (ms *failoverMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]any) (r3.Vector, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// If this API is not supported on primary, return error
	if !ms.primaryProps.LinearAccelerationSupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
	}

	if ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[r3.Vector](ctx, ms.primary, extra, linearAccelerationWrapper)
		if err == nil {
			ms.lastWorkingSensor = ms.primaryMovementSensor
			return reading, nil
		}
	}

	// Primary failed, find a working sensor
	workingSensor, err := ms.getLastWorkingBackup(ctx, extra)
	if err != nil {
		return r3.Vector{}, fmt.Errorf("failed to get linear acceleration: %w", err)
	}

	// get properties to determine if this API is supported on the next working backup.
	props, err := workingSensor.Properties(ctx, nil)
	if err != nil {
		return r3.Vector{}, err
	}
	if !props.LinearAccelerationSupported {
		return r3.Vector{}, fmt.Errorf("next backup sensor %s does not support linear acceleration", workingSensor.Name().ShortName())
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, workingSensor, linearAccelerationWrapper, extra)
	if err != nil {
		return r3.Vector{}, fmt.Errorf("failed to get linear acceleration: %w", err)
	}

	acc, ok := reading.(r3.Vector)
	if !ok {
		return r3.Vector{}, errors.New("failed to get linear acceleration: type assertion failed")
	}

	return acc, nil
}

func (ms *failoverMovementSensor) CompassHeading(ctx context.Context, extra map[string]any) (float64, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// If this API is not supported on primary, return error
	if !ms.primaryProps.CompassHeadingSupported {
		return 0, movementsensor.ErrMethodUnimplementedCompassHeading
	}

	if ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[float64](ctx, ms.primary, extra, compassHeadingWrapper)
		if err == nil {
			ms.lastWorkingSensor = ms.primaryMovementSensor
			return reading, nil
		}
	}

	// Primary failed, find a working sensor
	workingSensor, err := ms.getLastWorkingBackup(ctx, extra)
	if err != nil {
		return math.NaN(), fmt.Errorf("failed to get compass heading: %w", err)
	}

	// get properties to determine if this API is supported on the next working backup.
	props, err := workingSensor.Properties(ctx, nil)
	if err != nil {
		return math.NaN(), err
	}
	if !props.CompassHeadingSupported {
		return math.NaN(), fmt.Errorf("next backup sensor %s does not support compass heading", workingSensor.Name().ShortName())
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, workingSensor, compassHeadingWrapper, extra)
	if err != nil {
		return math.NaN(), fmt.Errorf("failed to get compass heading %w", err)
	}

	heading, ok := reading.(float64)
	if !ok {
		return math.NaN(), errors.New("failed to get compass heading: type assertion failed")
	}

	return heading, nil
}

func (ms *failoverMovementSensor) Orientation(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.primaryProps.OrientationSupported {
		return nil, movementsensor.ErrMethodUnimplementedOrientation
	}

	if ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[spatialmath.Orientation](ctx, ms.primary, extra, orientationWrapper)
		if err == nil {
			ms.lastWorkingSensor = ms.primaryMovementSensor
			return reading, nil
		}
	}

	// Primary failed, find a working sensor
	workingSensor, err := ms.getLastWorkingBackup(ctx, extra)
	if err != nil {
		return nil, fmt.Errorf("failed to get orientation: %w", err)
	}

	// get properties to determine if this API is supported on the next working backup.
	props, err := workingSensor.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	if !props.OrientationSupported {
		return nil, fmt.Errorf("next backup sensor %s does not support orientation", workingSensor.Name().ShortName())
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	reading, err := common.TryReadingOrFail(ctx, ms.timeout, workingSensor, orientationWrapper, extra)
	if err != nil {
		return nil, fmt.Errorf("failed to get orientation: %w", err)
	}

	ori, ok := reading.(spatialmath.Orientation)
	if !ok {
		return nil, errors.New("failed to get orientation: type assertion failed")
	}

	return ori, nil
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
	acc, err := getReading[*movementsensor.Accuracy](ctx, ms, accuracyWrapper, extra, ms.backup)
	if err != nil {
		return nil, fmt.Errorf("failed to get accuracy: %w", err)
	}
	return acc, nil
}

func (ms *failoverMovementSensor) Properties(ctx context.Context, extra map[string]any) (*movementsensor.Properties, error) {
	// reutrn the intersection of properties supported by primary and the last working sensor
	lastWorkngSensorProps, err := ms.lastWorkingSensor.Properties(ctx, extra)
	if err != nil {
		return nil, err
	}

	props := &movementsensor.Properties{
		PositionSupported:           lastWorkngSensorProps.PositionSupported && ms.primaryProps.PositionSupported,
		LinearVelocitySupported:     lastWorkngSensorProps.LinearVelocitySupported && ms.primaryProps.LinearVelocitySupported,
		AngularVelocitySupported:    lastWorkngSensorProps.AngularVelocitySupported && ms.primaryProps.AngularVelocitySupported,
		LinearAccelerationSupported: lastWorkngSensorProps.LinearAccelerationSupported && ms.primaryProps.LinearAccelerationSupported,
		CompassHeadingSupported:     lastWorkngSensorProps.CompassHeadingSupported && ms.primaryProps.CompassHeadingSupported,
		OrientationSupported:        lastWorkngSensorProps.OrientationSupported && ms.primaryProps.OrientationSupported,
	}

	return props, nil
}

func getReading[T any](ctx context.Context,
	ms *failoverMovementSensor,
	call common.Call,
	extra map[string]any, backups *common.Backups,
) (T, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.primary.UsePrimary() {
		reading, err := common.TryPrimary[T](ctx, ms.primary, extra, call)
		if err == nil {
			ms.lastWorkingSensor = ms.primaryMovementSensor
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
		return zero, fmt.Errorf("all movement sensors failed: %w", err)
	}
	return any(reading).(T), nil
}

func (ms *failoverMovementSensor) getLastWorkingBackup(ctx context.Context, extra map[string]any) (movementsensor.MovementSensor, error) {
	// Primary failed, find a working sensor
	workingSensor, err := ms.backup.GetWorkingSensor(ctx, extra)
	if err != nil {
		return nil, fmt.Errorf("failed to find a working sensor: %w", err)
	}

	movs := workingSensor.(movementsensor.MovementSensor)

	if ms.lastWorkingSensor != movs {
		ms.lastWorkingSensor = movs
	}
	return movs, nil
}

func (ms *failoverMovementSensor) Close(context.Context) error {
	if ms.primary != nil {
		ms.primary.Close()
	}
	return nil
}
