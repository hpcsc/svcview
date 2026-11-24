package status

import "time"

type Result struct {
	Ended   time.Time
	Stdout  string
	Output  *Output
	Failure *Failure
}

func (r Result) Time() time.Time {
	if r.Output != nil && r.Output.At != nil {
		return *r.Output.At
	}
	return r.Ended
}

func (r Result) Stale(staleAfter time.Duration, now time.Time) bool {
	if staleAfter <= 0 || r.Failure != nil || r.Output == nil || r.Output.At == nil {
		return false
	}
	return now.Sub(*r.Output.At) > staleAfter
}

type FailureKind int

const (
	FailureKindTemplateValue FailureKind = iota
	FailureKindError
	FailureKindTimeout
	FailureKindInvalid
)

func (k FailureKind) String() string {
	switch k {
	case FailureKindError:
		return "status error"
	case FailureKindTimeout:
		return "status timeout"
	case FailureKindInvalid:
		return "status invalid"
	}
	return "template value"
}

type Failure struct {
	Kind  FailureKind
	Text  string
	Lines []string
}

func (f *Failure) Error() string {
	return f.Text
}
