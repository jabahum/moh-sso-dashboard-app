package logger

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Logger struct{}

func NewLogger() *Logger {
	return &Logger{}
}

func (logger *Logger) Print(level zerolog.Level, args ...interface{}) {
	log.WithLevel(level).Msg(fmt.Sprint(args...))
}

func (logger *Logger) Printf(ctx context.Context, format string, v ...interface{}) {
	log.WithLevel(zerolog.DebugLevel).Msgf(format, v...)
}

func (logger *Logger) Debug(args ...interface{}) {
	logger.Print(zerolog.DebugLevel, args...)
}

func (logger *Logger) Info(args ...interface{}) {
	logger.Print(zerolog.InfoLevel, args...)
}

func (logger *Logger) Warn(args ...interface{}) {
	logger.Print(zerolog.WarnLevel, args...)
}

func (logger *Logger) Error(args ...interface{}) {
	logger.Print(zerolog.ErrorLevel, args...)
}

func (logger *Logger) Fatal(args ...interface{}) {
	log.Fatal().Msg(fmt.Sprint(args...))
}
func (logger *Logger) Panic(args ...interface{}) {
	logger.Print(zerolog.PanicLevel, args...)
}
func (logger *Logger) Trace(args ...interface{}) {
	logger.Print(zerolog.TraceLevel, args...)
}

func (logger *Logger) SetLevel(level zerolog.Level) {
	zerolog.SetGlobalLevel(level)
}

func (logger *Logger) GetLevel() zerolog.Level {
	return zerolog.GlobalLevel()
}
