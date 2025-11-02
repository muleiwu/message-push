package impl

import (
	"github.com/muleiwu/gsr"
	"go.uber.org/zap"
)

type Logger struct {
	logger *zap.Logger
}

func NewLogger() *Logger {
	logger := &Logger{}
	logger.logger = zap.NewExample()
	return logger
}

func (receiver *Logger) getFields(args ...gsr.LoggerField) []zap.Field {
	fields := make([]zap.Field, 0)

	for _, arg := range args {
		fields = append(fields, zap.Any(arg.GetKey(), arg.GetValue()))
	}

	return fields
}

func (receiver *Logger) Debug(format string, args ...gsr.LoggerField) {

	receiver.logger.Debug(format, receiver.getFields(args...)...)
}

func (receiver *Logger) Info(format string, args ...gsr.LoggerField) {
	receiver.logger.Info(format, receiver.getFields(args...)...)
}

func (receiver *Logger) Notice(format string, args ...gsr.LoggerField) {
	receiver.logger.Info(format, receiver.getFields(args...)...)
}

func (receiver *Logger) Error(format string, args ...gsr.LoggerField) {
	receiver.logger.Error(format, receiver.getFields(args...)...)
}

func (receiver *Logger) Warn(format string, args ...gsr.LoggerField) {
	receiver.logger.Warn(format, receiver.getFields(args...)...)
}

func (receiver *Logger) Fatal(format string, args ...gsr.LoggerField) {
	receiver.logger.Fatal(format, receiver.getFields(args...)...)
}
