package config

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/hpcsc/svcview/internal/status"
	"go.yaml.in/yaml/v3"
)

type Group struct {
	Name       string
	match      *regexp.Regexp
	Status     *status.Command
	Actions    []Action
	configFile string
}

var Other = Group{Name: "Other"}

func NewGroup(name, match string) (Group, error) {
	switch {
	case name == "":
		return Group{}, errors.New("a group has no name, and a name must be text, such as Workers")
	case name == Other.Name:
		return Group{}, fmt.Errorf("no group can have the name %q, because it is the name of the group of the services that no group matches", name)
	case match == "":
		return Group{}, fmt.Errorf(`group %q has no match, and a match must be a Go regular expression, such as ^com\.example\.worker\.`, name)
	}
	expression, err := regexp.Compile(match)
	if err != nil {
		return Group{}, fmt.Errorf("the match of group %q is not a Go regular expression: %w", name, err)
	}
	return Group{Name: name, match: expression}, nil
}

func readGroup(entry *yaml.Node, place int) (Group, error) {
	if entry.Kind != yaml.MappingNode {
		return Group{}, fmt.Errorf("line %d: group %d is not a set of settings with a name and a match", entry.Line, place)
	}
	valueOf := valuesOf(entry)
	name := text(valueOf["name"])
	label := fmt.Sprintf("group %d", place)
	if name != "" {
		label = fmt.Sprintf("group %q", name)
	}
	if err := checkSettings(entry, label, "name", "match", "status", "actions"); err != nil {
		return Group{}, err
	}
	if name == "" {
		return Group{}, fmt.Errorf("line %d: %s has no name, and a name must be text, such as Workers", entry.Line, label)
	}
	group, err := NewGroup(name, text(valueOf["match"]))
	if err != nil {
		return Group{}, fmt.Errorf("line %d: %w", entry.Line, err)
	}
	if value, ok := valueOf["status"]; ok {
		group.Status, err = readStatus(value, label)
		if err != nil {
			return Group{}, err
		}
	}
	if value, ok := valueOf["actions"]; ok {
		group.Actions, err = readActions(value, label)
		if err != nil {
			return Group{}, err
		}
	}
	return group, nil
}

func (g Group) matches(serviceName string) bool {
	return g.match != nil && g.match.MatchString(serviceName)
}

func (g Group) ShortName(serviceName string) string {
	if g.match == nil {
		return serviceName
	}
	at := g.match.FindStringIndex(serviceName)
	if at == nil || at[0] != 0 || at[1] == len(serviceName) {
		return serviceName
	}
	return serviceName[at[1]:]
}

func readStatus(node *yaml.Node, groupLabel string) (*status.Command, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: the setting \"status\" of %s must be a set of settings with a command, such as command: [cat, '{{.Env.WORKER_STATUS_FILE}}']", node.Line, groupLabel)
	}
	label := "the status of " + groupLabel
	if err := checkSettings(node, label, "command", "every", "timeout", "stale_after"); err != nil {
		return nil, err
	}
	valueOf := valuesOf(node)
	command, ok := valueOf["command"]
	if !ok {
		return nil, fmt.Errorf("line %d: %s has no command, and it must have one, such as command: [cat, '{{.Env.WORKER_STATUS_FILE}}']", node.Line, label)
	}
	texts, ok := textsOf(command)
	if !ok {
		return nil, fmt.Errorf("line %d: the setting \"command\" of %s must be a list of texts with the program first, such as [cat, '{{.Env.WORKER_STATUS_FILE}}']", command.Line, label)
	}
	durations := map[string]time.Duration{}
	for _, setting := range []string{"every", "timeout", "stale_after"} {
		if value, ok := valueOf[setting]; ok {
			duration, err := readDuration(value, fmt.Sprintf("%q of %s", setting, label))
			if err != nil {
				return nil, err
			}
			durations[setting] = duration
		}
	}
	statusCommand, err := status.NewCommand(texts, durations["every"], durations["timeout"])
	if err != nil {
		return nil, fmt.Errorf("line %d: %s: %w", command.Line, label, err)
	}
	statusCommand.StaleAfter = durations["stale_after"]
	return &statusCommand, nil
}

func readActions(node *yaml.Node, groupLabel string) ([]Action, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: the setting \"actions\" of %s must be a list of actions, each with a key, a name and a command", node.Line, groupLabel)
	}
	var actions []Action
	for i, entry := range node.Content {
		action, err := readAction(entry, fmt.Sprintf("action %d of %s", i+1, groupLabel))
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func checkSettings(node *yaml.Node, label string, known ...string) error {
	lineOf := map[string]int{}
	for i := 0; i < len(node.Content); i += 2 {
		setting := node.Content[i]
		if line, ok := lineOf[setting.Value]; ok {
			return fmt.Errorf("line %d: the setting %q of %s is already on line %d", setting.Line, setting.Value, label, line)
		}
		lineOf[setting.Value] = setting.Line
		if !slices.Contains(known, setting.Value) {
			return fmt.Errorf("line %d: svcview does not know the setting %q of %s", setting.Line, setting.Value, label)
		}
	}
	return nil
}

func valuesOf(node *yaml.Node) map[string]*yaml.Node {
	valueOf := map[string]*yaml.Node{}
	for i := 0; i < len(node.Content); i += 2 {
		valueOf[node.Content[i].Value] = node.Content[i+1]
	}
	return valueOf
}

func text(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag == "!!null" {
		return ""
	}
	return node.Value
}

func textsOf(node *yaml.Node) ([]string, bool) {
	if node.Kind != yaml.SequenceNode {
		return nil, false
	}
	var texts []string
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag == "!!null" {
			return nil, false
		}
		texts = append(texts, item.Value)
	}
	return texts, true
}
