package status

import (
	"cmp"
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
	defaultEvery   = 2 * time.Second
	defaultTimeout = time.Second
	stopDelay      = time.Second

	plainShellCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@%+=:,./-_"
)

type Command struct {
	template   CommandTemplate
	Every      time.Duration
	Timeout    time.Duration
	StaleAfter time.Duration
}

func NewCommand(texts []string, every, timeout time.Duration) (Command, error) {
	commandTemplate, err := NewCommandTemplate(texts)
	if err != nil {
		return Command{}, err
	}
	if every < 0 || timeout < 0 {
		return Command{}, fmt.Errorf("the every %s or the timeout %s of the command is less than 0, and each must be more than 0, or 0 for its default", every, timeout)
	}
	return Command{template: commandTemplate, Every: cmp.Or(every, defaultEvery), Timeout: cmp.Or(timeout, defaultTimeout)}, nil
}

func (c Command) Run(ctx context.Context, svc service.Service) Result {
	line, failure := c.template.Apply(svc)
	if failure != nil {
		return Result{Ended: time.Now(), Failure: failure}
	}
	runCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	program := exec.Command(line[0], line[1:]...)
	program.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	program.WaitDelay = stopDelay
	var stdout stdoutBuffer
	var stderr lastLines
	program.Stdout, program.Stderr = &stdout, &stderr
	stopped, err := c.runProgram(runCtx, program)
	result := Result{Ended: time.Now(), Stdout: string(stdout.data)}
	if ctx.Err() != nil {
		return result
	}
	switch exited, ok := errors.AsType[*exec.ExitError](err); {
	case stopped:
		result.Failure = &Failure{Kind: FailureKindTimeout, Text: c.commandLineText(line)}
	case stdout.tooLarge:
		result.Failure = &Failure{Kind: FailureKindInvalid, Text: "the output is larger than 1 MiB", Lines: stdout.firstLines()}
	case ok:
		result.Failure = &Failure{Kind: FailureKindError, Text: c.exitText(exited.ProcessState), Lines: stderr.lines()}
	case err != nil:
		result.Failure = &Failure{Kind: FailureKindError, Text: err.Error(), Lines: stderr.lines()}
	default:
		output, invalid := parseOutput(stdout.data)
		if invalid != nil {
			result.Failure = &Failure{Kind: FailureKindInvalid, Text: invalid.Error(), Lines: stdout.firstLines()}
		}
		result.Output = output
	}
	return result
}

func (c Command) CommandLine(svc service.Service) (string, error) {
	line, failure := c.template.Apply(svc)
	if failure != nil {
		return "", failure
	}
	return c.commandLineText(line), nil
}

func (c Command) runProgram(ctx context.Context, program *exec.Cmd) (stopped bool, err error) {
	if ctx.Err() != nil {
		return true, nil
	}
	if err := program.Start(); err != nil {
		return false, err
	}
	groupStopped := make(chan struct{})
	cancelGroupStop := context.AfterFunc(ctx, func() {
		c.stopProcessGroup(program.Process.Pid)
		close(groupStopped)
	})
	err = program.Wait()
	if !cancelGroupStop() {
		<-groupStopped
		stopped = true
	}
	c.stopProcessGroup(program.Process.Pid)
	return stopped, err
}

func (c Command) stopProcessGroup(pid int) {
	// a signal to the PID 0 goes to the group of svcview, and to the PID -1 to each process of the user
	if pid > 1 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}

func (c Command) exitText(state *os.ProcessState) string {
	if wait, _ := state.Sys().(syscall.WaitStatus); wait.Signaled() {
		return "ended by " + service.SignalName(int(wait.Signal()))
	}
	return fmt.Sprintf("exit code %d", state.ExitCode())
}

func (c Command) commandLineText(line []string) string {
	texts := make([]string, 0, len(line))
	for _, text := range line {
		texts = append(texts, c.shellText(text))
	}
	return strings.Join(texts, " ")
}

func (c Command) shellText(text string) string {
	plain := text != "" && !strings.ContainsFunc(text, func(r rune) bool { return !strings.ContainsRune(plainShellCharacters, r) })
	if plain {
		return text
	}
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}
