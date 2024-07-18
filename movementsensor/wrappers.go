package failovermovementsensor

import (
	"context"
	"errors"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Wrapping all of movementsensor APIs to return one struct containing their return values as any.
// These wrappers can be used as parameters in the generic helper functions.
// All wrapper functions must return any to store them in the backups struct.

type positionVals struct {
	position  *geo.Point
	altitiude float64
}

func positionWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	vals := positionVals{}

	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return vals, errors.New("type assertion to movement sensor failed")
	}

	pos, alt, err := ms.Position(ctx, extra)
	if err != nil {
		return vals, err
	}

	vals.position = pos
	vals.altitiude = alt

	return vals, nil
}

func linearVelocityWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return r3.Vector{}, errors.New("type assertion to movement sensor failed")
	}

	vel, err := ms.LinearVelocity(ctx, extra)
	if err != nil {
		return r3.Vector{}, err
	}
	return vel, nil
}

func angularVelocityWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return spatialmath.AngularVelocity{}, errors.New("type assertion to movement sensor failed")
	}

	vel, err := ms.AngularVelocity(ctx, extra)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}
	return vel, nil
}

func linearAccelerationWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return r3.Vector{}, errors.New("type assertion to movement sensor failed")
	}

	acc, err := ms.LinearAcceleration(ctx, extra)
	if err != nil {
		return r3.Vector{}, err
	}
	return acc, nil
}

func compassHeadingWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return 0, errors.New("type assertion to movement sensor failed")
	}

	heading, err := ms.CompassHeading(ctx, extra)
	if err != nil {
		return 0, err
	}
	return heading, nil
}

func orientationWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	orientation, err := ms.Orientation(ctx, extra)
	if err != nil {
		return nil, err
	}
	return orientation, nil
}
