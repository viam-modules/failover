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

	// CallsMap is only used for movement sensors to keep track of what API calls each backup supports.
	callsMap map[resource.Sensor][]Call
}

func CreateBackup(timeout int,
	backupList []resource.Sensor,
	calls []func(context.Context, resource.Sensor,
		map[string]any) (any, error),
) *Backups {
	backups := &Backups{
		backupList:        backupList,
		timeout:           timeout,
		lastWorkingSensor: backupList[0],
		calls:             calls,
	}

	return backups
}

func (b *Backups) GetWorkingSensor(ctx context.Context, extra map[string]interface{}) (resource.Sensor, error) {

	lastWorking := b.getLastWorkingSensor()

	// Get the API calls the last working sensor supports.
	var calls []Call
	if b.callsMap != nil {
		calls = b.callsMap[lastWorking]
	} else {
		calls = b.calls
	}

	// First call all supported APIs for the lastworkingsensor, if it succeeeds then return.
	err := CallAllFunctions(ctx, lastWorking, b.timeout, extra, calls)
	if err == nil {
		return lastWorking, nil
	}

	// If the lastWorkingsensor failed, we need to find a replacement, loop through the backups until one of them succeeds for all API calls.
	for _, backup := range b.backupList {
		// Already tried it.
		if backup == lastWorking {
			continue
		}
		// call all suported APIs, if one errors continue to the next backup.
		err := CallAllFunctions(ctx, backup, b.timeout, extra, calls)
		if err != nil {
			continue
		}
		// all calls were successful, replace lastworkingsensor and return.
		b.setLastWorkingSensor(backup)
		return backup, nil
	}

	switch len(b.backupList) {
	case 1:
		return nil, fmt.Errorf("%d backup sensor failed", len(b.backupList))
	default:
		return nil, fmt.Errorf("all %d backup sensors failed", len(b.backupList))
	}
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

func (b *Backups) SetCallsMap(callsMap map[resource.Sensor][]Call) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.callsMap = callsMap
}
