package failoversensor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

const primaryName = "primary"
const backup1Name = "backup1"
const backup2Name = "backup2"

var readingErr = fmt.Errorf("error!")

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

	return sensors, deps
}

func TestNewFailoverSensor(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	_, deps := setup(t)

	testPrimary := &inject.Sensor{}
	testBackup1 := &inject.Sensor{}
	testBackup2 := &inject.Sensor{}

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
				ConvertedAttributes: &Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			deps: deps,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newFailoverSensor(ctx, tc.deps, tc.config, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, s.Name(), test.ShouldResemble, tc.config.ResourceName())
			test.That(t, s, test.ShouldNotBeNil)
			fs := s.(*failoverSensor)
			test.That(t, fs.primary, test.ShouldResemble, testPrimary)
			test.That(t, fs.backups, test.ShouldResemble, []sensor.Sensor{testBackup1, testBackup2})
			test.That(t, fs.timeout, test.ShouldEqual, 1)

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
				ConvertedAttributes: &Config{
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
				ConvertedAttributes: &Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryReading:  nil,
			primaryErr:      readingErr,
			backup1Reading:  map[string]interface{}{"a": 1},
			expectedReading: map[string]interface{}{"a": 1},
			expectErr:       false,
		},
		{
			name: "if primary and backup1 fail, backup2 is returned",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: &Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryErr:      readingErr,
			backup1Err:      readingErr,
			backup2Reading:  map[string]interface{}{"a": 2},
			expectedReading: map[string]interface{}{"a": 2},
			expectErr:       false,
		},
		{
			name: "if all sensors error, return error",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: &Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryErr: readingErr,
			backup1Err: readingErr,
			backup2Err: readingErr,
			expectErr:  true,
		},
		{
			name: "a reading should timeout after default of 1 second",
			config: resource.Config{
				Name: "failover",
				ConvertedAttributes: &Config{
					Primary: "primary",
					Backups: []string{"backup1", "backup2"},
					Timeout: 1,
				},
			},
			primaryReading:     map[string]interface{}{"a": 1},
			primaryTimeSeconds: 2,
			backup1Reading:     map[string]interface{}{"a": 2},
			expectedReading:    map[string]interface{}{"a": 2},
			expectErr:          false,
		},
	}

	for _, tc := range tests {

		s, err := newFailoverSensor(ctx, deps, tc.config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryReading, tc.primaryErr
		}

		sensors.backup1.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			return tc.backup1Reading, nil
		}
		sensors.backup2.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			return tc.backup2Reading, nil
		}

		reading, err := s.Readings(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all sensors failed to get readings")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, reading, test.ShouldResemble, tc.expectedReading)
		}
		s.Close(ctx)

	}

}
