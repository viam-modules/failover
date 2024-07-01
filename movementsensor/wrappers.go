package failovermovementsensor

import (
	"context"
	"errors"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
)

type postionVals struct {
	position  *geo.Point
	altitiude float64
}

func positionWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	vals := postionVals{}

	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	pos, alt, err := ms.Position(ctx, extra)
	if err != nil {
		return nil, err
	}

	vals.position = pos
	vals.altitiude = alt

	return vals, nil

}

func linearVelocityWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	vel, err := ms.LinearVelocity(ctx, extra)
	if err != nil {
		return nil, err
	}

	return vel, nil

}

func angularVelocityWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	vel, err := ms.AngularVelocity(ctx, extra)
	if err != nil {
		return nil, err
	}

	return vel, nil
}

func linearAccelerationWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	acc, err := ms.LinearAcceleration(ctx, extra)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

func compassHeadingWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (any, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	heading, err := ms.CompassHeading(ctx, extra)

	if err != nil {
		return nil, err
	}
	return heading, nil
}

func orientationWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	orientation, err := ms.Orientation(ctx, extra)

	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["orientation"] = orientation
	return m, nil
}

func accuracyWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error,
) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	accuracy, err := ms.Accuracy(ctx, extra)

	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["accuracy"] = accuracy
	return m, nil
}
