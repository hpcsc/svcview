package launchd

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const psTimeout = 5 * time.Second

type uptimes map[int]time.Duration

func readUptimes(pids []int) uptimes {
	if len(pids) == 0 {
		return nil
	}
	list := make([]string, 0, len(pids))
	for _, pid := range pids {
		list = append(list, strconv.Itoa(pid))
	}
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "ps", "-o", "pid=,etime=", "-p", strings.Join(list, ","))
	command.WaitDelay = time.Second
	stdout, err := command.Output()
	if err != nil {
		return nil
	}
	return parseUptimes(string(stdout))
}

func parseUptimes(output string) uptimes {
	u := uptimes{}
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		if uptime, ok := parseUptime(fields[1]); ok {
			u[pid] = uptime
		}
	}
	return u
}

func parseUptime(text string) (time.Duration, bool) {
	days, clock, hasDays := strings.Cut(text, "-")
	if !hasDays {
		days, clock = "0", text
	}
	parts := strings.Split(clock, ":")
	if len(parts) == 2 && !hasDays {
		parts = append([]string{"0"}, parts...)
	}
	if len(parts) != 3 {
		return 0, false
	}
	var total time.Duration
	units := []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second}
	for i, part := range append([]string{days}, parts...) {
		number, err := strconv.ParseUint(part, 10, 0)
		if err != nil {
			return 0, false
		}
		total += time.Duration(number) * units[i]
	}
	return total, true
}
