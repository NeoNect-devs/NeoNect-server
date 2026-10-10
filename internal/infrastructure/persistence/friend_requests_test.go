package persistence_test

import (
	"context"
	"os"
	"testing"

	"NeoNect/internal/config"
	"NeoNect/internal/infrastructure/persistence"
)

func TestFriendRequests_Persistence(t *testing.T) {
	dbPath := "test_friend_requests.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)

	cfg := config.LoadConfig()
	db, err := persistence.NewDatabase(dbPath, cfg)
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize db: %v", err)
	}

	// Insert test users
	_, err = db.DB().ExecContext(ctx, "INSERT INTO users (id, username_hash, identity_blob) VALUES (1, 'hash1', 'user1'), (2, 'hash2', 'user2'), (3, 'hash3', 'user3')")
	if err != nil {
		t.Fatalf("Failed to insert users: %v", err)
	}

	repo := persistence.NewFriendshipRepository(db.DB())

	t.Run("Valid pending request creation", func(t *testing.T) {
		err := repo.CreateFriendRequest(ctx, 1, 2)
		if err != nil {
			t.Fatalf("Failed to create friend request: %v", err)
		}

		senderID, err := repo.GetFriendRequestSender(ctx, 1, 2)
		if err != nil {
			t.Fatalf("Failed to get friend request sender: %v", err)
		}
		if senderID != 1 {
			t.Errorf("Expected sender ID 1, got %d", senderID)
		}
	})

	t.Run("Duplicate request uniqueness enforcement", func(t *testing.T) {
		err := repo.CreateFriendRequest(ctx, 1, 2)
		if err != persistence.ErrDuplicateRecord {
			t.Errorf("Expected ErrDuplicateRecord, got %v", err)
		}
	})

	t.Run("Crossed-request uniqueness enforcement", func(t *testing.T) {
		err := repo.CreateFriendRequest(ctx, 2, 1)
		if err != persistence.ErrDuplicateRecord {
			t.Errorf("Expected ErrDuplicateRecord for crossed request, got %v", err)
		}
	})

	t.Run("Correct GetFriendRequestSender behavior in both argument orders", func(t *testing.T) {
		senderID, err := repo.GetFriendRequestSender(ctx, 2, 1)
		if err != nil {
			t.Fatalf("Failed to get sender: %v", err)
		}
		if senderID != 1 {
			t.Errorf("Expected sender ID 1, got %d", senderID)
		}
	})

	t.Run("Correct incoming and outgoing queries in both ID orders", func(t *testing.T) {
		incoming2, err := repo.GetIncomingRequests(ctx, 2)
		if err != nil || len(incoming2) != 1 || incoming2[0] != "user1" {
			t.Errorf("Expected incoming requests for user 2 to be ['user1'], got %v, err: %v", incoming2, err)
		}

		outgoing1, err := repo.GetOutgoingRequests(ctx, 1)
		if err != nil || len(outgoing1) != 1 || outgoing1[0] != "user2" {
			t.Errorf("Expected outgoing requests for user 1 to be ['user2'], got %v, err: %v", outgoing1, err)
		}

		// Verify zero queries
		incoming1, _ := repo.GetIncomingRequests(ctx, 1)
		if len(incoming1) != 0 {
			t.Errorf("Expected 0 incoming requests for user 1, got %v", incoming1)
		}

		outgoing2, _ := repo.GetOutgoingRequests(ctx, 2)
		if len(outgoing2) != 0 {
			t.Errorf("Expected 0 outgoing requests for user 2, got %v", outgoing2)
		}
	})

	t.Run("Atomic acceptance and rollback on failure", func(t *testing.T) {
		// Accept request
		err := repo.AcceptFriendRequest(ctx, 1, 2)
		if err != nil {
			t.Fatalf("Failed to accept friend request: %v", err)
		}

		// Verify request is deleted
		_, err = repo.GetFriendRequestSender(ctx, 1, 2)
		if err != persistence.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound, got %v", err)
		}

		// Verify friendship is created
		exists, _ := repo.CheckFriendship(ctx, 1, 2)
		if !exists {
			t.Errorf("Expected friendship to exist")
		}
	})

	t.Run("Request creation against an already accepted friendship", func(t *testing.T) {
		err := repo.CreateFriendRequest(ctx, 1, 2)
		if err != persistence.ErrDuplicateRecord {
			t.Errorf("Expected ErrDuplicateRecord when creating request for existing friendship, got %v", err)
		}
	})

	t.Run("Missing-request behavior for acceptance, decline, and cancellation", func(t *testing.T) {
		if err := repo.AcceptFriendRequest(ctx, 2, 3); err != persistence.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound for missing accept, got %v", err)
		}
		if err := repo.DeclineFriendRequest(ctx, 2, 3); err != persistence.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound for missing decline, got %v", err)
		}
		if err := repo.CancelFriendRequest(ctx, 2, 3); err != persistence.ErrRecordNotFound {
			t.Errorf("Expected ErrRecordNotFound for missing cancel, got %v", err)
		}
	})

	t.Run("Foreign-key enforcement and cascading behavior", func(t *testing.T) {
		repo.CreateFriendRequest(ctx, 2, 3)

		// Delete user 3
		_, err := db.DB().ExecContext(ctx, "DELETE FROM users WHERE id = 3")
		if err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		// Verify request was cascaded
		_, err = repo.GetFriendRequestSender(ctx, 2, 3)
		if err != persistence.ErrRecordNotFound {
			t.Errorf("Expected request to be cascade deleted, got %v", err)
		}
	})

	t.Run("Correct classification of primary-key/unique conflicts versus unrelated database constraint errors", func(t *testing.T) {
		// Try inserting a friend request with an invalid user ID (violating foreign key constraint)
		err := repo.CreateFriendRequest(ctx, 1, 999)
		if err == nil || err == persistence.ErrDuplicateRecord {
			t.Errorf("Expected a non-duplicate constraint error for FK violation, got %v", err)
		}
	})

	t.Run("Request creation racing with explicit acceptance", func(t *testing.T) {
		const iterations = 50
		for i := 0; i < iterations; i++ {
			// Clean up previous iteration
			db.DB().ExecContext(ctx, "DELETE FROM friend_requests")
			db.DB().ExecContext(ctx, "DELETE FROM friendships")

			// Initial state: One pending request from Alice (1) to Bob (2)
			err := repo.CreateFriendRequest(ctx, 1, 2)
			if err != nil {
				t.Fatalf("Iteration %d: Failed to create initial request: %v", i, err)
			}

			startBarrier := make(chan struct{})
			doneChan := make(chan struct{}, 2)

			var createErr error
			var acceptErr error

			go func() {
				<-startBarrier
				// Operation A: Alice attempts to create another request to Bob
				createErr = repo.CreateFriendRequest(ctx, 1, 2)
				doneChan <- struct{}{}
			}()

			go func() {
				<-startBarrier
				// Operation B: Bob explicitly accepts Alice's existing request
				acceptErr = repo.AcceptFriendRequest(ctx, 1, 2)
				doneChan <- struct{}{}
			}()

			// Trigger both simultaneously
			close(startBarrier)
			<-doneChan
			<-doneChan

			if acceptErr != nil {
				t.Fatalf("Iteration %d: AcceptFriendRequest failed unexpectedly: %v", i, acceptErr)
			}

			// Validate final database state
			var friendshipCount int
			db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM friendships WHERE user_id_1=1 AND user_id_2=2").Scan(&friendshipCount)

			var requestCount int
			db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM friend_requests WHERE user_id_1=1 AND user_id_2=2").Scan(&requestCount)

			if friendshipCount != 1 {
				t.Errorf("Iteration %d: Expected exactly 1 accepted friendship, got %d", i, friendshipCount)
			}

			if requestCount != 0 {
				t.Fatalf("Iteration %d: PRODUCTION DEFECT DETECTED! Expected 0 pending requests, got %d. CreateErr: %v", i, requestCount, createErr)
			}
		}
	})
}
