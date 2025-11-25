package log

import (
	"fmt"
	"github.com/spf13/cast"
	"github.com/vuuvv/errors"
	"go.uber.org/zap"
)

var logger *zap.Logger
var httpErrorLogger *zap.Logger
var defaultLogger *zap.Logger

func Logger() *zap.Logger {
	return logger
}

func SetLogger(l *zap.Logger) {
	logger = l
}

func DefaultLogger() *zap.Logger {
	return defaultLogger
}

func SetDefaultLogger(l *zap.Logger) {
	defaultLogger = l
	zap.ReplaceGlobals(l)
}

func HttpErrorLogger() *zap.Logger {
	return httpErrorLogger
}

func SetHttpErrorLogger(l *zap.Logger) {
	httpErrorLogger = l
}

func toString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case error:
		return fmt.Sprintf("%+v", v)
	default:
		return cast.ToString(val)
	}
}

func CastToError(reason any) (msg string, err error) {
	var ok bool

	err, ok = reason.(error)
	if !ok {
		err = errors.NewAndSkip(toString(reason), 2)
	}
	if err == nil {
		err = errors.NewAndSkip("Unknown Error", 2)
	} else {
		err = errors.WithStackAndSkip(err, 2)
	}

	if zap.L().Level().Enabled(zap.DebugLevel) {
		msg = fmt.Sprintf("%+v", err)
	} else {
		//goland:noinspection GoDfaNilDereference
		msg = err.Error()
	}

	return
}

func Error(reason any, field ...zap.Field) {
	msg, err := CastToError(reason)

	logger.Error(msg, append(field, zap.Error(err))...)
}

func Warn(reason any, field ...zap.Field) {
	msg, err := CastToError(reason)

	logger.Warn(msg, append(field, zap.Error(err))...)
}

func Info(msg string, field ...zap.Field) {
	logger.Info(msg, field...)
}

func Debug(msg string, field ...zap.Field) {
	logger.Debug(msg, field...)
}

func DebugWithStack(reason any, field ...zap.Field) {
	msg, _ := CastToError(reason)

	logger.Warn(msg, field...)
}
