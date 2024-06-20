package movementsensor

import (
	"go.viam.com/rdk/components/movementsensor"

	"context"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var Model = resource.NewModel("viam", "failover", " movementsensor")

func init() {
	resource.RegisterComponent(movementsensor.API, Model,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newfailovermovementsensor,
		},
	)
}

type Config struct {
	// Put config attributes here

	/* if your model  does not need a config,
	   replace *Config on line 13 with resource.NoNativeConfig */

	/* Uncomment this if your model does not need to be validated
	   and has no implicit dependecies. */
	// resource.TriviallyValidateConfig

}

func (cfg *Config) Validate(path string) ([]string, error) {
	// Add config validation code here
	return nil, nil
}

type failovermovementsensor struct {
	name resource.Name

	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()

	/* Uncomment this if your model does not need to reconfigure. */
	// resource.TriviallyReconfigurable

	// Uncomment this if the model does not have any goroutines that
	// need to be shut down while closing.
	// resource.TriviallyCloseable

}

func newfailovermovementsensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (movementsensor.MovementSensor, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &failovermovementsensor{
		name:       rawConf.ResourceName(),
		logger:     logger,
		cfg:        conf,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	return s, nil
}

func (s *failovermovementsensor) Name() resource.Name {
	return s.name
}

func (s *failovermovementsensor) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// Put reconfigure code here
	return nil
}

func (s *failovermovementsensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return nil, 0, nil
}

func (s *failovermovementsensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil

}

func (s *failovermovementsensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, nil
}

func (s *failovermovementsensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (s *failovermovementsensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, nil
}

func (s *failovermovementsensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return nil, nil
}

func (s *failovermovementsensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (s *failovermovementsensor) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error,
) {
	return nil, nil
}

func (s *failovermovementsensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return nil, nil
}

func (s *failovermovementsensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (s *failovermovementsensor) Close(context.Context) error {
	return nil
}
