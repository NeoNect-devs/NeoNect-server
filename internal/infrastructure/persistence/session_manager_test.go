package persistence

import (
	"NeoNect/internal/logger"
	"NeoNect/security"
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	db.SetMaxOpenConns(1)

	_, err = db.Exec(DatabaseSchema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}
	return db
}

func TestSessionRepository_Shutdown(t *testing.T) {
	repo := NewSessionRepository(nil, logger.New(true))
	repo.Shutdown() // Should not block or panic
}

func TestSessionRepository_CacheSecurity(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSessionRepository(db, logger.New(true))
	defer repo.Shutdown()
	sqlRepo := repo.(*sqlSessionRepository)

	ctx := context.Background()

	// 1. Create a dummy user
	usernameHash := security.ComputeHash("testuser")
	_, err := db.ExecContext(ctx, Queries.CreateUser, usernameHash, []byte("blob"))
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	var userID int64
	err = db.QueryRowContext(ctx, Queries.GetUserId, usernameHash).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to get user id: %v", err)
	}

	// 2. Create session
	sessionToken := "raw_session_token_123"
	err = repo.CreateSession(ctx, usernameHash, sessionToken)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 3. Verify lookup & valid authentication works
	uid, err := repo.GetUserIdBySession(ctx, sessionToken)
	if err != nil {
		t.Fatalf("Expected valid authentication to succeed, got err: %v", err)
	}
	if uid != userID {
		t.Fatalf("Expected uid %d, got %d", userID, uid)
	}

	// 4. Prove cache uses hashed token and NOT raw token
	hashedToken := security.ComputeHash(sessionToken)
	_, hasHashed := sqlRepo.activeSessionCache.Load(hashedToken)
	if !hasHashed {
		t.Errorf("Expected cache to contain hashed token as key")
	}

	_, hasRaw := sqlRepo.activeSessionCache.Load(sessionToken)
	if hasRaw {
		t.Errorf("FATAL: Cache contains raw session token")
	}

	// Also check values to ensure raw token isn't in value
	sqlRepo.activeSessionCache.Range(func(key, value interface{}) bool {
		kStr := key.(string)
		if kStr == sessionToken {
			t.Errorf("FATAL: Cache key is raw token")
		}
		// value is cachedSession, which only has uid and expiresAt
		return true
	})

	// 5. Invalid token fails
	_, err = repo.GetUserIdBySession(ctx, "invalid_token")
	if err == nil {
		t.Errorf("Expected invalid token to fail")
	}

	// 6. Revoked/deleted session fails
	err = repo.DeleteSession(ctx, sessionToken)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	_, err = repo.GetUserIdBySession(ctx, sessionToken)
	if err == nil {
		t.Errorf("Expected revoked session to fail")
	}

	// Verify it's removed from cache
	_, stillHasHashed := sqlRepo.activeSessionCache.Load(hashedToken)
	if stillHasHashed {
		t.Errorf("Expected deleted session to be removed from cache")
	}
}

func TestSessionRepository_MultipleSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSessionRepository(db, logger.New(true))
	defer repo.Shutdown()

	ctx := context.Background()
	usernameHash := security.ComputeHash("testuser2")
	db.ExecContext(ctx, Queries.CreateUser, usernameHash, []byte("blob"))

	token1 := "token_1"
	token2 := "token_2"

	repo.CreateSession(ctx, usernameHash, token1)
	repo.CreateSession(ctx, usernameHash, token2)

	uid1, _ := repo.GetUserIdBySession(ctx, token1)
	uid2, _ := repo.GetUserIdBySession(ctx, token2)

	if uid1 != uid2 || uid1 == 0 {
		t.Errorf("Expected both sessions to resolve to same valid uid")
	}

	// Revoke one
	repo.DeleteSession(ctx, token1)

	_, err := repo.GetUserIdBySession(ctx, token1)
	if err == nil {
		t.Errorf("Expected token1 to be revoked")
	}

	// token2 should still work
	_, err = repo.GetUserIdBySession(ctx, token2)
	if err != nil {
		t.Errorf("Expected token2 to remain valid after token1 is revoked")
	}
}

func TestSessionRepository_ConcurrentLookups(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSessionRepository(db, logger.New(true))
	defer repo.Shutdown()
	ctx := context.Background()
	usernameHash := security.ComputeHash("testuser3")
	db.ExecContext(ctx, Queries.CreateUser, usernameHash, []byte("blob"))

	token := "concurrent_token"
	repo.CreateSession(ctx, usernameHash, token)

	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := repo.GetUserIdBySession(ctx, token)
			errCh <- err
		}()
	}

	for i := 0; i < 10; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent lookup failed: %v", err)
		}
	}
}

func TestSessionRepository_ExpiredSessionRemoved(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSessionRepository(db, logger.New(true))
	defer repo.Shutdown()
	sqlRepo := repo.(*sqlSessionRepository)

	ctx := context.Background()
	usernameHash := security.ComputeHash("testuser4")
	db.ExecContext(ctx, Queries.CreateUser, usernameHash, []byte("blob"))

	token := "expired_token"
	repo.CreateSession(ctx, usernameHash, token)
	repo.GetUserIdBySession(ctx, token) // prime cache

	hashedToken := security.ComputeHash(token)

	// forcibly expire in cache
	if cached, ok := sqlRepo.activeSessionCache.Load(hashedToken); ok {
		cs := cached.(cachedSession)
		cs.expiresAt = time.Now().Unix() - 100
		sqlRepo.activeSessionCache.Store(hashedToken, cs)
	}

	// Lookup should fail and delete from cache
	_, err := repo.GetUserIdBySession(ctx, token)
	if err == nil {
		t.Errorf("Expected expired session to fail")
	}

	// Should be removed from cache
	if _, ok := sqlRepo.activeSessionCache.Load(hashedToken); ok {
		t.Errorf("Expected expired session to be removed from cache")
	}
}

func TestSessionRepository_DatabaseCleanup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSessionRepository(db, logger.New(true))
	defer repo.Shutdown()
	sqlRepo := repo.(*sqlSessionRepository)

	ctx := context.Background()
	usernameHash := security.ComputeHash("testuser5")
	db.ExecContext(ctx, Queries.CreateUser, usernameHash, []byte("blob"))

	// Create a session
	token := "cleanup_token"
	repo.CreateSession(ctx, usernameHash, token)
	hashedToken := security.ComputeHash(token)

	// Manually set created_at in the past (beyond config.SessionDuration)
	oldTime := time.Now().Add(-25 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	_, err := db.ExecContext(ctx, "UPDATE user_blocks SET created_at = ? WHERE sub_block_id = ?", oldTime, hashedToken)
	if err != nil {
		t.Fatalf("Failed to update created_at: %v", err)
	}

	// Verify it still exists in the DB before cleanup
	var count int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_blocks WHERE sub_block_id = ?", hashedToken).Scan(&count)
	if count != 1 {
		t.Fatalf("Expected 1 session in DB, got %d", count)
	}

	// Run cleanup
	err = sqlRepo.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify it's removed from DB
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_blocks WHERE sub_block_id = ?", hashedToken).Scan(&count)
	if count != 0 {
		t.Fatalf("Expected session to be cleaned up from DB, but it remains")
	}

	// Create a valid session to ensure it's not deleted
	tokenValid := "valid_token"
	repo.CreateSession(ctx, usernameHash, tokenValid)
	hashedTokenValid := security.ComputeHash(tokenValid)

	err = sqlRepo.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_blocks WHERE sub_block_id = ?", hashedTokenValid).Scan(&count)
	if count != 1 {
		t.Fatalf("Expected valid session to remain in DB, but it was cleaned up")
	}
}

func TestSessionRepository_CleanupFailureHandling(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSessionRepository(db, logger.New(true))
	defer repo.Shutdown()
	sqlRepo := repo.(*sqlSessionRepository)

	// Pass a cancelled context to simulate a DB timeout or closed connection
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sqlRepo.CleanupExpiredSessions(ctx)
	if err == nil {
		t.Errorf("Expected cleanup to fail with a cancelled context, got nil")
	}
}

func TestSessionRepository_ShutdownDuringCleanup(t *testing.T) {
	db := setupTestDB(t)
	// We do not defer db.Close() here initially because we want to see if Shutdown() correctly waits.
	repo := NewSessionRepository(db, logger.New(true))

	// Since evictionLoop is running, we can just call Shutdown.
	// It should return without hanging indefinitely, and it shouldn't leave goroutines.
	done := make(chan struct{})
	go func() {
		repo.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatalf("Shutdown() hung indefinitely")
	}
	db.Close()
}

func TestSessionRepository_CleanupLogoutConcurrency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewSessionRepository(db, logger.New(true))
	defer repo.Shutdown()
	sqlRepo := repo.(*sqlSessionRepository)

	ctx := context.Background()
	usernameHash := security.ComputeHash("testuser_concurrent")
	db.ExecContext(ctx, Queries.CreateUser, usernameHash, []byte("blob"))

	token := "concurrent_logout_token"
	repo.CreateSession(ctx, usernameHash, token)
	hashedToken := security.ComputeHash(token)

	// Make the session old
	oldTime := time.Now().Add(-25 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	db.ExecContext(ctx, "UPDATE user_blocks SET created_at = ? WHERE sub_block_id = ?", oldTime, hashedToken)

	// Concurrently delete and cleanup
	errCh := make(chan error, 2)
	go func() {
		errCh <- repo.DeleteSession(ctx, token)
	}()
	go func() {
		errCh <- sqlRepo.CleanupExpiredSessions(ctx)
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}

	// Verify session is gone
	var count int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_blocks WHERE sub_block_id = ?", hashedToken).Scan(&count)
	if count != 0 {
		t.Errorf("Expected session to be completely removed")
	}
}
