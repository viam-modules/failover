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

func voltageWrapper(ctx context.Context, ps resource.Sensor, extra map[string]any) (any, error) {

	ret := voltageVals{}

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return ret, errors.New("type assertion to power sensor failed")
	}

	volts, isAc, err := powersensor.Voltage(ctx, extra)

	if err != nil {
		return ret, err
	}

	ret.volts = volts
	ret.isAc = isAc

	return ret, nil
}

type currentVals struct {
	amps float64
	isAc bool
}

func currentWrapper(ctx context.Context, ps resource.Sensor, extra map[string]any) (any, error) {
	ret := currentVals{}

	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return nil, errors.New("type assertion to power sensor failed")
	}

	amps, isAc, err := powersensor.Current(ctx, extra)
	if err != nil {
		return nil, err
	}

	ret.amps = amps
	ret.isAc = isAc

	return ret, nil
}

func powerWrapper(ctx context.Context, ps resource.Sensor, extra map[string]any) (any, error) {
	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return nil, errors.New("type assertion to power sensor failed")
	}
	watts, err := powersensor.Power(ctx, extra)
	if err != nil {
		return nil, err
	}
	return watts, nil
}
