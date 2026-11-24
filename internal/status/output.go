package status

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	}
	return "info"
}

const (
	rfc3339Shape = `an RFC 3339 time in a text, such as "2026-11-23T09:12:00Z"`
	levelWords   = "info, warn or error"
)

var (
	levels = map[string]Level{"info": LevelInfo, "warn": LevelWarn, "error": LevelError}
	shapes = map[string]string{
		"/version":           "the number 1",
		"/at":                rfc3339Shape,
		"/summary":           "a text",
		"/level":             "the text " + levelWords,
		"/fields":            "a list of objects",
		"/fields/*":          "an object",
		"/fields/*/name":     "a text",
		"/fields/*/value":    "a text",
		"/fields/*/progress": "a number",
		"/events":            "a list of objects",
		"/events/*":          "an object",
		"/events/*/at":       rfc3339Shape,
		"/events/*/level":    "the text " + levelWords,
		"/events/*/text":     "a text",
	}
	jsonKinds = map[jsontext.Kind]string{'n': "null", 't': "boolean", 'f': "boolean", '"': "text", '0': "number", '[': "list"}
)

type Output struct {
	At      *time.Time
	Summary string
	Level   Level
	Fields  []Field
	Events  []Event
}

type Field struct {
	Name     string
	Value    string
	Progress *float64
}

type Event struct {
	At    time.Time
	Level Level
	Text  string
}

func parseOutput(stdout []byte) (*Output, error) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return nil, errors.New("the output is empty")
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(stdout))
	value, err := decoder.ReadValue()
	switch {
	case err != nil:
		return nil, fmt.Errorf("the output is not valid JSON: %s", strings.TrimPrefix(err.Error(), "jsontext: "))
	case value.Kind() != '{':
		return nil, fmt.Errorf("the output is a JSON %s, not an object", jsonKinds[value.Kind()])
	case len(bytes.TrimSpace(stdout[decoder.InputOffset():])) > 0:
		return nil, errors.New("the output has more text after its JSON object")
	}
	var parsed struct {
		Version *int       `json:"version"`
		At      *time.Time `json:"at"`
		Summary *string    `json:"summary"`
		Level   *string    `json:"level"`
		Fields  []struct {
			Name     *string  `json:"name"`
			Value    *string  `json:"value"`
			Progress *float64 `json:"progress"`
		} `json:"fields"`
		Events []struct {
			At    *time.Time `json:"at"`
			Level *string    `json:"level"`
			Text  *string    `json:"text"`
		} `json:"events"`
	}
	if err := json.Unmarshal(value, &parsed); err != nil {
		return nil, shapeError(err)
	}
	switch {
	case parsed.Version == nil:
		return nil, errors.New(`"/version" is missing, and it must be 1`)
	case *parsed.Version != 1:
		return nil, fmt.Errorf(`"/version" is %d, and it must be 1`, *parsed.Version)
	}
	output := Output{At: parsed.At, Level: LevelInfo}
	if parsed.Summary != nil {
		output.Summary = *parsed.Summary
	}
	if parsed.Level != nil {
		if output.Level, err = parseLevel("/level", *parsed.Level); err != nil {
			return nil, err
		}
	}
	for i, field := range parsed.Fields {
		switch {
		case field.Name == nil:
			return nil, fmt.Errorf(`"/fields/%d/name" is missing`, i)
		case field.Value == nil:
			return nil, fmt.Errorf(`"/fields/%d/value" is missing`, i)
		case field.Progress != nil && (*field.Progress < 0 || *field.Progress > 1):
			return nil, fmt.Errorf(`"/fields/%d/progress" is %g, and it must be from 0 to 1`, i, *field.Progress)
		}
		output.Fields = append(output.Fields, Field{Name: *field.Name, Value: *field.Value, Progress: field.Progress})
	}
	for i, event := range parsed.Events {
		switch {
		case event.At == nil:
			return nil, fmt.Errorf(`"/events/%d/at" is missing`, i)
		case event.Level == nil:
			return nil, fmt.Errorf(`"/events/%d/level" is missing`, i)
		case event.Text == nil:
			return nil, fmt.Errorf(`"/events/%d/text" is missing`, i)
		}
		level, err := parseLevel(fmt.Sprintf("/events/%d/level", i), *event.Level)
		if err != nil {
			return nil, err
		}
		output.Events = append(output.Events, Event{At: *event.At, Level: level, Text: *event.Text})
	}
	return &output, nil
}

func parseLevel(place, text string) (Level, error) {
	level, ok := levels[text]
	if !ok {
		return level, fmt.Errorf("%q is %q, and it must be %s", place, text, levelWords)
	}
	return level, nil
}

func shapeError(err error) error {
	semantic, ok := errors.AsType[*json.SemanticError](err)
	if !ok {
		return err
	}
	segments := strings.Split(string(semantic.JSONPointer), "/")
	for i, segment := range segments {
		if _, err := strconv.Atoi(segment); err == nil {
			segments[i] = "*"
		}
	}
	return fmt.Errorf("%q must be %s", semantic.JSONPointer, shapes[strings.Join(segments, "/")])
}
