package failovermovementsensor

import (
	"context"
	"errors"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
)

func positionWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	pos, alt, err := ms.Position(ctx, extra)

	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["position"] = pos
	m["altitude"] = alt
	return m, nil
}

func linearVelocityWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	vel, err := ms.LinearVelocity(ctx, extra)

	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["velocity"] = vel
	return m, nil

}

func angularVelocityWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	vel, err := ms.AngularVelocity(ctx, extra)

	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["velocity"] = vel
	return m, nil
}

func linearAccelerationWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	acc, err := ms.LinearAcceleration(ctx, extra)

	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["acceleration"] = acc
	return m, nil
}

func compassHeadingWrapper(ctx context.Context, s resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	ms, ok := s.(movementsensor.MovementSensor)
	if !ok {
		return nil, errors.New("type assertion to movement sensor failed")
	}

	heading, err := ms.CompassHeading(ctx, extra)

	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["heading"] = heading
	return m, nil
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
