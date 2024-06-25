package movementsensor

import (
	"context"
	"failover/common"
	"fmt"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/utils"
)

var Model = resource.NewModel("viam", "failover", "movementsensor")

func init() {
	resource.RegisterComponent(movementsensor.API, Model,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newfailovermovementsensor,
		},
	)
}

type Config struct {
	Primary string   `json:"primary"`
	Backups []string `json:"backups"`
	Timeout int      `json:"timeout_ms,omitempty"`
}

func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.Primary == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "primary")
	}
	deps = append(deps, cfg.Primary)

	if len(cfg.Backups) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "backups")
	}

	deps = append(deps, cfg.Backups...)

	return deps, nil
}

type failovermovementsensor struct {
	name resource.Name

	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()
	resource.TriviallyReconfigurable

	lastWorkingSensor movementsensor.MovementSensor
	primary           movementsensor.MovementSensor
	backups           []movementsensor.MovementSensor

	timeout int
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

	primary, err := movementsensor.FromDependencies(deps, conf.Primary)
	if err != nil {
		return nil, err
	}
	s.primary = primary
	s.backups = []movementsensor.MovementSensor{}

	for _, backup := range conf.Backups {
		backup, err := movementsensor.FromDependencies(deps, backup)
		if err != nil {
			s.logger.Errorf(err.Error())
		} else {
			s.backups = append(s.backups, backup)
		}
	}

	s.lastWorkingSensor = primary

	// default timeout is 1 second.
	s.timeout = 1000
	if conf.Timeout > 0 {
		s.timeout = conf.Timeout
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
	// Poll the last sensor we know is working

	reading, err := common.TryReadingOrFail[r3.Vector](ctx, s.timeout, s.lastWorkingSensor, s.lastWorkingSensor.LinearVelocity, extra)
	if err == nil {
		return reading, nil
	}
	// upon error of the last working sensor, log returned error.
	s.logger.Warnf(err.Error())

	// If the primary failed, start goroutine to check for it to get readings again.

	// Start reading from the list of backup sensors until one succeeds.
	for _, backup := range s.backups {
		// if the last working sensor is a backup, it was already tried above.
		if s.lastWorkingSensor == backup {
			continue
		}
		s.logger.Infof("calling backup %s", backup.Name())
		// dont pass backup, pass name or name as string
		// pass in API call as string
		reading, err := common.TryReadingOrFail(ctx, s.timeout, backup, backup.LinearVelocity, extra)
		if err != nil {
			s.logger.Warn(err.Error())
		} else {
			s.logger.Infof("successfully got reading from %s", backup.Name())
			s.lastWorkingSensor = backup
			return reading, nil
		}
	}
	// couldn't get reading from any sensors.
	return r3.Vector{}, fmt.Errorf("all sensors failed to get readings")
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
	l, _ := s.LinearVelocity(ctx, extra)
	return map[string]interface{}{"linear": l}, nil
}

func (s *failovermovementsensor) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error,
) {
	return nil, nil
}

func (s *failovermovementsensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		PositionSupported:       true,
		OrientationSupported:    true,
		CompassHeadingSupported: true,
		LinearVelocitySupported: true,
	}, nil
}

func (s *failovermovementsensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (s *failovermovementsensor) Close(context.Context) error {
	return nil
}
