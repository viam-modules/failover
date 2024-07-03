package failoverpowersensor

import (
	"context"
	"errors"
	"failover/common"
	"math"
	"sync"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var (
	Model = resource.NewModel("viam", "failover", "powersensor")
)

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

	// primary powersensor.PowerSensor
	backups common.Backups
	primary common.Primary

	logger logging.Logger

	timeout int

	mu sync.Mutex

	pollPrimaryChan chan bool
	Calls           []func(context.Context, resource.Sensor, map[string]any) (any, error)
}

func newFailoverPowerSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (powersensor.PowerSensor, error) {
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

	ps.primary = common.CreatePrimary()
	ps.primary.Logger = logger
	ps.primary.S = primary
	ps.backups = common.Backups{}

	for _, backup := range config.Backups {
		backup, err := powersensor.FromDependencies(deps, backup)
		if err != nil {
			ps.logger.Errorf(err.Error())
		} else {
			ps.backups.BackupList = append(ps.backups.BackupList, backup)
		}
	}

	ps.backups.LastWorkingSensor = ps.backups.BackupList[0]
	ps.Calls = []func(context.Context, resource.Sensor, map[string]any) (any, error){voltageWrapper, currentWrapper, powerWrapper, common.ReadingsWrapper}

	// default timeout is 1 second.
	ps.timeout = 1000
	ps.backups.Timeout = 1000
	ps.primary.Timeout = 1000
	if config.Timeout > 0 {
		ps.timeout = config.Timeout
	}

	// Check that all functions on primary are working, if not tell the goroutine to start polling and don't use the primary..
	err = common.CallAllFunctions(ctx, ps, ps.timeout, nil, ps.Calls)
	if err != nil {
		ps.primary.UsePrimary = false
		ps.primary.PollPrimaryChan <- true

	}

	return ps, nil

}

func (ps *failoverPowerSensor) Voltage(ctx context.Context, extra map[string]any) (float64, bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.primary.UsePrimary {
		readings, err := common.TryPrimary[*voltageVals](ctx, &ps.primary, extra, voltageWrapper)
		if err == nil {
			return readings.volts, readings.isAc, nil
		}
	}

	// Primary failed, find a working sensor
	err := ps.backups.GetWorkingSensor(ctx, extra, voltageWrapper, currentWrapper, powerWrapper, common.ReadingsWrapper)
	if err != nil {
		return math.NaN(), false, errors.New("all power sensors failed to get voltage")
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, ps.backups.LastWorkingSensor, voltageWrapper, extra)
	if err != nil {
		return math.NaN(), false, err
	}

	vals := readings.(*voltageVals)

	return vals.volts, vals.isAc, nil

}

// Current returns the current reading in amperes and a bool returning true if the current is AC.
func (ps *failoverPowerSensor) Current(ctx context.Context, extra map[string]any) (float64, bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.primary.UsePrimary {
		readings, err := common.TryPrimary[*currentVals](ctx, &ps.primary, extra, currentWrapper)
		if err == nil {
			return readings.amps, readings.isAc, nil
		}

	}

	err := ps.backups.GetWorkingSensor(ctx, extra, voltageWrapper, currentWrapper, powerWrapper, common.ReadingsWrapper)
	if err != nil {
		return math.NaN(), false, errors.New("all power sensors failed to get current")
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, ps.backups.LastWorkingSensor, currentWrapper, extra)
	if err != nil {
		return math.NaN(), false, err
	}

	currentVals := readings.(*currentVals)
	return currentVals.amps, currentVals.isAc, nil
}

// Power returns the power reading in watts.
func (ps *failoverPowerSensor) Power(ctx context.Context, extra map[string]any) (float64, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.primary.UsePrimary {
		// Poll the last sensor we know is working.
		// In the non-error case, the wrapper will never return its readings as nil.
		readings, err := common.TryPrimary[float64](ctx, &ps.primary, extra, powerWrapper)
		if err == nil {
			return readings, nil
		}
	}

	err := ps.backups.GetWorkingSensor(ctx, extra, voltageWrapper, currentWrapper, powerWrapper, common.ReadingsWrapper)
	if err != nil {
		return math.NaN(), errors.New("all power sensors failed to get power")
	}

	// Read from the backups last working sensor.
	// In the non-error case, the wrapper will never return its readings as nil.
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, ps.backups.LastWorkingSensor, powerWrapper, extra)
	if err != nil {
		return math.NaN(), err
	}
	watts := readings.(float64)
	return watts, nil
}

func (ps *failoverPowerSensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.primary.UsePrimary {
		readings, err := common.TryPrimary[map[string]any](ctx, &ps.primary, extra, common.ReadingsWrapper)
		if err == nil {
			return readings, nil
		}
	}

	err := ps.backups.GetWorkingSensor(ctx, extra, common.ReadingsWrapper)
	if err != nil {
		return nil, err
	}
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, ps.backups.LastWorkingSensor, common.ReadingsWrapper, extra)
	if err != nil {
		return nil, err
	}

	reading := readings.(map[string]interface{})
	return reading, nil

}

// func tryPrimary[T any](ctx context.Context, ps *failoverPowerSensor, extra map[string]any, call func(context.Context, resource.Sensor, map[string]any) (any, error)) (T, error) {
// 	readings, err := common.TryReadingOrFail(ctx, ps.timeout, ps.primary.S, call, extra)
// 	if err == nil {
// 		reading := any(readings).(T)
// 		return reading, nil
// 	}
// 	var zero T

// 	// upon error of the last working sensor, log the error.
// 	ps.logger.Warnf("primary powersensorfailed: %s", err.Error())

// 	// If the primary failed, tell the goroutine to start checking the health.
// 	ps.primary.PollPrimaryChan <- true
// 	ps.primary.UsePrimary = false
// 	return zero, err
// }

// pollPrimaryForHealth starts a background routine that waits for data to come into pollPrimary channel,
// then continuously polls the primary sensor until it returns a reading, and replaces lastWorkingSensor.
// func (s *failoverPowerSensor) PollPrimaryForHealth() {
// 	// poll every 10 ms.
// 	ticker := time.NewTicker(time.Millisecond * 10)
// 	s.workers.AddWorkers(func(ctx context.Context) {
// 		for {
// 			select {
// 			// wait for data to come into the channel before polling.
// 			case <-ctx.Done():
// 				return
// 			case <-s.pollPrimaryChan:
// 			}
// 			// label for loop so we can break out of it later.
// 		L:
// 			for {
// 				select {
// 				case <-ctx.Done():
// 					return
// 				case <-ticker.C:
// 					err := common.CallAllFunctions(ctx, s, s.timeout, nil, s.Calls)
// 					if err == nil {
// 						s.mu.Lock()
// 						s.usePrimary = true
// 						s.mu.Unlock()
// 						break L
// 					}
// 				}
// 			}
// 		}
// 	})
// }

func (s *failoverPowerSensor) Close(context.Context) error {
	s.primary.Close()
	return nil
}
