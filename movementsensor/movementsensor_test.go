package failovermovementsensor

import (
	"context"
	"errors"
	"failover/common"
	"runtime"
	"testing"
	"time"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
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

var config = resource.Config{
	Name: "failover",
	ConvertedAttributes: common.Config{
		Primary: "primary",
		Backups: []string{"backup1", "backup2"},
		Timeout: 1,
	},
}

type testMovementSensors struct {
	primary *inject.MovementSensor
	backup1 *inject.MovementSensor
	backup2 *inject.MovementSensor
}

func setup(t *testing.T) (testMovementSensors, resource.Dependencies) {
	t.Helper()

	deps := make(resource.Dependencies)
	movementSensors := testMovementSensors{}

	movementSensors.primary = inject.NewMovementSensor(primaryName)
	movementSensors.backup1 = inject.NewMovementSensor(backup1Name)
	movementSensors.backup2 = inject.NewMovementSensor(backup2Name)

	deps[movementsensor.Named(primaryName)] = movementSensors.primary
	deps[movementsensor.Named(backup1Name)] = movementSensors.backup1
	deps[movementsensor.Named(backup2Name)] = movementSensors.backup2

	return movementSensors, deps
}

func TestNewFailoverMovementSensor(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	_, deps := setup(t)

	tests := []struct {
		name        string
		deps        resource.Dependencies
		expectedErr error
	}{
		{
			name: "A valid config should successfully create failover movement sensor",
			deps: deps,
		},
		{
			name:        "config without dependencies should error",
			expectedErr: errors.New("Resource missing from dependencies"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newFailoverMovementSensor(ctx, tc.deps, config, logger)
			if tc.expectedErr != nil {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.expectedErr.Error())
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, s, test.ShouldNotBeNil)
				test.That(t, s.Name(), test.ShouldResemble, config.ResourceName())
				fs := s.(*failoverMovementSensor)
				test.That(t, fs.primary, test.ShouldNotBeNil)
				test.That(t, len(fs.allBackups), test.ShouldEqual, 2)
				test.That(t, fs.timeout, test.ShouldEqual, 1)
			}
		})
	}
}

func TestPosition(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	var tests = []struct {
		name string

		primaryPos         *geo.Point
		backup1Pos         *geo.Point
		backup2Pos         *geo.Point
		expectedPos        *geo.Point
		primaryAlt         float64
		backup1Alt         float64
		backup2Alt         float64
		expectedAlt        float64
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:        "if the primary succeeds, should return primary reading",
			primaryPos:  geo.NewPoint(1, 1),
			primaryAlt:  4,
			backup1Pos:  geo.NewPoint(2, 2),
			expectedPos: geo.NewPoint(1, 1),
			expectedAlt: 4,
			expectErr:   false,
		},
		{
			name:        "if the primary fails, backup1 is returned",
			primaryPos:  geo.NewPoint(1, 1),
			primaryAlt:  4,
			backup1Pos:  geo.NewPoint(2, 2),
			backup1Alt:  3,
			expectedPos: geo.NewPoint(2, 2),
			expectedAlt: 3,
			expectErr:   false,
		},
		{
			name:        "if primary and backup1 fail, backup2 is returned",
			primaryErr:  errTest,
			backup1Err:  errTest,
			primaryPos:  geo.NewPoint(1, 1),
			primaryAlt:  4,
			backup1Pos:  geo.NewPoint(2, 2),
			backup1Alt:  3,
			backup2Pos:  geo.NewPoint(3, 3),
			backup2Alt:  2,
			expectedPos: geo.NewPoint(3, 3),
			expectedAlt: 2,
			expectErr:   false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryPos:         geo.NewPoint(1, 1),
			primaryAlt:         4,
			backup1Pos:         geo.NewPoint(2, 2),
			backup1Alt:         3,
			expectedPos:        geo.NewPoint(2, 2),
			expectedAlt:        3,
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
		s, err := newFailoverMovementSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return &movementsensor.Properties{
				PositionSupported: true,
			}, nil
		}

		sensors.backup1.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return &movementsensor.Properties{
				PositionSupported: true,
			}, nil
		}

		sensors.backup2.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return &movementsensor.Properties{
				PositionSupported: true,
			}, nil
		}

		sensors.primary.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryPos, tc.primaryAlt, tc.primaryErr
		}

		sensors.backup1.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
			return tc.backup1Pos, tc.backup1Alt, tc.primaryErr
		}

		sensors.backup2.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
			return tc.backup2Pos, tc.backup2Alt, tc.primaryErr
		}

		pos, alt, err := s.Position(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get position")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos, test.ShouldEqual, tc.expectedPos)
			test.That(t, alt, test.ShouldEqual, tc.expectedAlt)
		}

		err = s.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}

}
