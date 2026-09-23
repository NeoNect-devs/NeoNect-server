package persistence

import (
	"NeoNect/internal/config"
	"NeoNect/internal/logger"
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
	wg                 sync.WaitGroup
	logger             logger.Logger
}

func NewSessionRepository(db *sql.DB, l logger.Logger) SessionRepository {
	repo := &sqlSessionRepository{
		db:       db,
		stopChan: make(chan struct{}),
		logger:   l,
	}
	repo.wg.Add(1)
	go repo.evictionLoop()
	return repo
}

func (m *sqlSessionRepository) evictionLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	defer m.wg.Done()

	var lastErr error
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := m.CleanupExpiredSessions(ctx)
			cancel()

			if err != nil {
				if lastErr == nil || lastErr.Error() != err.Error() {
					m.logger.Errorf("Session cleanup failed: %v", err)
					lastErr = err
				}
			} else if lastErr != nil {
				m.logger.Infof("Session cleanup recovered")
				lastErr = nil
			}
		case <-m.stopChan:
			return
		}
	}
}

func (m *sqlSessionRepository) CleanupExpiredSessions(ctx context.Context) error {
	now := time.Now().Unix()
	m.activeSessionCache.Range(func(key, value interface{}) bool {
		cs := value.(cachedSession)
		if now > cs.expiresAt {
			m.activeSessionCache.Delete(key)
		}
		return true
	})

	if m.db == nil {
		return nil
	}

	threshold := time.Now().Add(-config.SessionDuration).UTC().Format("2006-01-02 15:04:05")
	_, err := m.db.ExecContext(ctx, Queries.CleanupSess, config.BlockTypeSession, threshold)
	return err
}

func (m *sqlSessionRepository) Shutdown() {
	m.shutdownOnce.Do(func() {
		close(m.stopChan)
		m.wg.Wait()
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
	hashedToken := security.ComputeHash(sessionToken)
	if cached, exists := m.activeSessionCache.Load(hashedToken); exists {
		cs := cached.(cachedSession)
		if time.Now().Unix() > cs.expiresAt {
			m.activeSessionCache.Delete(hashedToken)
			return 0, errors.New("session expired")
		}
		return cs.uid, nil
	}

	var authenticatedUID int64
	var ts int64

	err := m.db.QueryRowContext(ctx, Queries.GetBySessWithTime, config.BlockTypeSession, hashedToken).Scan(&authenticatedUID, &ts)
	if err != nil {
		return 0, err
	}

	expiresAt := ts + int64(config.SessionDuration.Seconds())
	if time.Now().Unix() > expiresAt {
		_ = m.deleteSessionByHash(ctx, hashedToken)
		return 0, errors.New("session expired")
	}

	m.activeSessionCache.Store(hashedToken, cachedSession{uid: authenticatedUID, expiresAt: expiresAt})
	return authenticatedUID, nil
}

func (m *sqlSessionRepository) deleteSessionByHash(ctx context.Context, hashedToken string) error {
	m.activeSessionCache.Delete(hashedToken)

	_, err := m.db.ExecContext(ctx, Queries.DelSess, config.BlockTypeSession, hashedToken)
	return err
}

func (m *sqlSessionRepository) DeleteSession(ctx context.Context, sessionToken string) error {
	hashedToken := security.ComputeHash(sessionToken)
	return m.deleteSessionByHash(ctx, hashedToken)
}
