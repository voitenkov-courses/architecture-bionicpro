package logger

import (
	"log"

	"github.com/sirupsen/logrus"
)

type Logger struct{}

func New(level string) *Logger {
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		log.Fatalf("failed to parse the level: %v", err)
	}

	logrus.SetLevel(logLevel)
	return &Logger{}
}

func (l Logger) Info(msg ...interface{}) {
	logrus.Info(msg...)
}

func (l Logger) Infof(format string, args ...interface{}) {
	logrus.Infof(format, args...)
}

func (l Logger) Error(msg ...interface{}) {
	logrus.Error(msg...)
}

func (l Logger) Errorf(format string, args ...interface{}) {
	logrus.Errorf(format, args...)
}

func (l Logger) Warn(msg ...interface{}) {
	logrus.Warn(msg...)
}

func (l Logger) Warnf(format string, args ...interface{}) {
	logrus.Warnf(format, args...)
}

func (l Logger) Debug(msg ...interface{}) {
	logrus.Debug(msg...)
}
