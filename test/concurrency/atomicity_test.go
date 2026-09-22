package concurrency_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"NeoNect/test/harness"
	_ "modernc.org/sqlite"
)

func TestFanOutAtomicity_Busy(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "sender_atom", "Password123!")
	cookieSender := h.Login(t, "sender_atom", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_s1")

	h.RegisterUser(t, "rec_atom", "Password123!")
	cookieRec := h.Login(t, "rec_atom", "Password123!")

	// Create 10 devices for recipient
	for i := 0; i < 10; i++ {
		h.RegisterDevice(t, cookieRec, fmt.Sprintf("dev_r%d", i))
	}

	// 1. Get database path
	dbDir := os.Getenv("NEONECT_DB_DIR")
	var dbPath string
	filepath.Walk(dbDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".sqlite") {
			dbPath = path
		}
		return nil
	})

	if dbPath == "" {
		t.Fatalf("Could not find db")
	}

	// 2. Open separate connection and lock it EXCLUSIVE
	testDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	defer testDB.Close()

	// Wait, we need to make sure the app's timeout is short so the test doesn't take 5s per run.
	// But I can't change production config here easily if it was already initialized by Setup.
	// But `NEONECT_BUSY_TIMEOUT` env is not parsed, it's hardcoded to 5000 in config.
	// So it WILL block for 5 seconds. That's acceptable for a thorough DB test.

	// We'll run fanouts concurrently
	const numSends = 5
	var wg sync.WaitGroup
	h.Client = &http.Client{Timeout: 10 * time.Second}
	startCh := make(chan struct{})

	for i := 0; i < numSends; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startCh

			h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
				"from_device_id":   "dev_s1",
				"protocol_version": 1, "to_username": "rec_atom",
				"ciphertext": "YnVy",
				"timestamp":  time.Now().Unix(),
			}, cookieSender)
		}(i)
	}

	// Lock the DB
	tx, err := testDB.Begin()
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	_, err = tx.Exec("INSERT INTO delivery_queue (device_id, payload, expiry, sequence) VALUES ('lock', 'test', 0, 0)")
	if err != nil {
		t.Fatalf("Lock insert failed: %v", err)
	}

	// Fire the requests
	close(startCh)

	// Hold lock for 6 seconds (exceeding 5 second busy timeout)
	time.Sleep(6 * time.Second)

	// Rollback our lock transaction
	tx.Rollback()
	wg.Wait()

	// Verify that NO partial queues exist.
	var count int
	err = h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM delivery_queue WHERE device_id != 'lock'").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// If fan-out was atomic, it either succeeded (if somehow timeout wasn't hit or hit rate limits) or failed entirely.
	// It should fail entirely and count should be 0 because 6s > 5s.
	if count != 0 && count%10 != 0 {
		t.Errorf("Atomicity violated! Found %d records, expected multiples of 10", count)
	}

	if count > 0 {
		t.Logf("Warning: %d messages succeeded, expected 0 due to busy timeout.", count)
	}
}
