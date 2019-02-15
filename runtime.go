package actor

import (
	"go.uber.org/zap"
)

var (
	regActor = registeredActor{
		nameUUID:  make(map[string]string),
		uuidActor: make(map[string]Actor),
	}
)

func Cleanup() {
	logSync()
}

// Get return registered Actor by name
func Get(name string) (Actor, error) {
	return regActor.getByName(name)
}

// GetByName return registered Actor by name
func GetByName(actor string) (Actor, error) {
	return Get(actor)
}

// GetByUUID return registered Actor by UUID
func GetByUUID(uuid string) (Actor, error) {
	return regActor.getByUUID(uuid)
}

func (r *registeredActor) register(actor Actor) error {
	defer r.rwLock.Unlock()
	r.rwLock.Lock()

	if _, ok := r.nameUUID[actor.Name()]; ok {
		logger.Error(
			"register Actor failed",
			zap.String("actor", actor.Name()),
			zap.String("uuid", actor.UUID()),
			zap.String("message", "actor already registered by name"),
		)

		return RegisterActorError
	}

	r.nameUUID[actor.Name()] = actor.UUID()
	r.uuidActor[actor.UUID()] = actor

	logger.Info(
		"actor registered",
		zap.String("service", serviceName),
		zap.String("actor", actor.Name()),
		zap.String("uuid", actor.UUID()),
	)

	return nil
}

func (r *registeredActor) deregister(actor Actor) error {
	defer r.rwLock.Unlock()
	r.rwLock.Lock()

	if _, ok := r.nameUUID[actor.Name()]; !ok {
		logger.Error(
			"deregister Actor failed",
			zap.String("actor", actor.Name()),
			zap.String("uuid", actor.UUID()),
			zap.String("message", "actor haven't registered by name"),
		)

		return RegisterActorError
	}

	delete(r.uuidActor, actor.UUID())
	delete(r.nameUUID, actor.Name())

	logger.Info(
		"actor deregistered",
		zap.String("service", serviceName),
		zap.String("actor", actor.Name()),
		zap.String("uuid", actor.UUID()),
	)

	return nil
}

func (r *registeredActor) getByName(name string) (Actor, error) {
	defer r.rwLock.RUnlock()
	r.rwLock.RLock()

	if uuid, ok := r.nameUUID[name]; ok {
		if actor, ok := r.uuidActor[uuid]; ok {
			logger.Info(
				"get actor by name",
				zap.String("actor", name),
				zap.String("uuid", uuid),
				zap.String("message", "actor retrieved"),
			)

			return actor, nil
		}
	}

	logger.Error(
		"get actor by name failed",
		zap.String("actor", name),
		zap.String("message", "actor not registered"),
	)

	return nil, RetrieveActorError
}

func (r *registeredActor) getByUUID(uuid string) (Actor, error) {
	defer r.rwLock.RUnlock()
	r.rwLock.RLock()

	if actor, ok := r.uuidActor[uuid]; ok {
		logger.Info(
			"get actor by uuid",
			zap.String("uuid", uuid),
			zap.String("message", "actor retrieved"),
		)

		return actor, nil
	}

	logger.Error(
		"get actor by uuid failed",
		zap.String("uuid", uuid),
		zap.String("message", "actor not registered"),
	)

	return nil, RetrieveActorError
}
