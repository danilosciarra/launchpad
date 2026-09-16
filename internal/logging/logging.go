// Package logging provides the default log.Logger implementation used by a
// Launchpad application when no custom logger is supplied via
// launchpad.WithLogger. It is internal: callers only ever see it through the
// log.Logger interface.
package logging

import (
	"io"
	"os"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/log"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// logrusLogger adapts *logrus.Entry to log.Logger.
type logrusLogger struct {
	entry *logrus.Entry
}

// New builds the default logger from a config.LogConfig. A zero value
// LogConfig produces an info-level, JSON-formatted, stdout logger.
func New(cfg config.LogConfig) log.Logger {
	base := logrus.New()

	var formatter logrus.Formatter
	switch cfg.Format {
	case "text":
		formatter = &logrus.TextFormatter{FullTimestamp: true}
	default:
		formatter = &logrus.JSONFormatter{}
	}
	base.SetFormatter(formatter)

	var out io.Writer
	switch cfg.Output {
	case "file":
		out = &lumberjack.Logger{
			Filename:   "./app.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     15,
			Compress:   true,
		}
	default:
		out = os.Stdout
	}
	base.SetOutput(out)

	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	base.SetLevel(level)

	return &logrusLogger{entry: logrus.NewEntry(base)}
}

func (l *logrusLogger) Debugf(format string, args ...any) { l.entry.Debugf(format, args...) }
func (l *logrusLogger) Infof(format string, args ...any)  { l.entry.Infof(format, args...) }
func (l *logrusLogger) Warnf(format string, args ...any)  { l.entry.Warnf(format, args...) }
func (l *logrusLogger) Errorf(format string, args ...any) { l.entry.Errorf(format, args...) }

func (l *logrusLogger) WithFields(fields log.Fields) log.Logger {
	return &logrusLogger{entry: l.entry.WithFields(logrus.Fields(fields))}
}
