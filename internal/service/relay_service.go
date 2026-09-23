package service

import (
	"NeoNect/internal/config"
	"NeoNect/internal/infrastructure/persistence"
	"NeoNect/security"
	"context"
	"database/sql"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	ErrRecipientNotFound        = errors.New("recipient not found")
	ErrRecipientNoDevices       = errors.New("recipient has no registered devices")
	ErrRecipientDeviceNotFound  = errors.New("recipient device not found")
	ErrInvalidRecipientDeviceID = errors.New("invalid recipient device id")
	ErrDuplicateConflict        = errors.New("duplicate message conflict")
)

type MessageEnvelope struct {
	MessageID         string
	SenderDeviceID    string
	RecipientDeviceID string
	ProtocolVersion   int
	Ciphertext        []byte
}

type RelayService interface {
	Send(ctx context.Context, senderUID int64, toUsername string, payload []byte) error
	SendEnvelope(ctx context.Context, senderUID int64, envelope MessageEnvelope) error
	GetQueue(ctx context.Context, deviceID string) ([]persistence.DeliveryQueueItem, error)
	Acknowledge(ctx context.Context, deviceID string, messageID int64) error
	SetTTL(ttlSeconds int)
	CleanupNow(ctx context.Context) error
	Shutdown(ctx context.Context)
}

type RealtimeNotifier interface {
	NotifyMessage(recipientID int64, packet []byte)
}

type DeliveryProcessor interface {
	Process(item persistence.DeliveryQueueItem)
}

type RelayManager struct {
	userRepository     persistence.UserRepository
	deviceRepository   persistence.DeviceRepository
	deliveryRepository persistence.DeliveryQueueRepository
	friendshipRepo     persistence.FriendshipRepository
	notifier           RealtimeNotifier
	worker             DeliveryProcessor
	stateMutex         sync.Mutex
	isClosed           bool
	stopChannel        chan struct{}
	messageTTL         int
	maintenancePeriod  time.Duration
	wsManager          WebSocketManager
	workerWg           sync.WaitGroup
}

func NewRelayService(
	uRepo persistence.UserRepository,
	dRepo persistence.DeviceRepository,
	qRepo persistence.DeliveryQueueRepository,
	fRepo persistence.FriendshipRepository,
	n RealtimeNotifier,
	ws WebSocketManager,
	push PushAdapter,
) RelayService {
	manager := &RelayManager{
		userRepository:     uRepo,
		deviceRepository:   dRepo,
		deliveryRepository: qRepo,
		friendshipRepo:     fRepo,
		notifier:           n,
		stopChannel:        make(chan struct{}),
		messageTTL:         config.DefaultMessageTTL,
		maintenancePeriod:  config.DefaultCleanupInterval,
		wsManager:          ws,
	}
	manager.worker = &DeliveryWorker{
		deliveryRepository: qRepo,
		wsManager:          ws,
		pushAdapter:        push,
	}
	go manager.startMaintenanceLoop()
	go manager.schedulerLoop()
	return manager
}

func (m *RelayManager) Send(ctx context.Context, senderUID int64, toUsername string, payload []byte) error {
	if toUsername == "" {
		return errors.New("recipient username required")
	}

	usernameHash := security.ComputeHash(toUsername)
	recipientUID, err := m.userRepository.GetUserIdByUsername(ctx, usernameHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRecipientNotFound
		}
		return err
	}

	if senderUID != recipientUID {
		isFriend, err := m.friendshipRepo.CheckFriendship(ctx, senderUID, recipientUID)
		if err != nil {
			return err
		}
		if !isFriend {
			return errors.New("unauthorized sender")
		}
	}

	devices, err := m.deviceRepository.GetDevicesByUser(ctx, recipientUID)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		return ErrRecipientNoDevices
	}

	m.stateMutex.Lock()
	ttl := m.messageTTL
	m.stateMutex.Unlock()

	if err := m.deliveryRepository.FanOutMessage(ctx, devices, payload, ttl); err != nil {
		return err
	}

	if m.notifier != nil {
		m.notifier.NotifyMessage(recipientUID, payload)
	}

	return nil
}

func (m *RelayManager) SendEnvelope(ctx context.Context, senderUID int64, envelope MessageEnvelope) error {
	if envelope.MessageID == "" || envelope.SenderDeviceID == "" || envelope.RecipientDeviceID == "" {
		return errors.New("incomplete envelope")
	}

	recipientUID, err := m.deviceRepository.GetDeviceOwner(ctx, envelope.RecipientDeviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRecipientDeviceNotFound
		}
		return err
	}

	if senderUID != recipientUID {
		isFriend, err := m.friendshipRepo.CheckFriendship(ctx, senderUID, recipientUID)
		if err != nil {
			return err
		}
		if !isFriend {
			return errors.New("unauthorized sender")
		}
	}

	isActive, err := m.deviceRepository.IsActiveDeviceOwnedByUser(ctx, recipientUID, envelope.RecipientDeviceID)
	if err != nil {
		return err
	}
	if !isActive {
		return ErrRecipientDeviceNotFound
	}

	m.stateMutex.Lock()
	ttl := m.messageTTL
	m.stateMutex.Unlock()

	sequence, err := m.deliveryRepository.GetLastSequence(ctx, envelope.RecipientDeviceID)
	if err != nil {
		sequence = 0
	}
	sequence++

	if err := m.deliveryRepository.EnqueueEnvelope(ctx, envelope.MessageID, envelope.SenderDeviceID, envelope.RecipientDeviceID, envelope.ProtocolVersion, envelope.Ciphertext, ttl, sequence); err != nil {
		if errors.Is(err, persistence.ErrDuplicateConflict) {
			return ErrDuplicateConflict
		}
		return err
	}

	if m.notifier != nil {
		m.notifier.NotifyMessage(recipientUID, envelope.Ciphertext)
	}

	return nil
}

func (m *RelayManager) GetQueue(ctx context.Context, deviceID string) ([]persistence.DeliveryQueueItem, error) {
	return m.deliveryRepository.GetQueue(ctx, deviceID)
}

func (m *RelayManager) Acknowledge(ctx context.Context, deviceID string, messageID int64) error {
	return m.deliveryRepository.Acknowledge(ctx, deviceID, messageID)
}

func (m *RelayManager) SetTTL(ttlSeconds int) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	m.messageTTL = ttlSeconds
}

func (m *RelayManager) CleanupNow(ctx context.Context) error {
	return m.deliveryRepository.Cleanup(ctx)
}

func (m *RelayManager) schedulerLoop() {
	ticker := time.NewTicker(config.DefaultSchedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.dispatchDueMessages()
		case <-m.stopChannel:
			return
		}
	}
}

func (m *RelayManager) dispatchDueMessages() {
	now := time.Now().Unix()
	items, err := m.deliveryRepository.GetDueItems(context.Background(), now)
	if err != nil {
		log.Printf("ERROR: Failed to query due messages in scheduler: %v", err)
		return
	}
	for _, item := range items {
		m.workerWg.Add(1)
		go func(it persistence.DeliveryQueueItem) {
			defer m.workerWg.Done()
			m.worker.Process(it)
		}(item)
	}
}

func (m *RelayManager) startMaintenanceLoop() {
	m.stateMutex.Lock()
	period := m.maintenancePeriod
	m.stateMutex.Unlock()

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performMaintenance()
		case <-m.stopChannel:
			return
		}
	}
}

func (m *RelayManager) performMaintenance() {
	ctx, cancel := context.WithTimeout(context.Background(), config.MaintenanceTimeout)
	defer cancel()
	if err := m.deliveryRepository.Cleanup(ctx); err != nil {
		log.Printf("WARN: Maintenance cleanup failed: %v", err)
	}
}

func (m *RelayManager) Shutdown(ctx context.Context) {
	m.stateMutex.Lock()
	if m.isClosed {
		m.stateMutex.Unlock()
		return
	}
	m.isClosed = true
	close(m.stopChannel)
	m.stateMutex.Unlock()

	done := make(chan struct{})
	go func() {
		m.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		abortCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		select {
		case <-done:
		case <-abortCtx.Done():
		}
	}
}

type DeliveryWorker struct {
	deliveryRepository persistence.DeliveryQueueRepository
	wsManager          WebSocketManager
	pushAdapter        PushAdapter
}

func (w *DeliveryWorker) Process(item persistence.DeliveryQueueItem) {
	now := time.Now().Unix()

	const claimDuration = config.DeliveryLockDuration
	isClaimed, err := w.deliveryRepository.ClaimItem(context.Background(), item.ID, now+claimDuration, item.NextRetry)
	if err != nil || !isClaimed {
		return
	}

	retryCount := item.RetryCount
	if item.AckDeadline > 0 {
		retryCount++
	}

	if retryCount > config.MaximumRetryAttempts {
		if err := w.deliveryRepository.UpdateState(context.Background(), item.ID, retryCount, now+config.TerminalRetryBackoff, 0); err != nil {
			log.Printf("ERROR: Failed to update state for item %d after max retries: %v", item.ID, err)
		}
		return
	}

	if w.wsManager.DeliverMessage(item) {
		deadline := now + config.AcknowledgmentTimeout
		if err := w.deliveryRepository.UpdateState(context.Background(), item.ID, retryCount, deadline, deadline); err != nil {
			log.Printf("ERROR: Failed to update state for item %d during delivery: %v", item.ID, err)
		}
		return
	}

	if item.AckDeadline == 0 {
		retryCount++
	}

	backoff := int64(1<<uint(retryCount)) * config.InitialBackoffDuration
	if backoff > config.MaximumBackoffDuration {
		backoff = config.MaximumBackoffDuration
	}

	if err := w.deliveryRepository.UpdateState(context.Background(), item.ID, retryCount, now+backoff, 0); err != nil {
		log.Printf("ERROR: Failed to update state for item %d during backoff: %v", item.ID, err)
	}

	if item.RetryCount == 0 && w.pushAdapter != nil {
		if err := w.pushAdapter.SendNotification(item.DeviceID, item.Payload); err != nil {
			log.Printf("WARN: Push notification failed for device %s: %v", item.DeviceID, err)
		}
	}
}
