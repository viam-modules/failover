package failoverpowersensor

import (
	"context"
	"errors"
	"failover/common"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
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

	primary powersensor.PowerSensor
	backups []powersensor.PowerSensor

	logger  logging.Logger
	workers rdkutils.StoppableWorkers

	timeout int

	mu sync.Mutex

	lastWorkingPower    powersensor.PowerSensor
	lastWorkingCurrent  powersensor.PowerSensor
	lastWorkingVoltage  powersensor.PowerSensor
	lastWorkingReadings powersensor.PowerSensor

	pollVoltageChan  chan bool
	pollCurrentChan  chan bool
	pollPowerChan    chan bool
	pollReadingsChan chan bool
}

func newFailoverPowerSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (powersensor.PowerSensor, error) {
	config, err := resource.NativeConfig[common.Config](conf)
	if err != nil {
		return nil, err
	}

	ps := &failoverPowerSensor{
		Named:   conf.ResourceName().AsNamed(),
		logger:  logger,
		workers: rdkutils.NewStoppableWorkers(),
	}

	primary, err := powersensor.FromDependencies(deps, config.Primary)
	if err != nil {
		return nil, err
	}
	ps.primary = primary
	ps.backups = []powersensor.PowerSensor{}

	for _, backup := range config.Backups {
		backup, err := powersensor.FromDependencies(deps, backup)
		if err != nil {
			ps.logger.Errorf(err.Error())
		} else {
			ps.backups = append(ps.backups, backup)
		}
	}

	ps.lastWorkingReadings = primary
	ps.lastWorkingCurrent = primary
	ps.lastWorkingVoltage = primary
	ps.lastWorkingPower = primary

	// default timeout is 1 second.
	ps.timeout = 1000
	if config.Timeout > 0 {
		ps.timeout = config.Timeout
	}

	ps.pollVoltageChan = make(chan bool)
	ps.pollCurrentChan = make(chan bool)
	ps.pollPowerChan = make(chan bool)
	ps.pollReadingsChan = make(chan bool)

	PollPrimaryForHealth(ps, "voltage", ps.pollVoltageChan, voltageWrapper)
	PollPrimaryForHealth(ps, "current", ps.pollCurrentChan, currentWrapper)
	PollPrimaryForHealth(ps, "power", ps.pollPowerChan, powerWrapper)
	PollPrimaryForHealth(ps, "readings", ps.pollReadingsChan, common.ReadingsWrapper)

	return ps, nil

}

func (ps *failoverPowerSensor) Voltage(ctx context.Context, extra map[string]any) (float64, bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// Poll the last sensor we know is working
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, ps.lastWorkingVoltage, voltageWrapper, extra)

	if err == nil {
		return readings.volts, readings.isAc, nil
	}

	// upon error of the last working sensor, log the error.
	ps.logger.Warnf("powersensor %s failed to get voltage: %s", ps.lastWorkingVoltage.Name().ShortName(), err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	switch ps.lastWorkingVoltage {
	case ps.primary:
		ps.pollPowerChan <- true
	default:
	}

	readings, lastWorking, err := tryBackups(ctx, ps.lastWorkingVoltage, ps, voltageWrapper, extra)
	if err != nil {
		return 0, false, errors.New("all power sensors failed to get voltage")
	}
	ps.lastWorkingVoltage = lastWorking

	return readings.volts, readings.isAc, nil

}

// Current returns the current reading in amperes and a bool returning true if the current is AC.
func (ps *failoverPowerSensor) Current(ctx context.Context, extra map[string]any) (float64, bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// Poll the last sensor we know is working
	currentVals, err := common.TryReadingOrFail(ctx, ps.timeout, ps.lastWorkingCurrent, currentWrapper, extra)
	if err == nil {
		return currentVals.amps, currentVals.isAc, nil
	}

	// upon error of the last working sensor, log the error.
	ps.logger.Warnf("powersensor %s failed to get current: %s", ps.lastWorkingCurrent.Name().ShortName(), err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	switch ps.lastWorkingCurrent {
	case ps.primary:
		ps.pollCurrentChan <- true
	default:
	}

	currentVals, lastWorking, err := tryBackups(ctx, ps.lastWorkingCurrent, ps, currentWrapper, extra)
	if err != nil {
		return 0, false, errors.New("all power sensors failed to get current")
	}
	ps.lastWorkingCurrent = lastWorking
	return currentVals.amps, currentVals.isAc, nil
}

// Power returns the power reading in watts.
func (ps *failoverPowerSensor) Power(ctx context.Context, extra map[string]any) (float64, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Poll the last sensor we know is working
	watts, err := common.TryReadingOrFail(ctx, ps.timeout, ps.lastWorkingCurrent, powerWrapper, extra)
	if err == nil {
		return watts, nil
	}
	// upon error of the last working sensor, log the error.
	ps.logger.Warnf("powersensor %s failed to get power: %s", ps.lastWorkingCurrent.Name().ShortName(), err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	switch ps.lastWorkingPower {
	case ps.primary:
		ps.pollPowerChan <- true
	default:
	}

	watts, lastWorking, err := tryBackups(ctx, ps.lastWorkingPower, ps, powerWrapper, extra)
	if err != nil {
		return 0, errors.New("all power sensors failed to get power")
	}
	ps.lastWorkingPower = lastWorking
	return watts, nil
}

func (ps *failoverPowerSensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Poll the last sensor we know is working
	readings, err := common.TryReadingOrFail(ctx, ps.timeout, ps.lastWorkingReadings, common.ReadingsWrapper, extra)
	if err == nil {
		return readings, nil
	}

	// upon error of the last working sensor, log the error.
	ps.logger.Warnf("powersensor %s failed: %s", ps.lastWorkingReadings.Name().ShortName(), err.Error())

	// If the primary failed, tell the goroutine to start checking the health.
	switch ps.lastWorkingReadings {
	case ps.primary:
		ps.pollReadingsChan <- true
	default:
	}

	readings, lastWorking, err := tryBackups(ctx, ps.lastWorkingReadings, ps, common.ReadingsWrapper, extra)
	if err != nil {
		return nil, errors.New("all power sensors failed to get readings")
	}
	ps.lastWorkingReadings = lastWorking
	return readings, nil

}

func tryBackups[T any](ctx context.Context,
	lastWorkingSensor powersensor.PowerSensor,
	ps *failoverPowerSensor,
	call func(ctx context.Context, ps resource.Sensor, extra map[string]any) (T, error),
	extra map[string]any) (
	T, powersensor.PowerSensor, error) {
	var zero T
	for _, backup := range ps.backups {
		// if the last working sensor is a backup, it was already tried.
		if lastWorkingSensor == backup {
			continue
		}
		ps.logger.Infof("calling backup %s", backup.Name())
		reading, err := common.TryReadingOrFail[T](ctx, ps.timeout, backup, call, extra)
		if err != nil {
			ps.logger.Warn(err.Error())
		} else {
			ps.logger.Infof("successfully got reading from %s", backup.Name())
			return reading, backup, nil
		}
	}
	return zero, nil, fmt.Errorf("all power sensors failed")
}

// pollPrimaryForHealth starts a background routine that waits for data to come into the start channel,
// then continuously polls the primary sensor until it returns a reading, and replaces lastWorkingSensor.
func PollPrimaryForHealth[K any](s *failoverPowerSensor,
	name string,
	startChan chan bool,
	call func(context.Context, resource.Sensor, map[string]any) (K, error)) {
	// poll every 10 ms.
	ticker := time.NewTicker(time.Millisecond * 10)
	s.workers.AddWorkers(func(ctx context.Context) {
		for {
			select {
			// wait for data to come into the channel before polling.
			case <-ctx.Done():
				return
			case <-startChan:
			}
			// label for loop so we can break out of it later.
		L:
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_, err := common.TryReadingOrFail(ctx, s.timeout, s.primary, call, nil)
					if err == nil {
						s.logger.Infof("successfully got reading from primary sensor")
						// Change the last working sensor for the API to primary
						switch name {
						case "power":
							s.mu.Lock()
							s.lastWorkingPower = s.primary
							s.mu.Unlock()
						case "voltage":
							s.mu.Lock()
							s.lastWorkingVoltage = s.primary
							s.mu.Unlock()
						case "current":
							s.mu.Lock()
							s.lastWorkingVoltage = s.primary
							s.mu.Unlock()
						case "readings":
							s.mu.Lock()
							s.lastWorkingReadings = s.primary
							s.mu.Unlock()
						}
						break L
					}
				}
			}
		}
	})
}

func (s *failoverPowerSensor) Close(context.Context) error {

	if s.workers != nil {
		s.workers.Stop()
	}
	close(s.pollCurrentChan)
	close(s.pollPowerChan)
	close(s.pollVoltageChan)
	close(s.pollReadingsChan)
	return nil
}
