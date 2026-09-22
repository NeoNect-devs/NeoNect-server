package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func getFreePort() int {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestServerMain_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "server")

	// 1. Build the binary
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, string(out))
	}

	port := getFreePort()
	if port == 0 {
		t.Fatalf("Could not get free port")
	}

	// 2. Run the binary
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Env = append(os.Environ(),
		"NEONECT_ENV=development",
		"NEONECT_BOOTSTRAP_KEY=0123456789abcdef0123456789abcdef",
		fmt.Sprintf("NEONECT_BIND_ADDR=127.0.0.1:%d", port),
		fmt.Sprintf("NEONECT_DB_DIR=%s", filepath.Join(tmpDir, "db")),
		fmt.Sprintf("NEONECT_KEY_DIR=%s", filepath.Join(tmpDir, "keys")),
	)

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start binary: %v", err)
	}

	// Wait for initialization and hit the health endpoint
	var resp *http.Response
	var err error
	success := false
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", port))
		if err == nil {
			success = true
			break
		}
	}

	if !success {
		cmd.Process.Kill()
		cmd.Wait()
		t.Logf("Binary Stdout:\n%s", stdoutBuf.String())
		t.Logf("Binary Stderr:\n%s", stderrBuf.String())
		t.Fatalf("Failed to reach health endpoint after 2 seconds: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "status") {
		t.Fatalf("Unexpected response: %s", string(body))
	}

	// 4. Trigger Graceful Shutdown via SIGINT
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("Failed to send interrupt: %v", err)
	}

	// 5. Wait for the process to exit cleanly
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited
	case <-time.After(5 * time.Second):
		t.Fatalf("Server binary did not shut down gracefully within 5 seconds")
	}
}
