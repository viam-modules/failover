// Package failoverpowersensor implements a failover power sensor.
package failoverpowersensor

import (
	"context"
	"errors"
	"math"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

// Wrapping all of powersensor APIs to return one struct containing their return values as any.
// These wrappers can be used as parameters in the generic helper functions.
// All wrapper functions must return any to store them in the backups struct.

type voltageVals struct {
	volts float64
	isAc  bool
}

func voltageWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ps, err := convertToPowerSensor(s)
	if err != nil {
		return nil, err
	}

	volts, isAc, err := ps.Voltage(ctx, extra)
	if err != nil {
		return nil, err
	}

	return &voltageVals{volts: volts, isAc: isAc}, nil
}

type currentVals struct {
	amps float64
	isAc bool
}

func currentWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ps, err := convertToPowerSensor(s)
	if err != nil {
		return nil, err
	}

	amps, isAc, err := ps.Current(ctx, extra)
	if err != nil {
		return nil, err
	}

	return &currentVals{amps: amps, isAc: isAc}, nil
}

func powerWrapper(ctx context.Context, s resource.Sensor, extra map[string]any) (any, error) {
	ps, err := convertToPowerSensor(s)
	if err != nil {
		return math.NaN(), err
	}
	watts, err := ps.Power(ctx, extra)
	if err != nil {
		return math.NaN(), err
	}
	return watts, nil
}

func convertToPowerSensor(s resource.Sensor) (powersensor.PowerSensor, error) {
	ps, ok := s.(powersensor.PowerSensor)
	if !ok {
		return nil, errors.New("type assertion to power sensor failed")
	}
	return ps, nil
}
