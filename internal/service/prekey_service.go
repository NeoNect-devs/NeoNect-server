package service

import (
	"NeoNect/internal/infrastructure/persistence"
	"NeoNect/security"
	"context"
	"errors"
)

var (
	ErrInvalidPrekeyBatch  = errors.New("invalid prekey batch")
	ErrPrekeyLimitExceeded = errors.New("prekey limit exceeded")
)

const (
	MaxOneTimePrekeys = 200
)

type PrekeyService interface {
	UploadPrekeys(
		ctx context.Context,
		userID int64,
		deviceID string,
		signedCurve *persistence.SignedPrekeyRecord,
		oneTimeCurve []persistence.OneTimePrekeyRecord,
		signedPQ *persistence.SignedPrekeyRecord,
		oneTimePQ []persistence.OneTimePrekeyRecord,
	) error
	ClaimPrekeys(ctx context.Context, callerID int64, targetDeviceID string) (*persistence.PrekeyBundle, error)
	ClaimPrekeysIdempotent(
		ctx context.Context,
		callerID int64,
		targetDeviceID string,
		idempotencyKey string,
		fingerprint string,
		buildResponse func(*persistence.PrekeyBundle) ([]byte, int, error),
	) ([]byte, int, error)
}

type prekeyService struct {
	deviceRepo     persistence.DeviceRepository
	prekeyRepo     persistence.PrekeyRepository
	friendshipRepo persistence.FriendshipRepository
	vault          *security.SystemVault
}

func NewPrekeyService(dRepo persistence.DeviceRepository, pRepo persistence.PrekeyRepository, fRepo persistence.FriendshipRepository, vault *security.SystemVault) PrekeyService {
	return &prekeyService{
		deviceRepo:     dRepo,
		prekeyRepo:     pRepo,
		friendshipRepo: fRepo,
		vault:          vault,
	}
}

func (s *prekeyService) UploadPrekeys(
	ctx context.Context,
	userID int64,
	deviceID string,
	signedCurve *persistence.SignedPrekeyRecord,
	oneTimeCurve []persistence.OneTimePrekeyRecord,
	signedPQ *persistence.SignedPrekeyRecord,
	oneTimePQ []persistence.OneTimePrekeyRecord,
) error {
	// Validate ownership
	owned, err := s.deviceRepo.IsActiveDeviceOwnedByUser(ctx, userID, deviceID)
	if err != nil {
		return err
	}
	if !owned {
		return errors.New("unauthorized device access")
	}

	// Validate limits
	if len(oneTimeCurve) > MaxOneTimePrekeys || len(oneTimePQ) > MaxOneTimePrekeys {
		return ErrPrekeyLimitExceeded
	}

	// Structural validation
	if err := validateSignedPrekey(signedCurve); err != nil {
		return err
	}
	if err := validateSignedPrekey(signedPQ); err != nil {
		return err
	}
	if err := validateOneTimePrekeys(oneTimeCurve); err != nil {
		return err
	}
	if err := validateOneTimePrekeys(oneTimePQ); err != nil {
		return err
	}

	return s.prekeyRepo.UploadPrekeys(ctx, deviceID, signedCurve, oneTimeCurve, signedPQ, oneTimePQ)
}

func validateSignedPrekey(spk *persistence.SignedPrekeyRecord) error {
	if spk == nil {
		return nil
	}
	if len(spk.PublicKey) == 0 || len(spk.PublicKey) > 2048 {
		return ErrInvalidPrekeyBatch
	}
	if len(spk.Signature) == 0 || len(spk.Signature) > 2048 {
		return ErrInvalidPrekeyBatch
	}
	return nil
}

func validateOneTimePrekeys(opks []persistence.OneTimePrekeyRecord) error {
	for _, p := range opks {
		if len(p.PublicKey) == 0 || len(p.PublicKey) > 2048 {
			return ErrInvalidPrekeyBatch
		}
	}
	return nil
}

func (s *prekeyService) ClaimPrekeys(ctx context.Context, callerID int64, targetDeviceID string) (*persistence.PrekeyBundle, error) {
	ownerID, err := s.deviceRepo.GetDeviceOwner(ctx, targetDeviceID)
	if err != nil {
		return nil, err
	}

	if callerID != ownerID {
		isFriend, err := s.friendshipRepo.CheckFriendship(ctx, callerID, ownerID)
		if err != nil {
			return nil, err
		}
		if !isFriend {
			return nil, errors.New("unauthorized prekey claim")
		}
	}

	bundle, err := s.prekeyRepo.ClaimPrekeys(ctx, targetDeviceID)
	if err != nil {
		return nil, err
	}

	if bundle != nil && len(bundle.IdentityKey) > 0 {
		decrypted, err := s.vault.Reveal(bundle.IdentityKey)
		if err != nil {
			return nil, err
		}
		bundle.IdentityKey = []byte(decrypted)
	}

	return bundle, nil
}

func (s *prekeyService) ClaimPrekeysIdempotent(
	ctx context.Context,
	callerID int64,
	targetDeviceID string,
	idempotencyKey string,
	fingerprint string,
	buildResponse func(*persistence.PrekeyBundle) ([]byte, int, error),
) ([]byte, int, error) {
	ownerID, err := s.deviceRepo.GetDeviceOwner(ctx, targetDeviceID)
	if err != nil {
		return nil, 0, err
	}

	if callerID != ownerID {
		isFriend, err := s.friendshipRepo.CheckFriendship(ctx, callerID, ownerID)
		if err != nil {
			return nil, 0, err
		}
		if !isFriend {
			return nil, 0, errors.New("unauthorized prekey claim")
		}
	}

	return s.prekeyRepo.ClaimPrekeysIdempotent(
		ctx,
		callerID,
		targetDeviceID,
		idempotencyKey,
		fingerprint,
		func(bundle *persistence.PrekeyBundle) ([]byte, int, error) {
			if bundle != nil && len(bundle.IdentityKey) > 0 {
				decrypted, err := s.vault.Reveal(bundle.IdentityKey)
				if err != nil {
					return nil, 0, err
				}
				bundle.IdentityKey = []byte(decrypted)
			}
			return buildResponse(bundle)
		},
	)
}
