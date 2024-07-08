package common

import (
	"context"
	"errors"
	"sync"

	"go.viam.com/rdk/resource"
)

type Backups struct {
	BackupList        []resource.Sensor
	lastWorkingSensor resource.Sensor
	timeout           int
	calls             []func(context.Context, resource.Sensor, map[string]any) (any, error)
	mu                sync.Mutex
}

func CreateBackup(timeout int, backupList []resource.Sensor, calls []func(context.Context, resource.Sensor, map[string]any) (any, error)) *Backups {
	backups := &Backups{
		BackupList:        backupList,
		timeout:           timeout,
		lastWorkingSensor: backupList[0],
		calls:             calls,
	}
	return backups

}

func (b *Backups) GetWorkingSensor(ctx context.Context, extra map[string]interface{}) (resource.Sensor, error) {
	// First call all APIs for the lastworkingsensor, if it succeeeds then return.

	lastWorking := b.getLastWorkingSensor()
	err := CallAllFunctions(ctx, lastWorking, b.timeout, extra, b.calls)
	if err == nil {
		return lastWorking, nil
	}

	// If the lastWorkingsensor failed, we need to find a replacement, loop through the backups until one of them succeeds for all API calls.
	for _, backup := range b.BackupList {
		// Already tried it.
		if backup == lastWorking {
			continue
		}
		// Loop through all API calls and record the errors
		err := CallAllFunctions(ctx, backup, b.timeout, extra, b.calls)
		if err != nil {
			continue
		}
		// all calls were successful, replace lastworkingsensor and return.
		b.setLastWorkingSensor(backup)

		return backup, nil
	}
	return nil, errors.New("all sensors failed")

}

func (b *Backups) getLastWorkingSensor() resource.Sensor {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastWorkingSensor
}
func (b *Backups) setLastWorkingSensor(sensor resource.Sensor) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastWorkingSensor = sensor
}
