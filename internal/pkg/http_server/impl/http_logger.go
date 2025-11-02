package impl

import (
	"github.com/muleiwu/gsr"
)

type HttpLogger struct {
	logger  gsr.Logger
	traceId string
}

func NewHttpLogger(logger gsr.Logger, traceId string) gsr.Logger {
	l := &HttpLogger{
		logger:  logger,
		traceId: traceId,
	}
	return l
}

func (receiver *HttpLogger) Debug(format string, args ...gsr.LoggerField) {
	args = append(args, NewLoggerField("traceId", receiver.traceId))
	receiver.logger.Debug(format, args...)
}

func (receiver *HttpLogger) Info(format string, args ...gsr.LoggerField) {
	args = append(args, NewLoggerField("traceId", receiver.traceId))
	receiver.logger.Info(format, args...)
}

func (receiver *HttpLogger) Notice(format string, args ...gsr.LoggerField) {
	args = append(args, NewLoggerField("traceId", receiver.traceId))
	receiver.logger.Info(format, args...)
}

func (receiver *HttpLogger) Error(format string, args ...gsr.LoggerField) {
	args = append(args, NewLoggerField("traceId", receiver.traceId))
	receiver.logger.Error(format, args...)
}

func (receiver *HttpLogger) Warn(format string, args ...gsr.LoggerField) {
	args = append(args, NewLoggerField("traceId", receiver.traceId))
	receiver.logger.Warn(format, args...)
}

func (receiver *HttpLogger) Fatal(format string, args ...gsr.LoggerField) {
	args = append(args, NewLoggerField("traceId", receiver.traceId))
	receiver.logger.Fatal(format, args...)
}
