package failoversensor

import (
	"context"
	"errors"
	"failover/common"
	"runtime"
	"testing"
	"time"

	"go.viam.com/rdk/components/sensor"
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

var errReading = errors.New("error")

type testSensors struct {
	primary *inject.Sensor
	backup1 *inject.Sensor
	backup2 *inject.Sensor
}

func setup(t *testing.T) (testSensors, resource.Dependencies) {
	t.Helper()

	deps := make(resource.Dependencies)
	sensors := testSensors{}

	sensors.primary = inject.NewSensor(primaryName)
	sensors.backup1 = inject.NewSensor(backup1Name)
	sensors.backup2 = inject.NewSensor(backup2Name)

	deps[sensor.Named(primaryName)] = sensors.primary
	deps[sensor.Named(backup1Name)] = sensors.backup1
	deps[sensor.Named(backup2Name)] = sensors.backup2

	sensors.primary.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]any{"foo": 1}, nil
	}

	sensors.backup1.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]any{"foo": 1}, nil
	}
	sensors.backup2.ReadingsFunc = func(_ context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]any{"foo": 1}, nil
	}

	return sensors, deps
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
			name: "A valid config should successfully create failover sensor",
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
			s, err := newFailoverSensor(ctx, tc.deps, tc.config, logger)
			if tc.expectedErr != nil {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expectedErr.Error())
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, s, test.ShouldNotBeNil)
				test.That(t, s.Name(), test.ShouldResemble, tc.config.ResourceName())
				fs := s.(*failoverSensor)
				test.That(t, fs.primary, test.ShouldNotBeNil)
				test.That(t, fs.timeout, test.ShouldEqual, 1)
			}
		})
	}
}

func TestReadings(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name               string
		config             resource.Config
		primaryReading     map[string]interface{}
		primaryErr         error
		backup1Reading     map[string]interface{}
		backup1Err         error
		backup2Reading     map[string]interface{}
		backup2Err         error
		expectedReading    map[string]interface{}
		expectErr          bool
		primaryTimeSeconds int
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
			primaryErr:      errReading,
			backup1Reading:  map[string]interface{}{"a": 1},
			expectedReading: map[string]interface{}{"a": 1},
			expectErr:       false,
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
			primaryErr:      errReading,
			backup1Err:      errReading,
			backup2Reading:  map[string]interface{}{"a": 2},
			expectedReading: map[string]interface{}{"a": 2},
			expectErr:       false,
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
			primaryErr: errReading,
			backup1Err: errReading,
			backup2Err: errReading,
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
			primaryReading:     map[string]interface{}{"a": 1},
			primaryTimeSeconds: 1,
			backup1Reading:     map[string]interface{}{"a": 2},
			expectedReading:    map[string]interface{}{"a": 2},
			expectErr:          false,
		},
	}

	for _, tc := range tests {
		// Check how many goroutines are running before we create the power sensor to compare at the end.
		goRoutinesStart := runtime.NumGoroutine()

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

		s, err := newFailoverSensor(ctx, deps, tc.config, logger)
		test.That(t, err, test.ShouldBeNil)

		reading, err := s.Readings(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all sensors failed to get readings")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, reading, test.ShouldResemble, tc.expectedReading)
		}

		err = s.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		// Check how many routines are still running to ensure there are no leaks from sensor.
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}
