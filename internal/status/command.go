package status

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"text/template"
	"text/template/parse"
	"time"
)

const (
	defaultEvery   = 2 * time.Second
	defaultTimeout = time.Second
	templateValues = "{{.Name}}, {{.Env.X}}, {{.StdoutPath}}, {{.StderrPath}} and {{.PlistPath}}, and in a with only after $, such as {{$.Name}}"
)

var valuesWithoutKey = []string{"Name", "StdoutPath", "StderrPath", "PlistPath"}

type Command struct {
	templates []*template.Template
	Every     time.Duration
	Timeout   time.Duration
}

func NewCommand(texts []string, every, timeout time.Duration) (Command, error) {
	switch {
	case len(texts) == 0:
		return Command{}, errors.New("the command is empty, and it must be a list with the program first, such as [cat, '{{.Env.WORKER_STATUS_FILE}}']")
	case texts[0] == "":
		return Command{}, errors.New("the program of the command is empty, and it must be the first text of the command, such as cat")
	case every < 0 || timeout < 0:
		return Command{}, fmt.Errorf("the every %s or the timeout %s of the command is less than 0, and each must be more than 0, or 0 for its default", every, timeout)
	}
	command := Command{Every: cmp.Or(every, defaultEvery), Timeout: cmp.Or(timeout, defaultTimeout)}
	for _, text := range texts {
		parsed, err := template.New("text").Parse(text)
		if err != nil {
			return Command{}, fmt.Errorf("the text %q of the command is not a Go template: %w", text, err)
		}
		if len(parsed.Templates()) > 1 {
			return Command{}, fmt.Errorf("the text %q of the command defines a template, and a text of a status command cannot define one", text)
		}
		if value := otherValue(parsed.Root, true); value != "" {
			return Command{}, fmt.Errorf("the text %q of the command uses %s, and a text can use only the template values %s", text, value, templateValues)
		}
		command.templates = append(command.templates, parsed)
	}
	return command, nil
}

func otherValue(node parse.Node, dotIsService bool) string {
	switch node := node.(type) {
	case *parse.ListNode:
		if node != nil {
			return firstOtherValue(node.Nodes, dotIsService)
		}
	case *parse.ActionNode:
		return otherValue(node.Pipe, dotIsService)
	case *parse.PipeNode:
		for _, variable := range node.Decl {
			if variable.Ident[0] == "$" {
				return "{{" + node.String() + "}}, which gives $ a value other than the service"
			}
		}
		for _, command := range node.Cmds {
			if value := firstOtherValue(command.Args, dotIsService); value != "" {
				return value
			}
		}
	case *parse.IfNode:
		return otherValueInBranch(node.BranchNode, dotIsService, dotIsService)
	case *parse.RangeNode:
		return "{{range}}, which cannot go over the text of a template value"
	case *parse.WithNode:
		return otherValueInBranch(node.BranchNode, dotIsService, false)
	case *parse.FieldNode:
		if dotIsService {
			return otherFields(node.Ident, node)
		}
		return "{{" + node.String() + "}} in a with"
	case *parse.VariableNode:
		if node.Ident[0] == "$" {
			return otherFields(node.Ident[1:], node)
		}
		if len(node.Ident) > 1 {
			return "{{" + node.String() + "}}, a field of a variable that holds text"
		}
	case *parse.DotNode:
		if dotIsService {
			return "{{.}}"
		}
	case *parse.ChainNode:
		return "{{" + node.String() + "}}"
	case *parse.TemplateNode:
		return node.String()
	}
	return ""
}

func firstOtherValue(nodes []parse.Node, dotIsService bool) string {
	for _, node := range nodes {
		if value := otherValue(node, dotIsService); value != "" {
			return value
		}
	}
	return ""
}

func otherValueInBranch(branch parse.BranchNode, dotIsService, bodyDotIsService bool) string {
	return cmp.Or(otherValue(branch.Pipe, dotIsService), otherValue(branch.List, bodyDotIsService), otherValue(branch.ElseList, dotIsService))
}

func otherFields(fields []string, node parse.Node) string {
	if len(fields) == 1 && slices.Contains(valuesWithoutKey, fields[0]) || len(fields) == 2 && fields[0] == "Env" {
		return ""
	}
	return "{{" + node.String() + "}}"
}
