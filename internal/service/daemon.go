package service

import (
	"context"
	"time"

	callmeruntime "github.com/tungnguyensipher/callmeback/internal/runtime"
	"github.com/tungnguyensipher/callmeback/internal/store"
)

func RunSchedulerLoop(ctx context.Context, dbPath string, pollInterval time.Duration) error {
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	rt, err := callmeruntime.New(st, callmeruntime.Options{})
	if err != nil {
		return err
	}
	defer rt.Close()

	return rt.Run(ctx, pollInterval)
}
