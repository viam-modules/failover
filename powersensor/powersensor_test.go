package failoverpowersensor

import (
	"context"
	"errors"
	"failover/common"
	"fmt"
	"runtime"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	primaryName = "primary"
	backup1Name = "backup1"
	backup2Name = "backup2"
)

var errTest = errors.New("error")

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

	return powerSensors, deps
}

func TestNewFailoverSensor(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	_, deps := setup(t)

	tests := []struct {
		name        string
		config      resource.Config
		deps        resource.Dependencies
		expectedErr error
	}{
		{
			name: "A valid config should successfully create failover power sensor",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: common.Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			deps: deps,
		},
		{
			name: "config without dependencies should error",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: common.Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			expectedErr: errors.New("Resource missing from dependencies"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newFailoverPowerSensor(ctx, tc.deps, tc.config, logger)
			if tc.expectedErr != nil {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expectedErr.Error())
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, s, test.ShouldNotBeNil)
				test.That(t, s.Name(), test.ShouldResemble, tc.config.ResourceName())
				fs := s.(*failoverPowerSensor)
				test.That(t, fs.primary, test.ShouldNotBeNil)
				test.That(t, len(fs.backups), test.ShouldEqual, 2)
				test.That(t, fs.timeout, test.ShouldEqual, 1)
			}
		})
	}
}

func TestFunctions(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	var tests = []struct {
		name   string
		config resource.Config

		primaryReading  map[string]interface{}
		primaryErr      error
		backup1Reading  map[string]interface{}
		backup1Err      error
		backup2Reading  map[string]interface{}
		backup2Err      error
		expectedReading map[string]interface{}

		primaryVoltage  float64
		backup1Voltage  float64
		backup2Voltage  float64
		expectedVoltage float64

		primaryCurrent  float64
		backup1Current  float64
		backup2Current  float64
		expectedCurrent float64

		primaryPower  float64
		backup1Power  float64
		backup2Power  float64
		expectedPower float64

		primaryTimeSeconds int
		expectErr          bool
	}{
		{
			name: "if the primary succeeds, should return primary reading",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: common.Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryReading:  map[string]interface{}{"primary_reading": 1},
			backup1Reading:  map[string]interface{}{"a": 1},
			expectedReading: map[string]interface{}{"primary_reading": 1},
			primaryVoltage:  2.4,
			backup1Voltage:  3,
			backup2Voltage:  4,
			expectedVoltage: 2.4,
			primaryCurrent:  1,
			backup1Current:  3,
			backup2Current:  4,
			expectedCurrent: 1,
			primaryPower:    3,
			backup1Power:    4,
			backup2Power:    5,
			expectedPower:   3,
		},
		{
			name: "if the primary fails, backup1 is returned",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: common.Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryReading:  nil,
			primaryErr:      errTest,
			backup1Reading:  map[string]interface{}{"a": 1},
			expectedReading: map[string]interface{}{"a": 1},
			expectErr:       false,
			primaryVoltage:  2.4,
			backup1Voltage:  3,
			backup2Voltage:  4,
			expectedVoltage: 3,
			primaryCurrent:  1,
			backup1Current:  3,
			backup2Current:  4,
			expectedCurrent: 3,
			primaryPower:    3,
			backup1Power:    4,
			backup2Power:    5,
			expectedPower:   4,
		},
		{
			name: "if primary and backup1 fail, backup2 is returned",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: common.Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryErr:      errTest,
			backup1Err:      errTest,
			backup2Reading:  map[string]interface{}{"a": 2},
			expectedReading: map[string]interface{}{"a": 2},
			expectErr:       false,
			primaryVoltage:  2.4,
			backup1Voltage:  3,
			backup2Voltage:  4,
			expectedVoltage: 4,
			primaryCurrent:  1,
			backup1Current:  3,
			backup2Current:  4,
			expectedCurrent: 4,
			primaryPower:    3,
			backup1Power:    4,
			backup2Power:    5,
			expectedPower:   5,
		},
		{
			name: "if all sensors error, return error",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: common.Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryErr: errTest,
			backup1Err: errTest,
			backup2Err: errTest,
			expectErr:  true,
		},
		{
			name: "a reading should timeout after default of 1 second",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: common.Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryTimeSeconds: 1,
			primaryReading:     map[string]interface{}{"a": 1},
			backup1Reading:     map[string]interface{}{"a": 2},
			expectedReading:    map[string]interface{}{"a": 2},
			primaryVoltage:     2.4,
			backup1Voltage:     3,
			expectedVoltage:    3,
			primaryCurrent:     1,
			backup1Current:     3,
			expectedCurrent:    3,
			primaryPower:       3,
			backup1Power:       4,
			expectedPower:      4,
			expectErr:          false,
		},
	}

	for _, tc := range tests {
		goRoutinesStart := runtime.NumGoroutine()
		s, err := newFailoverPowerSensor(ctx, deps, tc.config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryReading, tc.primaryErr
		}

		sensors.backup1.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			return tc.backup1Reading, tc.backup1Err
		}
		sensors.backup2.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			return tc.backup2Reading, tc.backup2Err
		}

		sensors.primary.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryVoltage, false, tc.primaryErr
		}

		sensors.backup1.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
			return tc.backup1Voltage, false, tc.backup1Err
		}
		sensors.backup2.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
			return tc.backup2Voltage, false, tc.backup2Err
		}

		sensors.primary.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryCurrent, false, tc.primaryErr
		}

		sensors.backup1.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
			return tc.backup1Current, false, tc.backup1Err
		}
		sensors.backup2.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
			return tc.backup2Current, false, tc.backup2Err
		}
		sensors.primary.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryPower, tc.primaryErr
		}

		sensors.backup1.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return tc.backup1Power, tc.backup1Err
		}
		sensors.backup2.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return tc.backup2Power, tc.backup2Err
		}

		reading, err := s.Readings(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all power sensors failed to get readings")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, reading, test.ShouldResemble, tc.expectedReading)
		}

		volts, ac, err := s.Voltage(ctx, nil)
		fmt.Println(err)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all power sensors failed to get voltage")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, volts, test.ShouldEqual, tc.expectedVoltage)
			test.That(t, ac, test.ShouldEqual, false)
		}

		amps, ac, err := s.Current(ctx, nil)
		fmt.Println(err)

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
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}
