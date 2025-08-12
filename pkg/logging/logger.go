package logging

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ANSI color codes
const (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Dim   = "\033[2m"

	// Standard colors
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"

	// Bright colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

// ColoredLogger wraps zap.Logger with colored output
type ColoredLogger struct {
	*zap.Logger
	enableColors bool
}

// Component represents different parts of the system for color coding
type Component string

const (
	ComponentNode     Component = "NODE"
	ComponentRQLite   Component = "RQLITE"
	ComponentLibP2P   Component = "LIBP2P"
	ComponentStorage  Component = "STORAGE"
	ComponentDatabase Component = "DATABASE"
	ComponentClient   Component = "CLIENT"
	ComponentDHT      Component = "DHT"
	ComponentGeneral  Component = "GENERAL"
	ComponentAnyone   Component = "ANYONE"
)

// getComponentColor returns the color for a specific component
func getComponentColor(component Component) string {
	switch component {
	case ComponentNode:
		return BrightBlue
	case ComponentRQLite:
		return BrightMagenta
	case ComponentLibP2P:
		return BrightCyan
	case ComponentStorage:
		return BrightYellow
	case ComponentDatabase:
		return Green
	case ComponentClient:
		return Blue
	case ComponentDHT:
		return Magenta
	case ComponentGeneral:
		return Yellow
	case ComponentAnyone:
		return Cyan
	default:
		return White
	}
}

// getLevelColor returns the color for a log level
func getLevelColor(level zapcore.Level) string {
	switch level {
	case zapcore.DebugLevel:
		return Gray
	case zapcore.InfoLevel:
		return BrightWhite
	case zapcore.WarnLevel:
		return BrightYellow
	case zapcore.ErrorLevel:
		return BrightRed
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return Red
	default:
		return White
	}
}

// coloredConsoleEncoder creates a custom encoder with colors
func coloredConsoleEncoder(enableColors bool) zapcore.Encoder {
	config := zap.NewDevelopmentEncoderConfig()
	config.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		timeStr := t.Format("2006-01-02T15:04:05.000Z0700")
		if enableColors {
			enc.AppendString(fmt.Sprintf("%s%s%s", Dim, timeStr, Reset))
		} else {
			enc.AppendString(timeStr)
		}
	}

	config.EncodeLevel = func(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		levelStr := strings.ToUpper(level.String())
		if enableColors {
			color := getLevelColor(level)
			enc.AppendString(fmt.Sprintf("%s%s%-5s%s", color, Bold, levelStr, Reset))
		} else {
			enc.AppendString(fmt.Sprintf("%-5s", levelStr))
		}
	}

	config.EncodeCaller = func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		if enableColors {
			enc.AppendString(fmt.Sprintf("%s%s%s", Dim, caller.TrimmedPath(), Reset))
		} else {
			enc.AppendString(caller.TrimmedPath())
		}
	}

	return zapcore.NewConsoleEncoder(config)
}

// NewColoredLogger creates a new colored logger
func NewColoredLogger(component Component, enableColors bool) (*ColoredLogger, error) {
	// Create encoder
	encoder := coloredConsoleEncoder(enableColors)

	// Create core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	// Create logger with caller information
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &ColoredLogger{
		Logger:       logger,
		enableColors: enableColors,
	}, nil
}

// NewDefaultLogger creates a logger with default settings and color auto-detection
func NewDefaultLogger(component Component) (*ColoredLogger, error) {
	return NewColoredLogger(component, true)
}

// Component-specific logging methods
func (l *ColoredLogger) ComponentInfo(component Component, msg string, fields ...zap.Field) {
	if l.enableColors {
		color := getComponentColor(component)
		msg = fmt.Sprintf("%s[%s]%s %s", color, component, Reset, msg)
	} else {
		msg = fmt.Sprintf("[%s] %s", component, msg)
	}
	l.Info(msg, fields...)
}

func (l *ColoredLogger) ComponentWarn(component Component, msg string, fields ...zap.Field) {
	if l.enableColors {
		color := getComponentColor(component)
		msg = fmt.Sprintf("%s[%s]%s %s", color, component, Reset, msg)
	} else {
		msg = fmt.Sprintf("[%s] %s", component, msg)
	}
	l.Warn(msg, fields...)
}

func (l *ColoredLogger) ComponentError(component Component, msg string, fields ...zap.Field) {
	if l.enableColors {
		color := getComponentColor(component)
		msg = fmt.Sprintf("%s[%s]%s %s", color, component, Reset, msg)
	} else {
		msg = fmt.Sprintf("[%s] %s", component, msg)
	}
	l.Error(msg, fields...)
}

func (l *ColoredLogger) ComponentDebug(component Component, msg string, fields ...zap.Field) {
	if l.enableColors {
		color := getComponentColor(component)
		msg = fmt.Sprintf("%s[%s]%s %s", color, component, Reset, msg)
	} else {
		msg = fmt.Sprintf("[%s] %s", component, msg)
	}
	l.Debug(msg, fields...)
}

// StandardLogger provides colored standard library compatible logging
type StandardLogger struct {
	logger    *ColoredLogger
	component Component
}

// NewStandardLogger creates a standard library compatible colored logger
func NewStandardLogger(component Component) (*StandardLogger, error) {
	coloredLogger, err := NewDefaultLogger(component)
	if err != nil {
		return nil, err
	}

	return &StandardLogger{
		logger:    coloredLogger,
		component: component,
	}, nil
}

// Printf implements the standard library log interface with colors
func (s *StandardLogger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	// Remove trailing newline if present (zap adds its own)
	msg = strings.TrimSuffix(msg, "\n")
	s.logger.ComponentInfo(s.component, msg)
}

// Print implements the standard library log interface with colors
func (s *StandardLogger) Print(v ...interface{}) {
	msg := fmt.Sprint(v...)
	msg = strings.TrimSuffix(msg, "\n")
	s.logger.ComponentInfo(s.component, msg)
}

// Println implements the standard library log interface with colors
func (s *StandardLogger) Println(v ...interface{}) {
	msg := fmt.Sprintln(v...)
	msg = strings.TrimSuffix(msg, "\n")
	s.logger.ComponentInfo(s.component, msg)
}

func (s *StandardLogger) Errorf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	msg = strings.TrimSuffix(msg, "\n")
	s.logger.ComponentError(s.component, msg)
}
