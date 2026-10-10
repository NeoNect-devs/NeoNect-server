package harness_test

import (
	"NeoNect/test/harness"
	"testing"
)

type fakeTB struct {
	testing.TB
	failed bool
}

func (f *fakeTB) Fatalf(format string, args ...interface{}) {
	f.failed = true
}

func (f *fakeTB) Helper() {}

func TestSeedFriendshipIsolation(t *testing.T) {
	h := harness.Setup(t)

	// Users: A, B, C, D
	h.RegisterUser(t, "user_A", "Password123!")
	cookieA := h.Login(t, "user_A", "Password123!")

	h.RegisterUser(t, "user_B", "Password123!")
	_ = h.Login(t, "user_B", "Password123!")

	h.RegisterUser(t, "user_C", "Password123!")
	cookieC := h.Login(t, "user_C", "Password123!")

	h.RegisterUser(t, "user_D", "Password123!")

	// 1. Sending a request alone does not create an accepted friendship.
	h.AddFriend(t, cookieA, "user_B")

	var count int
	h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 friendships, got %d", count)
	}

	h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friend_requests").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 pending request, got %d", count)
	}

	// 2. Seed an accepted friendship between C and D
	h.SeedFriendship(t, cookieC, "user_D")

	// 3. Verify ONLY intended user pair is seeded and unrelated pending requests are not deleted or promoted
	h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 friendship (C-D), got %d", count)
	}

	h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friend_requests").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 pending request (A->B) still exists, got %d", count)
	}

	// 4. h.AddFriend fails when the server returns an unexpected conflict.
	ft := &fakeTB{TB: t}
	// Trying to add the same friend again (conflict)
	h.AddFriend(ft, cookieA, "user_B")
	if !ft.failed {
		t.Errorf("Expected h.AddFriend to fail on conflict, but it did not")
	}

	// 5. SeedFriendship fails if a pending request exists for the same pair.
	// A -> B already has a pending request from earlier.
	err := h.SeedFriendshipErr(cookieA, "user_B")
	if err == nil {
		t.Errorf("Expected SeedFriendshipErr to fail because a pending request exists, but it returned nil")
	}

	// Verify no new friendship was inserted due to the conflict
	h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&count)
	if count != 1 { // Expect 1 from C-D friendship
		t.Errorf("Expected exactly 1 friendship (C-D), got %d", count)
	}

	// Verify the pending request A->B still exists
	h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friend_requests").Scan(&count)
	if count != 1 {
		t.Errorf("Expected exactly 1 pending request (A->B), got %d", count)
	}
}
