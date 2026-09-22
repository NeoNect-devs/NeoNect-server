package service

import (
	"NeoNect/internal/infrastructure/persistence"
	"NeoNect/security"
	"context"
	"errors"
)

var ErrUserNotFound = errors.New("user not found")

type DeviceService interface {
	RegisterDevice(ctx context.Context, userID int64, deviceID string, publicKey []byte) error
	ValidateDeviceOwnership(ctx context.Context, userID int64, deviceID string) (bool, error)
	ValidateActiveDeviceOwnership(ctx context.Context, userID int64, deviceID string) (bool, error)
	GetDevicePublicKey(ctx context.Context, deviceID string) ([]byte, error)
	GetDevicesByUser(ctx context.Context, userID int64) ([]string, error)
	GetRecipientKeys(ctx context.Context, username string) ([]persistence.DeviceKey, error)
	DeleteDevice(ctx context.Context, userID int64, deviceID string) error
}

type userDeviceService struct {
	userRepository   persistence.UserRepository
	deviceRepository persistence.DeviceRepository
	vault            *security.SystemVault
	maxDevices       int
}

func NewDeviceService(uRepo persistence.UserRepository, dRepo persistence.DeviceRepository, vault *security.SystemVault, maxDevices int) DeviceService {
	return &userDeviceService{
		userRepository:   uRepo,
		deviceRepository: dRepo,
		vault:            vault,
		maxDevices:       maxDevices,
	}
}

func (s *userDeviceService) RegisterDevice(ctx context.Context, userID int64, deviceID string, publicKey []byte) error {
	protectedKey, err := s.vault.Protect(string(publicKey))
	if err != nil {
		return err
	}
	return s.deviceRepository.RegisterDevice(ctx, userID, deviceID, protectedKey, s.maxDevices)
}

func (s *userDeviceService) ValidateDeviceOwnership(ctx context.Context, userID int64, deviceID string) (bool, error) {
	return s.deviceRepository.IsDeviceOwnedByUser(ctx, userID, deviceID)
}

func (s *userDeviceService) ValidateActiveDeviceOwnership(ctx context.Context, userID int64, deviceID string) (bool, error) {
	return s.deviceRepository.IsActiveDeviceOwnedByUser(ctx, userID, deviceID)
}

func (s *userDeviceService) GetDevicePublicKey(ctx context.Context, deviceID string) ([]byte, error) {
	protectedKey, err := s.deviceRepository.GetDevicePublicKey(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	decryptedKey, err := s.vault.Reveal(protectedKey)
	if err != nil {
		return nil, err
	}
	return []byte(decryptedKey), nil
}

func (s *userDeviceService) GetDevicesByUser(ctx context.Context, userID int64) ([]string, error) {
	return s.deviceRepository.GetDevicesByUser(ctx, userID)
}

func (s *userDeviceService) GetRecipientKeys(ctx context.Context, username string) ([]persistence.DeviceKey, error) {
	usernameHash := security.ComputeHash(username)
	uid, err := s.userRepository.GetUserIdByUsername(ctx, usernameHash)
	if err != nil {
		return nil, ErrUserNotFound
	}
	deviceKeys, err := s.deviceRepository.GetDevicesAndKeysByUser(ctx, uid)
	if err != nil {
		return nil, err
	}

	var decryptedKeys []persistence.DeviceKey
	for _, dk := range deviceKeys {
		decrypted, err := s.vault.Reveal(dk.PublicKey)
		if err != nil {
			continue // skip corrupted keys
		}
		decryptedKeys = append(decryptedKeys, persistence.DeviceKey{
			DeviceID:  dk.DeviceID,
			PublicKey: []byte(decrypted),
		})
	}
	return decryptedKeys, nil
}

func (s *userDeviceService) DeleteDevice(ctx context.Context, userID int64, deviceID string) error {
	return s.deviceRepository.DeleteDevice(ctx, userID, deviceID)
}
