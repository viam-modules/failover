package failoverpowersensor

import (
	"context"
	"errors"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

// Wrapping all of powersensor APIs to return a map[string]interface{} containing their return values.
// These wrappers can used as parameters in the generic helper functions.

func voltageWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
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

func currentWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
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

func powerWrapper(ctx context.Context, ps resource.Sensor, extra map[string]interface{}) (map[string]interface{}, error) {
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
