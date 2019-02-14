package actor

import (
	"go.uber.org/zap"
)

type serviceLogger struct {
	*zap.Logger
	atom     *zap.AtomicLevel
	provided bool
}
