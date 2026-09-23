package service

import (
	"NeoNect/internal/logger"
)

type PushAdapter interface {
	SendNotification(deviceID string, payload []byte) error
	SetOnNotification(handler func(string, []byte))
}

type pushNotificationAdapter struct {
	notificationHandler func(deviceID string, payload []byte)
	logger              logger.Logger
}

func NewPushAdapter(l logger.Logger) PushAdapter {
	return &pushNotificationAdapter{logger: l}
}

func (a *pushNotificationAdapter) SendNotification(deviceID string, payload []byte) error {
	if a.notificationHandler != nil {
		a.notificationHandler(deviceID, payload)
	}

	a.logger.Debugf("Push notification dispatched for device: %s", deviceID)
	return nil
}

func (a *pushNotificationAdapter) SetOnNotification(handler func(string, []byte)) {
	a.notificationHandler = handler
}
