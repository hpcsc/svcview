package service

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

type Service struct {
	Name  string
	State State
}
