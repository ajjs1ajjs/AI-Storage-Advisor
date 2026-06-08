package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents log severity
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

var (
	currentLevel Level = INFO
	mu           sync.RWMutex
	output       io.Writer = os.Stderr
	logger       *log.Logger
	initOnce     sync.Once
)

func init() {
	SetOutput(os.Stderr)
}

// SetLevel changes the minimum log level
func SetLevel(l Level) {
	mu.Lock()
	defer mu.Unlock()
	currentLevel = l
}

// GetLevel returns current log level
func GetLevel() Level {
	mu.RLock()
	defer mu.RUnlock()
	return currentLevel
}

// SetOutput sets the output writer
func SetOutput(w io.Writer) {
	initOnce.Do(func() {})
	mu.Lock()
	defer mu.Unlock()
	output = w
	logger = log.New(w, "", 0)
}

func logf(level Level, format string, args ...interface{}) {
	mu.RLock()
	lvl := currentLevel
	out := logger
	mu.RUnlock()

	if level < lvl {
		return
	}

	msg := fmt.Sprintf(format, args...)
	_, file, line, ok := runtime.Caller(2)
	var caller string
	if ok {
		short := file
		if idx := strings.LastIndex(file, "/"); idx >= 0 {
			short = file[idx+1:]
		}
		caller = fmt.Sprintf("%s:%d", short, line)
	} else {
		caller = "???"
	}

	timestamp := time.Now().Format("2006/01/02 15:04:05")
	out.Output(0, fmt.Sprintf("%s [%s] %s %s", timestamp, level.String(), caller, msg))

	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs at DEBUG level
func Debug(format string, args ...interface{}) {
	logf(DEBUG, format, args...)
}

// Info logs at INFO level
func Info(format string, args ...interface{}) {
	logf(INFO, format, args...)
}

// Warn logs at WARN level
func Warn(format string, args ...interface{}) {
	logf(WARN, format, args...)
}

// Error logs at ERROR level
func Error(format string, args ...interface{}) {
	logf(ERROR, format, args...)
}

// Fatal logs at FATAL level then exits
func Fatal(format string, args ...interface{}) {
	logf(FATAL, format, args...)
}

// Errorf returns a formatted error (replaces fmt.Errorf in some cases)
func Errorf(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	Error("%s", msg)
	return fmt.Errorf("%s", msg)
}
