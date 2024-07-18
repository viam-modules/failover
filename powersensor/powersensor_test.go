package failoverpowersensor

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

const (
	primaryName = "primary"
	backup1Name = "backup1"
	backup2Name = "backup2"
)

var errTest = errors.New("error")

var config = resource.Config{
	Name: "failover",
	ConvertedAttributes: common.Config{
		Primary: "primary",
		Backups: []string{"backup1", "backup2"},
		Timeout: 1,
	},
}

type testPowerSensors struct {
	primary *inject.PowerSensor
	backup1 *inject.PowerSensor
	backup2 *inject.PowerSensor
}

func setup(t *testing.T) (testPowerSensors, resource.Dependencies) {
	t.Helper()

	deps := make(resource.Dependencies)
	powerSensors := testPowerSensors{}

	powerSensors.primary = inject.NewPowerSensor(primaryName)
	powerSensors.backup1 = inject.NewPowerSensor(backup1Name)
	powerSensors.backup2 = inject.NewPowerSensor(backup2Name)

	deps[powersensor.Named(primaryName)] = powerSensors.primary
	deps[powersensor.Named(backup1Name)] = powerSensors.backup1
	deps[powersensor.Named(backup2Name)] = powerSensors.backup2

	// Define defaults for the inject functions, these will be overridden in the tests.
	powerSensors.primary.VoltageFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
		return 1, false, nil
	}

	powerSensors.backup1.VoltageFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
		return 2, false, nil
	}
	powerSensors.backup2.VoltageFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
		return 3, false, nil
	}

	powerSensors.primary.CurrentFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
		return 4, false, nil
	}

	powerSensors.backup1.CurrentFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
		return 5, false, nil
	}
	powerSensors.backup2.CurrentFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
		return 6, false, nil
	}

	powerSensors.primary.PowerFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
		return 7, nil
	}

	powerSensors.backup1.PowerFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
		return 8, nil
	}
	powerSensors.backup2.PowerFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
		return 9, nil
	}

	powerSensors.primary.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
		return map[string]any{"foo": 1}, nil
	}

	powerSensors.backup1.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
		return map[string]any{"foo": 2}, nil
	}
	powerSensors.backup2.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
		return map[string]any{"foo": 3}, nil
	}

	return powerSensors, deps
}

func TestNewFailoverPowerSensor(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	_, deps := setup(t)

	tests := []struct {
		name        string
		deps        resource.Dependencies
		expectedErr error
	}{
		{
			name: "A valid config should successfully create failover power sensor",
			deps: deps,
		},
		{
			name:        "config without dependencies should error",
			expectedErr: errors.New("Resource missing from dependencies"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newFailoverPowerSensor(ctx, tc.deps, config, logger)
			if tc.expectedErr != nil {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expectedErr.Error())
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, s, test.ShouldNotBeNil)
				test.That(t, s.Name(), test.ShouldResemble, config.ResourceName())
				fs := s.(*failoverPowerSensor)
				test.That(t, fs.primary, test.ShouldNotBeNil)
				test.That(t, fs.timeout, test.ShouldEqual, 1)
			}
		})
	}
}

func TestPower(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryPower       float64
		backup1Power       float64
		backup2Power       float64
		expectedPower      float64
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:          "if the primary succeeds, should return primary reading",
			primaryPower:  2.4,
			backup1Power:  3,
			backup2Power:  4,
			expectedPower: 2.4,
			expectErr:     false,
		},
		{
			name:          "if the primary fails, backup1 is returned",
			primaryPower:  2.4,
			primaryErr:    errTest,
			backup1Power:  3,
			backup2Power:  4,
			expectedPower: 3,
		},
		{
			name:          "if primary and backup1 fail, backup2 is returned",
			primaryErr:    errTest,
			backup1Err:    errTest,
			expectErr:     false,
			primaryPower:  2.4,
			backup1Power:  3,
			backup2Power:  4,
			expectedPower: 4,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryPower:       2.4,
			backup1Power:       3,
			expectedPower:      3,
			expectErr:          false,
		},
		{
			name:       "if all sensors error, return error",
			primaryErr: errTest,
			backup1Err: errTest,
			backup2Err: errTest,
			expectErr:  true,
		},
	}

	for _, tc := range tests {
		goRoutinesStart := runtime.NumGoroutine()

		sensors.primary.PowerFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryPower, tc.primaryErr
		}

		sensors.backup1.PowerFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
			return tc.backup1Power, tc.backup1Err
		}
		sensors.backup2.PowerFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
			return tc.backup2Power, tc.backup2Err
		}

		s, err := newFailoverPowerSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		watts, err := s.Power(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all power sensors failed to get power")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, watts, test.ShouldEqual, tc.expectedPower)
		}

		err = s.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		// Check how many routines are still running to ensure there are no leaks from power sensor.
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestCurrent(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryCurrent     float64
		backup1Current     float64
		backup2Current     float64
		expectedCurrent    float64
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:            "if the primary succeeds, should return primary reading",
			primaryCurrent:  2.4,
			backup1Current:  3,
			backup2Current:  4,
			expectedCurrent: 2.4,
			expectErr:       false,
		},
		{
			name:            "if the primary fails, backup1 is returned",
			primaryCurrent:  2.4,
			primaryErr:      errTest,
			backup1Current:  3,
			backup2Current:  4,
			expectedCurrent: 3,
		},
		{
			name:            "if primary and backup1 fail, backup2 is returned",
			primaryErr:      errTest,
			backup1Err:      errTest,
			expectErr:       false,
			primaryCurrent:  2.4,
			backup1Current:  3,
			backup2Current:  4,
			expectedCurrent: 4,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryCurrent:     2.4,
			backup1Current:     3,
			expectedCurrent:    3,
			expectErr:          false,
		},
		{
			name:       "if all sensors error, return error",
			primaryErr: errTest,
			backup1Err: errTest,
			backup2Err: errTest,
			expectErr:  true,
		},
	}

	for _, tc := range tests {
		// Check how many goroutines are running before we create the power sensor to compare at the end.
		goRoutinesStart := runtime.NumGoroutine()

		sensors.primary.CurrentFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryCurrent, false, tc.primaryErr
		}

		sensors.backup1.CurrentFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
			return tc.backup1Current, false, tc.backup1Err
		}
		sensors.backup2.CurrentFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
			return tc.backup2Current, false, tc.backup2Err
		}
		s, err := newFailoverPowerSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)
		amps, ac, err := s.Current(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all power sensors failed to get current")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, amps, test.ShouldEqual, tc.expectedCurrent)
			test.That(t, ac, test.ShouldEqual, false)
		}

		err = s.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		// Check how many routines are still running to ensure there are no leaks from power sensor.
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestVoltage(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryVoltage     float64
		backup1Voltage     float64
		backup2Voltage     float64
		expectedVoltage    float64
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:            "if the primary succeeds, should return primary reading",
			primaryVoltage:  2.4,
			backup1Voltage:  3,
			backup2Voltage:  4,
			expectedVoltage: 2.4,
			expectErr:       false,
		},
		{
			name:            "if the primary fails, backup1 is returned",
			primaryVoltage:  2.4,
			primaryErr:      errTest,
			backup1Voltage:  3,
			backup2Voltage:  4,
			expectedVoltage: 3,
		},
		{
			name:            "if primary and backup1 fail, backup2 is returned",
			primaryErr:      errTest,
			backup1Err:      errTest,
			expectErr:       false,
			primaryVoltage:  2.4,
			backup1Voltage:  3,
			backup2Voltage:  4,
			expectedVoltage: 4,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryVoltage:     2.4,
			backup1Voltage:     3,
			expectedVoltage:    3,
			expectErr:          false,
		},
		{
			name:       "if all sensors error, return error",
			primaryErr: errTest,
			backup1Err: errTest,
			backup2Err: errTest,
			expectErr:  true,
		},
	}

	for _, tc := range tests {
		// Check how many goroutines are running before we create the power sensor to compare at the end.
		goRoutinesStart := runtime.NumGoroutine()

		sensors.primary.VoltageFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryVoltage, false, tc.primaryErr
		}

		sensors.backup1.VoltageFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
			return tc.backup1Voltage, false, tc.backup1Err
		}
		sensors.backup2.VoltageFunc = func(ctx context.Context, extra map[string]any) (float64, bool, error) {
			return tc.backup2Voltage, false, tc.backup2Err
		}

		s, err := newFailoverPowerSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		volts, ac, err := s.Voltage(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all power sensors failed to get voltage")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, volts, test.ShouldEqual, tc.expectedVoltage)
			test.That(t, ac, test.ShouldEqual, false)
		}

		err = s.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		// Check how many routines are still running to ensure there are no leaks from power sensor.
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestReadings(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryReading     map[string]any
		primaryErr         error
		backup1Reading     map[string]any
		backup1Err         error
		backup2Reading     map[string]any
		backup2Err         error
		expectedReading    map[string]any
		primaryTimeSeconds int
		expectErr          bool
	}{
		{
			name:            "if the primary succeeds, should return primary reading",
			primaryReading:  map[string]any{"primary_reading": 1},
			backup1Reading:  map[string]any{"a": 1},
			expectedReading: map[string]any{"primary_reading": 1},
		},
		{
			name:            "if the primary fails, backup1 is returned",
			primaryReading:  nil,
			primaryErr:      errTest,
			backup1Reading:  map[string]any{"a": 1},
			expectedReading: map[string]any{"a": 1},
			expectErr:       false,
		},
		{
			name:            "if primary and backup1 fail, backup2 is returned",
			primaryErr:      errTest,
			backup1Err:      errTest,
			backup2Reading:  map[string]any{"a": 2},
			expectedReading: map[string]any{"a": 2},
			expectErr:       false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryReading:     map[string]any{"a": 1},
			backup1Reading:     map[string]any{"a": 2},
			expectedReading:    map[string]any{"a": 2},
			expectErr:          false,
		},
		{
			name:       "if all sensors error, return error",
			primaryErr: errTest,
			backup1Err: errTest,
			backup2Err: errTest,
			expectErr:  true,
		},
	}

	for _, tc := range tests {
		// Check how many goroutines are running before we create the power sensor to compare at the end.
		goRoutinesStart := runtime.NumGoroutine()

		sensors.primary.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryReading, tc.primaryErr
		}

		sensors.backup1.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
			return tc.backup1Reading, tc.backup1Err
		}
		sensors.backup2.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
			return tc.backup2Reading, tc.backup2Err
		}

		s, err := newFailoverPowerSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		reading, err := s.Readings(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all power sensors failed to get readings")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, reading, test.ShouldResemble, tc.expectedReading)
		}

		err = s.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		// Check how many routines are still running to ensure there are no leaks from power sensor.
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}
