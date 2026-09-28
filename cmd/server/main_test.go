package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServerMain_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "server")

	// 1. Build the binary
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, string(out))
	}

	// 2. Run the binary
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath)
	cmd.Env = append(os.Environ(),
		"NEONECT_ENV=development",
		"NEONECT_BOOTSTRAP_KEY=0123456789abcdef0123456789abcdef",
		"NEONECT_BIND_ADDR=127.0.0.1:0",
		fmt.Sprintf("NEONECT_DB_DIR=%s", filepath.Join(tmpDir, "db")),
		fmt.Sprintf("NEONECT_KEY_DIR=%s", filepath.Join(tmpDir, "keys")),
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start binary: %v", err)
	}

	var stdoutBuf strings.Builder
	var stdoutMu sync.Mutex

	// Ensure cleanup of process and pipes, and print logs on failure
	t.Cleanup(func() {
		cancel() // Signal the process to kill
		_ = cmd.Wait() // Wait for process to exit and close pipes

		if t.Failed() {
			stdoutMu.Lock()
			t.Logf("Binary Stdout:\n%s", stdoutBuf.String())
			stdoutMu.Unlock()
			t.Logf("Binary Stderr:\n%s", stderrBuf.String())
		}
	})

	portChan := make(chan int, 1)

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()

			stdoutMu.Lock()
			stdoutBuf.WriteString(line + "\n")
			stdoutMu.Unlock()

			if strings.Contains(line, "Assigned Port:") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					var p int
					if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &p); err == nil && p > 0 {
						select {
						case portChan <- p:
						default:
						}
					}
				}
			}
		}
	}()

	var port int
	select {
	case port = <-portChan:
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for server to report assigned port")
	}

	// Wait for initialization and hit the health endpoint
	ctxReq, cancelReq := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelReq()

	req, err := http.NewRequestWithContext(ctxReq, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", port), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
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
