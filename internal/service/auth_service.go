package service

import (
	"NeoNect/internal/config"
	"NeoNect/internal/infrastructure/persistence"
	"NeoNect/security"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters long")
	ErrPasswordComplexity = errors.New("password must contain both letters and numbers")
)

type AuthService interface {
	Register(ctx context.Context, username, password string) (int64, error)
	Login(ctx context.Context, username, password string) (string, error)
	Logout(ctx context.Context, token string) error
	IsUsernameAvailable(ctx context.Context, username string) (bool, error)
	GetUserIdByToken(ctx context.Context, token string) (int64, error)
}

type userAuthService struct {
	userRepository    persistence.UserRepository
	sessionRepository persistence.SessionRepository
}

func NewAuthService(uRepo persistence.UserRepository, sRepo persistence.SessionRepository) AuthService {
	return &userAuthService{
		userRepository:    uRepo,
		sessionRepository: sRepo,
	}
}

func (s *userAuthService) IsUsernameAvailable(ctx context.Context, username string) (bool, error) {
	if err := s.validateUsernameFormat(username); err != nil {
		return false, nil
	}

	usernameHash := security.ComputeHash(username)
	isTaken, err := s.userRepository.IsUsernameTaken(ctx, usernameHash)
	return !isTaken, err
}

func (s *userAuthService) Register(ctx context.Context, username, password string) (int64, error) {
	if err := s.validateUsernameFormat(username); err != nil {
		return 0, err
	}
	if err := s.validatePasswordComplexity(password); err != nil {
		return 0, err
	}

	usernameHash := security.ComputeHash(username)
	isTaken, err := s.userRepository.IsUsernameTaken(ctx, usernameHash)
	if err != nil {
		return 0, err
	}
	if isTaken {
		return 0, ErrUsernameTaken
	}

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return 0, err
	}

	return s.userRepository.CreateUser(ctx, username, usernameHash, passwordHash)
}

func (s *userAuthService) Login(ctx context.Context, username, password string) (string, error) {
	usernameHash := security.ComputeHash(username)
	storedHash, err := s.userRepository.GetAuthKey(ctx, usernameHash)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	if !security.CheckPasswordHash(password, storedHash) {
		return "", ErrInvalidCredentials
	}

	sessionToken, err := s.generateSecureSessionToken()
	if err != nil {
		return "", err
	}

	if err := s.sessionRepository.CreateSession(ctx, usernameHash, sessionToken); err != nil {
		return "", err
	}

	return sessionToken, nil
}

func (s *userAuthService) Logout(ctx context.Context, token string) error {
	return s.sessionRepository.DeleteSession(ctx, token)
}

func (s *userAuthService) GetUserIdByToken(ctx context.Context, token string) (int64, error) {
	return s.sessionRepository.GetUserIdBySession(ctx, token)
}

func (s *userAuthService) validateUsernameFormat(username string) error {
	trimmed := strings.TrimSpace(username)
	if len(trimmed) < config.MinUsernameLength {
		return errors.New("username too short")
	}
	return nil
}

func (s *userAuthService) validatePasswordComplexity(password string) error {
	if len(password) < config.MinPasswordLength {
		return ErrPasswordTooShort
	}

	var hasLetter, hasNumber bool
	for _, character := range password {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') {
			hasLetter = true
		}
		if character >= '0' && character <= '9' {
			hasNumber = true
		}
	}

	if !hasLetter || !hasNumber {
		return ErrPasswordComplexity
	}
	return nil
}

func (s *userAuthService) generateSecureSessionToken() (string, error) {
	randomBytes := make([]byte, config.TokenEntropy)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(randomBytes), nil
}
