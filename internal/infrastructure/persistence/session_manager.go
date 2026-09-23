package persistence

import (
	"NeoNect/internal/config"
	"NeoNect/security"
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

type cachedSession struct {
	uid       int64
	expiresAt int64
}

type sqlSessionRepository struct {
	db                 *sql.DB
	activeSessionCache sync.Map
	stopChan           chan struct{}
	shutdownOnce       sync.Once
}

func NewSessionRepository(db *sql.DB) SessionRepository {
	repo := &sqlSessionRepository{
		db:       db,
		stopChan: make(chan struct{}),
	}
	go repo.evictionLoop()
	return repo
}

func (m *sqlSessionRepository) evictionLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now().Unix()
			m.activeSessionCache.Range(func(key, value interface{}) bool {
				cs := value.(cachedSession)
				if now > cs.expiresAt {
					m.activeSessionCache.Delete(key)
				}
				return true
			})
		case <-m.stopChan:
			return
		}
	}
}

func (m *sqlSessionRepository) Shutdown() {
	m.shutdownOnce.Do(func() {
		close(m.stopChan)
	})
}

func (m *sqlSessionRepository) CreateSession(ctx context.Context, usernameHash string, sessionToken string) error {
	var userID int64
	err := m.db.QueryRowContext(ctx, Queries.GetUserId, usernameHash).Scan(&userID)
	if err != nil {
		return err
	}

	_, err = m.db.ExecContext(ctx, Queries.CreateSess, userID, config.BlockTypeSession, security.ComputeHash(sessionToken), "")
	return err
}

func (m *sqlSessionRepository) GetUserIdBySession(ctx context.Context, sessionToken string) (int64, error) {
	if cached, exists := m.activeSessionCache.Load(sessionToken); exists {
		cs := cached.(cachedSession)
		if time.Now().Unix() > cs.expiresAt {
			m.activeSessionCache.Delete(sessionToken)
			return 0, errors.New("session expired")
		}
		return cs.uid, nil
	}

	var authenticatedUID int64
	var ts int64

	hashedToken := security.ComputeHash(sessionToken)
	err := m.db.QueryRowContext(ctx, Queries.GetBySessWithTime, config.BlockTypeSession, hashedToken).Scan(&authenticatedUID, &ts)
	if err != nil {
		return 0, err
	}

	expiresAt := ts + int64(config.SessionDuration.Seconds())
	if time.Now().Unix() > expiresAt {
		_ = m.DeleteSession(ctx, sessionToken)
		return 0, errors.New("session expired")
	}

	m.activeSessionCache.Store(sessionToken, cachedSession{uid: authenticatedUID, expiresAt: expiresAt})
	return authenticatedUID, nil
}

func (m *sqlSessionRepository) DeleteSession(ctx context.Context, sessionToken string) error {
	m.activeSessionCache.Delete(sessionToken)

	hashedToken := security.ComputeHash(sessionToken)
	_, err := m.db.ExecContext(ctx, Queries.DelSess, config.BlockTypeSession, hashedToken)
	return err
}
