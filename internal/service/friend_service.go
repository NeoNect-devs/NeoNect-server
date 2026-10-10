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
	ErrSelfFriendship         = errors.New("cannot add yourself")
	ErrDuplicateFriendship    = errors.New("friendship already exists")
	ErrRequestAlreadySent     = errors.New("friend request already sent")
	ErrRequestAlreadyReceived = errors.New("friend request already received")
	ErrRequestNotFound        = errors.New("friend request not found")
	ErrUnauthorizedAction     = errors.New("unauthorized action on friend request")
)

type FriendRequestList struct {
	Incoming []string `json:"incoming"`
	Outgoing []string `json:"outgoing"`
}

type FriendService interface {
	AddFriend(ctx context.Context, authenticatedUserID int64, targetUsername string) error
	GetFriends(ctx context.Context, authenticatedUserID int64) ([]string, error)
	RemoveFriend(ctx context.Context, authenticatedUserID int64, targetUsername string) error
	AreFriends(ctx context.Context, uid1, uid2 int64) (bool, error)

	GetRequests(ctx context.Context, authenticatedUserID int64) (FriendRequestList, error)
	AcceptRequest(ctx context.Context, authenticatedUserID int64, targetUsername string) error
	DeclineRequest(ctx context.Context, authenticatedUserID int64, targetUsername string) error
	CancelRequest(ctx context.Context, authenticatedUserID int64, targetUsername string) error
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

func (s *friendService) getTargetUserID(ctx context.Context, username string) (int64, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return 0, ErrUserNotFound
	}
	hash := security.ComputeHash(username)
	uid, err := s.userRepo.GetUserIdByUsername(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, err
	}
	return uid, nil
}

func (s *friendService) AddFriend(ctx context.Context, authenticatedUserID int64, targetUsername string) error {
	targetUserID, err := s.getTargetUserID(ctx, targetUsername)
	if err != nil {
		return err
	}

	if authenticatedUserID == targetUserID {
		return ErrSelfFriendship
	}

	for attempt := 0; attempt < 3; attempt++ {
		err = s.friendshipRepo.CreateFriendRequest(ctx, authenticatedUserID, targetUserID)
		if err == nil {
			return nil
		}

		if err == persistence.ErrDuplicateRecord {
			senderID, errSender := s.friendshipRepo.GetFriendRequestSender(ctx, authenticatedUserID, targetUserID)
			if errSender == nil {
				if senderID == authenticatedUserID {
					return ErrRequestAlreadySent
				}
				return ErrRequestAlreadyReceived
			}
			if errSender != persistence.ErrRecordNotFound {
				return errSender
			}

			exists, errCheck := s.friendshipRepo.CheckFriendship(ctx, authenticatedUserID, targetUserID)
			if errCheck != nil {
				return errCheck
			}
			if exists {
				return ErrDuplicateFriendship
			}
			continue
		}
		return err
	}

	return errors.New("failed to add friend due to concurrent modifications")
}

func (s *friendService) GetFriends(ctx context.Context, authenticatedUserID int64) ([]string, error) {
	return s.friendshipRepo.GetFriendsList(ctx, authenticatedUserID)
}

func (s *friendService) RemoveFriend(ctx context.Context, authenticatedUserID int64, targetUsername string) error {
	targetUserID, err := s.getTargetUserID(ctx, targetUsername)
	if err != nil {
		return err
	}
	return s.friendshipRepo.RemoveFriendship(ctx, authenticatedUserID, targetUserID)
}

func (s *friendService) AreFriends(ctx context.Context, uid1, uid2 int64) (bool, error) {
	return s.friendshipRepo.CheckFriendship(ctx, uid1, uid2)
}

func (s *friendService) GetRequests(ctx context.Context, authenticatedUserID int64) (FriendRequestList, error) {
	incoming, err := s.friendshipRepo.GetIncomingRequests(ctx, authenticatedUserID)
	if err != nil {
		return FriendRequestList{}, err
	}
	outgoing, err := s.friendshipRepo.GetOutgoingRequests(ctx, authenticatedUserID)
	if err != nil {
		return FriendRequestList{}, err
	}
	if incoming == nil {
		incoming = []string{}
	}
	if outgoing == nil {
		outgoing = []string{}
	}
	return FriendRequestList{Incoming: incoming, Outgoing: outgoing}, nil
}

func (s *friendService) AcceptRequest(ctx context.Context, authenticatedUserID int64, targetUsername string) error {
	targetUserID, err := s.getTargetUserID(ctx, targetUsername)
	if err != nil {
		return err
	}

	senderID, err := s.friendshipRepo.GetFriendRequestSender(ctx, authenticatedUserID, targetUserID)
	if err != nil {
		if errors.Is(err, persistence.ErrRecordNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	if senderID != targetUserID {
		return ErrUnauthorizedAction
	}

	err = s.friendshipRepo.AcceptFriendRequest(ctx, targetUserID, authenticatedUserID)
	if err != nil {
		if errors.Is(err, persistence.ErrRecordNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	return nil
}

func (s *friendService) DeclineRequest(ctx context.Context, authenticatedUserID int64, targetUsername string) error {
	targetUserID, err := s.getTargetUserID(ctx, targetUsername)
	if err != nil {
		return err
	}

	senderID, err := s.friendshipRepo.GetFriendRequestSender(ctx, authenticatedUserID, targetUserID)
	if err != nil {
		if errors.Is(err, persistence.ErrRecordNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	if senderID != targetUserID {
		return ErrUnauthorizedAction
	}

	err = s.friendshipRepo.DeclineFriendRequest(ctx, targetUserID, authenticatedUserID)
	if err != nil {
		if errors.Is(err, persistence.ErrRecordNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	return nil
}

func (s *friendService) CancelRequest(ctx context.Context, authenticatedUserID int64, targetUsername string) error {
	targetUserID, err := s.getTargetUserID(ctx, targetUsername)
	if err != nil {
		return err
	}

	senderID, err := s.friendshipRepo.GetFriendRequestSender(ctx, authenticatedUserID, targetUserID)
	if err != nil {
		if errors.Is(err, persistence.ErrRecordNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	if senderID != authenticatedUserID {
		return ErrUnauthorizedAction
	}

	err = s.friendshipRepo.CancelFriendRequest(ctx, authenticatedUserID, targetUserID)
	if err != nil {
		if errors.Is(err, persistence.ErrRecordNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	return nil
}
