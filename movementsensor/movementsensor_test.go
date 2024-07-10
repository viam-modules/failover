package failovermovementsensor

import (
	"context"
	"errors"
	"failover/common"
	"runtime"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
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

	movementSensors.primary.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]any, error) {
		return map[string]any{"a": 4}, nil
	}

	movementSensors.backup1.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]any, error) {
		return map[string]any{"a": 4}, nil
	}

	movementSensors.backup2.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]any, error) {
		return map[string]any{"a": 4}, nil
	}

	movementSensors.primary.LinearVelocityFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
		return r3.Vector{}, nil
	}

	movementSensors.backup1.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return r3.Vector{}, nil
	}

	movementSensors.backup2.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return r3.Vector{}, nil
	}

	movementSensors.primary.LinearAccelerationFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
		return r3.Vector{}, nil
	}

	movementSensors.backup1.LinearAccelerationFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
		return r3.Vector{}, nil
	}

	movementSensors.backup2.LinearAccelerationFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
		return r3.Vector{}, nil
	}

	movementSensors.primary.AngularVelocityFunc = func(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
		return spatialmath.AngularVelocity{}, nil
	}

	movementSensors.backup1.AngularVelocityFunc = func(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
		return spatialmath.AngularVelocity{}, nil
	}

	movementSensors.backup2.AngularVelocityFunc = func(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
		return spatialmath.AngularVelocity{}, nil
	}

	movementSensors.primary.CompassHeadingFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
		return 8, nil
	}

	movementSensors.backup1.CompassHeadingFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
		return 8, nil
	}

	movementSensors.backup2.CompassHeadingFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
		return 8, nil
	}

	movementSensors.primary.OrientationFunc = func(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
		return &spatialmath.OrientationVector{}, nil
	}

	movementSensors.backup1.OrientationFunc = func(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
		return &spatialmath.OrientationVector{}, nil
	}

	movementSensors.backup2.OrientationFunc = func(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
		return &spatialmath.OrientationVector{}, nil
	}
	movementSensors.primary.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
		return &geo.Point{}, 9, nil
	}
	movementSensors.backup1.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
		return &geo.Point{}, 9, nil
	}
	movementSensors.backup2.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
		return &geo.Point{}, 9, nil
	}

	movementSensors.primary.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{
			PositionSupported:           true,
			CompassHeadingSupported:     true,
			LinearAccelerationSupported: true,
			LinearVelocitySupported:     true,
			OrientationSupported:        true,
			AngularVelocitySupported:    true,
		}, nil
	}

	movementSensors.backup1.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{
			PositionSupported:           true,
			CompassHeadingSupported:     true,
			LinearAccelerationSupported: true,
			LinearVelocitySupported:     true,
			OrientationSupported:        true,
			AngularVelocitySupported:    true,
		}, nil
	}

	movementSensors.backup2.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{
			PositionSupported:           true,
			CompassHeadingSupported:     true,
			LinearAccelerationSupported: true,
			LinearVelocitySupported:     true,
			OrientationSupported:        true,
			AngularVelocitySupported:    true,
		}, nil
	}

	movementSensors.primary.AccuracyFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error) {
		return &movementsensor.Accuracy{NmeaFix: 2}, nil
	}
	movementSensors.backup1.AccuracyFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error) {
		return &movementsensor.Accuracy{NmeaFix: 2}, nil
	}
	movementSensors.backup2.AccuracyFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error) {
		return &movementsensor.Accuracy{NmeaFix: 2}, nil
	}

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
				test.That(t, fs.timeout, test.ShouldEqual, 1)
			}
		})
	}
}

func TestPosition(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
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
			primaryErr:  errTest,
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

		sensors.primary.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryPos, tc.primaryAlt, tc.primaryErr
		}

		sensors.backup1.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
			return tc.backup1Pos, tc.backup1Alt, tc.backup1Err
		}

		sensors.backup2.PositionFunc = func(ctx context.Context, extra map[string]any) (*geo.Point, float64, error) {
			return tc.backup2Pos, tc.backup2Alt, tc.backup2Err
		}

		pos, alt, err := s.Position(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get position")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos, test.ShouldResemble, tc.expectedPos)
			test.That(t, alt, test.ShouldEqual, tc.expectedAlt)
		}

		err = s.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestLinearVelocity(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryRet         r3.Vector
		backup1Ret         r3.Vector
		backup2Ret         r3.Vector
		expectedRet        r3.Vector
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:        "if the primary succeeds, should return primary reading",
			primaryRet:  r3.Vector{X: 1, Y: 2, Z: 3},
			backup1Ret:  r3.Vector{X: 3, Y: 1, Z: 2},
			backup2Ret:  r3.Vector{X: 4, Y: 5, Z: 6},
			expectedRet: r3.Vector{X: 1, Y: 2, Z: 3},
			expectErr:   false,
		},
		{
			name:        "if the primary fails, backup1 is returned",
			primaryRet:  r3.Vector{X: 1, Y: 2, Z: 3},
			backup1Ret:  r3.Vector{X: 3, Y: 1, Z: 2},
			backup2Ret:  r3.Vector{X: 4, Y: 5, Z: 6},
			expectedRet: r3.Vector{X: 3, Y: 1, Z: 2},
			expectErr:   false,
			primaryErr:  errTest,
		},
		{
			name:        "if primary and backup1 fail, backup2 is returned",
			primaryErr:  errTest,
			backup1Err:  errTest,
			primaryRet:  r3.Vector{X: 1, Y: 2, Z: 3},
			backup1Ret:  r3.Vector{X: 3, Y: 1, Z: 2},
			backup2Ret:  r3.Vector{X: 4, Y: 5, Z: 6},
			expectedRet: r3.Vector{X: 4, Y: 5, Z: 6},
			expectErr:   false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryRet:         r3.Vector{X: 1, Y: 2, Z: 3},
			backup1Ret:         r3.Vector{X: 3, Y: 1, Z: 2},
			backup2Ret:         r3.Vector{X: 4, Y: 5, Z: 6},
			expectedRet:        r3.Vector{X: 3, Y: 1, Z: 2},
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
		ms, err := newFailoverMovementSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.LinearVelocityFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryRet, tc.primaryErr
		}

		sensors.backup1.LinearVelocityFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
			return tc.backup1Ret, tc.backup1Err
		}

		sensors.backup2.LinearVelocityFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
			return tc.backup2Ret, tc.backup2Err
		}

		vel, err := ms.LinearVelocity(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get linear velocity")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, vel, test.ShouldResemble, tc.expectedRet)
		}

		err = ms.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestAngularVelocity(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryRet         spatialmath.AngularVelocity
		backup1Ret         spatialmath.AngularVelocity
		backup2Ret         spatialmath.AngularVelocity
		expectedRet        spatialmath.AngularVelocity
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:        "if the primary succeeds, should return primary reading",
			primaryRet:  spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3},
			backup1Ret:  spatialmath.AngularVelocity{X: 3, Y: 2, Z: 1},
			backup2Ret:  spatialmath.AngularVelocity{X: 4, Y: 5, Z: 6},
			expectedRet: spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3},
			expectErr:   false,
		},
		{
			name:        "if the primary fails, backup1 is returned",
			primaryRet:  spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3},
			backup1Ret:  spatialmath.AngularVelocity{X: 3, Y: 2, Z: 1},
			backup2Ret:  spatialmath.AngularVelocity{X: 4, Y: 5, Z: 6},
			expectedRet: spatialmath.AngularVelocity{X: 3, Y: 2, Z: 1},
			expectErr:   false,
			primaryErr:  errTest,
		},
		{
			name:        "if primary and backup1 fail, backup2 is returned",
			primaryErr:  errTest,
			backup1Err:  errTest,
			primaryRet:  spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3},
			backup1Ret:  spatialmath.AngularVelocity{X: 3, Y: 2, Z: 1},
			backup2Ret:  spatialmath.AngularVelocity{X: 4, Y: 5, Z: 6},
			expectedRet: spatialmath.AngularVelocity{X: 4, Y: 5, Z: 6},
			expectErr:   false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryRet:         spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3},
			backup1Ret:         spatialmath.AngularVelocity{X: 3, Y: 2, Z: 1},
			backup2Ret:         spatialmath.AngularVelocity{X: 4, Y: 5, Z: 6},
			expectedRet:        spatialmath.AngularVelocity{X: 3, Y: 2, Z: 1},
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
		ms, err := newFailoverMovementSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.AngularVelocityFunc = func(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryRet, tc.primaryErr
		}

		sensors.backup1.AngularVelocityFunc = func(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
			return tc.backup1Ret, tc.backup1Err
		}

		sensors.backup2.AngularVelocityFunc = func(ctx context.Context, extra map[string]any) (spatialmath.AngularVelocity, error) {
			return tc.backup2Ret, tc.backup2Err
		}

		val, err := ms.AngularVelocity(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get angular velocity")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, val, test.ShouldResemble, tc.expectedRet)
		}

		err = ms.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestLinearAcceleration(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryRet         r3.Vector
		backup1Ret         r3.Vector
		backup2Ret         r3.Vector
		expectedRet        r3.Vector
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:        "if the primary succeeds, should return primary reading",
			primaryRet:  r3.Vector{X: 1, Y: 2, Z: 3},
			backup1Ret:  r3.Vector{X: 3, Y: 2, Z: 1},
			backup2Ret:  r3.Vector{X: 4, Y: 5, Z: 6},
			expectedRet: r3.Vector{X: 1, Y: 2, Z: 3},
			expectErr:   false,
		},
		{
			name:        "if the primary fails, backup1 is returned",
			primaryRet:  r3.Vector{X: 1, Y: 2, Z: 3},
			backup1Ret:  r3.Vector{X: 3, Y: 2, Z: 1},
			backup2Ret:  r3.Vector{X: 4, Y: 5, Z: 6},
			expectedRet: r3.Vector{X: 3, Y: 2, Z: 1},
			expectErr:   false,
			primaryErr:  errTest,
		},
		{
			name:        "if primary and backup1 fail, backup2 is returned",
			primaryErr:  errTest,
			backup1Err:  errTest,
			primaryRet:  r3.Vector{X: 1, Y: 2, Z: 3},
			backup1Ret:  r3.Vector{X: 3, Y: 2, Z: 1},
			backup2Ret:  r3.Vector{X: 4, Y: 5, Z: 6},
			expectedRet: r3.Vector{X: 4, Y: 5, Z: 6},
			expectErr:   false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryRet:         r3.Vector{X: 1, Y: 2, Z: 3},
			backup1Ret:         r3.Vector{X: 3, Y: 2, Z: 1},
			backup2Ret:         r3.Vector{X: 4, Y: 5, Z: 6},
			expectedRet:        r3.Vector{X: 3, Y: 2, Z: 1},
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
		ms, err := newFailoverMovementSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.LinearAccelerationFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryRet, tc.primaryErr
		}

		sensors.backup1.LinearAccelerationFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
			return tc.backup1Ret, tc.backup1Err
		}

		sensors.backup2.LinearAccelerationFunc = func(ctx context.Context, extra map[string]any) (r3.Vector, error) {
			return tc.backup2Ret, tc.backup2Err
		}

		val, err := ms.LinearAcceleration(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get linear acceleration")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, val, test.ShouldResemble, tc.expectedRet)
		}

		err = ms.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestOrientation(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryRet         spatialmath.Orientation
		backup1Ret         spatialmath.Orientation
		backup2Ret         spatialmath.Orientation
		expectedRet        spatialmath.Orientation
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:        "if the primary succeeds, should return primary reading",
			primaryRet:  &spatialmath.OrientationVector{Theta: 20},
			backup1Ret:  &spatialmath.OrientationVector{Theta: 90},
			backup2Ret:  &spatialmath.OrientationVector{Theta: 180},
			expectedRet: &spatialmath.OrientationVector{Theta: 20},
			expectErr:   false,
		},
		{
			name:        "if the primary fails, backup1 is returned",
			primaryRet:  &spatialmath.OrientationVector{Theta: 20},
			backup1Ret:  &spatialmath.OrientationVector{Theta: 90},
			backup2Ret:  &spatialmath.OrientationVector{Theta: 180},
			expectedRet: &spatialmath.OrientationVector{Theta: 90},
			expectErr:   false,
			primaryErr:  errTest,
		},
		{
			name:        "if primary and backup1 fail, backup2 is returned",
			primaryErr:  errTest,
			backup1Err:  errTest,
			primaryRet:  &spatialmath.OrientationVector{Theta: 20},
			backup1Ret:  &spatialmath.OrientationVector{Theta: 90},
			backup2Ret:  &spatialmath.OrientationVector{Theta: 180},
			expectedRet: &spatialmath.OrientationVector{Theta: 180},
			expectErr:   false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryRet:         &spatialmath.OrientationVector{Theta: 20},
			backup1Ret:         &spatialmath.OrientationVector{Theta: 90},
			backup2Ret:         &spatialmath.OrientationVector{Theta: 180},
			expectedRet:        &spatialmath.OrientationVector{Theta: 90},
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
		ms, err := newFailoverMovementSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.OrientationFunc = func(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryRet, tc.primaryErr
		}

		sensors.backup1.OrientationFunc = func(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
			return tc.backup1Ret, tc.backup1Err
		}

		sensors.backup2.OrientationFunc = func(ctx context.Context, extra map[string]any) (spatialmath.Orientation, error) {
			return tc.backup2Ret, tc.backup2Err
		}

		val, err := ms.Orientation(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get orientation")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, val, test.ShouldResemble, tc.expectedRet)
		}

		err = ms.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestCompassHeading(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryRet         float64
		backup1Ret         float64
		backup2Ret         float64
		expectedRet        float64
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:        "if the primary succeeds, should return primary reading",
			primaryRet:  20,
			backup1Ret:  30,
			backup2Ret:  40,
			expectedRet: 20,
			expectErr:   false,
		},
		{
			name:        "if the primary fails, backup1 is returned",
			primaryRet:  20,
			backup1Ret:  30,
			backup2Ret:  40,
			expectedRet: 30,
			expectErr:   false,
			primaryErr:  errTest,
		},
		{
			name:        "if primary and backup1 fail, backup2 is returned",
			primaryErr:  errTest,
			backup1Err:  errTest,
			primaryRet:  20,
			backup1Ret:  30,
			backup2Ret:  40,
			expectedRet: 40,
			expectErr:   false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryRet:         20,
			backup1Ret:         30,
			backup2Ret:         40,
			expectedRet:        30,
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
		ms, err := newFailoverMovementSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.CompassHeadingFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryRet, tc.primaryErr
		}

		sensors.backup1.CompassHeadingFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
			return tc.backup1Ret, tc.backup1Err
		}

		sensors.backup2.CompassHeadingFunc = func(ctx context.Context, extra map[string]any) (float64, error) {
			return tc.backup2Ret, tc.backup2Err
		}

		val, err := ms.CompassHeading(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get compass heading")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, val, test.ShouldEqual, tc.expectedRet)
		}

		err = ms.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}

func TestAccuracy(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	sensors, deps := setup(t)

	tests := []struct {
		name string

		primaryRet         *movementsensor.Accuracy
		backup1Ret         *movementsensor.Accuracy
		backup2Ret         *movementsensor.Accuracy
		expectedRet        *movementsensor.Accuracy
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:        "if the primary succeeds, should return primary reading",
			primaryRet:  &movementsensor.Accuracy{NmeaFix: 2},
			backup1Ret:  &movementsensor.Accuracy{NmeaFix: 3},
			backup2Ret:  &movementsensor.Accuracy{NmeaFix: 4},
			expectedRet: &movementsensor.Accuracy{NmeaFix: 2},
			expectErr:   false,
		},
		{
			name:        "if the primary fails, backup1 is returned",
			primaryRet:  &movementsensor.Accuracy{NmeaFix: 2},
			backup1Ret:  &movementsensor.Accuracy{NmeaFix: 3},
			backup2Ret:  &movementsensor.Accuracy{NmeaFix: 4},
			expectedRet: &movementsensor.Accuracy{NmeaFix: 3},
			expectErr:   false,
			primaryErr:  errTest,
		},
		{
			name:        "if primary and backup1 fail, backup2 is returned",
			primaryErr:  errTest,
			backup1Err:  errTest,
			primaryRet:  &movementsensor.Accuracy{NmeaFix: 2},
			backup1Ret:  &movementsensor.Accuracy{NmeaFix: 3},
			backup2Ret:  &movementsensor.Accuracy{NmeaFix: 4},
			expectedRet: &movementsensor.Accuracy{NmeaFix: 4},
			expectErr:   false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryRet:         &movementsensor.Accuracy{NmeaFix: 2},
			backup1Ret:         &movementsensor.Accuracy{NmeaFix: 3},
			backup2Ret:         &movementsensor.Accuracy{NmeaFix: 4},
			expectedRet:        &movementsensor.Accuracy{NmeaFix: 3},
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
		ms, err := newFailoverMovementSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.AccuracyFunc = func(ctx context.Context, extra map[string]any) (*movementsensor.Accuracy, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryRet, tc.primaryErr
		}

		sensors.backup1.AccuracyFunc = func(ctx context.Context, extra map[string]any) (*movementsensor.Accuracy, error) {
			return tc.backup1Ret, tc.backup1Err
		}

		sensors.backup2.AccuracyFunc = func(ctx context.Context, extra map[string]any) (*movementsensor.Accuracy, error) {
			return tc.backup2Ret, tc.backup2Err
		}

		val, err := ms.Accuracy(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get accuracy")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, val, test.ShouldResemble, tc.expectedRet)
		}

		err = ms.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
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

		primaryRet         map[string]any
		backup1Ret         map[string]any
		backup2Ret         map[string]any
		expectedRet        map[string]any
		primaryErr         error
		backup1Err         error
		backup2Err         error
		expectErr          bool
		primaryTimeSeconds int
	}{
		{
			name:        "if the primary succeeds, should return primary reading",
			primaryRet:  map[string]any{"a": 1},
			backup1Ret:  map[string]any{"b": 2},
			backup2Ret:  map[string]any{"c": 3},
			expectedRet: map[string]any{"a": 1},
			expectErr:   false,
		},
		{
			name:        "if the primary fails, backup1 is returned",
			primaryRet:  map[string]any{"a": 1},
			backup1Ret:  map[string]any{"b": 2},
			backup2Ret:  map[string]any{"c": 3},
			expectedRet: map[string]any{"b": 2},
			expectErr:   false,
			primaryErr:  errTest,
		},
		{
			name:        "if primary and backup1 fail, backup2 is returned",
			primaryErr:  errTest,
			backup1Err:  errTest,
			primaryRet:  map[string]any{"a": 1},
			backup1Ret:  map[string]any{"b": 2},
			backup2Ret:  map[string]any{"c": 3},
			expectedRet: map[string]any{"c": 3},
			expectErr:   false,
		},
		{
			name:               "a reading should timeout after default of 1 second",
			primaryTimeSeconds: 1,
			primaryRet:         map[string]any{"a": 1},
			backup1Ret:         map[string]any{"b": 2},
			backup2Ret:         map[string]any{"c": 3},
			expectedRet:        map[string]any{"b": 2},
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
		ms, err := newFailoverMovementSensor(ctx, deps, config, logger)
		test.That(t, err, test.ShouldBeNil)

		sensors.primary.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
			time.Sleep(time.Duration(tc.primaryTimeSeconds) * time.Second)
			return tc.primaryRet, tc.primaryErr
		}

		sensors.backup1.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
			return tc.backup1Ret, tc.backup1Err
		}

		sensors.backup2.ReadingsFunc = func(ctx context.Context, extra map[string]any) (map[string]any, error) {
			return tc.backup2Ret, tc.backup2Err
		}

		val, err := ms.Readings(ctx, nil)

		if tc.expectErr {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "all movement sensors failed to get readings")
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, val, test.ShouldResemble, tc.expectedRet)
		}

		err = ms.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(1 * time.Second)
		goRoutinesEnd := runtime.NumGoroutine()
		test.That(t, goRoutinesStart, test.ShouldEqual, goRoutinesEnd)
	}
}
