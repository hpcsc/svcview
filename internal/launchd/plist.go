package launchd

import (
	"fmt"
	"maps"
	"math"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/hpcsc/svcview/internal/service"
	plistformat "howett.net/plist"
)

type plist struct {
	modTime    time.Time
	path       string
	name       string
	env        map[string]string
	stdoutPath string
	stderrPath string
	properties []service.Property
}

func readPlist(path string, modTime time.Time) plist {
	data, err := os.ReadFile(path)
	if err != nil {
		return plist{}
	}
	var decoded struct {
		Label                 string `plist:"Label"`
		Program               any    `plist:"Program"`
		ProgramArguments      any    `plist:"ProgramArguments"`
		WorkingDirectory      any    `plist:"WorkingDirectory"`
		RunAtLoad             any    `plist:"RunAtLoad"`
		KeepAlive             any    `plist:"KeepAlive"`
		StartInterval         any    `plist:"StartInterval"`
		StartCalendarInterval any    `plist:"StartCalendarInterval"`
		WatchPaths            any    `plist:"WatchPaths"`
		ThrottleInterval      any    `plist:"ThrottleInterval"`
		EnvironmentVariables  any    `plist:"EnvironmentVariables"`
		StandardOutPath       any    `plist:"StandardOutPath"`
		StandardErrorPath     any    `plist:"StandardErrorPath"`
	}
	if _, err := plistformat.Unmarshal(data, &decoded); err != nil {
		return plist{modTime: modTime}
	}
	p := plist{modTime: modTime, path: path, name: decoded.Label}
	p.stdoutPath, _ = decoded.StandardOutPath.(string)
	p.stderrPath, _ = decoded.StandardErrorPath.(string)
	if variables, ok := decoded.EnvironmentVariables.(map[string]any); ok {
		p.env = make(map[string]string, len(variables))
		for name, value := range variables {
			if text, ok := value.(string); ok {
				p.env[name] = text
			}
		}
	}
	p.properties = slices.Concat(
		p.programProperty(decoded.ProgramArguments, decoded.Program),
		p.textProperty("work folder", decoded.WorkingDirectory),
		p.booleanProperty("run at load", decoded.RunAtLoad),
		p.keepAliveProperty(decoded.KeepAlive),
		p.secondsProperty("start every", decoded.StartInterval),
		p.startTimesProperty(decoded.StartCalendarInterval),
		p.textsProperty("watch paths", decoded.WatchPaths),
		p.secondsProperty("throttle", decoded.ThrottleInterval),
		p.textProperty("stdout", decoded.StandardOutPath),
		p.textProperty("stderr", decoded.StandardErrorPath),
		p.environmentProperty(),
	)
	return p
}

func (p plist) readIfChanged(path string, modTime time.Time) plist {
	if p.modTime.Equal(modTime) {
		return p
	}
	return readPlist(path, modTime)
}

func (p plist) programProperty(arguments, program any) []service.Property {
	if texts, ok := p.texts(arguments); ok {
		return []service.Property{{Name: "program", Parts: texts}}
	}
	return p.textProperty("program", program)
}

func (p plist) textProperty(name string, value any) []service.Property {
	text, ok := value.(string)
	if !ok {
		return nil
	}
	return []service.Property{{Name: name, Parts: []string{text}}}
}

func (p plist) textsProperty(name string, value any) []service.Property {
	texts, ok := p.texts(value)
	if !ok {
		return nil
	}
	return []service.Property{{Name: name, Parts: texts}}
}

func (p plist) booleanProperty(name string, value any) []service.Property {
	held, ok := value.(bool)
	if !ok {
		return nil
	}
	return []service.Property{{Name: name, Parts: []string{p.yesOrNo(held)}}}
}

func (p plist) keepAliveProperty(value any) []service.Property {
	if conditions, ok := value.(map[string]any); ok {
		return []service.Property{{Name: "keep alive", Parts: p.conditions(conditions)}}
	}
	return p.booleanProperty("keep alive", value)
}

func (p plist) conditions(keepAlive map[string]any) []string {
	texts := slices.Concat(
		p.condition(keepAlive["SuccessfulExit"], "when the last exit is 0", "when the last exit is not 0"),
		p.condition(keepAlive["Crashed"], "when the last exit is a crash", "when the last exit is not a crash"),
		p.conditionOfEachName(keepAlive["PathState"], " exists", " does not exist"),
		p.conditionOfEachName(keepAlive["OtherJobEnabled"], " is loaded or running", " is not loaded"),
	)
	for _, name := range slices.Sorted(maps.Keys(keepAlive)) {
		if slices.Contains([]string{"SuccessfulExit", "Crashed", "PathState", "OtherJobEnabled"}, name) {
			continue
		}
		if held, ok := keepAlive[name].(bool); ok {
			texts = append(texts, name+" "+p.yesOrNo(held))
		} else {
			texts = append(texts, name)
		}
	}
	return texts
}

func (p plist) condition(value any, whenTrue, whenFalse string) []string {
	held, ok := value.(bool)
	switch {
	case !ok:
		return nil
	case held:
		return []string{whenTrue}
	}
	return []string{whenFalse}
}

func (p plist) conditionOfEachName(value any, whenTrue, whenFalse string) []string {
	byName, _ := value.(map[string]any)
	var texts []string
	for _, name := range slices.Sorted(maps.Keys(byName)) {
		texts = append(texts, p.condition(byName[name], "while "+name+whenTrue, "while "+name+whenFalse)...)
	}
	return texts
}

func (p plist) yesOrNo(held bool) string {
	if held {
		return "yes"
	}
	return "no"
}

func (p plist) secondsProperty(name string, value any) []service.Property {
	seconds, ok := p.wholeNumber(value)
	if !ok {
		return nil
	}
	return []service.Property{{Name: name, Parts: []string{strconv.FormatInt(seconds, 10) + "s"}}}
}

func (p plist) startTimesProperty(value any) []service.Property {
	var entries []any
	switch startTimes := value.(type) {
	case map[string]any:
		entries = []any{startTimes}
	case []any:
		entries = startTimes
	default:
		return nil
	}
	var texts []string
	for _, entry := range entries {
		if startTime, ok := entry.(map[string]any); ok {
			texts = append(texts, p.startTime(startTime))
		}
	}
	return []service.Property{{Name: "start at", Parts: texts}}
}

func (p plist) startTime(entry map[string]any) string {
	text := p.twoDigits(entry["Hour"]) + ":" + p.twoDigits(entry["Minute"])
	weekday, hasWeekday := p.wholeNumber(entry["Weekday"])
	day, hasDay := p.wholeNumber(entry["Day"])
	switch {
	case hasWeekday && hasDay:
		text += " on " + p.weekdayName(weekday) + " or on day " + strconv.FormatInt(day, 10)
	case hasWeekday:
		text += " on " + p.weekdayName(weekday)
	case hasDay:
		text += " on day " + strconv.FormatInt(day, 10)
	}
	if month, ok := p.wholeNumber(entry["Month"]); ok {
		text += " in " + p.monthName(month)
	}
	return text
}

func (p plist) twoDigits(value any) string {
	number, ok := p.wholeNumber(value)
	if !ok {
		return "*"
	}
	return fmt.Sprintf("%02d", number)
}

func (p plist) weekdayName(weekday int64) string {
	if weekday < 0 || weekday > 7 {
		return "weekday " + strconv.FormatInt(weekday, 10)
	}
	return time.Weekday(weekday % 7).String()
}

func (p plist) monthName(month int64) string {
	if month < 1 || month > 12 {
		return "month " + strconv.FormatInt(month, 10)
	}
	return time.Month(month).String()
}

func (p plist) wholeNumber(value any) (int64, bool) {
	switch number := value.(type) {
	case int64:
		return number, true
	case uint64:
		if number > math.MaxInt64 {
			return 0, false
		}
		return int64(number), true
	}
	return 0, false
}

func (p plist) environmentProperty() []service.Property {
	if p.env == nil {
		return nil
	}
	return []service.Property{{Name: "environment", Parts: slices.Sorted(maps.Keys(p.env))}}
}

func (p plist) texts(value any) ([]string, bool) {
	array, ok := value.([]any)
	if !ok {
		return nil, false
	}
	var texts []string
	for _, element := range array {
		if text, ok := element.(string); ok {
			texts = append(texts, text)
		}
	}
	return texts, true
}
