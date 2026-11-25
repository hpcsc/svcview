package config

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/hpcsc/svcview/internal/status"
	"go.yaml.in/yaml/v3"
)

type Action struct {
	Key     string
	Name    string
	Command status.CommandTemplate
	Confirm bool
}

func readAction(entry *yaml.Node, label string) (Action, error) {
	if entry.Kind != yaml.MappingNode {
		return Action{}, fmt.Errorf("line %d: %s is not a set of settings with a key, a name and a command", entry.Line, label)
	}
	if err := checkSettings(entry, label, "key", "name", "command", "confirm"); err != nil {
		return Action{}, err
	}
	valueOf := valuesOf(entry)
	var action Action
	key, ok := valueOf["key"]
	if !ok {
		return Action{}, fmt.Errorf("line %d: %s has no key, and it must have one, such as key: L", entry.Line, label)
	}
	if action.Key = text(key); !isActionKey(action.Key) {
		return Action{}, fmt.Errorf("line %d: the setting \"key\" of %s must be one character that prints and is not a space, such as L", key.Line, label)
	}
	name, ok := valueOf["name"]
	if !ok {
		return Action{}, fmt.Errorf("line %d: %s has no name, and it must have one, such as name: worker log", entry.Line, label)
	}
	if action.Name = text(name); action.Name == "" {
		return Action{}, fmt.Errorf("line %d: the setting \"name\" of %s must be text that is not empty, such as worker log", name.Line, label)
	}
	command, ok := valueOf["command"]
	if !ok {
		return Action{}, fmt.Errorf("line %d: %s has no command, and it must have one, such as command: [less, +F, '{{.Env.WORKER_LOG}}']", entry.Line, label)
	}
	texts, ok := textsOf(command)
	if !ok {
		return Action{}, fmt.Errorf("line %d: the setting \"command\" of %s must be a list of texts with the program first, such as [less, +F, '{{.Env.WORKER_LOG}}']", command.Line, label)
	}
	commandTemplate, err := status.NewCommandTemplate(texts)
	if err != nil {
		return Action{}, fmt.Errorf("line %d: %s: %w", command.Line, label, err)
	}
	action.Command = commandTemplate
	if confirm, ok := valueOf["confirm"]; ok {
		if confirm.Tag != "!!bool" || confirm.Decode(&action.Confirm) != nil {
			return Action{}, fmt.Errorf("line %d: the setting \"confirm\" of %s must be true or false", confirm.Line, label)
		}
	}
	return action, nil
}

func isActionKey(key string) bool {
	r, size := utf8.DecodeRuneInString(key)
	return size > 0 && size == len(key) && r != ' ' && unicode.IsPrint(r)
}
