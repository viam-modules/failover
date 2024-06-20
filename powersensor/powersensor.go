package powersensor

import (
	"errors"

	"go.viam.com/rdk/components/powersensor"

	"context"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var (
	Model            = resource.NewModel("viam", "failover", " powersensor")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.RegisterComponent(powersensor.API, Model,
		resource.Registration[powersensor.PowerSensor, *Config]{
			Constructor: newfailoverpowersensor,
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

type failoverpowersensor struct {
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

func newfailoverpowersensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (powersensor.PowerSensor, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &failoverpowersensor{
		name:       rawConf.ResourceName(),
		logger:     logger,
		cfg:        conf,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	return s, nil
}

func (s *failoverpowersensor) Name() resource.Name {
	return s.name
}

func (s *failoverpowersensor) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// Put reconfigure code here
	return nil
}

func (s *failoverpowersensor) Voltage(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	return 0, false, nil
}

// Current returns the current reading in amperes and a bool returning true if the current is AC.
func (s *failoverpowersensor) Current(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	return 0, false, nil
}

// Power returns the power reading in watts.
func (s *failoverpowersensor) Power(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, nil
}

func (s *failoverpowersensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (s *failoverpowersensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	s.logger.Error("DoCommand method unimplemented")
	return nil, errUnimplemented
}

func (s *failoverpowersensor) Close(context.Context) error {
	return nil
}
