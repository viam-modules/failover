package powersensor

import (
	"context"
	"errors"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
)

func VoltageWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (float64, error) {

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return 0, errors.New("failed")
	}

	volts, _, err := powersensor.Voltage(ctx, extra)
	if err != nil {
		return 0, err
	}

	return volts, err
}

func CurrentWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (float64, error) {

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return 0, errors.New("failed")
	}

	amps, _, err := powersensor.Current(ctx, extra)
	if err != nil {
		return 0, err
	}

	return amps, err
}

func PowerWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (float64, error) {

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return 0, errors.New("failed")
	}
	volts, err := powersensor.Power(ctx, extra)
	if err != nil {
		return 0, err
	}

	return volts, err
}

func ReadingsWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	powersensor, ok := ps.(sensor.Sensor)
	if !ok {
		return nil, errors.New("failed")
	}
	readings, err := powersensor.Readings(ctx, extra)
	if err != nil {
		return nil, err
	}

	return readings, err
}
