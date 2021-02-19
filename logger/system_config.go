package logger

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var (
	levelName = []string{
		"debug",
		"verbose",
		"info",
		"warn",
		"error",
		"fatal",
		"panic",
	}

	tab        = []byte{byte('\t')}
	comma      = []byte{byte(',')}
	newLine    = []byte{byte('\n')}
	openCurly  = []byte{byte('{')}
	closeCurly = []byte{byte('}')}
)

const (
	// levelOffset is the amount to add to change the lowest log level to zero so it aligns with the
	// levelName list
	levelOffset = 2
)

// SystemConfig defines the configuration the main system or a subsystem with custom settings.
type systemConfig struct {
	minLevel   Level
	stackLevel Level
	isText     bool
	output     Output
	fields     []Field
	format     int

	first bool
}

// Copy makes a separate copy so if the fields are modified in one copy they will not be in another.
func (sc systemConfig) Copy() systemConfig {
	result := sc
	result.fields = make([]Field, len(sc.fields))
	copy(result.fields, sc.fields)
	return result
}

// newSystemConfig creates a new logger system config.
// NOTE: isText doesn't work yet, but is meant to change from JSON to tab delimited.
func newSystemConfig(isDevelopment, isText bool, filePath string) (systemConfig, error) {
	result := systemConfig{
		isText:     isText,
		stackLevel: LevelError,
		minLevel:   LevelInfo,
		format:     IncludeCaller | IncludeLevel,
	}

	if isText {
		result.format |= IncludeDate | IncludeTime | IncludeMicro
	} else {
		result.format |= IncludeTimeStamp
	}

	if isDevelopment {
		result.minLevel = LevelVerbose
	}

	if len(filePath) > 0 {
		if filePath == "dummy" { // for benchmarking
			result.output = &dummyWriter{}
		} else {
			file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				panic(errors.Wrap(err, "open file"))
				return result, errors.Wrap(err, "open file")
			}

			result.output = &fileWriter{file: file}
		}
	} else {
		result.output = &printer{}
	}

	return result, nil
}

// newEmptySystemConfig a new logger system config that doesn't log.
func newEmptySystemConfig() (systemConfig, error) {
	return systemConfig{}, nil
}

// addField adds a field to the log outputs
func (s *systemConfig) addField(newField Field) {
	for i, field := range s.fields {
		if field.Name() == newField.Name() {
			s.fields[i] = newField
			return
		}
	}

	s.fields = append(s.fields, newField)
}

// addSubSystem adds a subsystem to the log outputs
func (s *systemConfig) addSubSystem(name string) {
	for i, field := range s.fields {
		if field.Name() == "subsystem" {
			s.fields[i] = String("subsystem", name)
			return
		}
	}

	s.fields = append(s.fields, String("subsystem", name))
}

// removeSubSystem removes the subsystem from the log outputs
func (s *systemConfig) removeSubSystem() {
	for i, field := range s.fields {
		if field.Name() == "subsystem" {
			s.fields = append(s.fields[:i], s.fields[i+1:]...)
			return
		}
	}
}

func (config *systemConfig) writeField(format string, values ...interface{}) {
	if config.first {
		config.first = false
	} else if config.isText {
		config.output.Write(tab)
	} else {
		config.output.Write(comma)
	}

	fmt.Fprintf(config.output, format, values...)
}

func (config *systemConfig) writeEntry(level Level, depth int, fields []Field, format string,
	values ...interface{}) error {

	if config.isText {
		return config.writeTextEntry(level, depth+1, fields, format, values...)
	}

	return config.writeJSONEntry(level, depth+1, fields, format, values...)
}

func (config *systemConfig) writeJSONEntry(level Level, depth int, fields []Field, format string,
	values ...interface{}) error {

	if config.output == nil {
		return nil
	}

	if config.minLevel > level {
		return nil // Level is below minimum
	}

	config.output.Lock()
	defer config.output.Unlock()

	config.first = true
	config.output.Write(openCurly)

	// Write Level
	if config.format&IncludeLevel != 0 {
		config.writeField("\"level\":\"%s\"", levelName[level+levelOffset])
	}

	// Create log entry
	now := time.Now()

	// Append timestamp
	if config.format&IncludeTimeStamp != 0 {
		config.writeField("\"ts\":%d.%06d", now.Unix(), now.Nanosecond()/1e3)
	}

	// Append Date
	var datetime bytes.Buffer
	if config.format&IncludeDate != 0 {
		year, month, day := now.Date()
		fmt.Fprintf(&datetime, "%04d/%02d/%02d", year, month, day)
		if config.format&IncludeTime != 0 {
			fmt.Fprint(&datetime, []byte(" "))
		}
	}

	// Append Time
	if config.format&IncludeTime != 0 {
		hour, min, sec := now.Clock()
		fmt.Fprintf(&datetime, "%02d:%02d:%02d", hour, min, sec)
		if config.format&IncludeMicro == 0 {
			fmt.Fprintf(&datetime, " %06d", now.Nanosecond()/1e3)
		}
	}

	if datetime.Len() > 0 {
		name := ""
		if config.format&IncludeDate != 0 {
			name = "date"
		}
		if config.format&IncludeTime != 0 {
			name += "time"
		}

		config.writeField("\"%s\":\"%s\"", name, string(datetime.Bytes()))
	}

	// Append Caller
	if config.format&IncludeCaller != 0 {
		_, filepath, line, ok := runtime.Caller(depth+1)
		if ok {
			fileParts := strings.Split(filepath, string(os.PathSeparator))
			l := len(fileParts)
			if l >= 2 {
				filepath = fileParts[l-2] + string(os.PathSeparator) + fileParts[l-1]
			} else if l != 0 {
				filepath = fileParts[0]
			}
		} else {
			filepath = "???"
			line = 0
		}

		config.writeField("\"caller\":\"%s:%d\"", filepath, line)
	}

	// Append actual log entry
	config.writeField("\"msg\":\"%s\"", fmt.Sprintf(format, values...))

	for _, field := range config.fields {
		config.writeField("\"%s\":%s", field.Name(), field.ValueJSON())
	}

	for _, field := range fields {
		config.writeField("\"%s\":%s", field.Name(), field.ValueJSON())
	}

	config.output.Write(closeCurly)
	config.output.Write(newLine)

	return nil
}

func (config *systemConfig) writeTextEntry(level Level, depth int, fields []Field, format string,
	values ...interface{}) error {

	if config.output == nil {
		return nil
	}

	if config.minLevel > level {
		return nil // Level is below minimum
	}

	// Write full entry to output
	config.output.Lock()
	defer config.output.Unlock()

	config.first = true

	// Write Level
	if config.format&IncludeLevel != 0 {
		config.writeField("%s", levelName[level+levelOffset])
	}

	// Create log entry
	now := time.Now()

	// Append timestamp
	if config.format&IncludeTimeStamp != 0 {
		config.writeField("ts %d.%06d", now.Unix(), now.Nanosecond()/1e3)
	}

	// Append Date
	var datetime bytes.Buffer
	if config.format&IncludeDate != 0 {
		year, month, day := now.Date()
		fmt.Fprintf(&datetime, "%04d/%02d/%02d", year, month, day)
		if config.format&IncludeTime != 0 {
			fmt.Fprint(&datetime, []byte(" "))
		}
	}

	// Append Time
	if config.format&IncludeTime != 0 {
		hour, min, sec := now.Clock()
		fmt.Fprintf(&datetime, "%02d:%02d:%02d", hour, min, sec)
		if config.format&IncludeMicro == 0 {
			fmt.Fprintf(&datetime, " %06d", now.Nanosecond()/1e3)
		}
	}

	if datetime.Len() > 0 {
		config.writeField("%s", string(datetime.Bytes()))
	}

	// Append Caller
	if config.format&IncludeCaller != 0 {
		_, filepath, line, ok := runtime.Caller(depth+1)
		if ok {
			fileParts := strings.Split(filepath, string(os.PathSeparator))
			l := len(fileParts)
			if l >= 2 {
				filepath = fileParts[l-2] + string(os.PathSeparator) + fileParts[l-1]
			} else if l != 0 {
				filepath = fileParts[0]
			}
		} else {
			filepath = "???"
			line = 0
		}

		config.writeField("%s:%d", filepath, line)
	}

	// Append actual log entry
	config.writeField("%s", fmt.Sprintf(format, values...))

	for _, field := range config.fields {
		fmt.Fprintf(config.output, ", %s: %s", field.Name(), field.ValueJSON())
	}

	for _, field := range fields {
		fmt.Fprintf(config.output, ", %s: %s", field.Name(), field.ValueJSON())
	}

	config.output.Write(newLine)

	return nil
}

type Output interface {
	Write([]byte) (int, error)
	Lock()
	Unlock()
}

type fileWriter struct {
	file *os.File
	lock sync.Mutex
}

func (w *fileWriter) Write(b []byte) (int, error) {
	return w.file.Write(b)
}

func (w *fileWriter) Lock() {
	w.lock.Lock()
}

func (w *fileWriter) Unlock() {
	w.file.Sync()
	w.lock.Unlock()
}

type printer struct {
	lock sync.Mutex
}

func (p *printer) Write(b []byte) (int, error) {
	return os.Stderr.Write(b)
}

func (p *printer) Lock() {
	p.lock.Lock()
}

func (p *printer) Unlock() {
	p.lock.Unlock()
}

type dummyWriter struct {
	lock sync.Mutex
}

func (d *dummyWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (d *dummyWriter) Lock() {
	d.lock.Lock()
}

func (d *dummyWriter) Unlock() {
	d.lock.Unlock()
}
