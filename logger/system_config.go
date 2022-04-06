package logger

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
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

	tab               = []byte{byte('\t')}
	comma             = []byte{byte(',')}
	newLine           = []byte{byte('\n')}
	openCurly         = []byte{byte('{')}
	closeCurlyNewLine = []byte{byte('}'), byte('\n')}
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

	lock sync.Mutex
}

// Copy makes a separate copy so if the fields are modified in one copy they will not be in another.
func (config systemConfig) Copy() systemConfig {
	result := config

	config.lock.Lock()
	result.fields = make([]Field, len(config.fields))
	copy(result.fields, config.fields)
	config.lock.Unlock()

	return result
}

// newSystemConfig creates a new logger system config.
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

// newSystemConfigFromSetup creates a new logger system config.
func newSystemConfigFromSetup(setup SetupConfig) (systemConfig, error) {
	result := systemConfig{
		isText:     setup.Format == FormatText,
		stackLevel: LevelError,
		minLevel:   LevelInfo,
		format:     IncludeCaller | IncludeLevel,
	}

	if setup.Format == FormatText {
		result.format |= IncludeDate | IncludeTime | IncludeMicro
	} else {
		result.format |= IncludeTimeStamp
	}

	result.minLevel = setup.Level

	if len(setup.Path) > 0 {
		if setup.Path == "dummy" { // for benchmarking
			result.output = &dummyWriter{}
		} else {
			file, err := os.OpenFile(setup.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
func (config *systemConfig) addField(newField Field) {
	config.lock.Lock()
	defer config.lock.Unlock()

	for i, field := range config.fields {
		if field.Name() == newField.Name() {
			// Insert new field in same location as previous field with same name.
			var prev []Field
			if i > 0 {
				prev = config.fields[:i]
			}

			var after []Field
			if i+1 < len(config.fields) {
				after = config.fields[i+1:]
			}

			config.fields = append(prev, newField)
			config.fields = append(config.fields, after...)
			return
		}
	}

	config.fields = append(config.fields, newField)
}

// addSubSystem adds a subsystem to the log outputs
func (config *systemConfig) addSubSystem(name string) {
	config.lock.Lock()
	defer config.lock.Unlock()

	for i, field := range config.fields {
		if field.Name() == "subsystem" {
			config.fields[i] = String("subsystem", name)
			return
		}
	}

	config.fields = append(config.fields, String("subsystem", name))
}

// removeSubSystem removes the subsystem from the log outputs
func (config *systemConfig) removeSubSystem() {
	config.lock.Lock()
	defer config.lock.Unlock()

	for i, field := range config.fields {
		if field.Name() == "subsystem" {
			config.fields = append(config.fields[:i], config.fields[i+1:]...)
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

func (config *systemConfig) writeEntry(level Level, caller string, fields []Field,
	format string, values ...interface{}) error {

	if config.isText {
		return config.writeTextEntry(level, caller, fields, format, values...)
	}

	return config.writeJSONEntry(level, caller, fields, format, values...)
}

func (config *systemConfig) writeJSONEntry(level Level, caller string, fields []Field,
	format string, values ...interface{}) error {

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
		if config.format&IncludeMicro != 0 {
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
		config.writeField("\"caller\":%s", strconv.Quote(caller))
	}

	// Append actual log entry
	config.writeField("\"msg\":%s", strconv.Quote(fmt.Sprintf(format, values...)))

	config.lock.Lock()
	for i, field := range config.fields {
		if fieldExists(field.Name(), config.fields[:i]) {
			continue // skip duplicate field name
		}
		config.writeField("\"%s\":%s", field.Name(), field.ValueJSON())
	}
	config.lock.Unlock()

	for i, field := range fields {
		if fieldExists(field.Name(), config.fields) || fieldExists(field.Name(), fields[:i]) {
			continue // skip duplicate field name
		}
		config.writeField("\"%s\":%s", field.Name(), field.ValueJSON())
	}

	config.output.Write(closeCurlyNewLine)

	switch level {
	case LevelFatal:
		defer os.Exit(1)
	case LevelPanic:
		defer panic(fmt.Sprintf(format, values...))
	}

	return nil
}

func (config *systemConfig) writeTextEntry(level Level, caller string, fields []Field,
	format string, values ...interface{}) error {

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
		if config.format&IncludeMicro != 0 {
			fmt.Fprintf(&datetime, ".%06d", now.Nanosecond()/1e3)
		}
	}

	if datetime.Len() > 0 {
		config.writeField("%s", string(datetime.Bytes()))
	}

	// Append Caller
	if config.format&IncludeCaller != 0 {
		config.writeField(caller)
	}

	// Append actual log entry
	config.writeField("%s", fmt.Sprintf(format, values...))

	config.lock.Lock()
	for i, field := range config.fields {
		if fieldExists(field.Name(), config.fields[:i]) {
			continue // skip duplicate field name
		}
		fmt.Fprintf(config.output, ", %s: %s", field.Name(), field.ValueJSON())
	}
	config.lock.Unlock()

	for i, field := range fields {
		if fieldExists(field.Name(), config.fields) || fieldExists(field.Name(), fields[:i]) {
			continue // skip duplicate field name
		}
		fmt.Fprintf(config.output, ", %s: %s", field.Name(), field.ValueJSON())
	}

	config.output.Write(newLine)

	switch level {
	case LevelFatal:
		defer os.Exit(1)
	case LevelPanic:
		defer panic(fmt.Sprintf(format, values...))
	}

	return nil
}

func fieldExists(name string, fields []Field) bool {
	for _, f := range fields {
		if f.Name() == name {
			return true
		}
	}

	return false
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
	os.Stderr.Sync()
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
