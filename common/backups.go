package common

import (
	"context"
	"errors"
	"sync"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

type Backups struct {
	mu                sync.Mutex
	logger            logging.Logger
	BackupList        []resource.Sensor
	LastWorkingSensor resource.Sensor
	timeout           int
	calls             []func(context.Context, resource.Sensor, map[string]any) (any, error)
}

func CreateBackup(timeout int, logger logging.Logger, backupList []resource.Sensor, calls []func(context.Context, resource.Sensor, map[string]any) (any, error)) *Backups {
	backups := &Backups{
		BackupList:        backupList,
		timeout:           timeout,
		logger:            logger,
		LastWorkingSensor: backupList[0],
		calls:             calls,
	}
	return backups

}

func (b *Backups) GetWorkingSensor(ctx context.Context, extra map[string]interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// First call all APIs for the lastworkingsensor, if it succeeeds then return.
	err := CallAllFunctions(ctx, b.LastWorkingSensor, b.timeout, extra, b.calls)
	if err == nil {
		return nil
	}

	// If the lastWorkingsensor failed, we need to find a replacement, loop through the backups until one of them succeeds for all API calls.
	for _, backup := range b.BackupList {
		// Already tried it.
		if backup == b.LastWorkingSensor {
			continue
		}
		// Loop through all API calls and record the errors
		err := CallAllFunctions(ctx, backup, b.timeout, extra, b.calls)
		if err != nil {
			continue
		}
		// all calls were successful, replace lastworkingsensor
		b.LastWorkingSensor = backup
		return nil
	}
	return errors.New("all sensors failed")

}
