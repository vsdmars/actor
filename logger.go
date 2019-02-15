package actor

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger serviceLogger
var origLogger serviceLogger

func init() {
	initLogger()
}

// Sync sync logger output
func logSync() {
	// ignore logger Sync error
	logger.Sync()
}

// SetLogger sets caller provided zap logger
//
// reset to service's default logger by passing in nil pointer
func SetLogger(l *zap.Logger) {
	if l != nil {
		logger.Logger = l
		logger.atom = nil
		logger.provided = true
		return
	}

	logger = origLogger
}

// SetLogLevel sets the service log level
//
// noop if caller provides it's own zap logger
func SetLogLevel(level zapcore.Level) {
	if logger.provided {
		return
	}

	logger.atom.SetLevel(level)
}

func initLogger() {
	// default log level set to 'info'
	atom := zap.NewAtomicLevelAt(zap.InfoLevel)

	config := zap.Config{
		Level:       atom,
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json", // console, json, toml
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	mylogger, err := config.Build()
	if err != nil {
		fmt.Printf("Initialize zap logger error: %v", err)
		os.Exit(1)
	}

	logger = serviceLogger{mylogger, &atom, false}
	origLogger = logger
}
