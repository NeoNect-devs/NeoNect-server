package service

import (
	"NeoNect/internal/logger"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"NeoNect/internal/config"
	"NeoNect/internal/infrastructure/persistence"
	"github.com/gorilla/websocket"
)

type WebSocketManager interface {
	HandleConnection(w http.ResponseWriter, r *http.Request, deviceID string)
	DeliverMessage(item persistence.DeliveryQueueItem) bool
	DisconnectDevice(deviceID string)
	Shutdown()
	IsDeviceOnline(deviceID string) bool
}

type webSocketMessage struct {
	ID              int64   `json:"id"`
	MessageID       *string `json:"message_id,omitempty"`
	SenderDeviceID  *string `json:"sender_device_id,omitempty"`
	ProtocolVersion *int    `json:"protocol_version,omitempty"`
	Payload         string  `json:"ciphertext"`
	Sequence        int64   `json:"sequence"`
}

type websocketSession struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type socketConnectionManager struct {
	protocolUpgrader websocket.Upgrader
	activeSessions   map[string]*websocketSession
	sessionMutex     sync.RWMutex
	connSem          chan struct{}
	logger           logger.Logger
}

const MaxGlobalWebSockets = 10000

func NewWebSocketManager(allowedOrigins []string, l logger.Logger) WebSocketManager {
	m := &socketConnectionManager{
		protocolUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				for _, allowed := range allowedOrigins {
					if allowed == origin {
						return true
					}
				}
				return false
			},
		},
		activeSessions: make(map[string]*websocketSession),
		connSem:        make(chan struct{}, MaxGlobalWebSockets),
		logger:         l,
	}
	return m
}

func (m *socketConnectionManager) HandleConnection(w http.ResponseWriter, r *http.Request, deviceID string) {
	select {
	case m.connSem <- struct{}{}:
	default:
		http.Error(w, "Service Unavailable: connection limit reached", http.StatusServiceUnavailable)
		return
	}

	connection, err := m.protocolUpgrader.Upgrade(w, r, nil)
	if err != nil {
		<-m.connSem
		m.logger.Warnf("WebSocket upgrade failed from %s for device %s: %v", r.RemoteAddr, deviceID, err)
		return
	}

	connection.SetReadLimit(config.GlobalMaxBodySize)

	sess := &websocketSession{
		conn: connection,
	}

	m.sessionMutex.Lock()
	if existing, exists := m.activeSessions[deviceID]; exists {
		_ = existing.conn.Close()
	}
	m.activeSessions[deviceID] = sess
	m.sessionMutex.Unlock()

	stopHeartbeat := make(chan struct{})
	var stopOnce sync.Once

	go func(s *websocketSession, stop <-chan struct{}) {
		ticker := time.NewTicker(config.PingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				_ = s.conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
				err := s.conn.WriteMessage(websocket.PingMessage, nil)
				s.mu.Unlock()
				if err != nil {
					_ = s.conn.Close()
					return
				}
			case <-stop:
				return
			}
		}
	}(sess, stopHeartbeat)

	func(s *websocketSession) {
		defer func() {
			stopOnce.Do(func() { close(stopHeartbeat) })
			_ = s.conn.Close()
			m.sessionMutex.Lock()
			if m.activeSessions[deviceID] == s {
				delete(m.activeSessions, deviceID)
			}
			m.sessionMutex.Unlock()
			<-m.connSem
		}()

		s.conn.SetReadDeadline(time.Now().Add(config.PongWait))
		s.conn.SetPongHandler(func(string) error {
			s.conn.SetReadDeadline(time.Now().Add(config.PongWait))
			return nil
		})
		for {
			if _, _, err := s.conn.ReadMessage(); err != nil {
				break
			}
		}
	}(sess)
}

func (m *socketConnectionManager) DeliverMessage(item persistence.DeliveryQueueItem) bool {
	m.sessionMutex.RLock()
	sess, exists := m.activeSessions[item.DeviceID]
	m.sessionMutex.RUnlock()

	if !exists {
		return false
	}

	// serialize writes per connection per session
	sess.mu.Lock()
	defer sess.mu.Unlock()

	message := webSocketMessage{
		ID:              item.ID,
		MessageID:       item.MessageID,
		SenderDeviceID:  item.SenderDeviceID,
		ProtocolVersion: item.ProtocolVersion,
		Payload:         base64.StdEncoding.EncodeToString(item.Payload),
		Sequence:        item.Sequence,
	}

	serialized, err := json.Marshal(message)
	if err != nil {
		return false
	}

	_ = sess.conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
	if err := sess.conn.WriteMessage(websocket.TextMessage, serialized); err != nil {
		_ = sess.conn.Close()

		m.sessionMutex.Lock()
		if m.activeSessions[item.DeviceID] == sess {
			delete(m.activeSessions, item.DeviceID)
		}
		m.sessionMutex.Unlock()

		return false
	}

	return true
}

func (m *socketConnectionManager) DisconnectDevice(deviceID string) {
	m.sessionMutex.Lock()
	defer m.sessionMutex.Unlock()

	if sess, exists := m.activeSessions[deviceID]; exists {
		_ = sess.conn.Close()
		delete(m.activeSessions, deviceID)
	}
}

func (m *socketConnectionManager) Shutdown() {
	m.sessionMutex.Lock()
	sessions := make([]*websocketSession, 0, len(m.activeSessions))
	for deviceID, sess := range m.activeSessions {
		sessions = append(sessions, sess)
		delete(m.activeSessions, deviceID)
	}
	m.sessionMutex.Unlock()

	for _, sess := range sessions {
		sess.mu.Lock()
		deadline := time.Now().Add(500 * time.Millisecond)
		_ = sess.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseServiceRestart, "Server shutting down"), deadline)
		sess.mu.Unlock()

		_ = sess.conn.Close()
	}
}

func (m *socketConnectionManager) IsDeviceOnline(deviceID string) bool {
	m.sessionMutex.RLock()
	_, exists := m.activeSessions[deviceID]
	m.sessionMutex.RUnlock()
	return exists
}
