package service

import (
	"log"
)

type PushAdapter interface {
	SendNotification(deviceID string, payload []byte) error
	SetOnNotification(handler func(string, []byte))
}

type pushNotificationAdapter struct {
	notificationHandler func(deviceID string, payload []byte)
}

func NewPushAdapter() PushAdapter {
	return &pushNotificationAdapter{}
}

func (a *pushNotificationAdapter) SendNotification(deviceID string, payload []byte) error {
	if a.notificationHandler != nil {
		a.notificationHandler(deviceID, payload)
	}

	log.Printf("Push notification dispatched")
	return nil
}

func (a *pushNotificationAdapter) SetOnNotification(handler func(string, []byte)) {
	a.notificationHandler = handler
}
