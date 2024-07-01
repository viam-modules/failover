package failoverpowersensor

import (
	"context"
	"errors"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

// Wrapping all of powersensor APIs to return a map[string]interface{} containing their return values.
// These wrappers can used as parameters in the generic helper functions.

type voltageVals struct {
	volts float64
	isAc  bool
}

func voltageWrapper(ctx context.Context, ps resource.Sensor, extra map[string]any) (voltageVals, error) {

	vals := voltageVals{}

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return vals, errors.New("type assertion to power sensor failed")
	}

	volts, isAc, err := powersensor.Voltage(ctx, extra)

	if err != nil {
		return vals, err
	}

	vals.volts = volts
	vals.isAc = isAc

	return vals, nil
}

type currentVals struct {
	amps float64
	isAc bool
}

func currentWrapper(ctx context.Context, ps resource.Sensor, extra map[string]any) (currentVals, error) {
	vals := currentVals{}

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return vals, errors.New("type assertion to power sensor failed")
	}

	amps, isAc, err := powersensor.Current(ctx, extra)
	if err != nil {
		return vals, err
	}

	vals.amps = amps
	vals.isAc = isAc

	return vals, nil
}

func powerWrapper(ctx context.Context, ps resource.Sensor, extra map[string]any) (float64, error) {
	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return 0, errors.New("type assertion to power sensor failed")
	}
	watts, err := powersensor.Power(ctx, extra)
	if err != nil {
		return 0, err
	}
	return watts, nil
}
