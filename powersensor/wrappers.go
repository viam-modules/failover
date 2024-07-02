package failoverpowersensor

import (
	"context"
	"errors"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

// Wrapping all of powersensor APIs to return one struct containing their return values.
// These wrappers can used as parameters in the generic helper functions.

type voltageVals struct {
	volts float64
	isAc  bool
}

func voltageWrapper(ctx context.Context, ps resource.Sensor, extra map[string]any) (*voltageVals, error) {
	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return nil, errors.New("type assertion to power sensor failed")
	}

	volts, isAc, err := powersensor.Voltage(ctx, extra)
	if err != nil {
		return nil, err
	}

	return &voltageVals{volts: volts, isAc: isAc}, nil
}

type currentVals struct {
	amps float64
	isAc bool
}

func currentWrapper(ctx context.Context, ps resource.Sensor, extra map[string]any) (*currentVals, error) {
	powersensor, ok := ps.(powersensor.PowerSensor)
	if !ok {
		return nil, errors.New("type assertion to power sensor failed")
	}

	amps, isAc, err := powersensor.Current(ctx, extra)
	if err != nil {
		return nil, err
	}

	return &currentVals{amps: amps, isAc: isAc}, nil
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
