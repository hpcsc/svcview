package launchd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/hpcsc/svcview/internal/service"
)

const (
	launchctlCommandTimeout   = 30 * time.Second
	launchctlCommandWaitDelay = time.Second
)

type launchctlCommand struct {
	args []string
}

func (c launchctlCommand) run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s did not run: %w", c.text(), err)
	}
	runCtx, cancel := context.WithTimeoutCause(ctx, launchctlCommandTimeout, fmt.Errorf("svcview killed it after %s", launchctlCommandTimeout))
	defer cancel()
	program := exec.Command("launchctl", c.args...)
	program.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	program.WaitDelay = launchctlCommandWaitDelay
	var stdout, stderr bytes.Buffer
	program.Stdout, program.Stderr = &stdout, &stderr
	killed, err := c.runProgram(runCtx, program)
	switch exited, ok := errors.AsType[*exec.ExitError](err); {
	case killed:
		return fmt.Errorf("%s did not end: %w", c.text(), context.Cause(runCtx))
	case ok:
		return c.failure(exited.ProcessState, stderr.String(), stdout.String())
	case errors.Is(err, exec.ErrWaitDelay):
		return nil
	case err != nil:
		return fmt.Errorf("run %s: %w", c.text(), err)
	}
	return nil
}

func (c launchctlCommand) runProgram(ctx context.Context, program *exec.Cmd) (killed bool, err error) {
	if err := program.Start(); err != nil {
		return false, err
	}
	groupKilled := make(chan struct{})
	cancelGroupKill := context.AfterFunc(ctx, func() {
		c.killProcessGroup(program.Process.Pid)
		close(groupKilled)
	})
	err = program.Wait()
	if !cancelGroupKill() {
		<-groupKilled
		killed = true
	}
	c.killProcessGroup(program.Process.Pid)
	return killed, err
}

func (c launchctlCommand) killProcessGroup(pid int) {
	// a signal to the PID 0 goes to the group of svcview, and to the PID -1 to each process of the user
	if pid > 1 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}

func (c launchctlCommand) failure(state *os.ProcessState, stderr, stdout string) error {
	if printed := strings.TrimSpace(strings.TrimSpace(stderr) + "\n" + strings.TrimSpace(stdout)); printed != "" {
		return errors.New(printed)
	}
	if wait, _ := state.Sys().(syscall.WaitStatus); wait.Signaled() {
		return fmt.Errorf("%s ended by %s", c.text(), service.SignalName(int(wait.Signal())))
	}
	return fmt.Errorf("%s exited with code %d", c.text(), state.ExitCode())
}

func (c launchctlCommand) text() string {
	return "launchctl " + strings.Join(c.args, " ")
}
