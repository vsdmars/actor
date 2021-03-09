package actor

import (
	l "github.com/vsdmars/actor/internal/logger"

	"go.uber.org/zap/zapcore"
)

// SetLoggingLevel sets the service log level
//
// noop if caller provides it's own zap logger
func SetLoggingLevel(level zapcore.Level) {
	l.SetLogLevel(level)
}
