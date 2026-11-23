package status

import (
	"errors"
	"strconv"
	"strings"
	"text/template"

	"github.com/hpcsc/svcview/internal/service"
)

type values struct {
	Name       string
	Env        map[string]string
	stdoutPath string
	stderrPath string
	plistPath  string
}

func newValues(svc service.Service) values {
	return values{Name: svc.Name, Env: svc.Env, stdoutPath: svc.StdoutPath, stderrPath: svc.StderrPath, plistPath: svc.PlistPath}
}

func (v values) StdoutPath() (string, error) {
	if v.stdoutPath == "" {
		return "", &Failure{Text: "the plist has no StandardOutPath"}
	}
	return v.stdoutPath, nil
}

func (v values) StderrPath() (string, error) {
	if v.stderrPath == "" {
		return "", &Failure{Text: "the plist has no StandardErrorPath"}
	}
	return v.stderrPath, nil
}

func (v values) PlistPath() (string, error) {
	if v.plistPath == "" {
		return "", &Failure{Text: "the service has no plist"}
	}
	return v.plistPath, nil
}

func (v values) apply(parsed *template.Template) (string, *Failure) {
	var applied strings.Builder
	err := parsed.Execute(&applied, v)
	if err == nil {
		return applied.String(), nil
	}
	if failure, ok := errors.AsType[*Failure](err); ok {
		return "", failure
	}
	// text/template names a missing key of a map only in the text of its error.
	if _, key, found := strings.Cut(err.Error(), "map has no entry for key "); found {
		name, _ := strconv.Unquote(key)
		return "", &Failure{Text: "the plist has no " + name}
	}
	return "", &Failure{Text: err.Error()}
}
