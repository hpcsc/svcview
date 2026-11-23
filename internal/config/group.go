package config

import (
	"errors"
	"fmt"
	"regexp"

	"go.yaml.in/yaml/v3"
)

type Group struct {
	Name  string
	match *regexp.Regexp
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
	valueOf := map[string]*yaml.Node{}
	for i := 0; i < len(entry.Content); i += 2 {
		valueOf[entry.Content[i].Value] = entry.Content[i+1]
	}
	name := text(valueOf["name"])
	label := fmt.Sprintf("group %d", place)
	if name != "" {
		label = fmt.Sprintf("group %q", name)
	}
	lineOf := map[string]int{}
	for i := 0; i < len(entry.Content); i += 2 {
		setting := entry.Content[i]
		if line, ok := lineOf[setting.Value]; ok {
			return Group{}, fmt.Errorf("line %d: the setting %q of %s is already on line %d", setting.Line, setting.Value, label, line)
		}
		lineOf[setting.Value] = setting.Line
		if setting.Value != "name" && setting.Value != "match" {
			return Group{}, fmt.Errorf("line %d: svcview does not know the setting %q of %s", setting.Line, setting.Value, label)
		}
	}
	if name == "" {
		return Group{}, fmt.Errorf("line %d: %s has no name, and a name must be text, such as Workers", entry.Line, label)
	}
	group, err := NewGroup(name, text(valueOf["match"]))
	if err != nil {
		return Group{}, fmt.Errorf("line %d: %w", entry.Line, err)
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

func text(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag == "!!null" {
		return ""
	}
	return node.Value
}
