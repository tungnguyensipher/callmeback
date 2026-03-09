//go:build !windows

package service

import (
	"context"
	"time"
)

func RunManagedService(dbPath string, pollInterval time.Duration) error {
	return RunSchedulerLoop(context.Background(), dbPath, pollInterval)
}
