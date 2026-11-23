package cli

import (
	"context"
	"sync"
	"time"

	"github.com/hpcsc/svcview/internal/service"
	"github.com/hpcsc/svcview/internal/status"
)

type statusRuns struct {
	ctx      context.Context
	cancel   context.CancelFunc
	lock     sync.Mutex
	notEnded sync.WaitGroup
}

func newStatusRuns(ctx context.Context) *statusRuns {
	ctx, cancel := context.WithCancel(ctx)
	return &statusRuns{ctx: ctx, cancel: cancel}
}

func (r *statusRuns) run(command status.Command, svc service.Service) status.Result {
	if !r.start() {
		return status.Result{Ended: time.Now()}
	}
	defer r.notEnded.Done()
	return command.Run(r.ctx, svc)
}

func (r *statusRuns) start() bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.ctx.Err() != nil {
		return false
	}
	r.notEnded.Add(1)
	return true
}

func (r *statusRuns) stop() {
	r.lock.Lock()
	r.cancel()
	r.lock.Unlock()
	// status.Command.Run stops the process group of a run in a goroutine of its own, so stop must wait for each run to end.
	r.notEnded.Wait()
}
