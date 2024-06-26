package powersensor

import (
	"context"
	"errors"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

func VoltageWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return nil, errors.New("type assertion to power sensor failed")
	}

	volts, isAc, err := powersensor.Voltage(ctx, extra)

	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["volts"] = volts
	m["isAC"] = isAc
	return m, nil
}

func CurrentWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return nil, errors.New("type assertion to power sensor failed")
	}

	amps, isAc, err := powersensor.Current(ctx, extra)
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	m["amps"] = amps
	m["isAC"] = isAc

	return m, nil
}

func PowerWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return nil, errors.New("type assertion to power sensor failed")
	}
	watts, err := powersensor.Power(ctx, extra)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})
	m["watts"] = watts

	return m, nil
}
