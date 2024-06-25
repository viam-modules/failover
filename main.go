// package main
package main

import (
	"context"

	failmovementsensor "failover/movementsensor"
	failpowersensor "failover/powersensor"
	failsensor "failover/sensor"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("failover"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	failover, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	if err = failover.AddModelFromRegistry(ctx, sensor.API, failsensor.Model); err != nil {
		return err
	}

	if err = failover.AddModelFromRegistry(ctx, movementsensor.API, failmovementsensor.Model); err != nil {
		return err
	}

	if err = failover.AddModelFromRegistry(ctx, powersensor.API, failpowersensor.Model); err != nil {
		return err
	}

	err = failover.Start(ctx)
	defer failover.Close(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}
