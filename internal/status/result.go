package status

import "time"

type Result struct {
	Ended   time.Time
	Output  *Output
	Failure *Failure
}

func (r Result) Time() time.Time {
	if r.Output != nil && r.Output.At != nil {
		return *r.Output.At
	}
	return r.Ended
}

type FailureKind int

const (
	FailureKindTemplateValue FailureKind = iota
	FailureKindError
	FailureKindTimeout
)

type Failure struct {
	Kind  FailureKind
	Text  string
	Lines []string
}

func (f *Failure) Error() string {
	return f.Text
}
