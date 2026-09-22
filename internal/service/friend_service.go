package service

import (
	"NeoNect/internal/infrastructure/persistence"
	"NeoNect/security"
	"context"
	"database/sql"
	"errors"
	"strings"
)

var (
	ErrSelfFriendship      = errors.New("cannot add yourself as a friend")
	ErrDuplicateFriendship = errors.New("friendship already exists")
)

type FriendService interface {
	AddFriend(ctx context.Context, authenticatedUserID int64, targetUsername string) error
	GetFriends(ctx context.Context, authenticatedUserID int64) ([]string, error)
	RemoveFriend(ctx context.Context, authenticatedUserID int64, targetUsername string) error
	AreFriends(ctx context.Context, uid1, uid2 int64) (bool, error)
}

type friendService struct {
	userRepo       persistence.UserRepository
	friendshipRepo persistence.FriendshipRepository
}

func NewFriendService(userRepo persistence.UserRepository, friendshipRepo persistence.FriendshipRepository) FriendService {
	return &friendService{
		userRepo:       userRepo,
		friendshipRepo: friendshipRepo,
	}
}

func (s *friendService) AddFriend(ctx context.Context, authenticatedUserID int64, targetUsername string) error {
	targetUsername = strings.TrimSpace(targetUsername)
	if targetUsername == "" {
		return ErrUserNotFound
	}

	targetHash := security.ComputeHash(targetUsername)
	targetUserID, err := s.userRepo.GetUserIdByUsername(ctx, targetHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}

	if authenticatedUserID == targetUserID {
		return ErrSelfFriendship
	}

	err = s.friendshipRepo.CreateFriendship(ctx, authenticatedUserID, targetUserID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicateFriendship
		}
		return err
	}

	return nil
}

func (s *friendService) GetFriends(ctx context.Context, authenticatedUserID int64) ([]string, error) {
	return s.friendshipRepo.GetFriendsList(ctx, authenticatedUserID)
}

func (s *friendService) RemoveFriend(ctx context.Context, authenticatedUserID int64, targetUsername string) error {
	targetUsername = strings.TrimSpace(targetUsername)
	if targetUsername == "" {
		return ErrUserNotFound
	}
	targetHash := security.ComputeHash(targetUsername)
	targetUserID, err := s.userRepo.GetUserIdByUsername(ctx, targetHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	return s.friendshipRepo.RemoveFriendship(ctx, authenticatedUserID, targetUserID)
}

func (s *friendService) AreFriends(ctx context.Context, uid1, uid2 int64) (bool, error) {
	return s.friendshipRepo.CheckFriendship(ctx, uid1, uid2)
}
