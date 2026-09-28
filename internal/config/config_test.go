package config

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGetStrictEnvInt_Fatal(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		// Valid
		_ = GetStrictEnvInt("VALID", 10, 1, 100)
		// Invalid
		_ = GetStrictEnvInt("INVALID", 10, 1, 100)
		return
	}

	tests := []struct {
		name     string
		envValue string
	}{
		{"below min", "0"},
		{"above max", "101"},
		{"not int", "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestGetStrictEnvInt_Fatal")
			cmd.Env = append(os.Environ(), "CRASH_TEST=1", "VALID=50", "INVALID="+tc.envValue)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected process to crash with fatal error for invalid env %q, but it exited cleanly. Output: %s", tc.envValue, out)
			}
			if !strings.Contains(string(out), "Critical:") {
				t.Fatalf("expected fatal error output containing 'Critical:', got: %s", string(out))
			}
		})
	}
}

func TestGetStrictEnvInt64_Fatal(t *testing.T) {
	if os.Getenv("CRASH_TEST_64") == "1" {
		_ = GetStrictEnvInt64("INVALID_64", 10, 1, 100)
		return
	}

	tests := []struct {
		name     string
		envValue string
	}{
		{"below min", "0"},
		{"above max", "101"},
		{"not int", "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestGetStrictEnvInt64_Fatal")
			cmd.Env = append(os.Environ(), "CRASH_TEST_64=1", "INVALID_64="+tc.envValue)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected process to crash with fatal error for invalid env %q, but it exited cleanly. Output: %s", tc.envValue, out)
			}
			if !strings.Contains(string(out), "Critical:") {
				t.Fatalf("expected fatal error output containing 'Critical:', got: %s", string(out))
			}
		})
	}
}
