package persistence_test

import (
	"NeoNect/internal/infrastructure/persistence"
	"testing"
)

func TestSessionRepository_Shutdown(t *testing.T) {
	repo := persistence.NewSessionRepository(nil)
	repo.Shutdown() // Should not block or panic
}
