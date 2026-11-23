package status

import (
	"encoding/json/v2"
	"time"
)

type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

var levels = map[string]Level{"info": LevelInfo, "warn": LevelWarn, "error": LevelError}

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

func parseOutput(stdout []byte) *Output {
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
	if err := json.Unmarshal(stdout, &parsed); err != nil || parsed.Version == nil || *parsed.Version != 1 {
		return nil
	}
	output := Output{At: parsed.At, Level: LevelInfo}
	if parsed.Summary != nil {
		output.Summary = *parsed.Summary
	}
	if parsed.Level != nil {
		level, ok := levels[*parsed.Level]
		if !ok {
			return nil
		}
		output.Level = level
	}
	for _, field := range parsed.Fields {
		if field.Name == nil || field.Value == nil || field.Progress != nil && (*field.Progress < 0 || *field.Progress > 1) {
			return nil
		}
		output.Fields = append(output.Fields, Field{Name: *field.Name, Value: *field.Value, Progress: field.Progress})
	}
	for _, event := range parsed.Events {
		if event.At == nil || event.Level == nil || event.Text == nil {
			return nil
		}
		level, ok := levels[*event.Level]
		if !ok {
			return nil
		}
		output.Events = append(output.Events, Event{At: *event.At, Level: level, Text: *event.Text})
	}
	return &output
}
