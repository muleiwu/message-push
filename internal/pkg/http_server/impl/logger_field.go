package impl

import (
	"github.com/muleiwu/gsr/logger_interface"
)

type LoggerField struct {
	Key   string
	Value any
}

func NewLoggerField(key string, value any) logger_interface.LoggerFieldInterface {
	return &LoggerField{
		Key:   key,
		Value: value,
	}
}

func (receiver *LoggerField) GetKey() string {
	return receiver.Key
}

func (receiver *LoggerField) GetValue() any {
	return receiver.Value
}
