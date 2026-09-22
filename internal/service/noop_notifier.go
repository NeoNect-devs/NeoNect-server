package service

type silentNotificationService struct{}

func NewNoopNotifier() RealtimeNotifier {
	return &silentNotificationService{}
}

func (n *silentNotificationService) NotifyMessage(recipientID int64, packet []byte) {
}
