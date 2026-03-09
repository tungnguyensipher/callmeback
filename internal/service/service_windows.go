//go:build windows

package service

import (
	"context"
	"time"

	"golang.org/x/sys/windows/svc"
)

func RunManagedService(dbPath string, pollInterval time.Duration) error {
	return svc.Run(DefaultWindowsName, &windowsService{
		dbPath:       dbPath,
		pollInterval: pollInterval,
	})
}

type windowsService struct {
	dbPath       string
	pollInterval time.Duration
}

func (w *windowsService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	changes <- svc.Status{State: svc.StartPending}

	go func() {
		errCh <- RunSchedulerLoop(runCtx, w.dbPath, w.pollInterval)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				changes <- svc.Status{State: svc.StopPending}
				return false, 1
			}
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				<-errCh
				return false, 0
			}
		}
	}
}
