package service

import (
	"cmp"
	"time"
)

type State int

const (
	NotLoaded State = iota
	Loaded
	Running
)

func (s State) String() string {
	switch s {
	case Running:
		return "running"
	case Loaded:
		return "loaded"
	default:
		return "not loaded"
	}
}

type LastExit struct {
	Code   int
	Signal string
}

func (e LastExit) Failed() bool {
	return e.Code != 0 || e.Signal != ""
}

type Property struct {
	Name  string
	Parts []string
}

type Service struct {
	Name       string
	State      State
	LastExit   LastExit
	PID        int
	Uptime     *time.Duration
	Env        map[string]string
	StdoutPath string
	StderrPath string
	PlistPath  string
	Properties []Property
}

func (s Service) LogPath() string {
	return cmp.Or(s.StdoutPath, s.StderrPath)
}
