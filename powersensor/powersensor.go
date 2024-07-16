// Package failoverpowersensor implements power sensor.
package failoverpowersensor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"failover/common"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("viam", "failover", "power_sensor")

func init() {
	resource.RegisterComponent(powersensor.API, Model,
		resource.Registration[powersensor.PowerSensor, common.Config]{
			Constructor: newFailoverPowerSensor,
		},
	)
}

type failoverPowerSensor struct {
	resource.AlwaysRebuild
	resource.Named
	logger logging.Logger

	mu      sync.Mutex
	primary *common.Primary
	backups *common.Backups

	timeout int
}

func newFailoverPowerSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (
	powersensor.PowerSensor, error,
) {
	config, err := resource.NativeConfig[common.Config](conf)
	if err != nil {
		return nil, err
	}

	ps := &failoverPowerSensor{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	primary, err := powersensor.FromDependencies(deps, config.Primary)
	if err != nil {
		return nil, err
	}

	// default timeout is 1 second.
	ps.timeout = 1000
	if config.Timeout > 0 {
		ps.timeout = config.Timeout
	}

	backups := []resource.Sensor{}

	for _, backup := range config.Backups {
		backup, err := powersensor.FromDependencies(deps, backup)
		if err != nil {
			ps.logger.Errorf(err.Error())
		}
		backups = append(backups, backup)
	}

	calls := []common.Call{voltageWrapper, currentWrapper, powerWrapper, common.ReadingsWrapper}
	ps.primary = common.CreatePrimary(ctx, ps.timeout, logger, primary, calls)
	ps.backups = common.CreateBackup(ps.timeout, backups, calls)

	return ps, nil
}

func (ps *failoverPowerSensor) Voltage(ctx context.Context, extra map[string]any) (float64, bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// if UsePrimary flag is set, call voltage on the primary
	if ps.primary.UsePrimary() {
		readings, err := common.TryPrimary[*voltageVals](ctx, ps.primary, extra, voltageWrapper)
		if err == nil {
			return readings.volts, readings.isAc, nil
		}
	}

	// Primary failed, find a working sensor
	workingSensor, err := ps.backups.GetWorkingSensor(ctx, extra)
	if err != nil {
		return math.NaN(), false, fmt.Errorf("all power sensors failed to get voltage: %w", err)
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, workingSensor, voltageWrapper, extra)
	if err != nil {
		return math.NaN(), false, fmt.Errorf("all power sensors failed to get voltage: %w", err)
	}

	vals, ok := readings.(*voltageVals)
	if !ok {
		return math.NaN(), false, errors.New("failed to get voltage: type assertion failed")
	}
	return vals.volts, vals.isAc, nil
}

// Current returns the current reading in amperes and a bool returning true if the current is AC.
func (ps *failoverPowerSensor) Current(ctx context.Context, extra map[string]any) (float64, bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.primary.UsePrimary() {
		readings, err := common.TryPrimary[*currentVals](ctx, ps.primary, extra, currentWrapper)
		if err == nil {
			return readings.amps, readings.isAc, nil
		}
	}

	// Primary failed, find a working sensor
	workingSensor, err := ps.backups.GetWorkingSensor(ctx, extra)
	if err != nil {
		return math.NaN(), false, fmt.Errorf("all power sensors failed to get current: %w", err)
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, workingSensor, currentWrapper, extra)
	if err != nil {
		return math.NaN(), false, fmt.Errorf("all power sensors failed to get current: %w", err)
	}

	currentVals, ok := readings.(*currentVals)
	if !ok {
		return math.NaN(), false, errors.New("failed to get current: type assertion failed")
	}
	return currentVals.amps, currentVals.isAc, nil
}

// Power returns the power reading in watts.
func (ps *failoverPowerSensor) Power(ctx context.Context, extra map[string]any) (float64, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.primary.UsePrimary() {
		// Poll the last sensor we know is working.
		// In the non-error case, the wrapper will never return its readings as nil.
		readings, err := common.TryPrimary[float64](ctx, ps.primary, extra, powerWrapper)
		if err == nil {
			return readings, nil
		}
	}

	// Primary failed, find a working sensor
	workingSensor, err := ps.backups.GetWorkingSensor(ctx, extra)
	if err != nil {
		return math.NaN(), fmt.Errorf("all power sensors failed to get power: %w", err)
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, workingSensor, powerWrapper, extra)
	if err != nil {
		return math.NaN(), fmt.Errorf("all power sensors failed to get power: %w", err)
	}
	watts, ok := readings.(float64)
	if !ok {
		return math.NaN(), errors.New("failed to get power: type assertion failed")
	}
	return watts, nil
}

func (ps *failoverPowerSensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.primary.UsePrimary() {
		readings, err := common.TryPrimary[map[string]any](ctx, ps.primary, extra, common.ReadingsWrapper)
		if err == nil {
			return readings, nil
		}
	}

	// Primary failed, find a working sensor
	workingSensor, err := ps.backups.GetWorkingSensor(ctx, extra)
	if err != nil {
		return nil, fmt.Errorf("all power sensors failed to get readings: %w", err)
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, workingSensor, common.ReadingsWrapper, extra)
	if err != nil {
		return nil, fmt.Errorf("all power sensors failed to get readings: %w", err)
	}

	reading, ok := readings.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to get readings: type assertion failed")
	}
	return reading, nil
}

func (ps *failoverPowerSensor) Close(context.Context) error {
	ps.primary.Close()
	return nil
}
