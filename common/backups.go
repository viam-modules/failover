package common

import (
	"context"
	"fmt"
	"sync"

	"go.viam.com/rdk/resource"
)

type Backups struct {
	backupList        []resource.Sensor
	mu                sync.Mutex
	lastWorkingSensor resource.Sensor

	timeout int
	calls   []func(context.Context, resource.Sensor, map[string]any) (any, error)
}

func CreateBackup(timeout int, backupList []resource.Sensor, calls []func(context.Context, resource.Sensor, map[string]any) (any, error)) *Backups {
	backups := &Backups{
		backupList:        backupList,
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
	for _, backup := range b.backupList {
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
	return nil, fmt.Errorf("all %d backup sensors failed", len(b.backupList))

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
