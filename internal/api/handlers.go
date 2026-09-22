package api

import (
	"net"

	"NeoNect/internal/infrastructure/persistence"
	"NeoNect/internal/service"
	"NeoNect/security"
)

const (
	ErrMethodNotAllowed = "method not allowed"
	ErrBadRequest       = "bad request"
	ErrUnauthorized     = "unauthorized"
	ErrServer           = "internal server error"
	ErrNotFound         = "resource not found"
)

type GenericResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type HandlerManager struct {
	AuthService    service.AuthService
	RelayService   service.RelayService
	WSManager      service.WebSocketManager
	DeviceService  service.DeviceService
	FriendService  service.FriendService
	PrekeyService  service.PrekeyService
	UserRepo       persistence.UserRepository
	IntegrityRepo  persistence.IntegrityRepository
	Vault          *security.SystemVault
	RateLimiter    *RequestRateLimiter
	TrustedProxies []*net.IPNet
}

func NewHandlerManager(
	auth service.AuthService,
	relay service.RelayService,
	ws service.WebSocketManager,
	device service.DeviceService,
	friend service.FriendService,
	prekey service.PrekeyService,
	user persistence.UserRepository,
	integrity persistence.IntegrityRepository,
	vault *security.SystemVault,
	trustedProxies []string,
) *HandlerManager {
	return &HandlerManager{
		AuthService:    auth,
		RelayService:   relay,
		WSManager:      ws,
		DeviceService:  device,
		FriendService:  friend,
		PrekeyService:  prekey,
		UserRepo:       user,
		IntegrityRepo:  integrity,
		Vault:          vault,
		RateLimiter:    NewRateLimiter(),
		TrustedProxies: ParseTrustedProxies(trustedProxies),
	}
}
