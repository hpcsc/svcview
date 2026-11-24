package status

import (
	"errors"
	"maps"
	"slices"
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
		return "", &Failure{Kind: FailureKindTemplateValue, Text: "the plist has no StandardOutPath"}
	}
	return v.stdoutPath, nil
}

func (v values) StderrPath() (string, error) {
	if v.stderrPath == "" {
		return "", &Failure{Kind: FailureKindTemplateValue, Text: "the plist has no StandardErrorPath"}
	}
	return v.stderrPath, nil
}

func (v values) PlistPath() (string, error) {
	if v.plistPath == "" {
		return "", &Failure{Kind: FailureKindTemplateValue, Text: "the service has no plist"}
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
		return "", &Failure{Kind: FailureKindTemplateValue, Text: "the plist has no " + name}
	}
	return "", &Failure{Kind: FailureKindTemplateValue, Text: err.Error()}
}

func (v values) list() []TemplateValue {
	list := []TemplateValue{{Template: "{{.Name}}", Value: v.Name}}
	for _, name := range slices.Sorted(maps.Keys(v.Env)) {
		list = append(list, TemplateValue{Template: "{{.Env." + name + "}}", Value: v.Env[name]})
	}
	for _, path := range []struct {
		template string
		value    func() (string, error)
	}{
		{"{{.StdoutPath}}", v.StdoutPath},
		{"{{.StderrPath}}", v.StderrPath},
		{"{{.PlistPath}}", v.PlistPath},
	} {
		value, err := path.value()
		failure, _ := errors.AsType[*Failure](err)
		list = append(list, TemplateValue{Template: path.template, Value: value, Failure: failure})
	}
	return list
}

type TemplateValue struct {
	Template string
	Value    string
	Failure  *Failure
}

func TemplateValues(svc service.Service) []TemplateValue {
	return newValues(svc).list()
}
